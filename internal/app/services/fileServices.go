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
)

type Dealer struct {
	Code        string
	Price       float64
	Dealer      string
	Description string
	Weight      string
}

type FileService interface {
	ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn, descriptionIndex, weightIndex int) ([]Dealer, error)
	ReadFileToMap(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, descriptionIndex int) (map[string]Dealer, error)
	CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwo map[string]Dealer, dealerColumn, dealerNumber int, withAdditionalData string) ([][]string, error)
}

type fileServiceImpl struct{}

func NewFileService() FileService {
	return &fileServiceImpl{}
}

func (s *fileServiceImpl) ReadFile(ctx context.Context, file multipart.File, delimiter rune, priceIndex, codeIndex, dealerColumn, descriptionIndex, weightIndex int) ([]Dealer, error) {
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
		var description, weight string

		if descriptionIndex >= 0 && descriptionIndex < len(record) {
			description = record[descriptionIndex]
		} else {
			description = ""
		}

		if weightIndex >= 0 && weightIndex < len(record) {
			weight = record[weightIndex]
		} else {
			weight = ""
		}

		if dealerColumn > 0 && dealerColumn < len(record) {
			dealers = append(dealers, Dealer{
				Code:        record[codeIndex],
				Price:       parsePrice(record[priceIndex]),
				Dealer:      record[dealerColumn],
				Description: description,
				Weight:      weight,
			})
		} else {
			dealers = append(dealers, Dealer{
				Code:        record[codeIndex],
				Price:       parsePrice(record[priceIndex]),
				Description: description,
				Weight:      weight,
			})
		}
	}

	fmt.Println(counter)

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

func (s *fileServiceImpl) CompareAndProcessFiles(ctx context.Context, dealerOne []Dealer, dealerTwoMap map[string]Dealer, dealerColumn, dealerNumber int, withAdditionalData string) ([][]string, error) {
	var results [][]string

	if withAdditionalData == "" {
		results = [][]string{{"Code", "Best Price", "Dealer Number", "Description", "Weight"}}
	} else {
		results = [][]string{{"Code", "Best Price", "Dealer Number", "Description", "Weight", "Worst Price", "Price Ratio"}}
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
		if !found {
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
				worstPrice = bestPrice
				bestPrice = d2.Price

				if dealerNumber > 0 {
					dealerNum = strconv.Itoa(dealerNumber)
				} else {
					dealerNum = "2"
				}
			} else {
				worstPrice = d2.Price
			}

			processedCodes[code] = struct{}{}

			if withAdditionalData == "" {
				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description, d1.Weight})
			} else {
				priceRatio := ((worstPrice - bestPrice) / bestPrice) * 100
				pr := fmt.Sprintf("%.0f%%", priceRatio)
				wp := fmt.Sprintf("%.2f", worstPrice)

				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description, d1.Weight, wp, pr})
			}
		} else {
			// Code not found in dealerTwoMap, append with N/A values
			if withAdditionalData == "" {
				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description, d1.Weight})
			} else {
				results = append(results, []string{code, fmt.Sprintf("%.2f", bestPrice), dealerNum, d1.Description, d1.Weight, "N/A", "N/A"})
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
		fmt.Printf("Failed to parse price '%s': %v\n", priceStr, err)
		return 0
	}
	return price
}
