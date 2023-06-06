package postgres

import (
	"context"
	"time"

	"github.com/trunov/virena/internal/app/util"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PersonalInformation struct {
	Name        string `json:"name"`
	Email       string `josn:"email"`
	PhoneNumber string `json:"phoneNumber"`
	Company     string `json:"company"`
	VATNumber   string `json:"vatNumber"`
	Country     string `json:"country"`
	City        string `json:"city"`
	ZipCode     string `json:"zipCode"`
	Address     string `json:"address"`
}

type Product struct {
	PartCode string  `json:"partCode"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
}

type Order struct {
	PersonalInformation PersonalInformation `json:"personalInformation"`
	Cart                []Product           `json:"cart"`
}

type DBStorager interface {
	Ping(ctx context.Context) error
	GetProduct(ctx context.Context, productID string) (util.GetProductResponse, error)
	SaveOrder(ctx context.Context, order Order) (string, time.Time, error)
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

func (s *dbStorage) SaveOrder(ctx context.Context, order Order) (string, time.Time, error) {
	// Start a transaction
	tx, err := s.dbpool.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	// Insert the order
	var orderID string
	var createdDate time.Time
	err = tx.QueryRow(ctx, "INSERT INTO orders (name, email, phoneNumber, company, vatNumber, country, city, zipCode, address) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, createdDate",
		order.PersonalInformation.Name, order.PersonalInformation.Email, order.PersonalInformation.PhoneNumber, order.PersonalInformation.Company, order.PersonalInformation.VATNumber, order.PersonalInformation.Country, order.PersonalInformation.City, order.PersonalInformation.ZipCode, order.PersonalInformation.Address).Scan(&orderID, &createdDate)
	if err != nil {
		tx.Rollback(ctx)
		return "", time.Time{}, err
	}

	for _, product := range order.Cart {
		_, err = tx.Exec(ctx, "INSERT INTO order_items (orderId, productCode, quantity) VALUES ($1, $2, $3)",
			orderID, product.PartCode, product.Quantity)
		if err != nil {
			tx.Rollback(ctx)
			return "", time.Time{}, err
		}
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	return orderID, createdDate, nil

}
