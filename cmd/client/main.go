package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

func main() {
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

		req, err := http.NewRequest(method, "http://localhost:3000"+path, nil)
		if err != nil {
			log.Fatal(err)
		}

		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("error making request:", err)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("error reading response:", err)
			continue
		}
		resp.Body.Close()
		bodyBytesLength := uint32(len(bodyBytes))

		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, bodyBytesLength)

		conn.Write([]byte(requestId + "\n"))
		conn.Write(lenBuf)
		conn.Write(bodyBytes)

		log.Println(line)
	}
}
