// Command wheregoes traces a URL through its full chain of HTTP redirects,
// printing each hop's status and redirect target. Pass --verbose to also
// print each hop's timing, response headers, and response body.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"golang.org/x/term"

	"github.com/terrancekhuang/wheregoes/internal/render"
	"github.com/terrancekhuang/wheregoes/internal/tracer"
)

// copyToClipboard writes text to the system clipboard. It's a package-level
// var (rather than a direct call to clipboard.WriteAll) so tests can inject
// a fake and assert both the success and failure paths without touching a
// real OS clipboard, which usually isn't available in CI/sandboxes.
var copyToClipboard = clipboard.WriteAll

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wheregoes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	copyFlag := fs.Bool("copy", false, "copy the final destination URL to the clipboard")
	verboseFlag := fs.Bool("verbose", false, "show per-hop timing, response headers, and response body")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: wheregoes [--copy] [--verbose] <url>")
		fmt.Fprintln(stderr, "\nTraces a URL through its full chain of HTTP redirects and prints each hop.")
		fmt.Fprintln(stderr, "\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(stderr, "Error: expected exactly 1 argument (a URL), got %d\n", fs.NArg())
		fs.Usage()
		return 2
	}

	url := fs.Arg(0)
	if strings.Contains(url, `\`) {
		fmt.Fprintf(stderr, "Warning: URL contains a backslash (%q); this is usually leftover shell escaping "+
			"and will be requested literally, likely producing a wrong path. Re-run with the raw URL in single quotes.\n", url)
	}

	result, err := tracer.Run(context.Background(), url, tracer.DefaultOptions())
	if err != nil {
		// A failed trace still carries every hop it got through before
		// giving up, and for a chain that only died on its last hop those
		// hops already show where the link goes. Print them before the
		// error rather than discarding the whole trace.
		if result != nil && len(result.Hops) > 0 {
			renderTrace(stdout, result, *verboseFlag)
			if *copyFlag {
				copyFinalURL(stdout, stderr, result)
			}
		}
		fmt.Fprintf(stderr, "Error: %s\n", err)
		var traceErr *tracer.TraceError
		if errors.As(err, &traceErr) {
			switch traceErr.Kind {
			case tracer.KindInvalidURL:
				return 3
			case tracer.KindHopLimit:
				return 4
			case tracer.KindNetwork:
				return 5
			}
		}
		return 1
	}

	if err := renderTrace(stdout, result, *verboseFlag); err != nil {
		fmt.Fprintf(stderr, "Error: failed to write output: %s\n", err)
		return 1
	}

	if *copyFlag {
		copyFinalURL(stdout, stderr, result)
	}
	return 0
}

// copyFinalURL puts the trace's destination on the clipboard. An
// incomplete trace still gets copied — a blocked last hop is usually the
// URL the user wanted — but is labeled as unverified so the notice can't
// be mistaken for a destination we actually reached.
func copyFinalURL(stdout, stderr io.Writer, t *tracer.Trace) {
	if t.FinalURL == "" {
		return
	}
	if err := copyToClipboard(t.FinalURL); err != nil {
		fmt.Fprintf(stderr, "Warning: could not copy to clipboard: %s\n", err)
		return
	}
	if t.Incomplete {
		fmt.Fprintf(stdout, "Copied unverified destination to clipboard: %s\n", t.FinalURL)
		return
	}
	fmt.Fprintf(stdout, "Copied to clipboard: %s\n", t.FinalURL)
}

// renderTrace writes t to stdout, picking the color or plain renderer
// based on whether color is appropriate for this destination.
func renderTrace(stdout io.Writer, t *tracer.Trace, verbose bool) error {
	renderFn := render.Text
	if shouldUseColor(os.LookupEnv, isTerminal(stdout)) {
		renderFn = render.Color
	}
	return renderFn(stdout, t, verbose)
}

// isTerminal reports whether w is a TTY. Only *os.File can be a terminal,
// so writers used in tests (e.g. *bytes.Buffer) always report false,
// keeping run() deterministic without a real TTY.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// shouldUseColor decides whether to emit ANSI color codes, honoring the
// NO_COLOR convention (https://no-color.org — presence of the variable
// disables color regardless of its value) and TERM=dumb before falling
// back to whether stdout is actually a terminal. lookupEnv and isTerminal
// are injected so tests can exercise every branch without a real TTY.
func shouldUseColor(lookupEnv func(string) (string, bool), terminal bool) bool {
	if _, set := lookupEnv("NO_COLOR"); set {
		return false
	}
	if termEnv, _ := lookupEnv("TERM"); termEnv == "dumb" {
		return false
	}
	return terminal
}
