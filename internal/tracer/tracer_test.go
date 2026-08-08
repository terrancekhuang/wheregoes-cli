package tracer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testClient builds an *http.Client suitable for injection via
// Options.Client: it must not auto-follow redirects, matching what
// buildClient would construct in production.
func testClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func testOptions() Options {
	opts := DefaultOptions()
	opts.Client = testClient()
	return opts
}

func TestRun_MultiHopEndsIn200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/b", http.StatusFound)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/c", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final destination"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	trace, err := Run(context.Background(), server.URL+"/a", testOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.HopCount != 3 {
		t.Fatalf("HopCount = %d; want 3", trace.HopCount)
	}
	if trace.FinalURL != server.URL+"/c" {
		t.Fatalf("FinalURL = %q; want %q", trace.FinalURL, server.URL+"/c")
	}
	if trace.Hops[0].StatusCode != http.StatusFound {
		t.Fatalf("Hops[0].StatusCode = %d; want %d", trace.Hops[0].StatusCode, http.StatusFound)
	}
	if trace.Hops[1].StatusCode != http.StatusMovedPermanently {
		t.Fatalf("Hops[1].StatusCode = %d; want %d", trace.Hops[1].StatusCode, http.StatusMovedPermanently)
	}
	if trace.Hops[2].StatusCode != http.StatusOK {
		t.Fatalf("Hops[2].StatusCode = %d; want %d", trace.Hops[2].StatusCode, http.StatusOK)
	}
	if string(trace.Hops[2].Body) != "final destination" {
		t.Fatalf("Hops[2].Body = %q; want %q", trace.Hops[2].Body, "final destination")
	}
	if trace.TotalTime <= 0 {
		t.Fatalf("TotalTime = %v; want > 0", trace.TotalTime)
	}
}

func TestRun_HopLimitExceeded(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	opts := testOptions()
	opts.MaxHops = 3

	_, err := Run(context.Background(), server.URL+"/loop", opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var traceErr *TraceError
	if !errors.As(err, &traceErr) {
		t.Fatalf("expected *TraceError, got %T: %v", err, err)
	}
	if traceErr.Kind != KindHopLimit {
		t.Fatalf("Kind = %v; want KindHopLimit", traceErr.Kind)
	}
}

type failIfCalledTransport struct{ t *testing.T }

func (f failIfCalledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Fatalf("unexpected HTTP request for malformed URL: %s", req.URL)
	return nil, nil
}

func TestRun_MalformedURLNoRequestMade(t *testing.T) {
	opts := DefaultOptions()
	opts.Client = &http.Client{Transport: failIfCalledTransport{t}}

	_, err := Run(context.Background(), "://not-a-url", opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var traceErr *TraceError
	if !errors.As(err, &traceErr) {
		t.Fatalf("expected *TraceError, got %T: %v", err, err)
	}
	if traceErr.Kind != KindInvalidURL {
		t.Fatalf("Kind = %v; want KindInvalidURL", traceErr.Kind)
	}
}

func TestRun_ConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := server.URL
	server.Close() // nothing listens on this address anymore

	_, err := Run(context.Background(), deadURL, testOptions())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var traceErr *TraceError
	if !errors.As(err, &traceErr) {
		t.Fatalf("expected *TraceError, got %T: %v", err, err)
	}
	if traceErr.Kind != KindNetwork {
		t.Fatalf("Kind = %v; want KindNetwork", traceErr.Kind)
	}
}

func TestRun_LargeBodyTruncated(t *testing.T) {
	body := strings.Repeat("x", 10000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body))
	}))
	defer server.Close()

	opts := testOptions()
	opts.BodyCap = 100

	trace, err := Run(context.Background(), server.URL, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hop := trace.Hops[0]
	if !hop.BodyTruncated {
		t.Fatalf("expected BodyTruncated=true")
	}
	if len(hop.Body) != 100 {
		t.Fatalf("len(Body) = %d; want 100", len(hop.Body))
	}
}

func TestRun_BinaryContentDetected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte{0x00, 0x01, 0x02, 0x03})
	}))
	defer server.Close()

	trace, err := Run(context.Background(), server.URL, testOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trace.Hops[0].IsBinary {
		t.Fatalf("expected IsBinary=true")
	}
}

func TestRun_RelativeLocationResolution(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/dir/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "next") // relative, no leading slash
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/dir/next", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	trace, err := Run(context.Background(), server.URL+"/dir/start", testOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := server.URL + "/dir/next"
	if trace.FinalURL != want {
		t.Fatalf("FinalURL = %q; want %q", trace.FinalURL, want)
	}
}
