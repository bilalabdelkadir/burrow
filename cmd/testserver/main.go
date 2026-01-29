package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("incoming request:", r.Method, r.URL.Path)
		w.Write([]byte("Hello from localhost:3000"))
	})

	log.Println("Test server listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
