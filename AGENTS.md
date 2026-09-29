# Concord repository contribution rules

## Scope and naming

- The project name is **Concord**. Use `concord` for module names, container names, configuration prefixes, database files, and package metadata.
- Keep changes focused on the requested behavior. Do not add speculative multi-channel, WebSocket, moderation, or federation abstractions.
- Preserve public behavior unless the task explicitly changes its contract.
- Never commit secrets, local configuration, generated binaries, dependency directories, or runtime SQLite files.

## Repository boundaries

- `backend/` is the HTTP application and a standalone Go module. It must not import migration code or apply schema changes.
- `database/` is the standalone Goose migration service and a separate Go module. SQL migrations belong only in `database/migrations/`.
- `frontend/` is the React client. Backend work must not alter it unless a shared contract or explicit request requires it.
- `data/` is the local bind-mount directory. Only `.gitkeep` may be committed from this directory.
- Root Docker Compose configuration coordinates processes but must not couple backend startup to migration execution.

## Go package design

- Keep `cmd/*` packages limited to configuration, dependency assembly, lifecycle management, and process exit behavior.
- Put business rules in feature services such as `internal/auth` and `internal/messages`; they must not depend on Echo or GORM.
- Express persistence dependencies as narrow interfaces owned by the consuming service.
- Keep GORM-specific queries in `internal/store` and connection configuration in `internal/database`.
- Keep HTTP binding, authentication middleware, status codes, and response DTOs in `internal/httpapi`.
- Keep application errors in `internal/apperror`; never expose database, hashing, or internal error messages to clients.
- Avoid package-level mutable state. Inject clocks, generators, stores, and other dependencies when determinism or testing requires it.
- Use `context.Context` for I/O boundaries and pass the request context through services and GORM calls.
- Wrap internal errors with an operation using `%w`. Use `errors.Is` or `errors.As` instead of comparing strings.
- All exported declarations and non-obvious major helpers require useful Go documentation comments.
- Format every touched Go file with `gofmt`. Keep imports and types explicit and avoid unnecessary reflection or `any` values outside transport serialization.

## Domain and API conventions

- Public entity IDs are signed `int64` Snowflakes with the epoch `2026-01-01T00:00:00Z`, 41 timestamp bits, 10 node bits, and 12 sequence bits.
- Serialize every public ID and cursor as a base-10 JSON string. Parse incoming cursor strings with signed 64-bit overflow checks.
- Store timestamps as UTC Unix epoch milliseconds in signed `int64`/SQLite `INTEGER` columns.
- Serialize timestamps as UTC RFC 3339 strings with exactly millisecond precision, for example `2026-09-29T12:00:00.123Z`.
- Every public user and message representation includes `created_at` and `updated_at`.
- Request validation returns at most one deterministic message per field. Validation priority is required, length/range, then format/content rules.
- Error responses use only `error.code` and `error.fields`; `fields` is always a JSON object.
- Authentication uses opaque random bearer tokens in the `Authorization: Bearer <token>` header. Persist only SHA-256 token hashes.
- Passwords must be hashed with bcrypt and must never appear in logs, persistence models returned by handlers, or API responses.
- JSON handlers reject unknown properties, trailing JSON values, and oversized bodies.
- Message list results are ordered oldest-to-newest. `before` and `after` cursors are mutually exclusive.

## GORM and SQLite

- GORM models describe application entities, but `AutoMigrate` is forbidden in production code.
- Goose SQL files are the sole production schema source of truth.
- Use GORM transactions when a use case must atomically persist multiple records.
- Always attach contexts with `WithContext` and check returned GORM errors.
- Translate `gorm.ErrRecordNotFound` and constraint errors at the store boundary into domain errors.
- Keep SQLite foreign keys enabled, configure a busy timeout, and avoid unbounded connection pools.
- Every schema change requires both an up and down migration and a migration test against a real SQLite database.

## Migration service

- Migration execution is explicit. Backend startup must never invoke Goose or `AutoMigrate`.
- The migration service supports deliberate `up`, `down`, `status`, and `version` commands.
- Name SQL migrations with increasing numeric prefixes, for example `00002_add_message_metadata.sql`.
- Never edit an already deployed migration to change history; add a new migration instead.
- Keep migration dependencies and Docker build independent from the backend module.

## Docker and configuration

- Keep separate Dockerfiles for backend and migration processes.
- Containers must run as an unprivileged UID/GID and write SQLite state only through the configured bind mount.
- Configuration comes from `CONCORD_*` environment variables. Development defaults must be safe and documented.
- Add new environment variables to `.env.example` and the relevant documentation.
- Validate Compose changes with `docker compose config` and build every affected image.

## Errors, logging, and security

- Centralize Echo error rendering. Handlers return errors rather than constructing ad hoc error JSON.
- Log internal causes server-side with structured `slog`; do not log passwords, bearer tokens, token hashes, or complete authorization headers.
- Return generic unauthorized responses for unknown usernames, incorrect passwords, expired sessions, and invalid tokens.
- Generate security tokens with `crypto/rand`. A Snowflake is an identifier, never an authentication secret.
- Treat malformed input as client failure and unexpected persistence/cryptographic failures as internal failure.

## Testing and validation

- Behavior changes require tests without exception.
- Unit-test validation priority, service decisions, error mapping, and Snowflake safety.
- Test HTTP handlers through `httptest` and assert status codes plus complete response contracts.
- Test GORM stores against a real temporary SQLite database, not a mocked SQL driver.
- Test Goose migrations against a real temporary SQLite file, including constraints and rollback.
- Critical authentication, registration, message creation/listing, and error paths must remain comprehensively covered.
- Before finishing backend work, run:

  ```text
  cd backend && gofmt -w . && go vet ./... && go test ./... && go build ./...
  cd database && gofmt -w . && go vet ./... && go test ./... && go build ./...
  docker compose config
  docker compose build backend migrate
  ```

- Report every command run and any validation that could not be completed.

## Documentation and Git

- Update `ARCHITECTURE.md` whenever package boundaries, persistence, authentication, deployment, or request flow changes.
- Update a relevant README when commands, configuration, API contracts, or developer workflows change.
- Keep commits small and logically scoped when the user explicitly asks for commits.
- Commit messages begin with a capital letter and describe the change in normal text.
- Never rewrite shared history or interact with remotes unless explicitly requested.
- Never commit build output, dependency directories, credentials, editor state, logs, or database files.
