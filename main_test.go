package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRun_MissingArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
	if !strings.Contains(stderr.String(), "expected exactly 1 argument") {
		t.Fatalf("stderr = %q; want message about missing argument", stderr.String())
	}
}

func TestRun_TooManyArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"https://a.example", "https://b.example"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage: wheregoes") {
		t.Fatalf("stderr = %q; want usage text", stderr.String())
	}
}

func TestRun_MalformedURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"://not-a-url"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d; want 3", code)
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Fatalf("stderr = %q; want an Error: message", stderr.String())
	}
}

func TestRun_UnreachableHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := server.URL
	server.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{deadURL}, &stdout, &stderr)
	if code != 5 {
		t.Fatalf("exit code = %d; want 5", code)
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Fatalf("stderr = %q; want an Error: message", stderr.String())
	}
}

func TestRun_SuccessPrintsTraceToStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{server.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Summary:") {
		t.Fatalf("stdout = %q; want trace output", stdout.String())
	}
}
