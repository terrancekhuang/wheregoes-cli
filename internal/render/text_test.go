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
	if err := Text(&buf, trace, false); err != nil {
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

func TestText_DefaultOmitsTimingHeadersAndBody(t *testing.T) {
	trace := &tracer.Trace{
		OriginalURL: "https://short.example/abc",
		FinalURL:    "https://final.example/page",
		HopCount:    1,
		Hops: []tracer.Hop{
			{
				RequestedURL: "https://short.example/abc",
				StatusCode:   http.StatusOK,
				Headers:      http.Header{"Content-Type": {"text/html"}},
				Body:         []byte("<html>final</html>"),
				Duration:     142 * time.Millisecond,
			},
		},
	}

	var buf strings.Builder
	if err := Text(&buf, trace, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, unwanted := range []string{"Time:", "Headers:", "Content-Type", "Body:", "<html>final</html>"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("default (non-verbose) output should omit %q\n---\n%s", unwanted, out)
		}
	}
}

func TestText_VerboseIncludesTimingHeadersAndBody(t *testing.T) {
	trace := &tracer.Trace{
		OriginalURL: "https://short.example/abc",
		FinalURL:    "https://final.example/page",
		HopCount:    1,
		Hops: []tracer.Hop{
			{
				RequestedURL: "https://short.example/abc",
				StatusCode:   http.StatusOK,
				Headers:      http.Header{"Content-Type": {"text/html"}},
				Body:         []byte("<html>final</html>"),
				Duration:     142 * time.Millisecond,
			},
		},
	}

	var buf strings.Builder
	if err := Text(&buf, trace, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"Time:   142ms", "Headers:", "Content-Type: text/html", "Body:", "<html>final</html>"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output missing %q\n---\n%s", want, out)
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
	if err := Text(&buf, trace, true); err != nil {
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
	if err := Text(&buf, trace, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[truncated, response continues beyond 100 bytes]") {
		t.Errorf("output missing truncation note\n---\n%s", out)
	}
}

// incompleteTrace is a one-hop chain that never reached its destination,
// mirroring what Run returns when the last hop is blocked.
func incompleteTrace() *tracer.Trace {
	return &tracer.Trace{
		OriginalURL: "https://short.example/abc",
		FinalURL:    "https://blocked.example/page",
		HopCount:    1,
		TotalTime:   142 * time.Millisecond,
		Incomplete:  true,
		Hops: []tracer.Hop{
			{
				RequestedURL:   "https://short.example/abc",
				StatusCode:     http.StatusMovedPermanently,
				Headers:        http.Header{"Location": {"https://blocked.example/page"}},
				ContentLength:  -1,
				RedirectTarget: "https://blocked.example/page",
				Duration:       142 * time.Millisecond,
			},
		},
	}
}

func TestText_IncompleteTraceLabelsStoppedAt(t *testing.T) {
	var sb strings.Builder
	if err := Text(&sb, incompleteTrace(), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "Stopped at:   https://blocked.example/page") {
		t.Fatalf("output missing 'Stopped at' summary line:\n%s", out)
	}
	if strings.Contains(out, "Final URL:") {
		t.Fatalf("incomplete trace must not claim a final URL:\n%s", out)
	}
	if !strings.Contains(out, "Redirects to: https://blocked.example/page") {
		t.Fatalf("output missing the hop that was observed before the failure:\n%s", out)
	}
}

func TestColor_IncompleteTraceOmitsFinalCallout(t *testing.T) {
	var sb strings.Builder
	if err := Color(&sb, incompleteTrace(), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out := sb.String(); strings.Contains(out, "Final destination") {
		t.Fatalf("incomplete trace must not print the success callout:\n%s", out)
	}
}
