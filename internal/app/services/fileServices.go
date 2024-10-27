package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/trunov/virena/internal/app/util"
)

type Dealer struct {
	Code              string
	Price             float64
	Dealer            string
	WorstPrice        string
	WorstDealerNumber string
}

type FileService interface {
	ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn int) ([]Dealer, error)
	ReadFileToMap(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex int) (map[string]Dealer, error)
	CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwo map[string]Dealer, dealerColumn, dealerNumber, offsetPercentage int) ([][]string, error)
}

type fileServiceImpl struct{}

func NewFileService() FileService {
	return &fileServiceImpl{}
}

func (s *fileServiceImpl) ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn int) ([]Dealer, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	processedContent := bytes.ReplaceAll(content, []byte(`"`), []byte(``))

	reader := csv.NewReader(bytes.NewReader(processedContent))
	reader.Comma = delimiter
	var dealers []Dealer
	_, _ = reader.Read() // Skip header

	var counter int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			counter++
			continue
		}

		if len(record) >= 6 {
			dealers = append(dealers, Dealer{
				Code:              record[codeIndex],
				Price:             parsePrice(record[priceIndex]),
				Dealer:            record[dealerColumn],
				WorstPrice:        record[len(record)-3],
				WorstDealerNumber: record[len(record)-2],
			})
		} else if dealerColumn > 0 && dealerColumn < len(record) {
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

func (s *fileServiceImpl) CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwoMap map[string]Dealer, dealerColumn, dealerNumber, offsetPercentage int) ([][]string, error) {
	results := [][]string{{"Code", "Best Price", "Dealer Number", "Worst Price", "Worst Dealer Number", "Price Ratio"}}

	processedCodes := make(map[string]struct{})

	for _, d1 := range dealerOne {
		code := d1.Code
		bestPrice := d1.Price
		var dealerNum string

		var worstPrice float64
		var worstDealerNum string

		if dealerColumn > 0 {
			dealerNum = d1.Dealer
		} else {
			dealerNum = "1"
		}

		d2, found := dealerTwoMap[code]
		// since some files might have empty codes
		if !found && len(code) > 1 {
			if code[0] == '0' {
				unprefixedCode := code[1:]
				d2, found = dealerTwoMap[unprefixedCode]
			} else {
				prefixedCode := "0" + code
				d2, found = dealerTwoMap[prefixedCode]
			}
		}

		if found {
			if d2.Price < bestPrice {
				if offsetPercentage > 0 {
					priceDifference := ((bestPrice - d2.Price) / bestPrice) * 100

					if priceDifference <= float64(offsetPercentage) {
						worstPrice = bestPrice
					} else {
						worstPrice = bestPrice
						bestPrice = d2.Price

						dealerNum = util.GetDealerNum(dealerNumber)
					}
				} else {
					worstPrice = bestPrice
					bestPrice = d2.Price

					// either 1 or if dealer number from csv
					worstDealerNum = dealerNum

					dealerNum = util.GetDealerNum(dealerNumber)
				}
			} else {
				var worstDealerFileFromFile float64
				worstPriceIsInDealer := d1.WorstPrice != "" && d1.WorstPrice != "N/A"
				if worstPriceIsInDealer {
					worstDealerFileFromFile = parsePrice(d1.WorstPrice)
					// case when error occurs
					if worstDealerFileFromFile == 0 {
						worstDealerFileFromFile = d1.Price
					}
				}

				// Update worst price only if it's better (lower) than the current worst price
				if d2.Price > worstDealerFileFromFile && worstPriceIsInDealer {
					worstDealerNum = d1.WorstDealerNumber
					worstPrice = worstDealerFileFromFile
				} else {
					// If the new price is worse than the best price but better than the current worst, update
					worstDealerNum = util.GetDealerNum(dealerNumber)
					worstPrice = d2.Price
				}
			}

			processedCodes[code] = struct{}{}

			priceRatio := ((worstPrice - bestPrice) / bestPrice) * 100
			pr := fmt.Sprintf("%.0f%%", priceRatio)
			wp := fmt.Sprintf("%.2f", worstPrice)

			results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, wp, worstDealerNum, pr})

		} else {
			// Code not found in dealerTwoMap, append with N/A values
			results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, "N/A", "N/A", "N/A"})
		}
	}

	for code, d2 := range dealerTwoMap {
		if _, found := processedCodes[code]; found {
			continue
		}

		dealerNum := "2"
		if dealerNumber > 0 {
			dealerNum = strconv.Itoa(dealerNumber)
		}

		results = append(results, []string{code, fmt.Sprintf("%.2f", d2.Price), dealerNum, "N/A", "N/A", "N/A"})
	}

	return results, nil
}

func parsePrice(priceStr string) float64 {
	priceStr = strings.Replace(priceStr, "\u00A0", "", -1) // Remove non-breaking spaces
	priceStr = strings.TrimSpace(priceStr)                 // Trim any leading or trailing whitespace
	priceStr = strings.Replace(priceStr, " ", "", -1)      // Remove regular spaces

	commaCount := strings.Count(priceStr, ",")
	if commaCount > 1 {
		priceStr = strings.Replace(priceStr, ",", "", commaCount-1)
	}

	priceStr = strings.Replace(priceStr, ",", ".", -1) // Convert comma to dot for parsing
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0
	}
	return price
}
