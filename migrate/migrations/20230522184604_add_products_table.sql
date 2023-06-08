-- +goose Up
-- +goose StatementBegin
CREATE TABLE products (
    code VARCHAR(20) PRIMARY KEY,
    price DECIMAL(10, 2) NOT NULL,
    description VARCHAR(255),
    note VARCHAR(255),
    weight DECIMAL(10, 2)
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SELECT
    'down SQL query';

-- +goose StatementEnd