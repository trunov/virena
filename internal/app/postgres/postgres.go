package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/trunov/virena/internal/app/util"
	"golang.org/x/sync/errgroup"

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
	SaveOrder(ctx context.Context, order Order, orderID int) (time.Time, error)
	GetAllBrandsPercentage(ctx context.Context) (util.BrandPercentageMap, error)
	CheckOrderIDExists(ctx context.Context, orderID int) (bool, error)
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

func (s *dbStorage) GetAllBrandsPercentage(ctx context.Context) (util.BrandPercentageMap, error) {
	query := "SELECT brand, percentage FROM brand_percentage"

	rows, err := s.dbpool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	brandPercentageMap := make(util.BrandPercentageMap)

	for rows.Next() {
		var brand string
		var percentage float64

		err := rows.Scan(&brand, &percentage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		brandPercentageMap[brand] = percentage
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return brandPercentageMap, nil
}

func (s *dbStorage) GetProductResults(ctx context.Context, productID string) ([]util.GetProductResponse, error) {
	var products []util.GetProductResponse
	var mutex sync.Mutex // Mutex to prevent race condition when appending to the slice

	tables := []string{"products", "jaguar_products", "ford_products", "volvo_products", "toyota_products", "nissan_products", "mazda_products"}

	var g errgroup.Group

	for _, tableName := range tables {
		tableName := tableName

		g.Go(func() error {
			query := fmt.Sprintf("SELECT code, price, description, note, weight, brand FROM %s WHERE code = $1", tableName)

			// if search was done with first letter 'A' we are removing it
			searchID := productID
			if strings.ToLower(string(productID[0])) == "a" {
				searchID = productID[1:]
			}

			var product util.GetProductResponse
			var description sql.NullString
			var note sql.NullString
			var weight sql.NullFloat64

			err := s.dbpool.QueryRow(ctx, query, searchID).Scan(
				&product.Code,
				&product.Price,
				&description,
				&note,
				&weight,
				&product.Brand,
			)

			if err != nil {
				if err == pgx.ErrNoRows {
					return nil
				}
				return err
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

			// Lock and unlock to prevent race condition
			mutex.Lock()
			products = append(products, product)
			mutex.Unlock()

			return nil
		})
	}

	// Wait for all goroutines to finish and return the first non-nil error (if any)
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return products, nil
}

func (s *dbStorage) SaveOrder(ctx context.Context, order Order, orderID int) (time.Time, error) {
	// Start a transaction
	tx, err := s.dbpool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	// Insert the order
	var createdDate time.Time
	err = tx.QueryRow(ctx, "INSERT INTO orders (id, name, email, phoneNumber, company, vatNumber, country, city, zipCode, address) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING createdDate",
		orderID, order.PersonalInformation.Name, order.PersonalInformation.Email, order.PersonalInformation.PhoneNumber, order.PersonalInformation.Company, order.PersonalInformation.VATNumber, order.PersonalInformation.Country, order.PersonalInformation.City, order.PersonalInformation.ZipCode, order.PersonalInformation.Address).Scan(&createdDate)
	if err != nil {
		tx.Rollback(ctx)
		return time.Time{}, err
	}

	for _, product := range order.Cart {
		_, err = tx.Exec(ctx, "INSERT INTO order_items (orderId, productCode, brand, quantity) VALUES ($1, $2, $3, $4)",
			orderID, product.PartCode, product.Brand, product.Quantity)
		if err != nil {
			tx.Rollback(ctx)
			return time.Time{}, err
		}
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return time.Time{}, err
	}

	return createdDate, nil

}

func (s *dbStorage) CheckOrderIDExists(ctx context.Context, orderID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1)`
	err := s.dbpool.QueryRow(ctx, query, orderID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
