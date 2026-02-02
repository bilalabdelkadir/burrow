package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var tunnelConn net.Conn
var waiters = make(map[string]chan Response)
var mu sync.Mutex

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func main() {

	go acceptTunnelClient()

	acceptHTTPRequests()
}

func readResponses() {

	reader := bufio.NewReader(tunnelConn)

	for {

		requestId, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading requestId:", err)
			return
		}
		requestId = strings.TrimSpace(requestId)

		statusCodeStr, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading statusCode:", err)
			return
		}
		statusCodeStr = strings.TrimSpace(statusCodeStr)
		statusCode, err := strconv.Atoi(statusCodeStr)
		if err != nil {
			log.Println("Failed to convert status code:", err)
			return
		}
		headers := make(http.Header)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				headers.Add(name, value)
			}
		}

		lenBuf := make([]byte, 4)
		_, err = io.ReadFull(reader, lenBuf)
		if err != nil {
			log.Println("Error reading body length:", err)
			return
		}
		bodyLength := binary.BigEndian.Uint32(lenBuf)

		body := make([]byte, bodyLength)
		_, err = io.ReadFull(reader, body)
		if err != nil {
			log.Println("Error reading body:", err)
			return
		}

		mu.Lock()
		ch, ok := waiters[requestId]
		mu.Unlock()
		if !ok {
			log.Println("No channel found for requestId:", requestId)
			continue
		}

		response := Response{
			StatusCode: statusCode,
			Body:       body,
			Headers:    headers,
		}

		ch <- response
	}
}

// this listens on tcp connection developer to connect to this
func acceptTunnelClient() {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal("Error listening:", err)
	}
	defer listener.Close()

	log.Println("Server waiting for tunnel client on :8081")

	conn, err := listener.Accept()
	if err != nil {
		log.Println("Error accepting conn:", err)
		return
	}

	tunnelConn = conn
	go readResponses()

	log.Println("tunnelConn assigned:", tunnelConn != nil)
	clientConn := conn.RemoteAddr()
	log.Println("Client connected:", clientConn)
}

func acceptHTTPRequests() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("HTTP request received, tunnelConn is nil:", tunnelConn == nil)
		if tunnelConn == nil {
			w.Write([]byte("tunnel not ready"))
			return
		}

		requestId := fmt.Sprintf("req-%d", time.Now().UnixNano())
		ch := make(chan Response)
		mu.Lock()
		waiters[requestId] = ch
		mu.Unlock()
		formattedMsg := requestId + " " + r.Method + " " + r.URL.RequestURI() + "\n"
		mu.Lock()
		tunnelConn.Write([]byte(formattedMsg))

		for name, values := range r.Header {
			for _, v := range values {
				tunnelConn.Write([]byte(name + ": " + v + "\n"))
			}
		}

		tunnelConn.Write([]byte("\n"))

		lenBuf := make([]byte, 4)
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Error reading body:", err)
			return
		}
		log.Printf("Server sending body: %d bytes, content: %s", len(bodyBytes), string(bodyBytes))

		binary.BigEndian.PutUint32(lenBuf, uint32(len(bodyBytes)))

		tunnelConn.Write(lenBuf)
		tunnelConn.Write(bodyBytes)
		mu.Unlock()

		response := <-ch
		log.Printf("Response received for %s: status=%d, body=%d bytes", requestId, response.StatusCode, len(response.Body))
		for name, values := range response.Headers {
			for _, v := range values {
				w.Header().Add(name, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		w.Write(response.Body)
		mu.Lock()
		delete(waiters, requestId)
		mu.Unlock()

	})

	log.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
