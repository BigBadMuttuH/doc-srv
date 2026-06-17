# PROJECT KNOWLEDGE BASE

**Generated:** 2026-06-17
**Commit:** 56a7d8d
**Branch:** main

## OVERVIEW

doc-srv — lightweight Go HTTP server for serving corporate PDF document trees. Single binary, embedded assets, Windows Service support.

## STRUCTURE

```
./
├── main.go         # Entry point + all HTTP handlers + logging middleware (295 lines)
├── config.go       # YAML config loader with defaults (120 lines)
├── doc_repo.go     # Filesystem scanner + cache + Markdown renderer (254 lines)
├── logger.go       # 10 MB size-based log rotation (117 lines)
├── *_{config,doc_repo,doc_repo_cache,health_handler,logger,logging_middleware}_test.go
├── static/style.css
├── templates/index.html
├── config.yaml     # Runtime YAML config
└── go.mod          # module doc-srv, Go 1.23, 3 deps
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Routes / handlers | `main.go` `Start()` | Inline closures, no handler files |
| Config schema | `config.go` `Config` struct + `yamlConfig` | Two-layer: YAML → CLI flags |
| Doc scanning logic | `doc_repo.go` `scan()` | Recursive WalkDir, double-checked lock cache |
| Markdown rendering | `doc_repo.go` `renderReadme()` | Goldmark AST walk for URL rewriting |
| Log rotation | `logger.go` `rotatingWriter` | 10 MB threshold, stderr fallback |
| Windows Service | `main.go` `program` struct + kardianos/service | Persists CLI args in SCM |
| Tests | `*_test.go` | White-box (`package main`), no test framework deps |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `Config` | struct | `config.go:12` | Final parsed config |
| `LoadConfig` | func | `config.go:51` | YAML → Config (optional file) |
| `DocRepository` | struct | `doc_repo.go:33` | Scans + caches doc tree |
| `GetSections` | method | `doc_repo.go:48` | TTL cache + double-checked lock |
| `scan` | method | `doc_repo.go:82` | WalkDir → sections + "Общее" |
| `renderReadme` | func | `doc_repo.go:199` | Goldmark → HTML, URL rewriting |
| `rotatingWriter` | struct | `logger.go:14` | File writer with rotation |
| `program` | struct | `main.go:36` | Server lifecycle (Start/Stop) |
| `loggingMiddleware` | func | `main.go:255` | nginx-format access log |
| `healthHandler` | func | `main.go:228` | GET /healthz, skips access log |

## CONVENTIONS

- **All code is `package main`** — no subpackages, no `internal/`, no `cmd/`
- **Module name bare `doc-srv`** — not path-based
- **Config optional** — missing file = defaults, no error
- **Comments in Russian** — identifiers and strings in English, prose comments in Russian
- **Handlers as closures** — all routes defined inline in `Start()`
- **No framework** — stdlib `http.ServeMux`
- **No linter/CI** — zero lint config, zero CI workflows
- **No Docker** — manual binary deploy + Windows Service install
- **White-box tests** — `package main`, no test framework libs

## ANTI-PATTERNS (THIS PROJECT)

- **`template.HTML` without sanitisation** — `doc_repo.go:243` returns Goldmark output unescaped. XSS vector if README.md contains `<script>`.
- **Hardcoded org name** — `templates/index.html:11-14` "Мурманская таможня". Requires code change to rebrand.
- **Global `accessLog` var** — `main.go:24-25` package-level state mutated directly by tests.
- **No panic recovery middleware** — any panic kills the server.
- **No OS signal handling** — `Ctrl+C` in interactive mode skips graceful shutdown.
- **`go.mod` vs README version mismatch** — `go 1.23.0` in mod, `1.25+` in docs.

## COMMANDS

```bash
go run .              # Dev server
go build -o doc-srv.exe .   # Build
go test -v            # Test (6 test files)
go mod tidy           # Tidy deps
```

## NOTES

- Port is a `string` — `":" + port` concatenation for `http.Server.Addr`
- `/healthz` explicitly excluded from access log by path match
- Relative paths (`./docs`) work identically on Windows and Linux
- Being `package main` means nothing is importable — pure binary project
