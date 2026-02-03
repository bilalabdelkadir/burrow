package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

func main() {
	var mu sync.Mutex
	conn, err := net.Dial("tcp", "localhost:8081")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Println("Connected to server, tunnel open")

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Connection closed:", err)
			return
		}
		parts := strings.Fields(line)
		requestId := parts[0]
		method := parts[1]
		path := parts[2]
		log.Printf("Forwarding %s %s to localhost", method, path)

		req, err := http.NewRequest(method, "http://localhost:3000"+path, nil)
		if err != nil {
			log.Fatal(err)
		}

		for {
			headerLine, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Error reading header:", err)
				break
			}

			headerLine = strings.TrimSpace(headerLine)
			if headerLine == "" {
				break
			}

			parts := strings.SplitN(headerLine, ":", 2)
			if len(parts) != 2 {
				log.Println("Invalid header line:", headerLine)
				continue
			}

			name := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			req.Header.Add(name, value)
		}

		reqLen := make([]byte, 4)

		_, err = io.ReadFull(reader, reqLen)
		if err != nil {
			log.Println("Connection closed while reading 4 bytes:", err)
			return
		}
		bodyLength := binary.BigEndian.Uint32(reqLen)

		body := make([]byte, bodyLength)
		_, err = io.ReadFull(reader, body)
		if err != nil {
			log.Println("Connection closed while reading body:", err)
			return
		}
		log.Printf("Client received body: %d bytes, content: %s", len(body), string(body))

		req.Body = io.NopCloser(bytes.NewReader(body))
		go func(requestId string, req *http.Request, conn net.Conn, mu *sync.Mutex) {
			client := http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Println("error making request:", err)
				// Send error response back to server
				mu.Lock()
				conn.Write([]byte(requestId + "\n"))
				conn.Write([]byte("504\n")) // Gateway Timeout
				conn.Write([]byte("\n"))    // no headers
				lenBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(lenBuf, 0)
				conn.Write(lenBuf) // zero-length body
				mu.Unlock()
				return
			}
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("error reading response:", err)
				return
			}
			resp.Body.Close()
			bodyBytesLength := uint32(len(bodyBytes))

			lenBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, bodyBytesLength)

			statusCode := resp.StatusCode

			mu.Lock()
			conn.Write([]byte(requestId + "\n"))
			conn.Write([]byte(strconv.Itoa(statusCode) + "\n"))
			for name, values := range resp.Header {
				for _, v := range values {
					conn.Write([]byte(name + ": " + v + "\n"))
				}
			}
			conn.Write([]byte("\n"))
			conn.Write(lenBuf)
			conn.Write(bodyBytes)
			mu.Unlock()

		}(requestId, req, conn, &mu)

	}
}
