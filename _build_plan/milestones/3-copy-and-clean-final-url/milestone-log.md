# Milestone 3 — Copy final URL

## What's new in the app

- A new `--copy` flag copies the final destination URL to your system clipboard after a trace finishes
- A confirmation line (`Copied to clipboard: <url>`) prints after a successful copy, so you know it worked
- If the clipboard isn't available (e.g. no clipboard utility installed, headless environment), the trace still completes and prints normally — you just get a warning instead of a silent failure or a crash

Note: the PRD's `--clean` flag (stripping tracking/marketing query parameters) was built, then removed — see "Deviations from the PRD" below.

## What was built

**Modified** (`main.go`):
- Added a `--copy` bool flag to the existing `flag.NewFlagSet`, plus an expanded `fs.Usage` that lists it via `fs.PrintDefaults()`.
- After rendering, if `--copy` is set, `result.FinalURL` is written to the clipboard via `github.com/atotto/clipboard`. Success prints a confirmation to stdout; failure prints a warning to stderr and the run still exits `0`, since the trace itself succeeded and that remains the tool's primary job.
- The clipboard call is routed through a package-level `var copyToClipboard = clipboard.WriteAll`, mirroring the existing `isTerminal`/`shouldUseColor` injection-seam pattern from milestone 2, so tests can swap in a fake writer and assert both the success and failure paths without touching a real OS clipboard.

**Modified** (`main_test.go`): added a `withFakeClipboard` test helper plus `TestRun_CopyFlagSuccessPrintsConfirmation` and `TestRun_CopyFlagFailureWarnsButKeepsExitZero`.

**Dependency**: `github.com/atotto/clipboard` (v0.1.4) added via `go get` + `go mod tidy` — the second external dependency after `golang.org/x/term` from milestone 2. Pure Go, no cgo; shells out to `xclip`/`xsel`/`wl-copy` on Linux, `pbcopy` on macOS, and Win32 syscalls on Windows, which keeps `go install` working without extra build requirements.

**Manual verification**: ran the built binary with `--copy` against a local test server and confirmed the confirmation line and (via a fake clipboard in tests) the correct URL is passed through. `go build ./...`, `go vet ./...`, `gofmt -l .` all clean; `go test ./...` passes, including all milestone 1/2 tests unmodified. Confirmed `--clean` is fully gone: `wheregoes --clean <url>` now fails with `flag provided but not defined: -clean` and exit code `2`, same as any other unknown flag.

Note: this sandbox's X11 environment has `xclip`/`wl-copy` binaries but no running clipboard manager, so a real paste-back (`xclip -o`) after the process exits couldn't be confirmed end-to-end here — that's an X11 clipboard-ownership property of this environment (the writer process must stay alive unless a clipboard manager takes over the selection), not a defect in the code. The Go-side write call itself reported success, and the `--copy` wiring is fully covered by hermetic unit tests using a fake clipboard writer that don't depend on any real clipboard being present.

## Decisions made during implementation (not pre-specified in the PRD)

- **Clipboard failure is a warning, not an error** (confirmed with the user before building) — `--copy`'s job is a nice-to-have on top of a trace that already succeeded and printed; failing the whole invocation over an unavailable clipboard utility would be a worse outcome than a warning. Exit code stays `0`; no new exit code was added for this case.
- **`github.com/atotto/clipboard` over `golang.design/x/clipboard`** (confirmed with the user before building) — the latter needs cgo and X11 dev headers to build on Linux, which would complicate `go install`ing this CLI; `atotto/clipboard` is pure Go and shells out to whatever clipboard tool is already on the system.

## Deviations from the PRD

**`--clean` was built, then removed — this is the one substantive deviation from the PRD in this milestone.** A `--clean` flag and an `internal/cleanurl` package were implemented first, matching the PRD's description (strip `utm_*`/`fbclid`/`gclid`/etc. from the final URL using "a fixed built-in list," not configurable). Manual testing against a real, live `google.com/aclk` ad-click redirect (supplied by the user) showed the initial fixed list missed several legitimate Google Ads tracking params (`gbraid`, `gad_source`, `gad_campaignid`) alongside the ones it did catch. Expanding the list to close that gap raised a broader question: a fixed enumerated list can only ever cover the vendor tracking schemes someone thought to add, and there's no reliable syntactic pattern that distinguishes a tracking parameter from a legitimate content parameter (a suffix like `clid` generalizes for one family of click-IDs, but doesn't help for `mc_cid`, `_hsenc`, `vero_id`, `ref`, or whatever the next vendor calls their tracking param) — so "done properly" would mean either an actively-maintained blocklist (like browser privacy extensions ship, and update regularly) or accepting a leaky, best-effort list forever. The user decided that tradeoff was more complexity/maintenance burden than this v1 CLI should take on, and asked to cut `--clean` entirely rather than ship it half-solved. `--copy` (the other half of this milestone) was unaffected and is complete as scoped.
