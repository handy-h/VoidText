# Repository Guidelines

## Project Structure & Module Organization

VoidText is a single Go module (`module voidtext`) for cleaning Chinese novel TXT files. The backend entrypoint is `cmd/voidtext/main.go`, which loads config, initializes SQLite, and starts the Gin server. Core packages live under `internal/`: `config/`, `database/`, `processor/`, `file/`, `external/`, `logging/`, and `review/manager/`. The five-step pipeline is in `internal/processor/pipeline.go`.

The web layer is split between `web/backend/` for Gin routes, handlers, and middleware, and `web/frontend/` for the static SPA (`index.html`, CSS, and modular JavaScript in `static/js/modules/`). Test scripts are under `testing/`; longer architecture and API notes are in `repowikis/`. Runtime files belong in `data/` and should not be committed.

## Build, Test, and Development Commands

- `make dev`: run locally with `go run`, console logs, and Gin debug mode.
- `make build`: tidy and verify modules, then build `./voidtext`.
- `make run`: run the compiled binary.
- `go test ./...`: run all Go tests.
- `go test -v -count=1 ./internal/processor/`: run one package without cache.
- `./testing/unit_testing/run_unit_tests.sh [module]`: run the project unit test script.
- `./testing/smoking_testing/run_smoke_tests.sh [server-url]`: run smoke tests against a live server.
- `go fmt ./...` and `go vet ./...`: format and check Go code before review.

## Coding Style & Naming Conventions

Use Go 1.25+. Keep formatting compatible with `gofmt`; local style prefers 2-space indentation, K&R braces, and lines under 120 characters. Use `camelCase` for variables/functions, `PascalCase` for types, and `UPPER_SNAKE_CASE` for constants. Database fields use `snake_case`; filenames use `kebab-case`. Avoid `any` and magic values. API responses should keep the `{ code, message, data }` shape.

## Testing Guidelines

Place Go tests beside source files or in existing `test/` subdirectories. Follow the project convention `should_行为_条件` for test names when adding new tests. Use `DATA_DIR=./test_data` or the provided scripts when tests need isolated runtime data. Smoke tests require the server to be running and authentication disabled or a valid API token supplied.

## Commit & Pull Request Guidelines

Recent history uses concise prefixes such as `feat:`, `fix:`, `refactor:`, and Chinese summaries. Keep commits scoped to one logical change. Pull requests should describe behavior changes, list test commands run, link related issues, and include screenshots or screen recordings for frontend UI changes.

## Security & Configuration Tips

Configure local settings through `.env`; never commit API keys, LLM credentials, databases, uploads, backups, or generated runtime logs. Check `API_TOKEN`, rate-limit settings, model endpoints, and `DATA_DIR` before running smoke tests or sharing a local build.

## Agent-Specific Instructions

All agent replies, explanations, analysis, suggestions, and code comments must be in Chinese. Keep code identifiers, keywords, function names, variables, and external API names in English as required by the language or framework.

When inspecting project code, use the `code-context` MCP service first for semantic search, symbol lookup, dependency analysis, and file context. Use `rg`, file globbing, or direct reads only when `code-context` is unavailable or insufficient. Do not guess APIs, function signatures, or call chains; confirm them with `code-context` before editing.

Before modifying code, confirm relevant dependencies and affected symbols through `code-context`. When debugging errors, trace the source through `code-context` first, then explain findings in concise, structured Chinese.
