package render

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/terrancekhuang/wheregoes/internal/tracer"
)

func hopTrace(status int) *tracer.Trace {
	return &tracer.Trace{
		OriginalURL: "https://short.example/abc",
		FinalURL:    "https://final.example/page",
		HopCount:    1,
		TotalTime:   50 * time.Millisecond,
		Hops: []tracer.Hop{
			{
				RequestedURL: "https://short.example/abc",
				StatusCode:   status,
				Headers:      http.Header{"Content-Type": {"text/plain"}},
				Body:         []byte("hi"),
				Duration:     10 * time.Millisecond,
			},
		},
	}
}

func TestColor_StatusCodeColoring(t *testing.T) {
	cases := []struct {
		status int
		ansi   string
	}{
		{http.StatusOK, ansiGreen},
		{http.StatusMovedPermanently, ansiYellow},
		{http.StatusNotFound, ansiRed},
		{http.StatusInternalServerError, ansiRed},
	}
	for _, c := range cases {
		var buf strings.Builder
		if err := Color(&buf, hopTrace(c.status), false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, c.ansi) {
			t.Errorf("status %d: output missing color %q\n---\n%s", c.status, c.ansi, out)
		}
	}
}

func TestColor_ContainsResetCodes(t *testing.T) {
	var buf strings.Builder
	if err := Color(&buf, hopTrace(http.StatusOK), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, ansiReset) {
		t.Errorf("colored output never resets\n---\n%s", out)
	}
}

func TestColor_FinalDestinationCallout(t *testing.T) {
	trace := hopTrace(http.StatusOK)
	var buf strings.Builder
	if err := Color(&buf, trace, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Final destination") {
		t.Errorf("output missing final destination callout\n---\n%s", out)
	}
	if !strings.Contains(out, trace.FinalURL) {
		t.Errorf("callout missing final URL\n---\n%s", out)
	}
}

func TestColor_DefaultOmitsTimingHeadersAndBody(t *testing.T) {
	var buf strings.Builder
	if err := Color(&buf, hopTrace(http.StatusOK), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, unwanted := range []string{"Time:", "Headers:", "Content-Type", "Body:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("default (non-verbose) output should omit %q\n---\n%s", unwanted, out)
		}
	}
}

func TestColor_StillContainsPlainContent(t *testing.T) {
	// The underlying structure (hop numbering, headers, summary) must
	// survive being wrapped in ANSI codes.
	var buf strings.Builder
	if err := Color(&buf, hopTrace(http.StatusMovedPermanently), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Hop 1: GET https://short.example/abc",
		"Content-Type: text/plain",
		"Summary:",
		"Total hops:   1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}
