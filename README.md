# rezz

┌─────────────────────────────────────┐
│  ██████╗ ███████╗███████╗███████╗   │
│  ██╔══██╗██╔════╝╚══███╔╝╚══███╔╝   │
│  ██████╔╝█████╗    ███╔╝   ███╔╝    │
│  ██╔══██╗██╔══╝   ███╔╝   ███╔╝     │
│  ██║  ██║███████╗███████╗███████╗   │
│  ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝   │
│  secret scanner  v1.0.0             │
│  github.com/RowanDark/rezz          │
└─────────────────────────────────────┘

A web crawler and secret scanner for authorized security testing. Crawls target sites using either a headless Chromium browser or a lightweight HTTP client, scans discovered pages and scripts for secrets and sensitive data using embedded regex pattern kits, and maps all reachable endpoints.

> **Legal notice**: rezz is intended for authorized security testing only. Do not use against systems you do not have permission to test.

## Install

```bash
go install github.com/RowanDark/rezz@latest
```

### Browser setup (headless mode)

Headless mode requires Chromium managed by playwright-go. Run once after installing:

```bash
make install-browsers
```

Or directly:

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
```

## Quick start

```bash
# Scan for secrets with default kits (api-keys, credentials, headers)
rezz --url https://target.example.com

# Static HTTP scan (no browser), all pattern kits, JSON output
rezz --url https://target.example.com --no-headless --patterns all --format json --output findings.json

# Map all endpoints on a target
rezz map --url https://target.example.com --format json --output endpoints.json

# Scan behind a login form (headless only)
rezz --url https://target.example.com/dashboard \
    --auth-form-url https://target.example.com/login \
    --auth-form-user admin \
    --auth-form-pass hunter2

# Scan with cookie injection, verbose, respecting robots.txt
rezz --url https://target.example.com \
    --auth-cookie "session=abc123; csrf=xyz" \
    --verbose
