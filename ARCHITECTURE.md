# Concord architecture

## Purpose

Concord is a deliberately small school-project chat application. The backend provides account authentication and one global message feed. There are no channels, WebSockets, message edits, reactions, user search, or administration features.

## Process boundaries

```text
Browser client
    │ HTTP + opaque bearer token
    ▼
backend module / container
    │ GORM queries only
    ▼
data/concord.db
    ▲
    │ explicit Goose commands
database module / migration container
```

The backend and migration tool are separate Go modules, binaries, and Docker images. Backend startup verifies that required tables exist but never creates or changes them. An operator runs the migration service explicitly when schema changes need to be applied.

## Backend package responsibilities

- `cmd/api`: configuration, dependency construction, signal handling, and server lifecycle.
- `internal/config`: environment parsing and validation.
- `internal/apperror`: framework-independent error codes, field details, HTTP status metadata, and wrapped causes.
- `internal/domain`: persistence-neutral sentinel errors shared across boundaries.
- `internal/model`: GORM entity declarations for users, sessions, and messages.
- `internal/snowflake`: concurrency-safe signed 64-bit Snowflake generation.
- `internal/validation`: deterministic, single-error-per-field validation using govalidator primitives.
- `internal/auth`: registration, bcrypt credential verification, opaque token generation, authentication, and revocation.
- `internal/messages`: global message creation and cursor-listing rules.
- `internal/database`: GORM/SQLite connection policy and schema presence checks.
- `internal/store`: GORM implementations of the service-owned persistence interfaces.
- `internal/httpapi`: Echo routes, JSON decoding, bearer middleware, DTO serialization, logging, and centralized error rendering.

Dependencies point inward: HTTP and GORM adapters depend on application services and models. Application services know only narrow repository and ID-generator interfaces; they do not import Echo or GORM.

## Data model

`users` stores a Snowflake primary key, unique lowercase username, optional display name, bcrypt password hash, and epoch-millisecond creation/update timestamps.

`sessions` stores a Snowflake primary key, owning user, SHA-256 hash of an opaque token, expiration timestamp, and audit timestamps. Raw tokens exist only in the login/registration response and subsequent client requests. Deleting a session immediately revokes its token.

`messages` stores a Snowflake primary key, author foreign key, content, and audit timestamps. Every list query preloads the author required by the response contract.

SQLite foreign keys cascade user deletion into sessions and messages. The database enforces essential length, uniqueness, token-hash, and relationship constraints; application validation supplies client-friendly errors.

## Identifiers and time

Snowflakes use this fixed layout:

```text
0 | 41-bit milliseconds since 2026-01-01 UTC | 10-bit node | 12-bit sequence
```

The sign bit remains zero. A detected backward clock fails ID generation rather than risking a duplicate. IDs stay as `int64` in Go and `INTEGER` in SQLite, but cross the JSON boundary as decimal strings to preserve JavaScript precision.

All database timestamps are signed epoch-millisecond integers. HTTP DTOs convert them to UTC RFC 3339 strings with fixed millisecond precision. `updated_at` initially equals `created_at`.

## Authentication

Registration validates input, hashes the password, generates an opaque 256-bit token, and atomically creates the user and initial session in one GORM transaction. Login uses the same generic unauthorized response for missing users and incorrect passwords.

Clients send `Authorization: Bearer <token>`. Middleware hashes the supplied token with SHA-256 and resolves an unexpired database session. Logout deletes the matching session and is idempotent for an already absent database row. The database never stores raw bearer credentials.

## Request and error flow

JSON bodies are size-limited, accept exactly one object, and reject unknown fields. Services validate business input and return typed application errors. The centralized Echo error handler is the only component that serializes failures:

```json
{
  "error": {
    "code": "validation_error",
    "fields": {
      "username": "This value must be at least 4 characters long"
    }
  }
}
```

Internal wrapped causes are logged with structured `slog` and are never sent to the client. Each field has at most one validation message, chosen in required/length/format priority.

## Message pagination

`GET /messages` returns the latest page when no cursor is supplied. `before` retrieves older messages and `after` retrieves newer messages for polling; the parameters are mutually exclusive. GORM queries in the efficient index direction and the store normalizes every response to oldest-to-newest order.

## Deployment

Compose exposes two services sharing the explicit `./data:/var/lib/concord` bind mount:

- `backend` runs continuously and never migrates.
- `migrate` belongs to the `tools` profile and runs only through an explicit command such as `docker compose run --rm migrate up`.

Both images build independently. Production schema history is owned exclusively by the SQL files in `database/migrations`.
