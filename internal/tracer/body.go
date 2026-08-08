package tracer

import (
	"io"
	"strings"
)

// readCapped reads up to cap+1 bytes from r, so it can report whether the
// response body extends beyond the cap without ever buffering more than
// cap+1 bytes regardless of how large the actual response is.
func readCapped(r io.Reader, cap int64) (data []byte, truncated bool, err error) {
	data, err = io.ReadAll(io.LimitReader(r, cap+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > cap {
		return data[:cap], true, nil
	}
	return data, false, nil
}

// isTextish decides whether a Content-Type value denotes body content
// that's safe to print as text. A missing Content-Type is treated as
// text-ish, since plain servers that omit it shouldn't have their body
// hidden by default.
func isTextish(contentType string) bool {
	mediaType := strings.TrimSpace(contentType)
	if mediaType == "" {
		return true
	}
	if idx := strings.Index(mediaType, ";"); idx != -1 {
		mediaType = mediaType[:idx]
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))

	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	if strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	switch mediaType {
	case "application/json", "application/xml", "application/xhtml+xml",
		"application/javascript", "application/x-javascript",
		"application/x-www-form-urlencoded":
		return true
	}
	return false
}
