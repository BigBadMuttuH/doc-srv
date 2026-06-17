# PROJECT KNOWLEDGE BASE

**Generated:** 2026-06-17
**Commit:** 56a7d8d
**Branch:** main
**Last refactor:** middleware.go + health.go extracted; XSS fix, recovery middleware, hardcoded org name externalised

## OVERVIEW

doc-srv — lightweight Go HTTP server for serving corporate PDF document trees. Single binary, embedded assets, Windows Service support.

## STRUCTURE

```
./
├── main.go         # Entry point + program lifecycle + index handler (217 lines)
├── config.go       # YAML config loader with defaults (126 lines)
├── doc_repo.go     # Filesystem scanner + cache + Markdown renderer (256 lines)
├── health.go       # GET /healthz handler (24 lines)
├── logger.go       # 10 MB size-based log rotation (117 lines)
├── middleware.go   # Logging middleware + panic recovery + accessLog global (117 lines)
├── static/style.css
├── templates/index.html
├── config.yaml     # Runtime YAML config (with org_name)
├── .golangci.yml   # Linter config
├── .editorconfig   # Editor style config
└── go.mod          # module doc-srv, Go 1.23, 4 deps (added bluemonday)
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Routes / handlers | `main.go` `Start()` | Inline closures, no handler files |
| Config schema | `config.go` `Config` struct + `yamlConfig` | Two-layer: YAML → CLI flags, has OrgName |
| Doc scanning logic | `doc_repo.go` `scan()` | Recursive WalkDir, double-checked lock cache |
| Markdown rendering | `doc_repo.go` `renderReadme()` | Goldmark → bluemonday sanitised HTML |
| Log rotation | `logger.go` `rotatingWriter` | 10 MB threshold, stderr fallback |
| Access logging | `middleware.go` `loggingMiddleware` | nginx-format, skips /healthz |
| Panic recovery | `middleware.go` `recoveryMiddleware` | Wraps mux, returns 500 on panic |
| Health check | `health.go` `healthHandler` | GET /healthz, checks docs dir |
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
| `loggingMiddleware` | func | `middleware.go:55` | nginx-format access log |
| `recoveryMiddleware` | func | `middleware.go:97` | Panic recovery, returns 500 |
| `healthHandler` | func | `health.go:10` | GET /healthz, checks docs dir |
| `setAccessLog` | func | `middleware.go:16` | Thread-safe accessLog update |
| `pageData` | struct | `main.go:211` | Template data (Sections + OrgName) |

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

## ANTI-PATTERNS (FORMER — FIXED)

- ~~**`template.HTML` without sanitisation**~~ — Fixed: bluemonday UGCPolicy sanitises Goldmark output in `renderReadme()`.
- ~~**Hardcoded org name**~~ — Fixed: `config.yaml/org_name` → `Config.OrgName` → template `{{.OrgName}}`.
- ~~**No panic recovery middleware**~~ — Fixed: `recoveryMiddleware` wraps mux in `middleware.go`.
- ~~**`go.mod` vs README version mismatch**~~ — Fixed: both say `go 1.23`.
- **Global `accessLog` var** — `middleware.go` package-level state; tests save/restore it, but still global.
- **No OS signal handling** — `kardianos/service` handles signals internally via `s.Run()`; not a real issue.

## COMMANDS

```bash
go run .              # Dev server
go build -o doc-srv.exe .   # Build
go test -v            # Test (10 tests, 6 test files)
go mod tidy           # Tidy deps
go vet ./...          # Static analysis
```

## NOTES

- Port is a `string` — `":" + port` concatenation for `http.Server.Addr`
- `/healthz` explicitly excluded from access log by path match
- Relative paths (`./docs`) work identically on Windows and Linux
- Being `package main` means nothing is importable — pure binary project
