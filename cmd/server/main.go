package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bilalabdelkadir/burrow/internal/protocol"
)

type Server struct {
	tunnelConn net.Conn
	waiters    map[string]chan protocol.Response
	mu         sync.Mutex
}

func main() {
	s := &Server{
		waiters: make(map[string]chan protocol.Response),
	}
	go s.acceptTunnelClient()
	s.acceptHTTPRequests()
}

func (s *Server) acceptTunnelClient() {
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

	s.mu.Lock()
	s.tunnelConn = conn
	s.mu.Unlock()

	log.Println("Client connected:", conn.RemoteAddr())
	s.readResponses()
}

func (s *Server) readResponses() {
	reader := bufio.NewReader(s.tunnelConn)

	for {
		id, resp, err := protocol.ReadResponse(reader)
		if err != nil {
			log.Println("Error reading response:", err)
			return
		}

		s.mu.Lock()
		ch, ok := s.waiters[id]
		s.mu.Unlock()
		if !ok {
			log.Println("No channel found for requestId:", id)
			continue
		}

		ch <- resp
	}
}

func (s *Server) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	conn := s.tunnelConn
	s.mu.Unlock()

	if conn == nil {
		w.Write([]byte("tunnel not ready"))
		return
	}

	requestId := fmt.Sprintf("req-%d", time.Now().UnixNano())
	ch := make(chan protocol.Response)

	s.mu.Lock()
	s.waiters[requestId] = ch
	s.mu.Unlock()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading body:", err)
		return
	}

	s.mu.Lock()
	err = protocol.WriteRequestLine(conn, requestId, r.Method, r.URL.RequestURI())
	if err == nil {
		err = protocol.WriteHeaders(conn, r.Header)
	}
	if err == nil {
		err = protocol.WriteBody(conn, bodyBytes)
	}
	s.mu.Unlock()

	if err != nil {
		log.Println("Error writing to tunnel:", err)
		return
	}

	response := <-ch

	for name, values := range response.Headers {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}
	w.WriteHeader(response.StatusCode)
	w.Write(response.Body)

	s.mu.Lock()
	delete(s.waiters, requestId)
	s.mu.Unlock()
}

func (s *Server) acceptHTTPRequests() {
	http.HandleFunc("/", s.handleHTTPRequest)
	log.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
