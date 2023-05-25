package repo

import (
	"context"
	"errors"

	"github.com/trunov/virena/internal/app/config"
	"github.com/trunov/virena/internal/app/postgres"
	"github.com/trunov/virena/migrate"

	"github.com/jackc/pgx/v4/pgxpool"
)

var ErrMissingDBURI = errors.New("database URI was not provided")

func CreateRepo(ctx context.Context, cfg config.Config) (postgres.DBStorager, *pgxpool.Pool, error) {
	var dbStorage postgres.DBStorager
	var dbpool *pgxpool.Pool

	if cfg.DatabaseURI != "" {
		var err error

		dbpool, err = pgxpool.Connect(ctx, cfg.DatabaseURI)
		if err != nil {
			return nil, nil, err
		}

		dbStorage = postgres.NewDBStorage(dbpool)

		err = migrate.Migrate(cfg.DatabaseURI, migrate.Migrations)
		if err != nil {
			return nil, nil, err
		}
	} else {
		return nil, nil, ErrMissingDBURI
	}

	return dbStorage, dbpool, nil
}
