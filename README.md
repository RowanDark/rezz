# v0x

A modern web wordlist generator written in Go. Conceptually similar to CeWL but with headless browser support, structured output formats, and optional authentication.

## Install

```bash
go install github.com/RowanDark/v0x@latest
```

## Browser binary setup (headless mode)

Headless mode requires Chromium managed by playwright-go. Run once after installing:

```bash
playwright install chromium
```

Or via the Makefile:

```bash
make install-browsers
```

## Usage examples

```bash
# Basic crawl — words printed to stdout, one per line (CeWL-compatible)
v0x --url https://target.com

# Headless crawl, depth 3, JSON output to file
v0x --url https://target.com --depth 3 --format json --output wordlist.json

# Form-based login (headless only)
v0x --url https://target.com/dashboard \
    --auth-form-url https://target.com/login \
    --auth-form-user admin --auth-form-pass hunter2

# Cookie injection
v0x --url https://target.com --auth-cookie "session=abc123; csrf=xyz"

# Bearer token
v0x --url https://app.target.com --auth-bearer eyJhbGci...

# Static mode (no headless), markdown report
v0x --url https://target.com --no-headless --format md --output report.md
```

## All flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | _(required)_ | Target URL to start crawling from |
| `--depth` | `2` | Maximum crawl depth |
| `--delay` | `500` | Delay in milliseconds between requests |
| `--min-word-length` | `3` | Minimum word length to collect |
| `--format` | `txt` | Output format: `txt`, `json`, `csv`, `md` |
| `--output` | _(stdout)_ | Output file path |
| `--headless` | `true` | Use headless Chromium via playwright-go |
| `--no-headless` | — | Disable headless; use `net/http` instead |
| `--user-agent` | `v0x/1.0` | Custom User-Agent string |
| `--verbose` | `false` | Log pages visited, word counts, and auth strategy |
| `--auth-form-url` | — | URL of the login form page (headless only) |
| `--auth-form-user` | — | Username to fill in the login form |
| `--auth-form-pass` | — | Password to fill in the login form |
| `--auth-form-user-field` | `username` | `name` attribute of the username input |
| `--auth-form-pass-field` | `password` | `name` attribute of the password input |
| `--auth-form-submit` | `[type=submit]` | CSS selector for the submit button |
| `--auth-basic-user` | — | HTTP Basic auth username |
| `--auth-basic-pass` | — | HTTP Basic auth password |
| `--auth-cookie` | — | Cookie string, e.g. `"session=abc; token=xyz"` |
| `--auth-bearer` | — | Bearer token for the `Authorization` header |
| `--auth-header` | — | Custom header in `"Name: Value"` format |

## Auth examples

### Form-based login (headless only)

Navigates to the login page, fills credentials, submits the form, then crawls the authenticated session.

```bash
v0x --url https://target.com/dashboard \
    --auth-form-url https://target.com/login \
    --auth-form-user admin \
    --auth-form-pass hunter2 \
    --auth-form-user-field email \
    --auth-form-pass-field password \
    --auth-form-submit "button[type=submit]"
```

### HTTP Basic auth

```bash
v0x --url https://target.com \
    --auth-basic-user admin \
    --auth-basic-pass hunter2
```

### Cookie injection

Inject one or more cookies from an existing authenticated session.

```bash
v0x --url https://target.com \
    --auth-cookie "session=abc123; csrf=xyz789"
```

### Bearer token

```bash
v0x --url https://app.target.com \
    --auth-bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Custom header

```bash
v0x --url https://app.target.com \
    --auth-header "X-Api-Key: my-secret-key"
```

## Output formats

| Format | Description |
|--------|-------------|
| `txt` | One word per line, sorted — compatible with tools that consume CeWL output |
| `json` | Structured JSON with words, emails, metadata, and crawl stats |
| `csv` | Word, length, and source columns |
| `md` | Human-readable Markdown report with wordlist, email, and metadata sections |

## Build from source

```bash
git clone https://github.com/RowanDark/v0x
cd v0x
make build
./v0x --url https://example.com
```

## Testing

```bash
# Unit tests
make test

# Integration smoke test (spins up an in-process HTTP server)
make integration
```

## Project layout

```
v0x/
├── cmd/
│   ├── root.go              # Cobra root command, flag binding, crawl pipeline
│   └── integration_test.go  # Integration smoke test (build tag: integration)
├── internal/
│   ├── auth/                # Auth strategies: form, basic, cookie, bearer
│   ├── config/              # Config struct
│   ├── crawler/             # Crawler interface, playwright + net/http impls
│   ├── extractor/           # Word, email, and metadata extraction
│   └── output/              # Output formatters: txt, json, csv, md
├── main.go
├── go.mod
└── Makefile
```
