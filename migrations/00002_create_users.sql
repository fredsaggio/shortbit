-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email TEXT NOT NULL,
    email_verified_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_email
        UNIQUE (email),

    CONSTRAINT chk_users_id_uuid_v7
        CHECK (uuid_extract_version(id) = 7),

    CONSTRAINT chk_users_email_not_blank
        CHECK (BTRIM(email) <> ''),

    CONSTRAINT chk_users_email_canonical
        CHECK (email = LOWER(BTRIM(email)))
);

CREATE TRIGGER set_updated_at_users
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE password_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_password_credentials_hash_not_blank
        CHECK (BTRIM(password_hash) <> '')
);

CREATE TRIGGER set_updated_at_password_credentials
    BEFORE UPDATE ON password_credentials
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE auth_identities (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_auth_identities
        PRIMARY KEY (provider, provider_user_id),

    CONSTRAINT uq_auth_identities_user_provider
        UNIQUE (user_id, provider),

    CONSTRAINT chk_auth_identities_provider_not_blank
        CHECK (BTRIM(provider) <> ''),

    CONSTRAINT chk_auth_identities_provider_canonical
        CHECK (provider = LOWER(BTRIM(provider))),

    CONSTRAINT chk_auth_identities_provider_user_id_not_blank
        CHECK (BTRIM(provider_user_id) <> '')
);

CREATE TABLE password_registration_attempts (
    token_hash BYTEA PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    verification_proof_hash BYTEA NOT NULL,
    failed_attempts SMALLINT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_code_sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    code_expires_at TIMESTAMPTZ NOT NULL,
    attempt_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_password_registration_attempts_token_hash_length
        CHECK (OCTET_LENGTH(token_hash) = 32),

    CONSTRAINT chk_password_registration_attempts_email_not_blank
        CHECK (BTRIM(email) <> ''),

    CONSTRAINT chk_password_registration_attempts_email_canonical
        CHECK (email = LOWER(BTRIM(email))),

    CONSTRAINT chk_password_registration_attempts_password_hash_not_blank
        CHECK (BTRIM(password_hash) <> ''),

    CONSTRAINT chk_password_registration_attempts_proof_hash_length
        CHECK (OCTET_LENGTH(verification_proof_hash) = 32),

    CONSTRAINT chk_password_registration_attempts_failed_attempts
        CHECK (failed_attempts >= 0),

    CONSTRAINT chk_password_registration_attempts_lock_period
        CHECK (
            locked_until IS NULL
            OR (
                locked_until > created_at
                AND locked_until <= attempt_expires_at
            )
        ),

    CONSTRAINT chk_password_registration_attempts_code_expiration
        CHECK (code_expires_at > created_at),

    CONSTRAINT chk_password_registration_attempts_attempt_expiration
        CHECK (attempt_expires_at > created_at),

    CONSTRAINT chk_password_registration_attempts_expiration_order
        CHECK (code_expires_at <= attempt_expires_at)
);

CREATE TRIGGER set_updated_at_password_registration_attempts
    BEFORE UPDATE ON password_registration_attempts
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

CREATE INDEX idx_password_registration_attempts_email
    ON password_registration_attempts(email);

CREATE INDEX idx_password_registration_attempts_attempt_expires_at
    ON password_registration_attempts(attempt_expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS set_updated_at_password_registration_attempts ON password_registration_attempts;
DROP TABLE password_registration_attempts;
DROP TABLE auth_identities;
DROP TRIGGER IF EXISTS set_updated_at_password_credentials ON password_credentials;
DROP TABLE password_credentials;
DROP TRIGGER IF EXISTS set_updated_at_users ON users;
DROP TABLE users;
-- +goose StatementEnd
