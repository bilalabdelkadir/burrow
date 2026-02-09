.PHONY: build test clean fmt vet run-server run-client run-testserver

build:
	go build -o bin/server ./cmd/server
	go build -o bin/client ./cmd/client
	go build -o bin/testserver ./cmd/testserver

test:
	go test ./...

clean:
	rm -rf bin/

fmt:
	go fmt ./...

vet:
	go vet ./...

run-server:
	go run ./cmd/server

run-client:
	go run ./cmd/client

run-testserver:
	go run ./cmd/testserver
