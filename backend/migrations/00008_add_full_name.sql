-- +goose Up
-- +goose StatementBegin
-- Nullable because accounts created before this migration have no name; new
-- password registrations always send one and Google logins fill it in.
ALTER TABLE users
    ADD COLUMN full_name TEXT,
    ADD CONSTRAINT chk_users_full_name
        CHECK (full_name IS NULL OR (BTRIM(full_name) <> '' AND CHAR_LENGTH(full_name) <= 100));

ALTER TABLE password_registration_attempts
    ADD COLUMN full_name TEXT,
    ADD CONSTRAINT chk_password_registration_attempts_full_name
        CHECK (full_name IS NULL OR (BTRIM(full_name) <> '' AND CHAR_LENGTH(full_name) <= 100));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE password_registration_attempts
    DROP COLUMN full_name;

ALTER TABLE users
    DROP COLUMN full_name;
-- +goose StatementEnd
