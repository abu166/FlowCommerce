package cache

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"marketflow/config"
	"marketflow/internal/app/logger"
	"marketflow/internal/domain/models"
	"net"
	"strconv"
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

func FetchDataFromEndpoint() ([]models.Price, error) {
	conn, err := net.DialTimeout("tcp", "exchange:40101", 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	scanner := bufio.NewScanner(conn)
	var prices []models.Price

	for scanner.Scan() {
		var price models.Price
		if err := json.Unmarshal(scanner.Bytes(), &price); err != nil {
			return nil, fmt.Errorf("invalid JSON: %v", err)
		}
		prices = append(prices, price)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no valid prices received")
	}

	return prices, nil
}

// CacheDataInRedis - improved version
func CacheDataInRedis(prices []models.Price) error {
	ctx := context.Background()
	expiration := 30 * time.Minute

	pipe := client.Pipeline()
	for _, p := range prices {
		pipe.HSet(ctx,
			fmt.Sprintf("price:%s", p.Symbol),
			map[string]interface{}{
				"price":     p.Price,
				"timestamp": p.Timestamp, // Stored as int64
				"updated":   time.Now().Unix(),
			},
		)
		pipe.Expire(ctx, fmt.Sprintf("price:%s", p.Symbol), expiration)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cache prices: %v", err)
	}

	logger.Infof("Cached %d price updates", len(prices))
	return nil
}

// GetPrice - improved with better type handling
func GetPrice(symbol string) (*models.Price, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := client.HGetAll(ctx, fmt.Sprintf("price:%s", symbol)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis error: %v", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("price not found for %s", symbol)
	}

	price := &models.Price{Symbol: symbol}

	// Better float parsing
	if price.Price, err = strconv.ParseFloat(result["price"], 64); err != nil {
		return nil, fmt.Errorf("invalid price format: %v", err)
	}

	// Better int64 parsing
	if price.Timestamp, err = strconv.ParseInt(result["timestamp"], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid timestamp: %v", err)
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
