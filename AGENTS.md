# CAPI

Go package for server-side conversion API integrations (Google Ads, Meta, LinkedIn, TikTok, etc.).

## Structure

- `cmd/` — CLI tools and entrypoints
- `backend/handler/` — HTTP handlers
- `backend/mware/` — middleware (URL rewriting, cache control)
- `backend/template/` — Go template renderer with build tags: `dev.go` for development, `prod.go` for production
- `backend/presenter/` — presenter structs for template data
- `backend/analytics/` — server-side analytics
- `ansible/` — provisioning and deploy automation (see `ansible/AGENTS.md`)

## Commands

See `Makefile` (root) for build targets.

```bash
make dev      # Hot reload
make test     # All tests
make fmt      # Format Go
make vet      # Lint all
```

**Important**: Always run `make test`, `make vet`, and other Make targets from the **project root directory** — never `cd` into a subdirectory first. Running Make targets from the wrong directory causes build failures.

## Rules

- **Bug fixes require TDD RED/GREEN**: write a failing test first, confirm it fails, then write the fix and confirm tests pass
- Use `-tags dev` for development builds (hot-reloads templates)
- Fix all test failures even if unrelated to the current changes

### Go code standards

- Complete implementations without placeholders or TODOs
- Minimal comments unless code is complex
- `context.Context` as first parameter, variable name `ctx`
- Wrap errors with context: `fmt.Errorf("<context>: %w", err)`
- Use package-level `slog.Info`, `slog.Error`, `slog.Warn`, `slog.Debug` instead of `slog.Default()`
- Use `any` instead of `interface{}`
- Never use naked returns
- Use `switch` with no condition instead of if/else chains; avoid `else`
- For non-cryptographic randomness, use `math/rand/v2`
- Use presenter structs over template func maps. Wrap DB models with display-ready fields and pass those into views. Use the `Presenter` suffix (e.g., `jobPostingPresenter`). All template data structs must live in `backend/presenter/` — never pass `map[string]any` to templates
- Never use `template.FuncMap` — always use a presenter struct instead. If a template needs ad-hoc data, create a dedicated struct in a presenter package
- Use `.gohtml` as the file extension for Go HTML templates
- Bubble up errors. Never swallow or ignore them.
- Add short doc comments to exported methods and functions except constructors and initializers
- Define interfaces at the call site that needs them; keep shared packages focused on concrete types and structs.
- Assume constructors and initializers fully initialize their values. Avoid nil-guarding those fields unless a caller can omit them.
- Never use http.DefaultClient in production code. It's unsafe.
- Use `github.com/avast/retry-go/v4` for backend retry logic. Do not add ad hoc retry loops or `time.Sleep`-based backoff; use `retry.Context`, `retry.Attempts`, `retry.DelayType`, `retry.BackOffDelay`, and `retry.Unrecoverable` where appropriate.
- Use `time.Now().UTC()` for wall-clock timestamps. Use UTC-explicit SQL date/time expressions such as `(CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date`; do not use timezone-sensitive `CURRENT_DATE`, `NOW()`, or bare `CURRENT_TIMESTAMP` in application queries. Use plain `time.Now()` only for monotonic duration/deadline measurement.
- Log messages: proper casing, no interpolation. Use structured fields for dynamic values (e.g., `slog.Error("Failed to capture pageview", "error", err)` not `slog.Error(fmt.Sprintf("failed: %v", err))`)
- Public methods and functions should never accept private types or interfaces.
- Go 1.26+ allows `new(<literal>)` to get a pointer from a literal. e.g. `new(42)`.
- Nest the error path, not the happy path. E.g. avoid `err == nil` and prefer `err != nil`

### Go test standards

- Always use testify's `require` (never `assert` or `t.Error`)
- Always use `t.Parallel()` in top-level tests, never in subtests
- Naming: `Test<FunctionName>` for functions, `Test<Type>_<FunctionName>` for methods
- Use `t.Run()` subtests with "happy path" and error condition cases
- Use `t.Context()` — never `context.Background()` or `context.TODO()`
- Use `synctest`, channels, or `errgroup.Group` for concurrency — never `time.Sleep` unless in a synctest bubble
- Prefer `require.EqualError` over `require.Contains` for error assertions
- Never test unexported functions; test through public interface only
- Ignore return values when testing error cases: `_, err := FunctionToTest()`
- Do not test `New` constructor/initializer functions
- Do not add tests under `cmd/`. Main commands and subcommands are wiring only; test underlying behavior in `backend/` packages instead.

### Test Helper Pattern

When exporting test helpers from a package, define a `TestingT` interface within that package rather than relying on a shared `testutil` package or importing `testing.T` directly. This keeps the interface minimal and co-located with the code that uses it.

**Example**: `backend/analytics/test_util.go` defines a local `TestingT` interface with only the methods needed (`Helper()`, `Errorf()`, `Fatalf()`). This avoids circular dependencies and keeps the package self-contained.


