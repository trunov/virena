package postgres

import (
	"context"

	"github.com/trunov/virena/internal/app/util"

	"github.com/jackc/pgx/v4/pgxpool"
)

type DBStorager interface {
	Ping(ctx context.Context) error
	GetProduct(ctx context.Context, productID string) (util.GetProductResponse, error)
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
