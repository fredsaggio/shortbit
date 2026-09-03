-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT chk_user_sessions_token_hash_length
        CHECK (OCTET_LENGTH(token_hash) = 32),

    CONSTRAINT chk_user_sessions_expires_at_after_created
        CHECK (expires_at > created_at)
);

CREATE INDEX idx_user_sessions_user
    ON user_sessions (user_id);

CREATE INDEX idx_user_sessions_expires_at
    ON user_sessions (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_sessions;
-- +goose StatementEnd
