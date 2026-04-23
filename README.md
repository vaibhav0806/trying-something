# URL Shortener

A simple, self-contained URL shortener written in Go with SQLite storage and a vanilla HTML/CSS/JS frontend.

## Features

- **POST /shorten** — accepts `{"url":"..."}` and returns a 6-character short code
- **GET /:code** — redirects to the original URL and increments the hit counter
- **GET /stats/:code** — returns hit count and creation timestamp
- **GET /** — single-page frontend with a form, recent shorts table, and modern styling
- SQLite persistence via `modernc.org/sqlite` (pure Go, no CGO)

## Requirements

- Go 1.22+

## Quick Start

```bash
make run
```

Then open http://localhost:8080

## API Usage

### Shorten a URL

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

Response:
```json
{"code":"aB3x9K"}
```

### Redirect

```bash
curl -v http://localhost:8080/aB3x9K
```

Returns `302 Found` with `Location: https://example.com`.

### Stats

```bash
curl http://localhost:8080/stats/aB3x9K
```

Response:
```json
{
  "code": "aB3x9K",
  "url": "https://example.com",
  "hits": 1,
  "created_at": "2026-04-23T12:00:00Z"
}
```

## Running Tests

```bash
make test
```

## Project Structure

```
.
├── go.mod          # Go module definition
├── main.go         # HTTP handlers and main entry point
├── main_test.go    # Unit tests
├── static/
│   └── index.html  # Frontend UI
├── Makefile        # run, test, deps, clean targets
└── README.md
```

## License

MIT
