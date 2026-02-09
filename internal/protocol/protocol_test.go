package protocol

import (
	"bufio"
	"bytes"
	"net/http"
	"testing"
)

func TestReadLine(t *testing.T) {
	t.Run("normal line", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("hello world\n"))
		line, err := ReadLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if line != "hello world" {
			t.Fatalf("got %q, want %q", line, "hello world")
		}
	})

	t.Run("empty line", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("\n"))
		line, err := ReadLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if line != "" {
			t.Fatalf("got %q, want empty", line)
		}
	})

	t.Run("EOF without newline", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("no newline"))
		_, err := ReadLine(r)
		if err == nil {
			t.Fatal("expected error for EOF without newline")
		}
	})
}

func TestReadHeaders(t *testing.T) {
	t.Run("multiple headers", func(t *testing.T) {
		input := "Content-Type: application/json\nX-Custom: test\n\n"
		r := bufio.NewReader(bytes.NewBufferString(input))
		headers, err := ReadHeaders(r)
		if err != nil {
			t.Fatal(err)
		}
		if got := headers.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want %q", got, "application/json")
		}
		if got := headers.Get("X-Custom"); got != "test" {
			t.Fatalf("X-Custom = %q, want %q", got, "test")
		}
	})

	t.Run("empty headers", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("\n"))
		headers, err := ReadHeaders(r)
		if err != nil {
			t.Fatal(err)
		}
		if len(headers) != 0 {
			t.Fatalf("expected 0 headers, got %d", len(headers))
		}
	})

	t.Run("multi-value headers", func(t *testing.T) {
		input := "Accept: text/html\nAccept: application/json\n\n"
		r := bufio.NewReader(bytes.NewBufferString(input))
		headers, err := ReadHeaders(r)
		if err != nil {
			t.Fatal(err)
		}
		values := headers.Values("Accept")
		if len(values) != 2 {
			t.Fatalf("expected 2 Accept values, got %d", len(values))
		}
	})

	t.Run("invalid lines skipped", func(t *testing.T) {
		input := "Good-Header: value\nbadline\n\n"
		r := bufio.NewReader(bytes.NewBufferString(input))
		headers, err := ReadHeaders(r)
		if err != nil {
			t.Fatal(err)
		}
		if got := headers.Get("Good-Header"); got != "value" {
			t.Fatalf("Good-Header = %q, want %q", got, "value")
		}
		if len(headers) != 1 {
			t.Fatalf("expected 1 header, got %d", len(headers))
		}
	})
}

func TestWriteAndReadHeaders(t *testing.T) {
	original := make(http.Header)
	original.Set("Content-Type", "text/plain")
	original.Set("X-Request-Id", "abc-123")

	var buf bytes.Buffer
	if err := WriteHeaders(&buf, original); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	got, err := ReadHeaders(r)
	if err != nil {
		t.Fatal(err)
	}

	if got.Get("Content-Type") != "text/plain" {
		t.Fatalf("Content-Type = %q, want %q", got.Get("Content-Type"), "text/plain")
	}
	if got.Get("X-Request-Id") != "abc-123" {
		t.Fatalf("X-Request-Id = %q, want %q", got.Get("X-Request-Id"), "abc-123")
	}
}

func TestReadBody(t *testing.T) {
	t.Run("normal body", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WriteBody(&buf, []byte("hello")); err != nil {
			t.Fatal(err)
		}
		r := bufio.NewReader(&buf)
		body, err := ReadBody(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "hello" {
			t.Fatalf("got %q, want %q", string(body), "hello")
		}
	})

	t.Run("zero-length body", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WriteBody(&buf, []byte{}); err != nil {
			t.Fatal(err)
		}
		r := bufio.NewReader(&buf)
		body, err := ReadBody(r)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) != 0 {
			t.Fatalf("expected empty body, got %d bytes", len(body))
		}
	})

	t.Run("short read error", func(t *testing.T) {
		// Only write the length prefix, no body bytes
		var buf bytes.Buffer
		if err := WriteBody(&buf, []byte("hello")); err != nil {
			t.Fatal(err)
		}
		// Truncate after the length prefix
		truncated := buf.Bytes()[:4]
		r := bufio.NewReader(bytes.NewReader(truncated))
		_, err := ReadBody(r)
		if err == nil {
			t.Fatal("expected error for short read")
		}
	})
}

