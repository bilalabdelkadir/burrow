package main

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
)

var tunnelConn net.Conn

func main() {

	go acceptTunnelClient()
	acceptHTTPRequests()
}

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
		formattedMsg := r.Method + " " + r.URL.Path + "\n"
		tunnelConn.Write([]byte(formattedMsg))

		lenBuf := make([]byte, 4)

		_, err := io.ReadFull(tunnelConn, lenBuf)
		if err != nil {
			log.Fatal(err)
		}

		bodyLength := binary.BigEndian.Uint32(lenBuf)

		lenBod := make([]byte, bodyLength)

		_, err = io.ReadFull(tunnelConn, lenBod)
		if err != nil {
			log.Fatal(err)
		}

		w.Write(lenBod)

	})

	log.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
