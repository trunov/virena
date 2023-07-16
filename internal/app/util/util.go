package util

import (
	"math/rand"
	"time"
)

type GetProductResponse struct {
	Code        string   `json:"code"`
	Price       float64  `json:"price"`
	Description *string  `json:"description"`
	Note        *string  `json:"note"`
	Weight      *float64 `json:"weight"`
	Brand       string   `json:"brand"`
}

type BrandPercentageMap map[string]float64

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateOrderID(length int) string {
	return StringWithCharset(length, charset)
}
