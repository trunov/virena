package util

type GetProductResponse struct {
	ID          int     `json:"id"`
	Code        string  `json:"code"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Note        string  `json:"note"`
	Weight      float64 `json:"weight"`
}
