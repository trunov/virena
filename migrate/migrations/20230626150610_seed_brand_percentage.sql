-- +goose Up
-- +goose StatementBegin
INSERT INTO brand_percentage (brand, percentage)
VALUES
  ('JGR', 0.5),
  ('BMW', 0),
  ('LRR', 0.5),
  ('MB', 0),
  ('SKD', 0.3),
  ('VAG', 0.3),
  ('FRD', 0),
  ('VLV', 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
