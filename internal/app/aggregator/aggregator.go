package aggregator

import (
	"context"
	"marketflow/internal/app/logger"
	"marketflow/internal/models/domain"
	"time"
)

type Aggregator struct {
	Input  <-chan domain.PriceUpdate
	Repo   domain.PriceRepository
	Cache  domain.Cache
	Window time.Duration
}

func NewAggregator(input <-chan domain.PriceUpdate, repo domain.PriceRepository, cache domain.Cache, window time.Duration) *Aggregator {
	return &Aggregator{
		Input:  input,
		Repo:   repo,
		Cache:  cache,
		Window: window,
	}
}

func (a *Aggregator) Start(ctx context.Context) {
	buffer := make(map[string][]float64)
	ticker := time.NewTicker(a.Window)
	cleanTicker := time.NewTicker(5 * time.Minute) // Очистка каждые 5 минут
	defer ticker.Stop()
	defer cleanTicker.Stop()

	logger.Info("starting price aggregator", "window", a.Window)

	for {
		select {
		case <-ctx.Done():
			a.flush(ctx, buffer, time.Now())
			logger.Info("aggregator stopped by context")
			return
		case update, ok := <-a.Input:
			if !ok {
				a.flush(ctx, buffer, time.Now())
				logger.Info("aggregator channel closed, stopping")
				return
			}
			key := update.Exchange + ":" + update.Pair
			buffer[key] = append(buffer[key], update.Price)

		case tickTime := <-ticker.C:
			a.flush(ctx, buffer, tickTime)
			buffer = make(map[string][]float64)
			logger.Info("flushed aggregation buffer", "time", tickTime)

		case <-cleanTicker.C:
			if cache, ok := a.Cache.(interface {
				CleanOld(ctx context.Context, pattern string) error
			}); ok {
				if err := cache.CleanOld(ctx, "latest:*"); err != nil {
					logger.Error("failed to clean old redis keys", "error", err)
				}
				logger.Info("ran cache cleanup")
			}
		}
	}
}

func (a *Aggregator) flush(ctx context.Context, buffer map[string][]float64, ts time.Time) {
	var stats []domain.PriceStats
	for key, prices := range buffer {
		if len(prices) == 0 {
			continue
		}
		parts := splitKey(key)
		exchange, pair := parts[0], parts[1]

		var sum, min, max float64
		min = prices[0]
		max = prices[0]
		for _, p := range prices {
			sum += p
			if p < min {
				min = p
			}
			if p > max {
				max = p
			}
		}
		avg := sum / float64(len(prices))

		stat := domain.PriceStats{
			Exchange:  exchange,
			Pair:      pair,
			Timestamp: ts,
			Average:   avg,
			Min:       min,
			Max:       max,
		}
		stats = append(stats, stat)
		logger.Debug("created stat", "exchange", exchange, "pair", pair, "avg", avg, "min", min, "max", max, "count", len(prices))
	}

	if len(stats) > 0 {
		if err := a.Repo.StoreStatsBatch(stats); err != nil {
			logger.Error("failed to store batch stats", "error", err)
		} else {
			logger.Info("stored batch stats", "count", len(stats))
		}
	}
}

func splitKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{"", key}
}
