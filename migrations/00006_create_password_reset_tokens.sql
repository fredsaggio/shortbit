-- +goose Up
-- +goose StatementBegin
CREATE TABLE password_reset_tokens (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,

    CONSTRAINT chk_password_reset_tokens_hash_length
        CHECK (OCTET_LENGTH(token_hash) = 32),

    CONSTRAINT chk_password_reset_tokens_expires_at_after_created
        CHECK (expires_at > created_at),

    CONSTRAINT chk_password_reset_tokens_used_at
        CHECK (
            used_at IS NULL
            OR (
                used_at >= created_at
                AND used_at <= expires_at
            )
        ),

    CONSTRAINT uq_password_reset_tokens_user
        UNIQUE (user_id)
);

CREATE INDEX idx_password_reset_tokens_expires_at
    ON password_reset_tokens (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE password_reset_tokens;
-- +goose StatementEnd
