-- +goose Up
-- +goose StatementBegin
CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    orderId VARCHAR(10) NOT NULL,
    productCode VARCHAR(16) NOT NULL,
    brand VARCHAR(3) NOT NULL,
    quantity INT NOT NULL,
    FOREIGN KEY (orderId) REFERENCES orders(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
