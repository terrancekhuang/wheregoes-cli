// Package render formats a tracer.Trace for display.
package render

import (
	"fmt"
	"io"
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
// followed by a summary of the whole chain. Per-hop timing, headers, and
// body are only included when verbose is true; the default view shows
// just enough per hop to follow the chain (status and redirect target).
func Text(w io.Writer, t *tracer.Trace, verbose bool) error {
	return render(w, t, plainStyle, verbose)
}

// render writes a hop-by-hop trace followed by a summary, using s to
// control whether (and how) output is color-wrapped. Text and Color are
// both thin wrappers over this so the two never drift structurally apart.
func render(w io.Writer, t *tracer.Trace, s style, verbose bool) error {
	ew := &errWriter{w: w}
	for i, hop := range t.Hops {
		writeHop(ew, i+1, hop, s, verbose)
		ew.printf("\n")
	}
	if s.showFinalCallout && !t.Incomplete {
		writeFinalCallout(ew, t, s)
	}
	writeSummary(ew, t, s)
	return ew.err
}

func writeHop(ew *errWriter, number int, hop tracer.Hop, s style, verbose bool) {
	ew.printf("%s\n", s.cyan(fmt.Sprintf("Hop %d: GET %s", number, hop.RequestedURL)))

	statusColor := s.statusColor(hop.StatusCode)
	ew.printf("  Status: %s\n", statusColor(statusLabel(hop.StatusCode)))
	if hop.RedirectTarget != "" {
		ew.printf("  Redirects to: %s\n", s.bold(hop.RedirectTarget))
	}
	if !verbose {
		return
	}

	ew.printf("  Time:   %s\n", hop.Duration.Round(time.Millisecond))

	ew.printf("  %s\n", s.dim("Headers:"))
	keys := make([]string, 0, len(hop.Headers))
	for k := range hop.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range hop.Headers[k] {
			ew.printf("    %s\n", s.dim(fmt.Sprintf("%s: %s", k, v)))
		}
	}

	ew.printf("  %s\n", s.dim("Body:"))
	if hop.IsBinary {
		ew.printf("    %s\n", s.dim(binaryPlaceholder(hop)))
		return
	}
	writeBody(ew, hop, s)
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

func writeBody(ew *errWriter, hop tracer.Hop, s style) {
	body := string(hop.Body)
	if body == "" {
		ew.printf("    %s\n", s.dim("(empty)"))
	} else {
		for _, line := range strings.Split(body, "\n") {
			ew.printf("    %s\n", s.dim(line))
		}
	}
	if hop.BodyTruncated {
		ew.printf("    %s\n", s.dim(fmt.Sprintf("[truncated, response continues beyond %d bytes]", len(hop.Body))))
	}
}

func writeFinalCallout(ew *errWriter, t *tracer.Trace, s style) {
	border := strings.Repeat("─", 40)
	ew.printf("%s\n", s.dim(border))
	ew.printf("%s\n", s.bold("✓ Final destination"))
	ew.printf("  %s\n", s.bold(s.statusColor(200)(t.FinalURL)))
	ew.printf("%s\n\n", s.dim(border))
}

func writeSummary(ew *errWriter, t *tracer.Trace, s style) {
	ew.printf("%s\n", s.dim("Summary:"))
	ew.printf("  Original URL: %s\n", t.OriginalURL)
	// An incomplete trace never confirmed its last URL, so label it as
	// where the chain stopped rather than as a final destination.
	if t.Incomplete {
		ew.printf("  Stopped at:   %s\n", s.bold(t.FinalURL))
	} else {
		ew.printf("  Final URL:    %s\n", s.bold(t.FinalURL))
	}
	ew.printf("  Total hops:   %d\n", t.HopCount)
	ew.printf("  Total time:   %s\n", t.TotalTime.Round(time.Millisecond))
}
