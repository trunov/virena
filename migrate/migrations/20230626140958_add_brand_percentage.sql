-- +goose Up
-- +goose StatementBegin
CREATE TABLE brand_percentage (
  brand VARCHAR(3) NOT NULL,
  percentage FLOAT NOT NULL,
  PRIMARY KEY (brand)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
