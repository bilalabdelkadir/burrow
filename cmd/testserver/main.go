package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "burrow-test")
		w.WriteHeader(201) // Created, not 200
		w.Write([]byte(`{"received": "` + string(body) + `"}`))
	})
	log.Println("Test server on :3000")
	http.ListenAndServe(":3000", nil)
}