func TestWriteAndReadBody(t *testing.T) {
	original := []byte("test body content with special chars: éàü")
	var buf bytes.Buffer
	if err := WriteBody(&buf, original); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(&buf)
	got, err := ReadBody(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("body mismatch: got %q, want %q", got, original)
	}
}

func TestReadRequestLine(t *testing.T) {
	t.Run("normal request line", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("req-123 GET /api/users\n"))
		id, method, path, err := ReadRequestLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if id != "req-123" {
			t.Fatalf("id = %q, want %q", id, "req-123")
		}
		if method != "GET" {
			t.Fatalf("method = %q, want %q", method, "GET")
		}
		if path != "/api/users" {
			t.Fatalf("path = %q, want %q", path, "/api/users")
		}
	})

	t.Run("with query string", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("req-456 POST /search?q=hello\n"))
		id, method, path, err := ReadRequestLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if id != "req-456" {
			t.Fatalf("id = %q, want %q", id, "req-456")
		}
		if method != "POST" {
			t.Fatalf("method = %q, want %q", method, "POST")
		}
		if path != "/search?q=hello" {
			t.Fatalf("path = %q, want %q", path, "/search?q=hello")
		}
	})

	t.Run("malformed line", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewBufferString("only-two-parts GET\n"))
		_, _, _, err := ReadRequestLine(r)
		if err == nil {
			t.Fatal("expected error for malformed request line")
		}
	})
}

func TestWriteRequestLine(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRequestLine(&buf, "req-789", "DELETE", "/items/5"); err != nil {
		t.Fatal(err)
	}
	expected := "req-789 DELETE /items/5\n"
	if buf.String() != expected {
		t.Fatalf("got %q, want %q", buf.String(), expected)
	}
}

func TestWriteResponse(t *testing.T) {
	resp := Response{
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": {"application/json"}},
		Body:       []byte(`{"ok":true}`),
	}

	var buf bytes.Buffer
	if err := WriteResponse(&buf, "req-100", resp); err != nil {
		t.Fatal(err)
	}

	// Read it back
	r := bufio.NewReader(&buf)
	gotID, gotResp, err := ReadResponse(r)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "req-100" {
		t.Fatalf("id = %q, want %q", gotID, "req-100")
	}
	if gotResp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", gotResp.StatusCode)
	}
	if gotResp.Headers.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", gotResp.Headers.Get("Content-Type"))
	}
	if string(gotResp.Body) != `{"ok":true}` {
		t.Fatalf("body = %q", string(gotResp.Body))
	}
}

func TestRoundTripRequestFrame(t *testing.T) {
	var buf bytes.Buffer

	// Write a full request frame
	if err := WriteRequestLine(&buf, "req-rt", "POST", "/data"); err != nil {
		t.Fatal(err)
	}
	headers := make(http.Header)
	headers.Set("Content-Type", "text/plain")
	if err := WriteHeaders(&buf, headers); err != nil {
		t.Fatal(err)
	}
	if err := WriteBody(&buf, []byte("request body")); err != nil {
		t.Fatal(err)
	}

	// Read it back
	r := bufio.NewReader(&buf)
	id, method, path, err := ReadRequestLine(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != "req-rt" || method != "POST" || path != "/data" {
		t.Fatalf("request line: %q %q %q", id, method, path)
	}

	gotHeaders, err := ReadHeaders(r)
	if err != nil {
		t.Fatal(err)
	}
	if gotHeaders.Get("Content-Type") != "text/plain" {
		t.Fatalf("Content-Type = %q", gotHeaders.Get("Content-Type"))
	}

	body, err := ReadBody(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "request body" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestRoundTripResponseFrame(t *testing.T) {
	var buf bytes.Buffer

	resp := Response{
		StatusCode: 404,
		Headers:    http.Header{"X-Error": {"not found"}},
		Body:       []byte("page not found"),
	}
	if err := WriteResponse(&buf, "req-404", resp); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	gotID, gotResp, err := ReadResponse(r)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "req-404" {
		t.Fatalf("id = %q", gotID)
	}
	if gotResp.StatusCode != 404 {
		t.Fatalf("status = %d", gotResp.StatusCode)
	}
	if gotResp.Headers.Get("X-Error") != "not found" {
		t.Fatalf("X-Error = %q", gotResp.Headers.Get("X-Error"))
	}
	if string(gotResp.Body) != "page not found" {
		t.Fatalf("body = %q", string(gotResp.Body))
	}
}
