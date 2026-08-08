package tracer

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "already has https scheme", in: "https://example.com/path", want: "https://example.com/path"},
		{name: "already has http scheme", in: "http://example.com", want: "http://example.com"},
		{name: "bare host gets https prepended", in: "example.com", want: "https://example.com"},
		{name: "bare host with path", in: "example.com/path", want: "https://example.com/path"},
		{name: "whitespace trimmed", in: "  example.com  ", want: "https://example.com"},
		{name: "empty", in: "", wantErr: true},
		{name: "whitespace only", in: "   ", wantErr: true},
		{name: "unsupported scheme", in: "ftp://example.com", wantErr: true},
		{name: "missing host", in: "http://", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeURL(%q) = %q, nil; want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeURL(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeURL(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveRedirect(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		loc     string
		want    string
		wantErr bool
	}{
		{name: "absolute location", base: "https://x.com/a", loc: "https://y.com/z", want: "https://y.com/z"},
		{name: "absolute path", base: "https://x.com/a/b", loc: "/c", want: "https://x.com/c"},
		{name: "relative path", base: "https://x.com/a/b", loc: "c", want: "https://x.com/a/c"},
		{name: "unsupported scheme", base: "https://x.com/a", loc: "ftp://y.com/z", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveRedirect(tc.base, tc.loc)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveRedirect(%q, %q) = %q, nil; want error", tc.base, tc.loc, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveRedirect(%q, %q) unexpected error: %v", tc.base, tc.loc, err)
			}
			if got != tc.want {
				t.Fatalf("resolveRedirect(%q, %q) = %q; want %q", tc.base, tc.loc, got, tc.want)
			}
		})
	}
}