```

## Commands

### `rezz` — secret scanner

Crawls a target and scans every page, stylesheet, and script for secrets matching the loaded pattern kits.

```bash
rezz [flags]
```

### `rezz map` — endpoint mapper

Crawls a target and classifies all discovered URLs by type: crawled pages, forms, JS endpoints, scripts, assets, out-of-scope links, and sensitive paths.

```bash
rezz map [flags]
```

## Flag reference

### scan (`rezz`)

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(required)* | Target URL to start crawling from |
| `--depth` | `2` | Maximum crawl depth |
| `--delay` | `500` | Delay in ms between requests |
| `--jitter` | `0.5` | Random jitter factor applied to `--delay` (0 = none, 1.0 = up to 2× delay) |
| `--timeout` | `5m` | Max crawl duration; `0` disables the timeout |
| `--user-agent` | `rezz/1.0` | Custom User-Agent string |
| `--headless` | `true` | Use headless Chromium via playwright-go |
| `--no-headless` | — | Disable headless; use `net/http` instead |
| `--no-robots` | `false` | Skip robots.txt fetching and checking |
| `--patterns` | `api-keys,credentials,headers` | Pattern kits to load, comma-separated |
| `--custom` | — | Path to a custom patterns YAML file |
| `--format` | `stream` | Output format: `stream`, `json`, `csv` |
| `--output` | *(stdout)* | Output file path |
| `--dedup-global` | `false` | Deduplicate findings globally — each unique match shown once across all pages |
| `--no-dedup` | `false` | Disable all deduplication — emit every match raw |
| `--scope` | — | Comma-separated in-scope hosts; base domains expand to subdomains |
| `--strict-scope` | `false` | Disable subdomain expansion; all scope entries match exact host only |
| `--verbose` | `false` | Log pages visited, script fetches, robots.txt activity |
| `--quiet` | `false` | Suppress banner and non-essential output |
| `--summary` | `false` | Print severity breakdown after scan completes |
| `--no-color` | `false` | Disable ANSI color output |
| `--color` | `false` | Force ANSI color even when stdout is not a terminal |
| `--auth-form-url` | — | URL of the login form page (headless only) |
| `--auth-form-user` | — | Username to fill in the login form |
| `--auth-form-pass` | — | Password to fill in the login form |
| `--auth-form-user-field` | `username` | `name` attribute of the username input |
| `--auth-form-pass-field` | `password` | `name` attribute of the password input |
| `--auth-form-submit` | `[type=submit]` | CSS selector for the submit button |
| `--auth-verify-selector` | — | CSS selector that must be present after login to confirm success |
| `--auth-basic-user` | — | HTTP Basic auth username |
| `--auth-basic-pass` | — | HTTP Basic auth password |
| `--auth-cookie` | — | Cookie string, e.g. `"session=abc; token=xyz"` |
| `--auth-bearer` | — | Bearer token for the `Authorization` header |
| `--auth-header` | — | Custom header in `"Name: Value"` format |

### map (`rezz map`)

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(required)* | Target URL to start crawling from |
| `--depth` | `2` | Maximum crawl depth |
| `--delay` | `500` | Delay in ms between requests |
| `--jitter` | `0.5` | Random jitter factor applied to `--delay` |
| `--timeout` | `5m` | Max crawl duration; `0` disables the timeout |
| `--user-agent` | `rezz/1.0` | Custom User-Agent string |
| `--headless` | `true` | Use headless Chromium via playwright-go |
| `--no-headless` | — | Disable headless; use `net/http` instead |
| `--no-robots` | `false` | Skip robots.txt fetching and checking |
| `--patterns` | `endpoints,javascript` | Pattern kits for endpoint classification |
| `--custom` | — | Path to a custom patterns YAML file |
| `--scope` | — | Comma-separated in-scope hosts |
| `--strict-scope` | `false` | Exact host matching only |
| `--format` | `json` | Output format: `stream`, `json` |
| `--output` | *(stdout)* | Output file path |
| `--verbose` | `false` | Log each mapped page and endpoint count |
| `--quiet` | `false` | Suppress banner |
| `--no-color` | `false` | Disable ANSI color output |
| `--color` | `false` | Force ANSI color output |
| `--auth-form-url` | — | Login form URL |
| `--auth-form-user` | — | Login username |
| `--auth-form-pass` | — | Login password |
| `--auth-cookie` | — | Cookie string to inject |
| `--auth-bearer` | — | Bearer token |
| `--auth-header` | — | Custom auth header |

## Pattern kits

| Kit | Description | Example patterns | Included by default |
|-----|-------------|------------------|---------------------|
| `api-keys` | API keys and tokens | AWS access keys, GitHub tokens, Stripe keys | ✓ |
| `credentials` | Credentials and secrets | Passwords in HTML, secret fields, env vars | ✓ |
| `headers` | HTTP response header leaks | Server banners, X-Powered-By, debug headers | ✓ |
| `financial` | Payment card and account data | Visa/Mastercard PANs, IBAN, SSN, Bitcoin addresses | — |
| `endpoints` | API and admin endpoint paths | REST paths, admin panels, config endpoints | — |
| `javascript` | JS-embedded secrets | Hardcoded tokens and keys in JS bundles | — |
| `cloud` | Cloud provider credentials | AWS, GCP, Azure tokens and ARNs | — |

Load multiple kits with `--patterns api-keys,financial,cloud` or all kits with `--patterns all`.

## Custom patterns

Create a YAML file matching the embedded kit format:

```yaml
kit: my-patterns
patterns:
  - name: Internal API Key
    regex: 'internal_key_[a-zA-Z0-9]{32}'
    category: API Key
    severity: high

  - name: Legacy Session Token
    regex: 'LEGACYSESSION=[a-f0-9]{40}'
    category: Credentials
    severity: medium
```

Load with `--custom /path/to/custom.yaml`. Custom patterns are added on top of any kit patterns. Patterns that fail to compile are skipped with a warning — the scan continues with the valid ones.

## Scope control

By default rezz crawls only the exact host of `--url`. Use `--scope` to expand or restrict the crawl:

```bash
# Also crawl subdomains of paylution.com and hyperwallet.com
rezz --url https://www.paylution.com \
    --scope paylution.com,hyperwallet.com

