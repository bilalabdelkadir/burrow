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
	"sync"
	"time"
)

var tunnelConn net.Conn
var waiters = make(map[string]chan []byte)
var mu sync.Mutex

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
		ch := make(chan []byte)
		mu.Lock()
		waiters[requestId] = ch
		mu.Unlock()
		formattedMsg := requestId + " " + r.Method + " " + r.URL.Path + "\n"
		tunnelConn.Write([]byte(formattedMsg))

		for name, values := range r.Header {
			for _, v := range values {
				tunnelConn.Write([]byte(name + ": " + v + "\n"))
			}
		}
		tunnelConn.Write([]byte("\n"))

		body := <-ch
		w.Write(body)
		mu.Lock()
		delete(waiters, requestId)
		mu.Unlock()

	})

	log.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
