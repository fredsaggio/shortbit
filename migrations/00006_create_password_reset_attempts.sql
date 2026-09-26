-- +goose Up
-- +goose StatementBegin
CREATE TABLE password_reset_attempts (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    verification_proof_hash BYTEA NOT NULL,
    failed_attempts SMALLINT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_code_sent_at TIMESTAMPTZ NOT NULL,
    code_expires_at TIMESTAMPTZ NOT NULL,
    attempt_expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_password_reset_attempts_user
        UNIQUE (user_id),

    CONSTRAINT chk_password_reset_attempts_token_hash_length
        CHECK (OCTET_LENGTH(token_hash) = 32),

    CONSTRAINT chk_password_reset_attempts_proof_hash_length
        CHECK (OCTET_LENGTH(verification_proof_hash) = 32),

    CONSTRAINT chk_password_reset_attempts_failed_attempts
        CHECK (failed_attempts >= 0),

    CONSTRAINT chk_password_reset_attempts_expirations
        CHECK (
            code_expires_at > created_at
            AND attempt_expires_at > created_at
            AND code_expires_at <= attempt_expires_at
        ),

    CONSTRAINT chk_password_reset_attempts_locked_until
        CHECK (
            locked_until IS NULL
            OR (locked_until > created_at AND locked_until <= attempt_expires_at)
        ),

    CONSTRAINT chk_password_reset_attempts_used_at
        CHECK (
            used_at IS NULL
            OR (used_at >= created_at AND used_at <= attempt_expires_at)
        )
);

CREATE INDEX idx_password_reset_attempts_expiration
    ON password_reset_attempts (attempt_expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE password_reset_attempts;
-- +goose StatementEnd
