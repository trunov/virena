-- +goose Up
-- +goose StatementBegin
INSERT INTO brand_percentage (brand, percentage)
VALUES
  ('JGR', 0.05),
  ('BMW', 0),
  ('LRR', 0.05),
  ('MB', 0),
  ('SKD', 0.03),
  ('VAG', 0.03),
  ('FRD', 0),
  ('VLV', 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
