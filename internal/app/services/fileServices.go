package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
)

type Dealer struct {
	Code   string
	Price  float64
	Dealer string
}

type FileService interface {
	ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn int) ([]Dealer, error)
	ReadFileToMap(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex int) (map[string]Dealer, error)
	CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwo map[string]Dealer, dealerColumn, dealerNumber int) ([][]string, error)
}

type fileServiceImpl struct{}

func NewFileService() FileService {
	return &fileServiceImpl{}
}

func (s *fileServiceImpl) ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn int) ([]Dealer, error) {
	reader := csv.NewReader(file)
	reader.Comma = delimiter
	var dealers []Dealer

	_, _ = reader.Read() // Skip header

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if dealerColumn > 0 {
			dealers = append(dealers, Dealer{
				Code:   record[codeIndex],
				Price:  parsePrice(record[priceIndex]),
				Dealer: record[dealerColumn],
			})
		} else {
			dealers = append(dealers, Dealer{
				Code:  record[codeIndex],
				Price: parsePrice(record[priceIndex]),
			})
		}
	}

	return dealers, nil
}

func (s *fileServiceImpl) ReadFileToMap(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex int) (map[string]Dealer, error) {
	reader := csv.NewReader(file)
	reader.Comma = delimiter
	dealersMap := make(map[string]Dealer)

	_, _ = reader.Read() // Skip header

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		dealer := Dealer{
			Code:  record[codeIndex],
			Price: parsePrice(record[priceIndex]),
		}

		dealersMap[dealer.Code] = dealer
	}

	return dealersMap, nil
}

func (s *fileServiceImpl) CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwoMap map[string]Dealer, dealerColumn, dealerNumber int) ([][]string, error) {
	results := [][]string{{"Code", "Best Price", "Dealer Number"}}

	for _, d1 := range dealerOne {
		code := d1.Code
		bestPrice := d1.Price
		var dealerNum string

		if dealerColumn > 0 {
			dealerNum = d1.Dealer
		} else {
			dealerNum = "1"
		}

		if d2, found := dealerTwoMap[code]; found {
			if d2.Price < bestPrice {
				bestPrice = d2.Price
				if dealerNumber > 0 {
					dealerNum = strconv.Itoa(dealerNumber)
				} else {
					dealerNum = "2"
				}
			}
		}

		results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum})
	}

	for code, d2 := range dealerTwoMap {
		if _, found := dealerTwoMap[code]; found {
			continue
		}

		dealerNum := "2"
		if dealerNumber > 0 {
			dealerNum = strconv.Itoa(dealerNumber)
		}

		results = append(results, []string{code, fmt.Sprintf("%.2f", d2.Price), dealerNum})
	}

	return results, nil
}

func parsePrice(priceStr string) float64 {
	priceStr = strings.Replace(priceStr, "\u00A0", "", -1) // Remove non-breaking spaces
	priceStr = strings.TrimSpace(priceStr)                 // Trim any leading or trailing whitespace
	priceStr = strings.Replace(priceStr, ",", ".", -1)     // Convert comma to dot for parsing
	priceStr = strings.Replace(priceStr, " ", "", -1)      // Remove regular spaces
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		fmt.Printf("Failed to parse price '%s': %v\n", priceStr, err)
		return 0
	}
	return price
}
