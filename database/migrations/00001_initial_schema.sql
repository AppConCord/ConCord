-- +goose Up
CREATE TABLE users (
    id INTEGER NOT NULL PRIMARY KEY,
    username TEXT NOT NULL COLLATE BINARY UNIQUE,
    display_name TEXT,
    password_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CONSTRAINT users_username_length CHECK (length(username) BETWEEN 4 AND 16),
    CONSTRAINT users_username_characters CHECK (username NOT GLOB '*[^a-z0-9_]*'),
    CONSTRAINT users_display_name_length CHECK (
        display_name IS NULL OR length(display_name) BETWEEN 2 AND 32
    )
);

CREATE TABLE sessions (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token_hash BLOB NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CONSTRAINT sessions_token_hash_length CHECK (length(token_hash) = 32),
    CONSTRAINT sessions_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE messages (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CONSTRAINT messages_content_length CHECK (length(content) BETWEEN 1 AND 2000),
    CONSTRAINT messages_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX messages_user_id_idx ON messages(user_id);

-- +goose Down
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
