package postgres

import (
	"context"

	"github.com/trunov/virena/internal/app/util"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PersonalInformation struct {
	Name        string `json:"name"`
	PhoneNumber string `json:"phoneNumber"`
	Company     string `json:"company"`
	VATNumber   string `json:"vatNumber"`
	Country     string `json:"country"`
	City        string `json:"city"`
	ZipCode     string `json:"zipCode"`
	Address     string `json:"address"`
}

type Product struct {
	_        float64 `json:"amount"`
	PartCode string  `json:"partCode"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type Order struct {
	PersonalInformation PersonalInformation `json:"personalInformation"`
	Cart                []Product           `json:"cart"`
}

type DBStorager interface {
	Ping(ctx context.Context) error
	GetProduct(ctx context.Context, productID string) (util.GetProductResponse, error)
	SaveOrder(ctx context.Context, order Order) error
}

type dbStorage struct {
	dbpool *pgxpool.Pool
}

func NewDBStorage(conn *pgxpool.Pool) *dbStorage {
	return &dbStorage{dbpool: conn}
}

func (s *dbStorage) Ping(ctx context.Context) error {
	err := s.dbpool.Ping(ctx)

	if err != nil {
		return err
	}
	return nil
}

func (s *dbStorage) GetProduct(ctx context.Context, productID string) (util.GetProductResponse, error) {
	var product util.GetProductResponse

	err := s.dbpool.QueryRow(ctx, "SELECT id, code, price, description, note, weight FROM products WHERE code = $1", productID).Scan(
		&product.ID,
		&product.Code,
		&product.Price,
		&product.Description,
		&product.Note,
		&product.Weight,
	)

	if err != nil {
		return product, err
	}

	return product, nil
}

func (s *dbStorage) SaveOrder(ctx context.Context, order Order) error {
	// Start a transaction
	tx, err := s.dbpool.Begin(ctx)
	if err != nil {
		return err
	}

	// Insert the order
	orderID := 0
	err = tx.QueryRow(ctx, "INSERT INTO orders (name, phone_number, company, vat_number, country, city, zip_code, address) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id",
		order.PersonalInformation.Name, order.PersonalInformation.PhoneNumber, order.PersonalInformation.Company, order.PersonalInformation.VATNumber, order.PersonalInformation.Country, order.PersonalInformation.City, order.PersonalInformation.ZipCode, order.PersonalInformation.Address).Scan(&orderID)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, product := range order.Cart {
		_, err = tx.Exec(ctx, "INSERT INTO order_items (order_id, part_code, price, quantity) VALUES ($1, $2, $3, $4)",
			orderID, product.PartCode, product.Price, product.Quantity)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil

}
