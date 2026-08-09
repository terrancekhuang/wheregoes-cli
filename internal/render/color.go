package render

import (
	"io"

	"github.com/terrancekhuang/wheregoes/internal/tracer"
)

// Color writes an ANSI color-coded rendering of the trace: numbered,
// visually-separated hops with status-coded coloring, followed by a
// highlighted final-destination callout and summary. Callers are
// responsible for deciding when color is appropriate (e.g. only when
// stdout is a terminal) — Color always emits escape codes. Per-hop timing,
// headers, and body are only included when verbose is true.
func Color(w io.Writer, t *tracer.Trace, verbose bool) error {
	return render(w, t, colorStyle, verbose)
}
