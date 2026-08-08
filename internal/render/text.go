// Package render formats a tracer.Trace for display.
package render

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/terrancekhuang/wheregoes/internal/tracer"
)

// errWriter accumulates the first write error encountered so callers can
// write a sequence of lines without checking every individual error.
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) printf(format string, args ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, args...)
}

// Text writes a plain-text rendering of the trace: one block per hop
// followed by a summary of the whole chain.
func Text(w io.Writer, t *tracer.Trace) error {
	ew := &errWriter{w: w}
	for i, hop := range t.Hops {
		writeHop(ew, i+1, hop)
		ew.printf("\n")
	}
	writeSummary(ew, t)
	return ew.err
}

func writeHop(ew *errWriter, number int, hop tracer.Hop) {
	ew.printf("Hop %d: GET %s\n", number, hop.RequestedURL)

	if statusText := http.StatusText(hop.StatusCode); statusText != "" {
		ew.printf("  Status: %d %s\n", hop.StatusCode, statusText)
	} else {
		ew.printf("  Status: %d\n", hop.StatusCode)
	}
	ew.printf("  Time:   %s\n", hop.Duration.Round(time.Millisecond))
	if hop.RedirectTarget != "" {
		ew.printf("  Redirects to: %s\n", hop.RedirectTarget)
	}

	ew.printf("  Headers:\n")
	keys := make([]string, 0, len(hop.Headers))
	for k := range hop.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range hop.Headers[k] {
			ew.printf("    %s: %s\n", k, v)
		}
	}

	ew.printf("  Body:\n")
	if hop.IsBinary {
		ew.printf("    %s\n", binaryPlaceholder(hop))
		return
	}
	writeBody(ew, hop)
}

// binaryPlaceholder describes a non-text body without dumping its bytes,
// preferring the declared Content-Length and falling back to what was
// actually captured (up to the body cap) when the length is unknown.
func binaryPlaceholder(hop tracer.Hop) string {
	switch {
	case hop.ContentLength >= 0:
		return fmt.Sprintf("[binary content, %d bytes not shown]", hop.ContentLength)
	case hop.BodyTruncated:
		return fmt.Sprintf("[binary content, at least %d bytes not shown]", len(hop.Body))
	default:
		return fmt.Sprintf("[binary content, %d bytes not shown]", len(hop.Body))
	}
}

func writeBody(ew *errWriter, hop tracer.Hop) {
	body := string(hop.Body)
	if body == "" {
		ew.printf("    (empty)\n")
	} else {
		for _, line := range strings.Split(body, "\n") {
			ew.printf("    %s\n", line)
		}
	}
	if hop.BodyTruncated {
		ew.printf("    [truncated, response continues beyond %d bytes]\n", len(hop.Body))
	}
}

func writeSummary(ew *errWriter, t *tracer.Trace) {
	ew.printf("Summary:\n")
	ew.printf("  Original URL: %s\n", t.OriginalURL)
	ew.printf("  Final URL:    %s\n", t.FinalURL)
	ew.printf("  Total hops:   %d\n", t.HopCount)
	ew.printf("  Total time:   %s\n", t.TotalTime.Round(time.Millisecond))
}
