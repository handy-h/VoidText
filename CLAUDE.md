# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**VoidText (湮文)** is a Chinese novel TXT text cleaning tool. Built with Go backend (Go 1.25+), SQLite database (pure-Go driver, no CGO), Gin web framework, and vanilla JavaScript frontend.

## Build & Run Commands

```bash
# Build
make build                    # Compiles to ./voidtext binary

# Development (with console logging)
make dev                      # LOG_TO_CONSOLE=true GIN_MODE=debug go run ./cmd/voidtext/

# Production
make run                      # Runs compiled ./voidtext binary (requires .env)

# Clean
make clean                    # Kills process, deletes binary, cleans data/logs/tmp/memory
```

## Testing

```bash
# All tests
go test ./...

# Single package (disable cache)
go test -v -count=1 ./internal/database/

# Combined test runner (sets DATA_DIR=./test_data)
./run_tests.sh

# Unit tests
./testing/unit_testing/run_unit_tests.sh [module]

# Smoke tests (requires running server)
./testing/smoke_testing/run_smoke_tests.sh [server-url]
```

**Test quirks:**
- `run_tests.sh` sets `DATA_DIR=./test_data`
- Test data directory cleaned before and after run
- Tests: `internal/*/test/`, `web/backend/*/test/`, and `*_test.go` files
- Go test files exist for: config, database, file, processor, review manager, middleware, handlers

## Architecture

### Entry Point

`cmd/voidtext/main.go` → loads config → inits SQLite → starts Gin HTTP server

### 5-Step Processing Pipeline

`cleaning` → `indexing` → `llm_fix` → `review` → `finalizing`

Each step saves intermediate file + version record; skipped steps auto-advance.

### Directory Structure

```
cmd/voidtext/          — main.go (entrypoint)
internal/
  config/              — .env loader, AppConfig struct, rate-limit config
  database/            — SQLite init (WAL mode, SetMaxOpenConns(1)), CRUD repos
  processor/           — pipeline, cleaners, vector detector, model repairer, worker pool
    preprocess/        — encoding detection (GBK/UTF-8), text normalization
    postprocess/       — output formatting
    rules/             — regex-based rule engine
    model/             — embedding/similarity
  file/                — MD5 computation, filename→author+title parser
  external/            — API client (api.go) + Ollama client (ollama.go)
  logging/             — structured JSON logger
  review/manager/      — review session management
web/
  backend/             — Gin router, handlers, middleware (auth, rate-limit, error, recovery)
  frontend/            — index.html + modular JS in static/js/modules/
config/prompts/        — prompt templates (v1.0.0, v1.0.1, v1.1.0)
data/                  — runtime data (gitignored): cleaning.db, uploads/, backups/, temp/
```

### SQLite Details

- WAL journal mode, `SetMaxOpenConns(1)` for write serialization
- Tables: `files`, `versions`, `review_items`, `processing_logs`, `chunk_repair_cache`, `retry_queue`, `prompt_versions`
- DB file: `{DATA_DIR}/cleaning.db`

## Configuration

- `.env` file loaded via `godotenv`. Template at `.env.template`.
- Port defaults to `8080` (`PORT` env var)
- LLM API: `LLM_API_URL`, `LLM_API_KEY`, `COMPLETION_MODEL_NAME`
- Local Ollama fallback: `ENABLE_LOCAL_MODEL=true`, `LOCAL_MODEL_URL`, `LOCAL_MODEL_NAME`
- Data directory defaults to `./data` (gitignored, auto-created on startup)

## Code Style (from claude.md)

- **Indent:** 2 spaces, K&R braces, line width ≤ 120
- **Naming:**
  - camelCase: variables/functions (bool prefix: `is`/`has`/`can`)
  - PascalCase: types/components
  - UPPER_SNAKE_CASE: constants
  - snake_case: database fields
  - kebab-case: filenames
- **No `any`** — strict typing. Functions ≤ 80 lines. No magic values.
- **Public API** gets JSDoc-style comments
- **Tests:** `should_行为_条件` naming
- **API response format:** `{ code, message, data }`

## Key Gotchas

- Upload filename format: `author - title.txt` (separators configurable via `NAME_SEPARATORS`)
- File identity = MD5 of content (dedup by MD5 on upload)
- `memory/errors.jsonl` stores runtime error logs (gitignored)
- Hybrid model architecture: local Ollama first, fallback to remote API, then local dict
- `ENABLE_EVOLVER` controls self-tuning prompt optimizer (Python script at `scripts/evolver.py`)
- Processing can run in background after browser closes (server-side state)
- Large files default limit: 100MB (`MAX_FILE_SIZE`)

## Tooling

- **Go 1.25 required** (see go.mod)
- No formatter/linter config — rely on `go fmt` and `go vet`
- `go vet ./...` recommended before commits
- No pre-commit hooks or CI/CD configured
