FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY main.go .

RUN go build -ldflags="-s -w" -o proxy main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/proxy .

RUN touch /app/proxy_access.log

EXPOSE 8080

CMD ["./proxy"]