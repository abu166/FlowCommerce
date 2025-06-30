package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"marketflow/config"
	"marketflow/internal/app/logger"
	"marketflow/internal/domain/models"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// InitRedis initializes the Redis client with configuration
func InitRedis(cfg *config.Config) error {
	client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	logger.Info("Successfully connected to Redis")
	return nil
}

// CloseRedis closes the Redis connection
func CloseRedis() error {
	if client != nil {
		err := client.Close()
		if err != nil {
			return fmt.Errorf("error closing Redis connection: %v", err)
		}
		logger.Info("Redis connection closed")
	}
	return nil
}

// FetchDataFromEndpoint fetches price data from the specified endpoint
func FetchDataFromEndpoint() ([]models.Price, error) {
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var prices []models.Price
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var p models.Price
		if err := decoder.Decode(&p); err != nil {
			logger.Warnf("Failed to decode price data: %v", err)
			continue
		}
		prices = append(prices, p)
	}

	return prices, nil
}

// CacheDataInRedis stores price data in Redis with expiration
func CacheDataInRedis(prices []models.Price) error {
	ctx := context.Background()
	expiration := 30 * time.Minute // Prices expire after 30 minutes

	for _, p := range prices {
		// Store price data as a hash
		err := client.HSet(ctx,
			fmt.Sprintf("price:%s", p.Symbol),
			map[string]interface{}{
				"price":     p.Price,
				"timestamp": p.Timestamp,
				"updated":   time.Now().Unix(),
			},
		).Err()
		if err != nil {
			return fmt.Errorf("failed to cache %s: %v", p.Symbol, err)
		}

		// Set expiration
		err = client.Expire(ctx, fmt.Sprintf("price:%s", p.Symbol), expiration).Err()
		if err != nil {
			return fmt.Errorf("failed to set expiration for %s: %v", p.Symbol, err)
		}
	}

	logger.Infof("Successfully cached %d price updates", len(prices))
	return nil
}

// GetPrice retrieves a price from Redis cache
func GetPrice(symbol string) (*models.Price, error) {
	ctx := context.Background()
	result, err := client.HGetAll(ctx, fmt.Sprintf("price:%s", symbol)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get price for %s: %v", symbol, err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no price data found for %s", symbol)
	}

	price := &models.Price{Symbol: symbol}
	if price.Price, err = parseFloat(result["price"]); err != nil {
		return nil, err
	}
	if price.Timestamp, err = parseInt(result["timestamp"]); err != nil {
		return nil, err
	}

	return price, nil
}

// GetAllPrices retrieves all cached prices
func GetAllPrices() ([]models.Price, error) {
	ctx := context.Background()
	var prices []models.Price

	keys, err := client.Keys(ctx, "price:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get price keys: %v", err)
	}

	for _, key := range keys {
		symbol := key[6:] // Remove "price:" prefix
		price, err := GetPrice(symbol)
		if err != nil {
			logger.Warnf("Failed to get price for %s: %v", symbol, err)
			continue
		}
		prices = append(prices, *price)
	}

	return prices, nil
}

// Helper functions for parsing
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseInt(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
