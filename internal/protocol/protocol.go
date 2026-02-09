package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Response represents an HTTP response sent back through the tunnel.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// ReadLine reads a single newline-terminated line and trims whitespace.
func ReadLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadHeaders reads HTTP headers until an empty line is encountered.
func ReadHeaders(r *bufio.Reader) (http.Header, error) {
	headers := make(http.Header)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return headers, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers.Add(name, value)
		}
	}
	return headers, nil
}

// WriteHeaders writes HTTP headers followed by an empty line terminator.
func WriteHeaders(w io.Writer, h http.Header) error {
	for name, values := range h {
		for _, v := range values {
			if _, err := fmt.Fprintf(w, "%s: %s\n", name, v); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}

// ReadBody reads a length-prefixed body (4-byte big-endian uint32 length + body bytes).
func ReadBody(r *bufio.Reader) ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("reading body length: %w", err)
	}
	bodyLength := binary.BigEndian.Uint32(lenBuf)
	body := make([]byte, bodyLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	return body, nil
}

// WriteBody writes a length-prefixed body (4-byte big-endian uint32 length + body bytes).
func WriteBody(w io.Writer, body []byte) error {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(body)))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// ReadRequestLine reads a request line in the format "<id> <METHOD> <path>\n"
// and returns the parsed components.
func ReadRequestLine(r *bufio.Reader) (id, method, path string, err error) {
	line, err := ReadLine(r)
	if err != nil {
		return "", "", "", err
	}
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("malformed request line: %q", line)
	}
	return parts[0], parts[1], parts[2], nil
}

// WriteRequestLine writes a request line in the format "<id> <METHOD> <path>\n".
func WriteRequestLine(w io.Writer, id, method, path string) error {
	_, err := fmt.Fprintf(w, "%s %s %s\n", id, method, path)
	return err
}

// WriteResponse writes a full response frame: request ID, status code, headers, and body.
func WriteResponse(w io.Writer, id string, resp Response) error {
	if _, err := fmt.Fprintf(w, "%s\n", id); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%d\n", resp.StatusCode); err != nil {
		return err
	}
	if err := WriteHeaders(w, resp.Headers); err != nil {
		return err
	}
	return WriteBody(w, resp.Body)
}

// ReadResponse reads a full response frame: request ID, status code, headers, and body.
func ReadResponse(r *bufio.Reader) (id string, resp Response, err error) {
	id, err = ReadLine(r)
	if err != nil {
		return "", Response{}, fmt.Errorf("reading request id: %w", err)
	}

	statusStr, err := ReadLine(r)
	if err != nil {
		return "", Response{}, fmt.Errorf("reading status code: %w", err)
	}
	statusCode, err := strconv.Atoi(statusStr)
	if err != nil {
		return "", Response{}, fmt.Errorf("parsing status code %q: %w", statusStr, err)
	}

	headers, err := ReadHeaders(r)
	if err != nil {
		return "", Response{}, fmt.Errorf("reading headers: %w", err)
	}

	body, err := ReadBody(r)
	if err != nil {
		return "", Response{}, fmt.Errorf("reading body: %w", err)
	}

	return id, Response{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}
