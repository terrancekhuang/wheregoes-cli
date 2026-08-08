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

	if err := render.Text(stdout, result); err != nil {
		fmt.Fprintf(stderr, "Error: failed to write output: %s\n", err)
		return 1
	}
	return 0
}
