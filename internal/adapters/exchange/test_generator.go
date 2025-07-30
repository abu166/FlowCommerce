package exchange

import (
	"context"
	"math/rand"
	"time"

	"marketflow/internal/models/domain"
)

type TestGenerator struct {
	exchange string
	stopCh   chan struct{}
}

func NewTestGenerator(exchange string) *TestGenerator {
	return &TestGenerator{
		exchange: exchange,
		stopCh:   make(chan struct{}),
	}
}

func (g *TestGenerator) Start(ctx context.Context, out chan<- domain.PriceUpdate) error {
	pairs := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT"}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, pair := range pairs {
				update := domain.PriceUpdate{
					Exchange: g.exchange,
					Pair:     pair,
					Price:    randomPrice(pair),
					Time:     time.Now(),
				}
				out <- update
			}
		case <-g.stopCh:
			return nil
		}
	}
}

func (g *TestGenerator) Stop() error {
	close(g.stopCh)
	return nil
}

func randomPrice(pair string) float64 {
	base := map[string]float64{
		"BTCUSDT":  60000,
		"ETHUSDT":  3000,
		"DOGEUSDT": 0.12,
		"TONUSDT":  5.5,
		"SOLUSDT":  160,
	}[pair]
	return base + rand.Float64()*base*0.02 // ±2%
}
