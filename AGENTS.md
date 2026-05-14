# AGENTS.md — VoidText (湮文)

Chinese novel TXT cleaning tool. Go backend, SQLite, Gin web framework, SPA frontend.

## Entrypoint & Build

```
# Build
go build -o voidtext ./cmd/voidtext/

# Run directly
go run ./cmd/voidtext/

# Run built binary (reads .env from executable dir first, then working dir)
./voidtext
```

## Config

- `.env` file loaded via `godotenv`. Template at `.env.template`.
- Port defaults to `8080` (`PORT` env var).
- LLM API: `LLM_API_URL`, `LLM_API_KEY`, `COMPLETION_MODEL_NAME`.
- Local Ollama fallback: `ENABLE_LOCAL_MODEL=true`, `LOCAL_MODEL_URL`, `LOCAL_MODEL_NAME`.
- Data directory defaults to `./data` (gitignored, auto-created on startup).
- Old config keys (`EXTERNAL_API_URL`, `EXTERNAL_API_KEY`, `EMBEDDING_MODEL_NAME`) auto-mapped for backward compat.

## Architecture

- **Single Go module** (`module voidtext`, Go 1.25, `modernc.org/sqlite` — no CGO).
- Entrypoint: `cmd/voidtext/main.go` → loads config → inits SQLite → starts Gin HTTP server.
- **5-step pipeline** (in order): `cleaning` → `indexing` → `llm_fix` → `review` → `finalizing`.
- Each step saves intermediate file + version record; skipped steps auto-advance.
- Pipeline steps: `internal/processor/pipeline.go` (step definitions, file pipeline.go also has older `Process()` for single-run).

### Directory layout

```
cmd/voidtext/          — main.go (entrypoint)
internal/
  config/              — .env loader, AppConfig struct, rate-limit config
  database/            — SQLite init (WAL mode, SetMaxOpenConns(1)), CRUD repos
  processor/           — pipeline, cleaners, vector detector, model repairer, worker pool, rules engine
    preprocess/        — encoding detection (GBK/UTF-8), text normalization
    postprocess/       — output formatting
    rules/             — regex-based rule engine
    model/             — embedding/similarity
  file/                — MD5 computation, filename→author+title parser
  external/            — API client (api.go) + Ollama client (ollama.go)
  logging/             — structured JSON logger
  review/manager/      — review session management
web/
  backend/             — Gin router, handlers, middleware (rate-limit, error, recovery)
  frontend/            — index.html + modular JS in static/js/modules/
scripts/               — run.sh, evolver.py (Python, for prompt tuning)
config/prompts/        — (directory exists, empty — intended for prompt templates)
testing/
  unit_testing/        — test plans + run_unit_tests.sh
  smoking_testing/     — test plans + run_smoke_tests.sh (needs running server)
repowikis/             — Chinese technical docs (architecture, API, deploy, dev guide)
data/                  — runtime data (gitignored): cleaning.db, uploads/, backups/, temp/
```

### SQLite specifics (from db.go)

- WAL journal mode, `SetMaxOpenConns(1)` (SQLite write serialization).
- `SetConnMaxLifetime(5 * time.Minute)`.
- Tables: `files`, `versions`, `review_items`, `processing_logs`, `chunk_repair_cache`, `retry_queue`, `prompt_versions`.
- DB file: `{DATA_DIR}/cleaning.db`.

## Testing

```bash
# All tests
go test ./...

# Single package (with cache disabled)
go test -v -count=1 ./internal/database/

# Unit test runner script
./testing/unit_testing/run_unit_tests.sh [module]

# Smoke test runner (requires running server)
./testing/smoke_testing/run_smoke_tests.sh [server-url]

# Combined test runner (root-level)
./run_tests.sh    # sets DATA_DIR=./test_data, cleans up before/after
```

**Test quirks:**
- `run_tests.sh` sets `DATA_DIR=./test_data`.
- Test data directory cleaned before and after run.
- Tests at: `internal/*/test/`, `web/backend/*/test/`, and alongside source files (`*_test.go`).
- Go test files exist for: config, database, file (md5, parser), processor (pipeline, model_repairer, vector_detector, rules, preprocess, postprocess), review manager, middleware (rate_limit), handlers (health).
- Smoke tests operate against a live server via curl.

## Tooling quirks

- **Go 1.25 required** (see go.mod).
- No formatter/linter config found — rely on `go fmt` and `go vet`.
- `go vet ./...` recommended before commits.
- Pre-commit hooks: none detected.
- CI: none detected (no `.github/workflows/`).
- `claude.md` at root is a coding-style instruction file (2-space indent, K&R braces, ≤120 line width, camelCase/PascalCase/UPPER_SNAKE_CASE naming, Chinese responses for AI, test names `should_行为_条件`). Treat as agent-style guide.

## Styling conventions (from claude.md)

- Indent: 2 spaces. K&R braces. Line width ≤ 120.
- camelCase for vars/functions (bool prefix: `is`/`has`/`can`).
- PascalCase for types, UPPER_SNAKE_CASE for constants.
- snake_case for DB fields, kebab-case for filenames.
- No `any` — strict typing. Functions ≤ 80 lines. No magic values.
- Public API gets JSDoc-style comments.
- Tests: `should_行为_条件` naming.
- API response format: `{ code, message, data }`.

## Key gotchas

- **Auth must be disabled** or token provided for curl/smoke tests.
- Upload filename format: `author - title.txt` (separators configurable via `NAME_SEPARATORS`).
- File identity = MD5 of content (dedup by MD5 on upload).
- `config/prompts/` exists but is empty — don't expect files there unless created.
- `memory/errors.jsonl` stores runtime error logs (gitignored).
- Hybrid model architecture: local Ollama first, fallback to remote API, then local dict.
- `ENABLE_EVOLVER` controls self-tuning prompt optimizer (Python script at `scripts/evolver.py`).
- Processing can run in background after browser closes (server-side state).
- Large files default limit: 100MB (`MAX_FILE_SIZE`).
