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

func TestShouldUseColor(t *testing.T) {
	env := func(vals map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			v, ok := vals[key]
			return v, ok
		}
	}

	cases := []struct {
		name     string
		env      map[string]string
		terminal bool
		want     bool
	}{
		{"no env, not a terminal", nil, false, false},
		{"no env, is a terminal", nil, true, true},
		{"NO_COLOR set disables even on a terminal", map[string]string{"NO_COLOR": "1"}, true, false},
		{"NO_COLOR set empty still disables", map[string]string{"NO_COLOR": ""}, true, false},
		{"TERM=dumb disables even on a terminal", map[string]string{"TERM": "dumb"}, true, false},
		{"TERM set to something else on a terminal", map[string]string{"TERM": "xterm-256color"}, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldUseColor(env(c.env), c.terminal)
			if got != c.want {
				t.Errorf("shouldUseColor(%v, %v) = %v; want %v", c.env, c.terminal, got, c.want)
			}
		})
	}
}

func TestIsTerminal_NonFileWriterIsNeverATerminal(t *testing.T) {
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Errorf("bytes.Buffer reported as a terminal")
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
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("stdout = %q; want no ANSI escapes when stdout isn't a terminal", stdout.String())
	}
}
