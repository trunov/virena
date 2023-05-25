package migrate

import (
	"database/sql"
	"embed"
	"io/fs"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations
var Migrations embed.FS

func Migrate(dsn string, path fs.FS) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	defer db.Close()

	goose.SetBaseFS(path)
	return goose.Up(db, "migrations")
}
