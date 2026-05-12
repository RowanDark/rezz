# v0x

A modern web wordlist generator written in Go. Conceptually similar to CeWL but with headless browser support, structured output formats, and optional authentication.

## Usage

```
v0x --url <target> [flags]

Flags:
      --delay int             Delay in ms between requests (default 500)
      --depth int             Max crawl depth (default 2)
      --format string         Output format: txt, json, csv, md (default "txt")
      --headless              Use headless browser via playwright-go (default true)
      --min-word-length int   Minimum word length to collect (default 3)
      --no-headless           Disable headless, use net/http instead
      --output string         Output file path (default: stdout)
      --url string            Target URL (required)
      --user-agent string     Custom User-Agent string (default "v0x/1.0")
      --verbose               Verbose logging
```

## Build

```bash
go build -o v0x .
```

## Browser setup (headless mode)

Headless mode requires Chromium binaries managed by playwright-go.
Run once after cloning:

```bash
make install-browsers
```

or manually:

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
```

## Project layout

```
v0x/
├── cmd/
│   └── root.go          # cobra root command, flag binding
├── internal/
│   ├── crawler/         # crawl logic
│   ├── auth/            # auth strategies
│   ├── extractor/       # word + email extraction
│   ├── output/          # output formatters
│   └── config/          # config struct, flag → config mapping
├── main.go
└── go.mod
```
