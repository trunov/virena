package file

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/trunov/virena/internal/app/util"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ProductDB struct {
	pool   *pgxpool.Pool
	buffer []util.GetProductResponse
}

func (db *ProductDB) GetProductPrice(ctx context.Context, code string) (float64, error) {
	var price float64
	err := db.pool.QueryRow(ctx, `
	SELECT price FROM products 
	WHERE code = $1 AND brand = 'SKD'`, code).Scan(&price)
	if err != nil {
		return 0, err
	}
	return price, nil
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
		if _, err := tx.Exec(ctx, "INSERT INTO products(code, price, description, note, weight) VALUES($1, $2, $3, $4, $5)", p.Code, p.Price, p.Description, p.Note, p.Weight); err != nil {
			fmt.Println("hey bug is here")
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

func FormatProduct(p []string) util.GetProductResponse {
	price, err := strconv.ParseFloat(p[2], 64)
	if err != nil {
		fmt.Println("Error converting Price:", err)
	}

	var description *string
	if p[3] != "" {
		description = &p[3]
	}

	var note *string
	if p[4] != "" {
		note = &p[4]
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
		Code:        p[1],
		Price:       price,
		Description: description,
		Note:        note,
		Weight:      &weight,
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

		price, err := strconv.ParseFloat(l[1], 64)
		if err != nil {
			fmt.Println("Error converting Price:", err)
		}

		v := util.GetProductResponse{
			Code:  l[0],
			Price: price,
		}

		// v := formatProduct(l)

		if v.Price != 0 {
			err = db.AddProduct(ctx, &v)
			if err != nil {
				return err
			}
		}
	}

	fmt.Println(counter)

	err := db.Flush(ctx)
	if err != nil {
		return err
	}

	return nil
}

func CsvToCsv(inputFile string, outputFile string, db *ProductDB, ctx context.Context) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	output, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer output.Close()

	writer := csv.NewWriter(output)
	defer writer.Flush()

	_, err = reader.Read()
	if err != nil {
		return err
	}

	writer.Write([]string{"Brand,Part number, Requested quantity, price"})

	// Iterate through records from the input CSV
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fmt.Println(record)

		// Get the product code from the second field (index 1)
		code := record[1]
		price, err := db.GetProductPrice(ctx, code)
		if err != nil {
			// log.Printf("Could not fetch price for product code %s: %v", code, err)
			continue
		}

		// Add the price to the record
		priceStr := fmt.Sprintf("%.2f", price)
		record = append(record, priceStr)

		// Write the record to the output CSV
		err = writer.Write(record)
		if err != nil {
			return err
		}
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
	csvReader.Comma = ';'

	start := time.Now()
	err = ProductFromCsvToDB(ctx, csvReader, db)
	if err != nil {
		return err
	}

	// err := csvToCsv("vag.csv", "skodaOutput.csv", db, ctx)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	fmt.Println("Execution time for inserting videos: ", time.Since(start))

	return nil
}
