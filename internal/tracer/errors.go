package tracer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

// Kind categorizes why a trace failed, so callers (main.go) can map it to
// a distinct exit code and message without re-parsing the error text.
type Kind int

const (
	KindInvalidURL Kind = iota
	KindHopLimit
	KindNetwork
)

// TraceError is the single error type returned by Trace. Err is nil for
// KindHopLimit, which has no single underlying cause.
type TraceError struct {
	Kind    Kind
	URL     string
	MaxHops int
	Err     error
}

func (e *TraceError) Unwrap() error { return e.Err }

func (e *TraceError) Error() string {
	switch e.Kind {
	case KindInvalidURL:
		return fmt.Sprintf("invalid URL %q: %v", e.URL, e.Err)
	case KindHopLimit:
		return fmt.Sprintf("stopped after %d redirects without reaching a final destination (possible redirect loop)", e.MaxHops)
	case KindNetwork:
		return fmt.Sprintf("request to %s failed: %s", e.URL, describeNetErr(e.Err))
	default:
		return e.Err.Error()
	}
}

// describeNetErr turns common low-level network errors into messages a
// terminal user can act on, instead of surfacing raw Go error chains.
func describeNetErr(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Sprintf("could not resolve host %q", dnsErr.Name)
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused"
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return "TLS certificate verification failed"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "request timed out"
	}
	if isPeerStreamReset(err) {
		return fmt.Sprintf("%s — the server accepted the connection then killed the HTTP/2 stream, "+
			"which usually means its bot protection rejected a non-browser client rather than that "+
			"the URL is wrong", err)
	}
	return err.Error()
}

// isPeerStreamReset reports whether err is an HTTP/2 stream reset sent by
// the server. net/http vendors its http2 package privately, so the typed
// http2.StreamError isn't reachable here and the wire text is the only
// available signal; both markers are matched to keep the match narrow.
func isPeerStreamReset(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "stream error:") && strings.Contains(msg, "received from peer")
}
