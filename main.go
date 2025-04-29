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
	"time"
)

const (
	listenAddr  = ":9000"
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
	serv, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}
	fmt.Println("Listening on " + listenAddr)

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
		log.Println("读取请求行失败:", err)
		return
	}
	requestLine = strings.TrimSpace(requestLine)
	if requestLine == "" {
		return
	}

	parts := strings.Split(requestLine, " ")
	if len(parts) < 2 {
		return
	}
	method := parts[0]
	target := parts[1]

	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
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
		send407(conn)
		return false
	}

	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 || credentials[0] != username || credentials[1] != password {
		send407(conn)
		return false
	}

	return true
}

func send407(conn net.Conn) {
	response := "HTTP/1.1 407 Proxy Authentication Required\r\n" +
		"Proxy-Authenticate: Basic realm=\"Proxy\"\r\n" +
		"\r\n"
	conn.Write([]byte(response))
}

func handleHTTPS(target string, clientConn net.Conn, reader *bufio.Reader) {
	serverConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Println("连接服务器失败:", err)
		return
	}
	defer serverConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(serverConn, reader)
	io.Copy(clientConn, serverConn)
}

func handleHTTP(method, rawURL string, headers map[string]string, reader *bufio.Reader, clientConn net.Conn, firstLine string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Println("解析URL失败:", err)
		return
	}

	host := parsedURL.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}

	serverConn, err := net.Dial("tcp", host)
	if err != nil {
		log.Println("连接服务器失败:", err)
		return
	}
	defer serverConn.Close()

	serverWriter := bufio.NewWriter(serverConn)

	// 发送请求行
	requestLine := fmt.Sprintf("%s %s %s\r\n", method, parsedURL.RequestURI(), "HTTP/1.1")
	serverWriter.WriteString(requestLine)

	// 发送请求头
	for key, value := range headers {
		if key != "proxy-authorization" {
			serverWriter.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}
	serverWriter.WriteString("\r\n")
	serverWriter.Flush()

	go io.Copy(serverConn, reader)
	io.Copy(clientConn, serverConn)
}

func logRequest(conn net.Conn, method, target string) {
	clientAddr := conn.RemoteAddr().String()
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s 请求 %s %s", timeStr, clientAddr, method, target)

	fmt.Println(logEntry)
	logToFile(logEntry)
}

func logToFile(message string) {
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("写入日志失败:", err)
		return
	}
	defer f.Close()

	f.WriteString(message + "\n")
}
