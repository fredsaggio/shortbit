-- +goose Up
-- +goose StatementBegin
-- Clicks of each link per day and per source: 'link' for someone opening the
-- short link, 'qr' for someone scanning its QR code. The day is the date in
-- America/Sao_Paulo when the click happened. A row exists only for days with
-- clicks; the API fills the gaps with zero.
CREATE TABLE url_daily_clicks (
    url_id BIGINT NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    source TEXT NOT NULL,
    clicks BIGINT NOT NULL,

    PRIMARY KEY (url_id, day, source),

    CONSTRAINT chk_url_daily_clicks_source
        CHECK (source IN ('link', 'qr')),
    CONSTRAINT chk_url_daily_clicks_positive
        CHECK (clicks > 0)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE url_daily_clicks;
-- +goose StatementEnd
