-- +goose Up
-- +goose StatementBegin
CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    orderId INT NOT NULL,
    productCode INT NOT NULL,
    amount INT NOT NULL,
    FOREIGN KEY (orderId) REFERENCES orders(id),
    FOREIGN KEY (productCode) REFERENCES products(code)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
