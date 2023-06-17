package file

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"time"

	scraper "github.com/trunov/virena/internal/app/scrapper"
	"github.com/trunov/virena/internal/app/util"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ProductDB struct {
	pool    *pgxpool.Pool
	buffer  []util.GetProductResponse
	scraper *scraper.Scraper
}

func newProductDB(filename string, dbpool *pgxpool.Pool, scraper *scraper.Scraper) *ProductDB {
	return &ProductDB{
		pool:    dbpool,
		buffer:  make([]util.GetProductResponse, 0, 10000),
		scraper: scraper,
	}
}

func (db *ProductDB) Flush(ctx context.Context) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	for _, p := range db.buffer {
		if _, err := tx.Exec(ctx, "INSERT INTO originaalosad_products(code, ronaxPrice) VALUES($1, $2)", p.Code, p.RonaxPrice); err != nil {
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
			fmt.Println(err)
			return errors.New("cannot add records to database")
		}
	}
	return nil
}

func formatProduct(code string, ronaxPrice float64, scraper *scraper.Scraper) util.GetProductResponse {
	// productURL := fmt.Sprintf("https://originaalosad.ee/zaptsasti/?s=%s", code)
	// originaalosadPrice, err := scraper.GetOriginaalosadPriceData(productURL)
	// if err != nil {
	// 	fmt.Printf("Failed to fetch product data for ID %s: %v\n", code, err)
	// 	return util.GetProductResponse{}
	// }

	calculatedVirenaPrice := math.Round(ronaxPrice*100) / 100

	// Create a GetProductResponse struct with the parsed values
	return util.GetProductResponse{
		Code:               code,
		// OriginaalosadPrice: originaalosadPrice,
		RonaxPrice:         calculatedVirenaPrice,
	}

}

func ProductFromCsvToDB(ctx context.Context, r *csv.Reader, db *ProductDB) error {
	_, _ = r.Read()
	_, _ = r.Read()
	_, _ = r.Read()

	var counter int

	for {
		counter++
		l, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		// fmt.Println(l)

		secondPartPrice := strings.Replace(l[1], ";", "", -1)

		code, ronaxPrice, err := util.FormatCodeAndPrice(l[0]+secondPartPrice, len(secondPartPrice))
		if err != nil {
			fmt.Println("Error:", err)
			return err
		}

		v := formatProduct(code, ronaxPrice, db.scraper)

		// fmt.Println(v)

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
	scraper := scraper.NewScraper()
	db := newProductDB(fileName, dbpool, scraper)

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	csvReader := csv.NewReader(file)
	// csvReader.LazyQuotes = true

	start := time.Now()
	err = ProductFromCsvToDB(ctx, csvReader, db)
	if err != nil {
		return err
	}
	fmt.Println("Execution time for inserting videos: ", time.Since(start))

	return nil
}
