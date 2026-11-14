-- +goose Up
-- +goose StatementBegin
CREATE TYPE url_visibility AS ENUM ('public', 'private');

CREATE TABLE urls (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    short_code TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    original_url TEXT NOT NULL,
    visibility url_visibility NOT NULL,
    password_hash TEXT,
    click_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_urls_short_code
        UNIQUE (short_code),

    CONSTRAINT chk_urls_short_code_min_length
        CHECK (CHAR_LENGTH(short_code) >= 6),

    CONSTRAINT chk_urls_short_code_base62
        CHECK (short_code ~ '^[0-9A-Za-z]+$'),

    CONSTRAINT chk_urls_original_url_not_blank
        CHECK (BTRIM(original_url) <> ''),

    CONSTRAINT chk_urls_click_count_non_negative
        CHECK (click_count >= 0),

    CONSTRAINT chk_urls_password
        CHECK (
            (visibility = 'public' AND password_hash IS NULL)
            OR
            (
                visibility = 'private'
                AND password_hash IS NOT NULL
                AND BTRIM(password_hash) <> ''
            )
        )
);

CREATE TRIGGER set_updated_at_urls
    BEFORE UPDATE OF short_code, user_id, original_url, visibility, password_hash ON urls
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

CREATE INDEX idx_urls_user_created
    ON urls (user_id, created_at DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS set_updated_at_urls ON urls;
DROP TABLE urls;
DROP TYPE url_visibility;
-- +goose StatementEnd
