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

func GenerateOrderID() int {
	rand.Seed(time.Now().UnixNano())
	min := 10000
	max := 99999
	return rand.Intn(max-min+1) + min
}

func ConvertToGMTPlus3(createdDate time.Time) string {
	gmtPlus3 := time.FixedZone("GMT+3", 3*60*60)

	convertedDate := createdDate.In(gmtPlus3)

	humanReadable := convertedDate.Format("2006-01-02 15:04:05")

	return humanReadable
}
