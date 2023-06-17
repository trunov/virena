package util

import (
	"fmt"
	"strconv"
	"strings"
)

type GetProductResponse struct {
	Code               string  `json:"code"`
	// OriginaalosadPrice float64 `json:"originaalosadPrice"`
	RonaxPrice         float64 `json:"ronaxPrice"`
}

func FormatCodeAndPrice(data string, decimalPlaces int) (string, float64, error) {
	parts := strings.Split(data, ";")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid data format")
	}

	code := parts[0]
	priceString := parts[1]

	priceString = strings.ReplaceAll(priceString, " ", "")
	ronaxPrice, err := convertToFloat(priceString, decimalPlaces)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse price: %v", err)
	}

	return code, ronaxPrice, nil
}

func convertToFloat(number string, decimalPlaces int) (float64, error) {
	// Convert the number string to an integer
	num, err := strconv.Atoi(number)
	if err != nil {
		return 0, fmt.Errorf("failed to convert number to integer: %v", err)
	}

	// Calculate the divisor based on the decimal places
	divisor := float64(10)
	for i := 1; i < decimalPlaces; i++ {
		divisor *= 10
	}

	// Divide the number by the divisor to get the float value
	result := float64(num) / divisor
	return result, nil
}
