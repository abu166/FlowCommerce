package models

import (
	"time"
)

type Price struct {
	Exchange string `json:"exchange"`
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

// ToDBModel converts Price to database model (if needed)
func (p *Price) ToDBModel() *PriceDB {
	return &PriceDB{
		Symbol:    p.Symbol,
		Price:     p.Price,
		Timestamp: time.Unix(0, p.Timestamp),
	}
}

type PriceDB struct {
	Symbol    string
	Price     float64
	Timestamp time.Time
}
