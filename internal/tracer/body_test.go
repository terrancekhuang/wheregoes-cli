package tracer

import (
	"strings"
	"testing"
)

func TestReadCapped(t *testing.T) {
	t.Run("under cap", func(t *testing.T) {
		data, truncated, err := readCapped(strings.NewReader("hello"), 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if truncated {
			t.Fatalf("expected truncated=false")
		}
		if string(data) != "hello" {
			t.Fatalf("got %q, want %q", data, "hello")
		}
	})

	t.Run("exactly at cap", func(t *testing.T) {
		data, truncated, err := readCapped(strings.NewReader("hello"), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if truncated {
			t.Fatalf("expected truncated=false")
		}
		if string(data) != "hello" {
			t.Fatalf("got %q, want %q", data, "hello")
		}
	})

	t.Run("over cap", func(t *testing.T) {
		data, truncated, err := readCapped(strings.NewReader("hello world"), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !truncated {
			t.Fatalf("expected truncated=true")
		}
		if string(data) != "hello" {
			t.Fatalf("got %q, want %q", data, "hello")
		}
		if len(data) != 5 {
			t.Fatalf("got len %d, want 5", len(data))
		}
	})
}

func TestIsTextish(t *testing.T) {
	cases := []struct {
		contentType string
		want        bool
	}{
		{"text/html", true},
		{"text/plain; charset=utf-8", true},
		{"application/json", true},
		{"application/ld+json", true},
		{"application/xml", true},
		{"application/xhtml+xml", true},
		{"", true},
		{"image/png", false},
		{"application/octet-stream", false},
		{"video/mp4", false},
	}

	for _, tc := range cases {
		t.Run(tc.contentType, func(t *testing.T) {
			got := isTextish(tc.contentType)
			if got != tc.want {
				t.Fatalf("isTextish(%q) = %v; want %v", tc.contentType, got, tc.want)
			}
		})
	}
}
