package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// listenAddr  = ":9000" // Commented out as it's no longer directly used
	username    = "user"
	password    = "pass"
	logFilePath = "proxy_access.log"
)

func getListenAddr() string {
	port := os.Getenv("PROXY_PORT")
	if port == "" {
		port = "8080" // 默认端口
	}
	return ":" + port
}

func main() {
	addr := getListenAddr()
	serv, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	fmt.Println("Listening on " + addr)

	defer func(serv net.Listener) {
		_ = serv.Close()
	}(serv)

	for {
		conn, err := serv.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go processConn(conn)
	}
}

func processConn(conn net.Conn) {
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	reader := bufio.NewReader(conn)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("processConn: error reading request line from %s: %v", conn.RemoteAddr(), err)
		return
	}
	requestLine = strings.TrimSpace(requestLine)
	if requestLine == "" {
		log.Printf("processConn: received empty request line from %s", conn.RemoteAddr())
		return
	}

	parts := strings.Split(requestLine, " ")
	if len(parts) < 2 {
		log.Printf("processConn: malformed request line from %s: %q", conn.RemoteAddr(), requestLine)
		return
	}
	method := parts[0]
	target := parts[1]

	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// If it's EOF and the line is empty, it means the headers ended correctly.
			// Otherwise, it's an unexpected error.
			if err == io.EOF && strings.TrimSpace(line) == "" {
				break 
			}
			log.Printf("processConn: error reading headers from %s: %v", conn.RemoteAddr(), err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		headerParts := strings.SplitN(line, ":", 2)
		if len(headerParts) == 2 {
			headers[strings.ToLower(strings.TrimSpace(headerParts[0]))] = strings.TrimSpace(headerParts[1])
		}
	}

	if !authenticate(headers, conn) {
		log.Printf("processConn: authentication failed for %s", conn.RemoteAddr())
		return
	}

	logRequest(conn, method, target)

	if method == "CONNECT" {
		handleHTTPS(target, conn, reader)
	} else {
		handleHTTP(method, target, headers, reader, conn, requestLine)
	}
}

func authenticate(headers map[string]string, conn net.Conn) bool {
	authHeader, exists := headers["proxy-authorization"]
	if !exists {
		send407(conn)
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		send407(conn)
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		log.Printf("authenticate: error decoding base64 auth string from %s: %v", conn.RemoteAddr(), err)
		send407(conn)
		return false
	}

	expectedUser := os.Getenv("PROXY_USER")
	if expectedUser == "" {
		expectedUser = username // Fallback to const
	}
	expectedPassword := os.Getenv("PROXY_PASSWORD")
	if expectedPassword == "" {
		expectedPassword = password // Fallback to const
	}

	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 || credentials[0] != expectedUser || credentials[1] != expectedPassword {
		log.Printf("authenticate: invalid credentials or format from %s. Decoded: %q", conn.RemoteAddr(), string(decoded))
		send407(conn)
		return false
	}

	return true
}

func send407(conn net.Conn) {
	response := "HTTP/1.1 407 Proxy Authentication Required\r\n" +
		"Proxy-Authenticate: Basic realm=\"Proxy\"\r\n" +
		"\r\n"
	if _, err := conn.Write([]byte(response)); err != nil {
		log.Printf("send407: error writing 407 response to %s: %v", conn.RemoteAddr(), err)
	}
}

