# Burrow

A reverse proxy tunnel written in Go with zero external dependencies. Expose a local server to the internet through a TCP tunnel — like ngrok, built from scratch.

## Architecture

```
                         Public Internet                          Private Network
                    ┌─────────────────────────┐            ┌─────────────────────────┐
                    │                         │            │                         │
HTTP client ──────► │  Server (:8080)         │            │  Client                 │
  (curl, browser)   │    │                    │            │    │                    │
                    │    │  HTTP listener      │            │    │  Forwards to       │
                    │    ▼                    │            │    ▼  local service     │
                    │  Tunnel Socket (:8081) ◄──TCP conn──► Tunnel Socket           │
                    │                         │            │    │                    │
                    └─────────────────────────┘            │    ▼                    │
                                                           │  Backend (:3000)        │
                                                           └─────────────────────────┘
```

1. The **server** listens for HTTP requests on `:8080` and accepts a tunnel client on `:8081`.
2. The **client** connects to `:8081`, establishing a persistent TCP tunnel.
3. When an HTTP request hits `:8080`, the server writes it into the tunnel.
4. The client reads the request from the tunnel, forwards it to the local backend on `:3000`, and writes the response back through the tunnel.
5. The server reads the response from the tunnel and sends it back to the original HTTP caller.

## Protocol

Burrow uses a custom text+binary protocol over a single TCP connection. No HTTP/2, no WebSockets, no framing libraries — just raw reads and writes.

### Request (Server → Client)

```
<request-id> <METHOD> <path>\n
Header-Name: value\n
Header-Name: value\n
\n
[4 bytes: body length, big-endian uint32]
[body bytes]
```

### Response (Client → Server)

```
<request-id>\n
<status-code>\n
Header-Name: value\n
Header-Name: value\n
\n
[4 bytes: body length, big-endian uint32]
[body bytes]
```

### Multiplexing

Each request gets a unique ID (`req-<unix-nano>`). The server stores a channel in a `map[string]chan Response` keyed by request ID. When a response arrives, it's routed to the correct waiting goroutine via that channel. This allows multiple HTTP requests to be in-flight over a single tunnel connection simultaneously.

## Concurrency Model

- **Server**: Each incoming HTTP request is handled in its own goroutine (standard `net/http` behavior). Writes to the tunnel connection are serialized with a `sync.Mutex`. A dedicated `readResponses` goroutine continuously reads responses from the tunnel and dispatches them to waiting handlers via channels.
- **Client**: Reads requests sequentially from the tunnel, then spawns a goroutine per request to forward it to the backend. Response writes back to the tunnel are also mutex-protected.

## Building

```bash
make build
```

This compiles all three binaries to `bin/`.

## Testing

```bash
make test
```

Runs all unit tests, including the `internal/protocol` package tests.

## Running It

Start all three components in separate terminals:

```bash
# 1. Start the test backend server (listens on :3000)
make run-testserver

# 2. Start the tunnel server (HTTP on :8080, tunnel on :8081)
make run-server

# 3. Start the tunnel client (connects to :8081, forwards to :3000)
make run-client
```

Or run directly with `go run`:

```bash
go run ./cmd/testserver
go run ./cmd/server
go run ./cmd/client
```

Then make requests against the server:

```bash
# GET request
curl http://localhost:8080/

# POST request with a body
curl -X POST http://localhost:8080/ -d "hello"
# → {"received": "hello"}
```

The test server returns a `201` with `Content-Type: application/json` and an `X-Custom-Header: burrow-test` header.

## Development

```bash
make fmt    # Format all Go files
make vet    # Run static analysis
make clean  # Remove compiled binaries
```

## Project Structure

```
burrow/
├── cmd/
│   ├── server/          # Tunnel server — accepts HTTP (:8080) and tunnel client (:8081)
│   │   └── main.go
│   ├── client/          # Tunnel client — connects to server, forwards to local backend
│   │   └── main.go
│   └── testserver/      # Simple HTTP backend for testing (:3000)
│       └── main.go
├── internal/
│   └── protocol/        # Shared wire protocol (read/write frames)
│       ├── protocol.go
│       └── protocol_test.go
├── Makefile
├── go.mod
├── Readme.md
└── FUTURE.md
```
