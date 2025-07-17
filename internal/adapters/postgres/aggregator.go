package postgres

import (
	"context"
	"fmt"
	"marketflow/internal/app/logger"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Aggregator struct {
	db     *sqlx.DB
	redis  *redis.Client
	ctx    context.Context
	ticker *time.Ticker
}

func NewAggregator(db *sqlx.DB, redis *redis.Client) *Aggregator {
	return &Aggregator{
		db:     db,
		redis:  redis,
		ctx:    context.Background(),
		ticker: time.NewTicker(1 * time.Minute), // 1-minute aggregation
	}
}

func (a *Aggregator) Start() {
	go func() {
		for {
			select {
			case <-a.ticker.C:
				if err := a.aggregateAndStore(); err != nil {
					logger.Error("Failed to aggregate and store data", "error", err)
				}
			case <-a.ctx.Done():
				a.ticker.Stop()
				logger.Info("Aggregator stopped")
				return
			}
		}
	}()
}

func (a *Aggregator) Stop() {
	a.ticker.Stop()
}

func (a *Aggregator) aggregateAndStore() error {
	keys, err := a.redis.Keys(a.ctx, "price:*:*:updates").Result()
	if err != nil {
		return fmt.Errorf("failed to get price keys: %v", err)
	}
	logger.Debug("Found price keys", "count", len(keys))

	type Aggregate struct {
		MinPrice float64
		MaxPrice float64
		SumPrice float64
		Count    int
	}
	aggregates := make(map[string]*Aggregate)

	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) != 4 {
			continue
		}
		exchange, symbol := parts[1], parts[2]

		prices, err := a.redis.HGetAll(a.ctx, key).Result()
		if err != nil {
			logger.Warnf("Failed to get prices for %s: %v", key, err)
			continue
		}

		agg := &Aggregate{MinPrice: 1e9, MaxPrice: -1e9}
		for _, priceStr := range prices {
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				continue
			}
			if price < agg.MinPrice {
				agg.MinPrice = price
			}
			if price > agg.MaxPrice {
				agg.MaxPrice = price
			}
			agg.SumPrice += price
			agg.Count++
		}

		if agg.Count > 0 {
			aggregates[fmt.Sprintf("%s:%s", exchange, symbol)] = agg
		}
	}

	// Store aggregates in PostgreSQL
	tx, err := a.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	for key, agg := range aggregates {
		parts := strings.Split(key, ":")
		exchange, symbol := parts[0], parts[1]
		avgPrice := agg.SumPrice / float64(agg.Count)
		_, err := tx.Exec(`
            INSERT INTO price_aggregates (pair_name, exchange, timestamp, average_price, min_price, max_price)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (pair_name, exchange, timestamp) DO UPDATE
            SET average_price = EXCLUDED.average_price,
                min_price = EXCLUDED.min_price,
                max_price = EXCLUDED.max_price
        `, symbol, exchange, time.Now().Truncate(time.Minute), avgPrice, agg.MinPrice, agg.MaxPrice)
		if err != nil {
			return fmt.Errorf("failed to insert aggregate for %s:%s: %v", exchange, symbol, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	logger.Infof("Stored %d aggregates in PostgreSQL", len(aggregates))
	return nil
}
