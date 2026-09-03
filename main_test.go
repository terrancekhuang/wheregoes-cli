package main

import (
	"bytes"
	"errors"
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

func TestRun_BackslashInURLWarnsButStillTraces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{server.URL + `/a\b`}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Warning: URL contains a backslash") {
		t.Fatalf("stderr = %q; want a backslash warning", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Summary:") {
		t.Fatalf("stdout = %q; want trace output despite the warning", stdout.String())
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
	if strings.Contains(stdout.String(), "Headers:") || strings.Contains(stdout.String(), "Body:") {
		t.Fatalf("stdout = %q; want headers/body omitted by default", stdout.String())
	}
}

func TestRun_VerboseFlagShowsHeadersAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--verbose", server.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Time:", "Headers:", "Content-Type: text/plain", "Body:", "ok"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q; want %q present with --verbose", stdout.String(), want)
		}
	}
}

func withFakeClipboard(t *testing.T, fn func(string) error) {
	t.Helper()
	original := copyToClipboard
	copyToClipboard = fn
	t.Cleanup(func() { copyToClipboard = original })
}

func TestRun_CopyFlagSuccessPrintsConfirmation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	var copied string
	withFakeClipboard(t, func(s string) error {
		copied = s
		return nil
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"--copy", server.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Copied to clipboard: "+server.URL) {
		t.Fatalf("stdout = %q; want a copy confirmation", stdout.String())
	}
	if copied != server.URL {
		t.Fatalf("copied = %q; want %q", copied, server.URL)
	}
}

func TestRun_CopyFlagFailureWarnsButKeepsExitZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	withFakeClipboard(t, func(s string) error {
		return errors.New("no clipboard utility available")
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"--copy", server.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0 even when clipboard copy fails", code)
	}
	if !strings.Contains(stderr.String(), "Warning: could not copy to clipboard") {
		t.Fatalf("stderr = %q; want a clipboard warning", stderr.String())
	}
	if strings.Contains(stdout.String(), "Copied to clipboard") {
		t.Fatalf("stdout = %q; want no confirmation when copy failed", stdout.String())
	}
}

// A chain blocked on its last hop still resolved a destination, so --copy
// should put it on the clipboard rather than leaving the user with
// nothing — labeled unverified, since it was never actually reached.
func TestRun_CopyFlagCopiesIncompleteDestination(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close() // nothing listens on this address anymore

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, deadURL+"/blocked", http.StatusMovedPermanently)
	}))
	defer server.Close()

	var copied string
	withFakeClipboard(t, func(s string) error {
		copied = s
		return nil
	})

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--copy", server.URL}, &stdout, &stderr); code != 5 {
		t.Fatalf("exit code = %d; want 5; stderr=%q", code, stderr.String())
	}
	if copied != deadURL+"/blocked" {
		t.Fatalf("copied = %q; want %q", copied, deadURL+"/blocked")
	}
	if !strings.Contains(stdout.String(), "Copied unverified destination to clipboard: "+deadURL+"/blocked") {
		t.Fatalf("stdout = %q; want an unverified copy confirmation", stdout.String())
	}
}

// With nothing traced there is no destination to copy, so the clipboard
// must be left untouched rather than clobbered with an empty string.
func TestRun_CopyFlagSkipsClipboardWhenNothingTraced(t *testing.T) {
	called := false
	withFakeClipboard(t, func(s string) error {
		called = true
		return nil
	})

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--copy", "://not-a-url"}, &stdout, &stderr); code != 3 {
		t.Fatalf("exit code = %d; want 3; stderr=%q", code, stderr.String())
	}
	if called {
		t.Fatalf("clipboard was written for a trace that never started")
	}
}
