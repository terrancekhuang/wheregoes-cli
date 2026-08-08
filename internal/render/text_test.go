package render

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/terrancekhuang/wheregoes/internal/tracer"
)

func TestText_ContainsHopsAndSummary(t *testing.T) {
	trace := &tracer.Trace{
		OriginalURL: "https://short.example/abc",
		FinalURL:    "https://final.example/page",
		HopCount:    2,
		TotalTime:   230 * time.Millisecond,
		Hops: []tracer.Hop{
			{
				RequestedURL:   "https://short.example/abc",
				StatusCode:     http.StatusMovedPermanently,
				Headers:        http.Header{"Content-Type": {"text/html"}, "Location": {"https://final.example/page"}},
				ContentLength:  -1,
				Body:           []byte("<html>redirecting</html>"),
				RedirectTarget: "https://final.example/page",
				Duration:       142 * time.Millisecond,
			},
			{
				RequestedURL:  "https://final.example/page",
				StatusCode:    http.StatusOK,
				Headers:       http.Header{"Content-Type": {"text/html"}},
				ContentLength: -1,
				Body:          []byte("<html>final</html>"),
				Duration:      88 * time.Millisecond,
			},
		},
	}

	var buf strings.Builder
	if err := Text(&buf, trace); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Hop 1: GET https://short.example/abc",
		"Status: 301 Moved Permanently",
		"Redirects to: https://final.example/page",
		"Hop 2: GET https://final.example/page",
		"Status: 200 OK",
		"Summary:",
		"Original URL: https://short.example/abc",
		"Final URL:    https://final.example/page",
		"Total hops:   2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}

func TestText_BinaryPlaceholder(t *testing.T) {
	trace := &tracer.Trace{
		OriginalURL: "https://x.example/img",
		FinalURL:    "https://x.example/img",
		HopCount:    1,
		Hops: []tracer.Hop{
			{
				RequestedURL:  "https://x.example/img",
				StatusCode:    http.StatusOK,
				Headers:       http.Header{"Content-Type": {"image/png"}},
				ContentLength: 15234,
				IsBinary:      true,
			},
		},
	}

	var buf strings.Builder
	if err := Text(&buf, trace); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[binary content, 15234 bytes not shown]") {
		t.Errorf("output missing binary placeholder\n---\n%s", out)
	}
	if strings.Contains(out, "\x00") {
		t.Errorf("raw binary bytes leaked into output")
	}
}

func TestText_TruncatedBodyNote(t *testing.T) {
	trace := &tracer.Trace{
		OriginalURL: "https://x.example",
		FinalURL:    "https://x.example",
		HopCount:    1,
		Hops: []tracer.Hop{
			{
				RequestedURL:  "https://x.example",
				StatusCode:    http.StatusOK,
				Headers:       http.Header{"Content-Type": {"text/plain"}},
				ContentLength: -1,
				Body:          []byte(strings.Repeat("a", 100)),
				BodyTruncated: true,
			},
		},
	}

	var buf strings.Builder
	if err := Text(&buf, trace); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[truncated, response continues beyond 100 bytes]") {
		t.Errorf("output missing truncation note\n---\n%s", out)
	}
}
