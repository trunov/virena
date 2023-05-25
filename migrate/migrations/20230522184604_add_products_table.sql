-- +goose Up
-- +goose StatementBegin
CREATE TABLE products (
    id INT,
    code VARCHAR(16) PRIMARY KEY,
    price DECIMAL(10, 2) NOT NULL,
    description VARCHAR(255) NOT NULL,
    note VARCHAR(255),
    weight DECIMAL(10, 2)
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SELECT
    'down SQL query';

-- +goose StatementEnd