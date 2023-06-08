package util

type GetProductResponse struct {
	Code        string   `json:"code"`
	Price       float64  `json:"price"`
	Description *string  `json:"description"`
	Note        *string  `json:"note"`
	Weight      *float64 `json:"weight"`
}
