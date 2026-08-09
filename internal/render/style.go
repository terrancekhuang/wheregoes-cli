package render

import (
	"fmt"
	"net/http"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiCyan   = "\x1b[36m"
)

// style controls how the shared renderer wraps text: plainStyle leaves it
// untouched, colorStyle wraps it in ANSI escapes. Keeping this as data
// (rather than branching on a bool everywhere) lets Text and Color share
// every line of hop/header/body assembly logic.
type style struct {
	bold        func(string) string
	dim         func(string) string
	cyan        func(string) string
	statusColor func(code int) func(string) string
	// showFinalCallout controls whether a bordered "Final destination"
	// block is printed before the Summary. Plain text keeps milestone 1's
	// Summary-only ending; the callout is a color-mode visual affordance.
	showFinalCallout bool
}

func identity(s string) string { return s }

var plainStyle = style{
	bold:        identity,
	dim:         identity,
	cyan:        identity,
	statusColor: func(int) func(string) string { return identity },
}

var colorStyle = style{
	bold: wrap(ansiBold),
	dim:  wrap(ansiDim),
	cyan: wrap(ansiBold + ansiCyan),
	statusColor: func(code int) func(string) string {
		switch {
		case code >= 200 && code < 300:
			return wrap(ansiGreen)
		case code >= 300 && code < 400:
			return wrap(ansiYellow)
		case code >= 400:
			return wrap(ansiRed)
		default:
			return identity
		}
	},
	showFinalCallout: true,
}

func wrap(code string) func(string) string {
	return func(s string) string { return code + s + ansiReset }
}

// statusLabel formats a status code with its text ("301 Moved Permanently"),
// falling back to just the code when http.StatusText doesn't recognize it.
func statusLabel(code int) string {
	if text := http.StatusText(code); text != "" {
		return fmt.Sprintf("%d %s", code, text)
	}
	return fmt.Sprintf("%d", code)
}
