package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var tunnelConn net.Conn
var waiters = make(map[string]chan []byte)

func main() {

	go acceptTunnelClient()

	acceptHTTPRequests()
}

func readResponses() {
	// 1️⃣ create bufio.Reader from tunnelConn
	reader := bufio.NewReader(tunnelConn)

	// 2️⃣ loop forever to keep reading responses
	for {
		// 3️⃣ read request ID (until \n)
		requestId, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading requestId:", err)
			return
		}
		requestId = strings.TrimSpace(requestId)

		// 4️⃣ read 4 bytes length
		lenBuf := make([]byte, 4)
		_, err = io.ReadFull(reader, lenBuf)
		if err != nil {
			log.Println("Error reading body length:", err)
			return
		}
		bodyLength := binary.BigEndian.Uint32(lenBuf)

		// 5️⃣ read body
		body := make([]byte, bodyLength)
		_, err = io.ReadFull(reader, body)
		if err != nil {
			log.Println("Error reading body:", err)
			return
		}

		// 6️⃣ find channel in waiters map
		ch, ok := waiters[requestId]
		if !ok {
			log.Println("No channel found for requestId:", requestId)
			continue
		}

		// 7️⃣ send body to channel (wakes the handler)
		ch <- body
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
		formattedMsg := requestId + " " + r.Method + " " + r.URL.Path + "\n"
		ch := make(chan []byte)
		waiters[requestId] = ch
		tunnelConn.Write([]byte(formattedMsg))

		body := <-ch
		w.Write(body)
		delete(waiters, requestId)

	})

	log.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
