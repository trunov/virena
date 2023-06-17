-- +goose Up
-- +goose StatementBegin
CREATE TABLE originaalosad_products (
    code VARCHAR(16) PRIMARY KEY,
    originaalosadPrice DECIMAL(10, 2),
    ronaxPrice DECIMAL(10, 2)
);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SELECT
    'down SQL query';

-- +goose StatementEnd