-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email TEXT NOT NULL,
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
        PRIMARY KEY (provider, provider_subject),

    CONSTRAINT uq_auth_identities_user_provider
        UNIQUE (user_id, provider),

    CONSTRAINT chk_auth_identities_provider_not_blank
        CHECK (BTRIM(provider) <> ''),

    CONSTRAINT chk_auth_identities_provider_canonical
        CHECK (provider = LOWER(BTRIM(provider))),

    CONSTRAINT chk_auth_identities_provider_subject_not_blank
        CHECK (BTRIM(provider_subject) <> '')
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE auth_identities;
DROP TRIGGER IF EXISTS set_updated_at_password_credentials ON password_credentials;
DROP TABLE password_credentials;
DROP TRIGGER IF EXISTS set_updated_at_users ON users;
DROP TABLE users;
-- +goose StatementEnd
