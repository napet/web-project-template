-- +goose Up
CREATE TABLE counters (
    name  TEXT PRIMARY KEY,
    value BIGINT NOT NULL DEFAULT 0 CHECK (value >= 0)
);

INSERT INTO counters (name, value) VALUES ('visitors', 0);

-- +goose Down
DROP TABLE counters;
