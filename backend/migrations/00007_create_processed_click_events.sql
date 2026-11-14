-- +goose Up
-- +goose StatementBegin
CREATE TABLE processed_click_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_processed_click_events_processed_at
    ON processed_click_events (processed_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE processed_click_events;
-- +goose StatementEnd
