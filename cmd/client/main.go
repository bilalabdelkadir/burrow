package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/bilalabdelkadir/burrow/internal/protocol"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8081")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Println("Connected to server, tunnel open")

	var mu sync.Mutex
	reader := bufio.NewReader(conn)

	for {
		id, req, err := readRequest(reader)
		if err != nil {
			log.Println("Connection closed:", err)
			return
		}
		log.Printf("Forwarding %s %s to localhost", req.Method, req.URL.Path)
		go forwardRequest(id, req, conn, &mu)
	}
}

func readRequest(r *bufio.Reader) (string, *http.Request, error) {
	id, method, path, err := protocol.ReadRequestLine(r)
	if err != nil {
		return "", nil, err
	}

	headers, err := protocol.ReadHeaders(r)
	if err != nil {
		return "", nil, err
	}

	body, err := protocol.ReadBody(r)
	if err != nil {
		return "", nil, err
	}

	req, err := http.NewRequest(method, "http://localhost:3000"+path, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return "", nil, err
	}
	req.Header = headers

	return id, req, nil
}

func forwardRequest(id string, req *http.Request, conn net.Conn, mu *sync.Mutex) {
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("error making request:", err)
		mu.Lock()
		protocol.WriteResponse(conn, id, protocol.Response{
			StatusCode: 504,
			Headers:    make(http.Header),
			Body:       nil,
		})
		mu.Unlock()
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading response:", err)
		return
	}
	resp.Body.Close()

	mu.Lock()
	protocol.WriteResponse(conn, id, protocol.Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       bodyBytes,
	})
	mu.Unlock()
}