func handleHTTPS(target string, clientConn net.Conn, reader *bufio.Reader) {
	log.Printf("handleHTTPS: attempting to connect to target %s for client %s", target, clientConn.RemoteAddr())
	serverConn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		log.Printf("handleHTTPS: failed to connect to target server %s (timeout 10s): %v", target, err)
		return
	}
	defer serverConn.Close()
	log.Printf("handleHTTPS: successfully connected to target %s for client %s. Sending '200 Connection Established'.", target, clientConn.RemoteAddr())

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		log.Printf("handleHTTPS: error writing 200 Connection Established to %s: %v", clientConn.RemoteAddr(), err)
		return
	}
	log.Printf("handleHTTPS: '200 Connection Established' sent to client %s for target %s. Starting bidirectional copy.", clientConn.RemoteAddr(), target)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		// It's important to use the reader here, as it might contain buffered data
		// from the client that was read after the CONNECT request line and headers.
		if _, err := io.Copy(serverConn, reader); err != nil && err != io.EOF {
			log.Printf("handleHTTPS: error copying client to server for client %s, target %s: %v", clientConn.RemoteAddr(), target, err)
		}
		log.Printf("handleHTTPS: client-to-server copy finished for client %s, target %s", clientConn.RemoteAddr(), target)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(clientConn, serverConn); err != nil && err != io.EOF {
			log.Printf("handleHTTPS: error copying server to client for client %s, target %s: %v", clientConn.RemoteAddr(), target, err)
		}
		log.Printf("handleHTTPS: server-to-client copy finished for client %s, target %s", clientConn.RemoteAddr(), target)
	}()

	log.Printf("handleHTTPS: waiting for copy goroutines to complete for client %s and target %s", clientConn.RemoteAddr(), target)
	wg.Wait()
	log.Printf("handleHTTPS: copy goroutines completed for client %s and target %s", clientConn.RemoteAddr(), target)
}

func handleHTTP(method, rawURL string, headers map[string]string, reader *bufio.Reader, clientConn net.Conn, firstLine string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("handleHTTP: failed to parse URL %s: %v", rawURL, err)
		return
	}

	host := parsedURL.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}

	serverConn, err := net.Dial("tcp", host)
	if err != nil {
		log.Printf("handleHTTP: failed to connect to target server %s (from URL %s): %v", host, rawURL, err)
		return
	}
	defer serverConn.Close()

	serverWriter := bufio.NewWriter(serverConn)

	// 发送请求行
	requestLineStr := fmt.Sprintf("%s %s %s\r\n", method, parsedURL.RequestURI(), "HTTP/1.1")
	if _, err := serverWriter.WriteString(requestLineStr); err != nil {
		log.Printf("handleHTTP: error writing request line to server %s: %v", host, err)
		return
	}

	// 发送请求头
	for key, value := range headers {
		if key != "proxy-authorization" {
			headerLine := fmt.Sprintf("%s: %s\r\n", key, value)
			if _, err := serverWriter.WriteString(headerLine); err != nil {
				log.Printf("handleHTTP: error writing header '%s' to server %s: %v", key, host, err)
				return
			}
		}
	}
	if _, err := serverWriter.WriteString("\r\n"); err != nil {
		log.Printf("handleHTTP: error writing end-of-headers to server %s: %v", host, err)
		return
	}
	if err := serverWriter.Flush(); err != nil {
		log.Printf("handleHTTP: error flushing headers to server %s: %v", host, err)
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Copies request body from client to server
		// The 'reader' is used here as it may contain the body if it was partially read
		// along with headers.
		if _, err := io.Copy(serverConn, reader); err != nil {
			log.Printf("handleHTTP: error copying client request body to server: %v", err)
		}
		// We need to close the serverConn write side to signal EOF to the server
		// especially for POST/PUT requests where the server expects the body to end.
		if tcpConn, ok := serverConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		} else {
			// For other types of net.Conn, Close may be the only option.
			// This might also close the read side, which could be an issue if the server
			// sends a response very quickly before this CloseWrite can take effect.
			// However, for HTTP, usually the server waits for the full request body.
			// serverConn.Close() // Avoid this if possible, prefer CloseWrite
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Copies response from server to client
		if _, err := io.Copy(clientConn, serverConn); err != nil {
			log.Printf("handleHTTP: error copying server response to client: %v", err)
		}
	}()

	wg.Wait()
}

func logRequest(conn net.Conn, method, target string) {
	clientAddr := conn.RemoteAddr().String()
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s 请求 %s %s", timeStr, clientAddr, method, target)

	fmt.Println(logEntry)
	logToFile(logEntry)
}

func logToFile(message string) {
	path := os.Getenv("PROXY_LOG_PATH")
	if path == "" {
		path = logFilePath // default path "proxy_access.log"
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("logToFile: failed to open log file %s: %v", path, err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(message + "\n"); err != nil {
		log.Printf("logToFile: error writing message to log file %s: %v", path, err)
	}
}
