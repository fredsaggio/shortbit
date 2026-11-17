-- +goose Up
-- +goose StatementBegin
-- Visits of each link per day that are not plain clicks (ADR 0007):
-- unique_visitors counts people by the day's salted hash, robot_visits counts
-- link preview robots and crawlers, which stay out of the clicks. The day is
-- the date in America/Sao_Paulo when the visit happened.
CREATE TABLE url_daily_audience (
    url_id BIGINT NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    unique_visitors BIGINT NOT NULL DEFAULT 0,
    robot_visits BIGINT NOT NULL DEFAULT 0,

    PRIMARY KEY (url_id, day),

    CONSTRAINT chk_url_daily_audience_not_negative
        CHECK (unique_visitors >= 0 AND robot_visits >= 0)
);

-- Clicks of people per day and country (ISO 3166-1 alpha-2). Clicks with an
-- unknown country have no row: they are the difference to url_daily_clicks.
CREATE TABLE url_daily_countries (
    url_id BIGINT NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    country TEXT NOT NULL,
    clicks BIGINT NOT NULL,

    PRIMARY KEY (url_id, day, country),

    CONSTRAINT chk_url_daily_countries_country
        CHECK (country ~ '^[A-Z]{2}$'),
    CONSTRAINT chk_url_daily_countries_positive
        CHECK (clicks > 0)
);

-- One random salt per day. It is deleted once the day is over, and with it
-- any way to rebuild a visitor hash from an IP.
CREATE TABLE visitor_salts (
    day DATE PRIMARY KEY,
    salt BYTEA NOT NULL,

    CONSTRAINT chk_visitor_salts_size
        CHECK (octet_length(salt) = 32)
);

-- Visitors already counted on a link on a day, to count each one once. Kept
-- until the day after, for events that wait in the queue past midnight.
CREATE TABLE url_daily_visitor_hashes (
    url_id BIGINT NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    visitor_hash BYTEA NOT NULL,

    PRIMARY KEY (url_id, day, visitor_hash),

    CONSTRAINT chk_url_daily_visitor_hashes_size
        CHECK (octet_length(visitor_hash) = 16)
);

CREATE INDEX idx_url_daily_visitor_hashes_day
    ON url_daily_visitor_hashes (day);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE url_daily_visitor_hashes;
DROP TABLE visitor_salts;
DROP TABLE url_daily_countries;
DROP TABLE url_daily_audience;
-- +goose StatementEnd
