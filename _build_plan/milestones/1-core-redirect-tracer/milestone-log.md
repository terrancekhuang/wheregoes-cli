# Milestone 1 — Core redirect tracer

## What's new in the app

- A new command-line tool, `wheregoes`, that takes any URL and traces it through every HTTP redirect until it reaches its final destination
- For every hop along the way, it shows the requested URL, status code, all response headers, the response body, and how long that hop took
- A summary at the end shows the original URL, the final destination, total number of hops, and total time for the whole trace
- If a link redirects in a loop or an unusually long chain, tracing stops safely after 20 redirects with a clear message instead of hanging forever
- If you give it a bad or unreachable URL (typo, no such domain, connection refused, etc.), it prints a plain-English error instead of crashing
- Very large response bodies are cut off in the display (after 4KB) with a note that they were truncated, so huge pages don't flood the terminal
- Non-text responses (images, binary files, etc.) are shown as a placeholder ("binary content, N bytes not shown") instead of dumping garbage bytes to the terminal
- Bare URLs typed without `http://`/`https://` (e.g. `wheregoes example.com`) are handled automatically by assuming `https://`

Output is plain text only in this milestone — no color yet (that's milestone 2), and no clipboard/URL-cleaning support yet (milestone 3).

## What was built

**Module**: `github.com/terrancekhuang/wheregoes`, Go 1.23.

```
wheregoes/
├── go.mod
├── main.go                       CLI entrypoint
├── main_test.go
├── internal/
│   ├── tracer/                   redirect-following logic, no I/O beyond HTTP
│   │   ├── tracer.go              Trace/Hop/Options structs, Run(), fetchHop()
│   │   ├── url.go                 normalizeURL(), resolveRedirect()
│   │   ├── body.go                readCapped(), isTextish()
│   │   ├── errors.go              TraceError, Kind enum, describeNetErr()
│   │   └── *_test.go
│   └── render/
│       ├── text.go                Text(w io.Writer, t *tracer.Trace) error
│       └── text_test.go
└── .gitignore
```

**Core library** (`internal/tracer`):
- `tracer.Run(ctx, rawURL, opts) (*Trace, error)` — the main entry point. Uses an `*http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse` so each hop's raw response (headers, body, status) can be captured before deciding whether to follow it.
- A hop is a redirect when status is 3xx **and** a `Location` header is present; relative `Location` values are resolved against the current request URL via `net/url`'s `ResolveReference`.
- Always uses GET (the data model requires body content, not just headers).
- Defaults: `DefaultMaxHops = 20`, `DefaultBodyCap = 4096` bytes, `DefaultTimeout = 15s` per hop, `DefaultUserAgent = "wheregoes-cli/0.1"`. These are exported constants, not flags, in this milestone — kept as named values so a later milestone can expose them as CLI flags without restructuring.
- `Options.Client` is an injection seam: tests supply a custom client/`RoundTripper` for hermetic `httptest`-based testing without hitting the network.
- Errors are wrapped in a single `TraceError{Kind, URL, MaxHops, Err}` type with `Kind` one of `KindInvalidURL`, `KindHopLimit`, `KindNetwork`. `describeNetErr` turns DNS failures, connection refused, TLS errors, and timeouts into readable one-liners.

**Rendering** (`internal/render`): `Text(w, trace)` formats the trace as plain text — one block per hop (status, time, redirect target, sorted headers, body or truncation/binary note), then a summary block. Headers are sorted alphabetically since Go's `http.Header` iteration order is undefined otherwise, and unsorted output would also make tests non-deterministic.

**CLI** (`main.go`): a `run(args, stdout, stderr) int` function is factored out of `main()` so tests can exercise argument parsing and exit codes directly, without subprocesses. Uses stdlib `flag.NewFlagSet` for the single positional URL argument — no CLI framework dependency needed yet.

**Exit codes**: `0` success (including chains that end in 404/500 — the tool traces, it doesn't judge the destination), `2` usage error, `3` invalid/malformed URL, `4` hop limit exceeded, `5` network failure (DNS/refused/TLS/timeout), `1` fallback for anything unexpected.

**Tests**: all hermetic via `net/http/httptest`, no live network dependency. Covers multi-hop chains, hop-limit-exceeded (self-redirecting loop terminates immediately, doesn't hang), malformed URLs short-circuiting before any HTTP call (verified via a `RoundTripper` that fails the test if invoked), connection failures, body truncation, binary content detection, relative `Location` resolution, and the CLI's argument handling / exit codes. `go build`, `go vet`, and `gofmt -l` are all clean.

**Manual verification**: ran against a real multi-hop redirect chain (`httpbin.org/redirect/3`, including relative `Location` headers) and confirmed correct hop-by-hop output and final destination; confirmed malformed-URL and DNS-failure inputs produce clean one-line errors with no stack trace and no hang; confirmed piped output (`| cat`) has no escape-sequence artifacts (expected, since color isn't implemented yet).

## Decisions made during implementation (not pre-specified in the PRD)

- **Function naming collision**: the PRD's data model names the top-level entity "Trace." Since Go doesn't allow a type and a function of the same name in one package, the struct is `tracer.Trace` and the entry-point function is `tracer.Run(...)` (not `tracer.Trace(...)`).
- **Package split**: `internal/tracer` (pure logic) and `internal/render` (formatting) — not specified in the PRD, but chosen so milestone 2's colorized renderer is a new file in `internal/render` rather than a rewrite of the tracing logic.
- **Empty/missing `Content-Type`** is treated as text-ish by default (body gets printed rather than hidden behind the binary placeholder), on the theory that plain servers omitting the header shouldn't have their body hidden.
- **Hop-limit semantics**: `MaxHops` counts *redirects followed*, not total requests — the loop allows up to `MaxHops + 1` total requests (1 initial + up to 20 redirects) before erroring.
- **Body truncation** uses `io.LimitReader(r, cap+1)` so the exact captured length is either the true (short) body length or exactly the cap — no response is ever buffered beyond `cap+1` bytes regardless of actual size.
- **Exit code scheme** (0/1/2/3/4/5 mapped to distinct failure kinds) goes beyond the PRD's literal wording ("a clear error is shown") but is a small, standard Unix practice that costs little and gives future scripting/milestone-3 work meaningful codes to branch on.
- **`.gitignore`** added to exclude the harness's `.claude/` state directory and the pre-existing `.impeccable/` tooling config from version control, plus standard Go build artifacts — neither is part of the application.

## Notes for milestone 2 (colorized output polish)

- Milestone 2 should add a new renderer (e.g. `internal/render/color.go`) implementing the same shape as `Text` — it does not need to touch `internal/tracer` at all.
- `Hop` already carries everything needed for color-coding: `StatusCode` (for green/yellow/red), `IsBinary`/`BodyTruncated` (for dimmed/labeled sections), `RedirectTarget` (for highlighting).
- For "degrades gracefully to plain text when color isn't supported," check `os.Stdout` with something like `golang.org/x/term.IsTerminal` or inspect `NO_COLOR`/`TERM=dumb`; `main.go`'s `run()` already takes `stdout io.Writer` so the color-vs-plain decision can be made once in `main()` before calling into `render`, without changing the renderer's function signature shape.
- No new dependencies were added in milestone 1 (stdlib only) — if milestone 2 wants a color library (e.g. `fatih/color`), that will be the first external dependency; `go.sum` doesn't exist yet.

## Deviations from the PRD

None of substance — the naming collision (`Trace` struct vs. function) and the package-split choice above are implementation details, not scope changes. All "what gets built" and "done when" items for milestone 1 are implemented and verified.
