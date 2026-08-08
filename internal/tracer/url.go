package tracer

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// normalizeURL validates a user-supplied starting URL, auto-prepending
// "https://" when no scheme was given (e.g. "example.com" -> "https://example.com").
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("URL is empty")
	}

	candidate := raw
	if !strings.Contains(raw, "://") {
		candidate = "https://" + raw
	}

	u, err := url.Parse(candidate)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q (only http and https are supported)", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("missing host")
	}
	return u.String(), nil
}

// resolveRedirect resolves a Location header value (which may be relative)
// against the URL that produced it.
func resolveRedirect(baseURL, location string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return "", fmt.Errorf("unsupported redirect scheme %q", resolved.Scheme)
	}
	return resolved.String(), nil
}
