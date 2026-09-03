// Package tracer follows a URL through its full chain of HTTP redirects,
// capturing per-hop metadata (status, headers, body, timing) along the way.
package tracer

import (
	"context"
	"net/http"
	"time"
)

const (
	// DefaultMaxHops caps how many redirects are followed before Trace
	// gives up with a KindHopLimit error, matching common browser limits
	// and guarding against redirect loops.
	DefaultMaxHops = 20
	// DefaultBodyCap is the number of response body bytes captured per hop.
	DefaultBodyCap = 4096
	// DefaultTimeout bounds each individual hop's request, so a stalled
	// connection can never hang the whole trace indefinitely.
	DefaultTimeout   = 15 * time.Second
	DefaultUserAgent = "wheregoes-cli/0.1"
)

// Hop captures everything observed for a single request in the chain.
type Hop struct {
	RequestedURL   string
	StatusCode     int
	Headers        http.Header
	ContentLength  int64 // from the Content-Length header; -1 if unknown
	Body           []byte
	BodyTruncated  bool
	IsBinary       bool   // decided from Content-Type; Body is not meant to be printed as text when true
	RedirectTarget string // Location header value if this hop redirected, else ""
	Duration       time.Duration
}

// Trace is the full result of following a URL to its final destination.
type Trace struct {
	OriginalURL string
	FinalURL    string
	Hops        []Hop
	HopCount    int
	TotalTime   time.Duration
	// Incomplete is true when the chain stopped early because a hop
	// failed. Hops then holds every hop observed before the failure and
	// FinalURL is the URL that could not be fetched, rather than a
	// confirmed destination.
	Incomplete bool
}

// Options configures a Trace run. The zero value is not directly usable;
// callers should start from DefaultOptions().
type Options struct {
	MaxHops   int
	BodyCap   int64
	Timeout   time.Duration
	UserAgent string
	// Client, if set, is used instead of building one from Timeout. This
	// is the injection seam tests use to supply a custom RoundTripper.
	Client *http.Client
}

func DefaultOptions() Options {
	return Options{
		MaxHops:   DefaultMaxHops,
		BodyCap:   DefaultBodyCap,
		Timeout:   DefaultTimeout,
		UserAgent: DefaultUserAgent,
	}
}

func buildClient(opts Options) *http.Client {
	if opts.Client != nil {
		return opts.Client
	}
	return &http.Client{
		Timeout: opts.Timeout,
		// Returning ErrUseLastResponse makes Do() hand back the raw 3xx
		// response instead of auto-following it, so each hop can be
		// inspected and recorded individually.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Run follows rawURL through its full redirect chain, returning every
// hop observed along the way.
func Run(ctx context.Context, rawURL string, opts Options) (*Trace, error) {
	start := time.Now()

	normalized, err := normalizeURL(rawURL)
	if err != nil {
		return nil, &TraceError{Kind: KindInvalidURL, URL: rawURL, Err: err}
	}

	client := buildClient(opts)
	currentURL := normalized
	trace := &Trace{OriginalURL: normalized}

	// finish stamps the summary fields and hands back the trace. Every
	// exit below goes through it, including the failure paths: a chain
	// that dies on its last hop has usually already answered the question
	// the user asked, so the hops observed so far are returned alongside
	// the error rather than thrown away.
	finish := func(stoppedAt string, incomplete bool) *Trace {
		trace.FinalURL = stoppedAt
		trace.HopCount = len(trace.Hops)
		trace.TotalTime = time.Since(start)
		trace.Incomplete = incomplete
		return trace
	}

	for {
		hop, err := fetchHop(ctx, client, currentURL, opts)
		if err != nil {
			return finish(currentURL, true), &TraceError{Kind: KindNetwork, URL: currentURL, Err: err}
		}
		trace.Hops = append(trace.Hops, *hop)

		if hop.RedirectTarget == "" {
			break
		}
		if len(trace.Hops) > opts.MaxHops {
			return finish(currentURL, true), &TraceError{Kind: KindHopLimit, MaxHops: opts.MaxHops}
		}

		next, err := resolveRedirect(currentURL, hop.RedirectTarget)
		if err != nil {
			return finish(currentURL, true), &TraceError{Kind: KindInvalidURL, URL: hop.RedirectTarget, Err: err}
		}
		currentURL = next
	}

	return finish(currentURL, false), nil
}

func fetchHop(ctx context.Context, client *http.Client, rawURL string, opts Options) (*Hop, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", opts.UserAgent)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, truncated, err := readCapped(resp.Body, opts.BodyCap)
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)

	hop := &Hop{
		RequestedURL:  rawURL,
		StatusCode:    resp.StatusCode,
		Headers:       resp.Header,
		ContentLength: resp.ContentLength,
		Body:          body,
		BodyTruncated: truncated,
		IsBinary:      !isTextish(resp.Header.Get("Content-Type")),
		Duration:      duration,
	}
	if loc := resp.Header.Get("Location"); resp.StatusCode >= 300 && resp.StatusCode < 400 && loc != "" {
		hop.RedirectTarget = loc
	}
	return hop, nil
}
