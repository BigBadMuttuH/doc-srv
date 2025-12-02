package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test that loggingMiddleware logs regular requests and captures status/bytes.
func TestLoggingMiddleware_LogsRequest(t *testing.T) {
	// Prepare a simple handler that writes a known body and status.
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	})

	wrapped := loggingMiddleware(baseHandler)

	// Capture access log output.
	var buf bytes.Buffer
	accessLog = log.New(&buf, "", 0)

	req := httptest.NewRequest(http.MethodGet, "/docs/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 from handler, got %d", rec.Code)
	}

	// Ensure something was logged.
	logLine := buf.String()
	if logLine == "" {
		t.Fatalf("expected log output, got empty string")
	}

	if !strings.Contains(logLine, " /docs/test ") {
		t.Errorf("expected log line to contain path /docs/test, got %q", logLine)
	}
	if !strings.Contains(logLine, " 201 ") {
		t.Errorf("expected log line to contain status 201, got %q", logLine)
	}
	if !strings.Contains(logLine, " 5 ") { // body_bytes_sent should be 5 ("hello")
		t.Errorf("expected log line to contain body size 5, got %q", logLine)
	}
}

// Test that loggingMiddleware does not log /healthz requests.
func TestLoggingMiddleware_SkipsHealthz(t *testing.T) {
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := loggingMiddleware(baseHandler)

	var buf bytes.Buffer
	accessLog = log.New(&buf, "", 0)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from handler, got %d", rec.Code)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected no log output for /healthz, got %q", buf.String())
	}
}
