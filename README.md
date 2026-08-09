# wheregoes

A CLI that traces a URL through its full chain of HTTP redirects, printing
each hop's status and destination until it lands on the final page.

## Install

```sh
go install github.com/terrancekhuang/wheregoes@latest
```

Or build from source:

```sh
git clone https://github.com/terrancekhuang/wheregoes
cd wheregoes
go build
```

## Usage

```sh
wheregoes [--copy] [--verbose] <url>
```

```
$ wheregoes http://github.com
Hop 1: GET http://github.com
  Status: 301 Moved Permanently
  Redirects to: https://github.com/

Hop 2: GET https://github.com/
  Status: 200 OK

Summary:
  Original URL: http://github.com
  Final URL:    https://github.com/
  Total hops:   2
  Total time:   239ms
```

Output is colorized automatically when stdout is a terminal (respecting the
[`NO_COLOR`](https://no-color.org) convention and `TERM=dumb`).

### Flags

| Flag        | Description                                                            |
| ----------- | ------------------------------------------------------------------------ |
| `--verbose` | Also print each hop's timing, response headers, and response body        |
| `--copy`    | Copy the final destination URL to the clipboard                          |

### Exit codes

| Code | Meaning                                    |
| ---- | ------------------------------------------- |
| 0    | Success                                     |
| 1    | Output or clipboard write failure           |
| 2    | Bad arguments                               |
| 3    | Invalid URL                                 |
| 4    | Hop limit exceeded (possible redirect loop) |
| 5    | Network error                               |

## How it works

`wheregoes` sends a `GET` request to the given URL and, instead of letting Go's
HTTP client auto-follow redirects, inspects each `3xx` response itself,
records it as a hop, and follows the `Location` header to the next URL. It
stops at the first non-redirect response (or after 20 hops, to guard against
redirect loops) and reports the full chain.

## Development

```sh
go test ./...
```
