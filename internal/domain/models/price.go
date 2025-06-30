package models

import (
	"time"
)

type Price struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
