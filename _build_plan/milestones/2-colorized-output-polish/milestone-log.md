# Milestone 2 — Colorized output polish

## What's new in the app

- Trace output is now color-coded when printed to a real terminal: status codes are green for success (2xx), yellow for redirects (3xx), and red for errors (4xx/5xx)
- Each hop is clearly labeled ("Hop 1", "Hop 2", ...) in bold so the chain reads top-to-bottom at a glance
- Headers and response bodies are dimmed and set apart from the status/timing lines so the eye can quickly find the parts that matter
- The final destination is called out in its own bordered, highlighted block right after the trace, before the summary — no more hunting for it in a wall of text
- Output automatically drops all color when it isn't appropriate: piping to a file, redirecting into another program, running with `TERM=dumb`, or setting the `NO_COLOR` environment variable all produce clean plain text with no stray escape codes
- No new flags to learn — coloring turns on and off automatically based on where the output is going

## What was built

**New files** (`internal/render`):
- `style.go` — ANSI escape constants (`ansiReset`, `ansiBold`, `ansiDim`, `ansiGreen`, `ansiYellow`, `ansiRed`, `ansiCyan`) and a `style` struct of string-wrapping functions (`bold`, `dim`, `cyan`, `statusColor`, plus a `showFinalCallout` flag). Two `style` values are defined: `plainStyle` (all identity functions) and `colorStyle` (ANSI-wrapping functions, `showFinalCallout: true`). Also holds `statusLabel(code int) string`, moved here from `text.go` since both styles use it.
- `color.go` — `Color(w io.Writer, t *tracer.Trace) error`, a one-line wrapper: `return render(w, t, colorStyle)`.
- `color_test.go` — asserts status-code color buckets (2xx/3xx/4xx/5xx → correct ANSI color), reset codes are present, the final-destination callout contains "Final destination" and the actual final URL, and that all the structural content (hop numbering, headers, summary) still appears once color-wrapped.

**Modified**:
- `text.go` — the old `Text` function body was extracted into an unexported `render(w io.Writer, t *tracer.Trace, s style) error`, used by both `Text` (→ `plainStyle`) and `Color` (→ `colorStyle`). `writeHop`, `writeBody`, and `writeSummary` now take a `style` parameter and wrap the relevant substrings through it instead of writing raw text. A new `writeFinalCallout` prints a bordered "✓ Final destination" block, gated by `s.showFinalCallout` so plain-text output is byte-for-byte unchanged from milestone 1 (verified: all of milestone 1's `text_test.go` assertions pass unmodified against the refactored code).
- `main.go` — added `isTerminal(w io.Writer) bool` (type-asserts to `*os.File`, then calls `term.IsTerminal`; always `false` for non-file writers like `bytes.Buffer`, which is what keeps `run()` deterministic in tests) and `shouldUseColor(lookupEnv func(string) (string, bool), terminal bool) bool` (checks `NO_COLOR` presence, then `TERM=dumb`, then falls back to the terminal check). `run()` now picks `render.Color` or `render.Text` based on `shouldUseColor(os.LookupEnv, isTerminal(stdout))` before writing output — the renderer functions themselves stay pure and side-effect-free.
- `main_test.go` — added `TestShouldUseColor` (table test covering: no env + non-terminal, no env + terminal, `NO_COLOR` set to a value, `NO_COLOR` set empty, `TERM=dumb`, `TERM` set to something else) and `TestIsTerminal_NonFileWriterIsNeverATerminal`. Extended `TestRun_SuccessPrintsTraceToStdout` to assert no `\x1b[` sequences appear when stdout is a `bytes.Buffer`.
- `go.mod` / `go.sum` — added `golang.org/x/term` (first external dependency); `go mod tidy` bumped the `go` directive from `1.23` to `1.25.0` because `x/term`'s own `go.mod` requires it. `golang.org/x/sys` came along as `x/term`'s indirect dependency.

**Manual verification**: ran the built binary under a real pty (`script`) against `https://httpbin.org/redirect/2` and visually confirmed green/yellow/red status coloring, dimmed headers/body, bold hop headers, and the final-destination callout box render correctly; ran the same command piped to a file and confirmed zero `\x1b` bytes in the output; ran with `NO_COLOR=1` under the same real pty and confirmed it still produced plain text despite being an actual terminal. `go build ./...`, `go vet ./...`, and `gofmt -l .` are all clean; `go test ./...` passes, including all of milestone 1's tests unmodified.

## Decisions made during implementation (not pre-specified in the PRD)

- **Shared renderer, not two independent implementations.** The PRD/milestone-1 log suggested `Text` and `Color` as two same-shaped functions, but implementing them as fully separate functions would duplicate all the hop/header/body/truncation-note assembly logic and let plain and color output drift apart over time. Instead both are thin wrappers over one unexported `render(w, t, style)`, parameterized by a `style` value. `Text` and `Color` keep the exact same public signature (`func(io.Writer, *tracer.Trace) error`) the milestone-1 log called for.
- **Hand-rolled ANSI codes instead of a color library** (confirmed with the user before building) — a handful of `bold`/`dim`/`green`/`yellow`/`red`/`cyan`/`reset` constants covers everything this milestone needs, and avoids taking on a runtime dependency (e.g. `fatih/color`) for something this small. `golang.org/x/term` is used only for the isatty check, not for coloring itself.
- **Final-destination callout as a distinct bordered block** rather than just bolding the "Final URL:" line in the existing Summary (confirmed with the user) — reads as a clearer "here's your answer" moment. This block only appears in color mode (`style.showFinalCallout`); plain-text output is intentionally unchanged from milestone 1 so existing scripts/tests parsing it aren't affected.
- **No `--color`/`--no-color` flags added.** The PRD's "Done when" criteria for this milestone only describes automatic detection (terminal vs. piped), and flags weren't listed under "what gets built" — `NO_COLOR` and `TERM=dumb` are the only manual overrides, per the existing environment-variable convention.
- **`NO_COLOR` presence, not value, disables color** — following the https://no-color.org convention, `shouldUseColor` uses `os.LookupEnv`'s second return value (whether the key is set at all) rather than checking for a truthy value, so `NO_COLOR=""` also disables color.
- **Color decision lives in `main.go`, not `internal/render`** — matches the milestone-1 log's note that `run()`'s `stdout io.Writer` parameter lets this be decided once before calling into the renderer. `isTerminal` type-asserts the writer to `*os.File`; any other writer (tests, pipes represented as buffers) is treated as non-interactive.

## Notes for milestone 3 (copy & clean final URL)

- `tracer.Trace.FinalURL` is the field milestone 3's `--copy`/`--clean` flags will operate on — already plumbed through unchanged from milestone 1.
- `main.go`'s `run()` is the natural place to add the `--copy`/`--clean` flag parsing (alongside the existing `flag.NewFlagSet` usage) and to print a copy confirmation message after rendering.
- This is the second external dependency point: milestone 3 will need a clipboard library (there's no clipboard access in the stdlib). `go.sum` already exists now, so adding one is a normal `go get`, not a special first-dependency event.
- The color vs. plain rendering split doesn't affect milestone 3 — clipboard/clean logic operates on `trace.FinalURL` as a plain string, independent of which renderer printed the trace.

## Deviations from the PRD

None of substance. The `go` directive bump to `1.25.0` (from `go get`/`go mod tidy` satisfying `golang.org/x/term`'s own requirement) and the internal `style`-based refactor of `text.go` are implementation details, not scope changes. All "what gets built" and "done when" items for milestone 2 are implemented and verified.
