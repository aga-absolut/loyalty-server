package model

import "time"

type Credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type ListOrders struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Balance struct {
	Current   float64 `json:"current"`
	WithDrawn int     `json:"withdrawn"`
}

type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type WithdrawResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type Sum struct {
	Sum float64 `json:"sum"`
}

type AccrualResponse struct {
	Order   string `json:"order"`
	Status  string `json:"status"`
	Accrual int    `json:"accrual,omitempty"`
}

type TypeForChannel struct {
	OrderNum string
	User     string
}
