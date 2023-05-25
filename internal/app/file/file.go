package file

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/trunov/virena/internal/app/util"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ProductDB struct {
	pool   *pgxpool.Pool
	buffer []util.GetProductResponse
}

func newProductDB(filename string, dbpool *pgxpool.Pool) *ProductDB {
	return &ProductDB{
		pool:   dbpool,
		buffer: make([]util.GetProductResponse, 0, 10000),
	}
}

func (db *ProductDB) Flush(ctx context.Context) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	for _, p := range db.buffer {
		if _, err := tx.Exec(ctx, "INSERT INTO products(id, code, price, description, note, weight) VALUES($1, $2, $3, $4, $5, $6)", p.ID, p.Code, p.Price, p.Description, p.Note, p.Weight); err != nil {
			fmt.Println("hey bug is here")
			fmt.Println(p)
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("seed products: unable to commit: %v", err)
		return err
	}

	db.buffer = db.buffer[:0]
	return nil
}

func (db *ProductDB) AddProduct(ctx context.Context, p *util.GetProductResponse) error {
	db.buffer = append(db.buffer, *p)

	if cap(db.buffer) == len(db.buffer) {
		err := db.Flush(ctx)
		if err != nil {
			return errors.New("cannot add records to database")
		}
	}
	return nil
}

func formatProduct(p []string) util.GetProductResponse {
	id, err := strconv.Atoi(p[0])
	if err != nil {
		fmt.Println("Error converting ID:", err)
	}

	price, err := strconv.ParseFloat(p[2], 64)
	if err != nil {
		fmt.Println("Error converting Price:", err)
	}

	var weight float64
	if p[5] == "" {
		weight = 0
	} else {
		weight, err = strconv.ParseFloat(p[5], 64)
		if err != nil {
			fmt.Println("Error converting Weight:", err)
		}
	}

	// Create a GetProductResponse struct with the parsed values
	return util.GetProductResponse{
		ID:          id,
		Code:        p[1],
		Price:       price,
		Description: p[3],
		Note:        p[4],
		Weight:      weight,
	}

}

func ProductFromCsvToDB(ctx context.Context, r *csv.Reader, db *ProductDB) error {
	_, _ = r.Read()

	var counter int

	for {
		counter++
		l, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			log.Panic(err)
		}

		v := formatProduct(l)

		err = db.AddProduct(ctx, &v)
		if err != nil {
			return err
		}
	}

	fmt.Println(counter)

	err := db.Flush(ctx)
	if err != nil {
		return err
	}

	return nil
}

func SeedTheDB(fileName string, dbpool *pgxpool.Pool, ctx context.Context) error {
	db := newProductDB(fileName, dbpool)

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	csvReader := csv.NewReader(file)

	start := time.Now()
	err = ProductFromCsvToDB(ctx, csvReader, db)
	if err != nil {
		return err
	}
	fmt.Println("Execution time for inserting videos: ", time.Since(start))

	return nil
}
