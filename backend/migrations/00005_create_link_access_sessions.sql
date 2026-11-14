-- +goose Up
-- +goose StatementBegin
CREATE TABLE link_access_sessions (
    token_hash BYTEA PRIMARY KEY,
    link_id BIGINT NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT chk_link_access_sessions_token_hash_length
        CHECK (OCTET_LENGTH(token_hash) = 32),

    CONSTRAINT chk_link_access_sessions_expires_at_after_created
        CHECK (expires_at > created_at)
);

CREATE INDEX idx_link_access_sessions_link
    ON link_access_sessions (link_id);

CREATE INDEX idx_link_access_sessions_expires_at
    ON link_access_sessions (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE link_access_sessions;
-- +goose StatementEnd
