-- +goose Up
-- +goose StatementBegin
INSERT INTO brand_percentage (brand, percentage)
VALUES
  ('JGR', 5),
  ('BMW', 0),
  ('LRR', 5),
  ('MB', 0),
  ('SKD', 3),
  ('VAG', 3),
  ('FRD', 0),
  ('VLV', 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
