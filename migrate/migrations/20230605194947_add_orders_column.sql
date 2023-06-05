-- +goose Up
-- +goose StatementBegin
CREATE TABLE orders (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    phoneNumber VARCHAR(20) NOT NULL,
    company VARCHAR(255),
    vatNumber VARCHAR(20),
    country VARCHAR(255) NOT NULL,
    city VARCHAR(255) NOT NULL,
    zipCode VARCHAR(10) NOT NULL,
    address VARCHAR(255) NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
