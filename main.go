// Command wheregoes traces a URL through its full chain of HTTP redirects,
// printing each hop's status, headers, body, and timing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/terrancekhuang/wheregoes/internal/render"
	"github.com/terrancekhuang/wheregoes/internal/tracer"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wheregoes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: wheregoes <url>")
		fmt.Fprintln(stderr, "\nTraces a URL through its full chain of HTTP redirects and prints each hop.")
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

	result, err := tracer.Run(context.Background(), fs.Arg(0), tracer.DefaultOptions())
	if err != nil {
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

	renderFn := render.Text
	if shouldUseColor(os.LookupEnv, isTerminal(stdout)) {
		renderFn = render.Color
	}
	if err := renderFn(stdout, result); err != nil {
		fmt.Fprintf(stderr, "Error: failed to write output: %s\n", err)
		return 1
	}
	return 0
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
