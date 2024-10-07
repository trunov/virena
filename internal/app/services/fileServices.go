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
	Code        string
	Price       float64
	Dealer      string
	Description string
}

type FileService interface {
	ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn, descriptionIndex int) ([]Dealer, error)
	ReadFileToMap(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, descriptionIndex int) (map[string]Dealer, error)
	CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwo map[string]Dealer, dealerColumn, dealerNumber, offsetPercentage int, withAdditionalData string) ([][]string, error)
}

type fileServiceImpl struct{}

func NewFileService() FileService {
	return &fileServiceImpl{}
}

func (s *fileServiceImpl) ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn, descriptionIndex int) ([]Dealer, error) {
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

		// Handle cases where optional indices might be missing
		var description string
		if descriptionIndex >= 0 && descriptionIndex < len(record) {
			description = record[descriptionIndex]
		} else {
			description = ""
		}

		if dealerColumn > 0 && dealerColumn < len(record) {
			dealers = append(dealers, Dealer{
				Code:        record[codeIndex],
				Price:       parsePrice(record[priceIndex]),
				Dealer:      record[dealerColumn],
				Description: description,
			})
		} else {
			dealers = append(dealers, Dealer{
				Code:        record[codeIndex],
				Price:       parsePrice(record[priceIndex]),
				Description: description,
			})
		}
	}

	return dealers, nil
}

func (s *fileServiceImpl) ReadFileToMap(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, descriptionIndex int) (map[string]Dealer, error) {
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

		var description string

		if descriptionIndex >= 0 && descriptionIndex < len(record) {
			description = record[descriptionIndex]
		} else {
			description = ""
		}

		dealer := Dealer{
			Code:        record[codeIndex],
			Price:       parsePrice(record[priceIndex]),
			Description: description,
		}

		dealersMap[dealer.Code] = dealer
	}

	return dealersMap, nil
}

func (s *fileServiceImpl) CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwoMap map[string]Dealer, dealerColumn, dealerNumber, offsetPercentage int, withAdditionalData string) ([][]string, error) {
	var results [][]string

	if withAdditionalData == "" {
		results = [][]string{{"Code", "Best Price", "Dealer Number", "Description"}}
	} else {
		results = [][]string{{"Code", "Best Price", "Dealer Number", "Description", "Worst Price", "Price Ratio"}}
	}

	processedCodes := make(map[string]struct{})

	for _, d1 := range dealerOne {
		code := d1.Code
		bestPrice := d1.Price
		var dealerNum string

		var worstPrice float64

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

		// let's try to compare product code from description which represents replacementCode in some cases
		if !found {
			d2, found = dealerTwoMap[d1.Description]
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

					dealerNum = util.GetDealerNum(dealerNumber)
				}
			} else {
				worstPrice = d2.Price
			}

			processedCodes[code] = struct{}{}

			if withAdditionalData == "" {
				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description})
			} else {
				priceRatio := ((worstPrice - bestPrice) / bestPrice) * 100
				pr := fmt.Sprintf("%.0f%%", priceRatio)
				wp := fmt.Sprintf("%.2f", worstPrice)

				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description, wp, pr})
			}
		} else {
			// Code not found in dealerTwoMap, append with N/A values
			if withAdditionalData == "" {
				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description})
			} else {
				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description, "N/A", "N/A"})
			}
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

		if withAdditionalData == "" {
			results = append(results, []string{code, fmt.Sprintf("%.2f", d2.Price), dealerNum, d2.Description})
		} else {
			results = append(results, []string{code, fmt.Sprintf("%.2f", d2.Price), dealerNum, d2.Description, "N/A", "N/A"})
		}
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
		return 0
	}
	return price
}