# Exact-host only — no subdomain expansion
rezz --url https://app.example.com \
    --scope app.example.com \
    --strict-scope
```

A scope entry containing a single dot (e.g. `example.com`) expands to all subdomains unless `--strict-scope` is set. A multi-dot entry (e.g. `app.example.com`) matches only that exact host.

## robots.txt

By default rezz fetches `/robots.txt` from the target host at the start of the crawl and skips any paths disallowed for `User-agent: *`. Use `--no-robots` to skip robots.txt enforcement entirely:

```bash
rezz --url https://target.example.com --no-robots --verbose
```

With `--verbose`, skipped URLs are logged to stderr:

```
rezz: skip https://target.example.com/admin (robots.txt)
```

## Auth strategies

### Form-based login (headless only)

Navigates to the login page, fills credentials, submits the form, then crawls the authenticated session.

```bash
rezz --url https://target.example.com/dashboard \
    --auth-form-url https://target.example.com/login \
    --auth-form-user admin \
    --auth-form-pass hunter2 \
    --auth-form-user-field email \
    --auth-form-pass-field password \
    --auth-form-submit "button[type=submit]"
```

Use `--auth-verify-selector` to confirm the login succeeded before crawling (recommended when SSO redirects to a different domain):

```bash
rezz --url https://target.example.com/dashboard \
    --auth-form-url https://target.example.com/login \
    --auth-form-user admin \
    --auth-form-pass hunter2 \
    --auth-verify-selector "#account-menu"
```

### HTTP Basic auth

```bash
rezz --url https://target.example.com \
    --auth-basic-user admin \
    --auth-basic-pass hunter2
```

### Cookie injection

```bash
rezz --url https://target.example.com \
    --auth-cookie "session=abc123; csrf=xyz789"
```

### Bearer token

```bash
rezz --url https://api.target.example.com \
    --auth-bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Custom header

```bash
rezz --url https://api.target.example.com \
    --auth-header "X-Api-Key: my-secret-key"
```

## Output formats

### scan

| Format | Description |
|--------|-------------|
| `stream` | Findings printed as they are discovered (default) |
| `json` | Structured JSON with target, pages crawled, and findings array |
| `csv` | CSV with columns: url, pattern, category, severity, match, context, status_code, from_js |

### map

| Format | Description |
|--------|-------------|
| `json` | Structured JSON with target, pages_crawled, total_found, and endpoints array (default) |
| `stream` | Endpoint lines printed as they are discovered |

## Deduplication

By default rezz deduplicates by `pattern + match + URL` — the same secret on the same page is emitted once. Two additional modes are available:

| Flag | Behaviour |
|------|-----------|
| *(default)* | Deduplicate by pattern + match + URL |
| `--dedup-global` | Deduplicate by pattern + match across all pages; shows "Also found on N other page(s)" |
| `--no-dedup` | Emit every raw match — no deduplication |

## Build from source

```bash
git clone https://github.com/RowanDark/rezz
cd rezz
make build
./rezz --url https://example.com
```

## Testing

```bash
# Unit and integration tests
make test

# Integration smoke test only
make integration
```

## Project layout

```
rezz/
├── cmd/
│   ├── root.go              # scan command, flag binding, crawl pipeline
│   ├── map.go               # map subcommand
│   ├── integration_test.go  # integration smoke test (build tag: integration)
│   └── timeout_test.go      # timeout tests
├── internal/
│   ├── auth/                # Auth strategies: form, basic, cookie, bearer
│   ├── config/              # Config struct
│   ├── crawler/             # Crawler interface, playwright + net/http impls
│   ├── extractor/           # Endpoint extraction and classification
│   ├── output/              # Output formatters: json, csv
│   ├── patterns/            # Pattern registry, matching, finding store
│   │   └── kits/            # Embedded YAML pattern kits
│   ├── ratelimit/           # Rate limiting with jitter
│   ├── robots/              # robots.txt fetching and path checking
│   └── scope/               # URL scope engine
├── main.go
├── go.mod
└── Makefile
```
