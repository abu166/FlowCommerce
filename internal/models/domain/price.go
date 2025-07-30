package domain

import "time"

type PriceUpdate struct {
	Exchange string
	Pair     string
	Price    float64
	Time     time.Time
}

type PriceStats struct {
	Exchange  string
	Pair      string
	Timestamp time.Time
	Average   float64
	Min       float64
	Max       float64
}
