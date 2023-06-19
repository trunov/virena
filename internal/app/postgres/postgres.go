package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trunov/virena/internal/app/util"

	"github.com/jackc/pgx/v4"
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
	PartCode    string  `json:"partCode"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Amount      float64 `json:"amount"`
	Description *string `json:"description"`
	Brand       string  `json:"brand"`
}

type Order struct {
	PersonalInformation PersonalInformation `json:"personalInformation"`
	Cart                []Product           `json:"cart"`
}

type DBStorager interface {
	Ping(ctx context.Context) error
	GetProductResults(ctx context.Context, productID string) ([]util.GetProductResponse, error)
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

func (s *dbStorage) GetProductResults(ctx context.Context, productID string) ([]util.GetProductResponse, error) {
	var products []util.GetProductResponse

	tables := []string{"products", "jaguar_products", "ford_products"}

	for _, tableName := range tables {
		query := fmt.Sprintf("SELECT code, price, description, note, weight, brand FROM %s WHERE code = $1", tableName)

		var product util.GetProductResponse
		var description sql.NullString
		var note sql.NullString
		var weight sql.NullFloat64

		err := s.dbpool.QueryRow(ctx, query, productID).Scan(
			&product.Code,
			&product.Price,
			&description,
			&note,
			&weight,
			&product.Brand,
		)

		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			return products, err
		}

		if description.Valid {
			product.Description = &description.String
		}
		if note.Valid {
			product.Note = &note.String
		}
		if weight.Valid {
			product.Weight = &weight.Float64
		}

		products = append(products, product)
	}

	return products, nil
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
		_, err = tx.Exec(ctx, "INSERT INTO order_items (orderId, productCode, brand, quantity) VALUES ($1, $2, $3, $4)",
			orderID, product.PartCode, product.Brand, product.Quantity)
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
