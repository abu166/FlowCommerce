package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"marketflow/config"
	"marketflow/internal/app/logger"
	"marketflow/internal/domain/models"
	"net"
	"os"
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
    exchangeAddr := os.Getenv("EXCHANGE_ADDR") // Just "exchange:40101"
    if exchangeAddr == "" {
        exchangeAddr = "exchange:40101"
    }

    maxRetries := 3
    initialTimeout := 5 * time.Second
    maxTimeout := 15 * time.Second
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        timeout := initialTimeout + (time.Duration(i) * 5 * time.Second)
        if timeout > maxTimeout {
            timeout = maxTimeout
        }

        conn, err := net.DialTimeout("tcp", exchangeAddr, timeout)
        if err != nil {
            lastErr = fmt.Errorf("dial failed (attempt %d): %v", i+1, err)
            time.Sleep(1 * time.Second)
            continue
        }

        // Set connection deadlines
        conn.SetDeadline(time.Now().Add(timeout))

        // Simple protocol: send request and read until EOF
        if _, err := conn.Write([]byte("GET_PRICES\n")); err != nil {
            conn.Close()
            lastErr = fmt.Errorf("write failed (attempt %d): %v", i+1, err)
            time.Sleep(1 * time.Second)
            continue
        }

        data, err := io.ReadAll(conn)
        conn.Close()

        if err != nil {
            lastErr = fmt.Errorf("read failed (attempt %d): %v", i+1, err)
            time.Sleep(1 * time.Second)
            continue
        }

        var prices []models.Price
        if err := json.Unmarshal(data, &prices); err != nil {
            lastErr = fmt.Errorf("decode failed (attempt %d): %v", i+1, err)
            continue
        }

        return prices, nil
    }

    return nil, fmt.Errorf("after %d attempts, last error: %v", maxRetries, lastErr)
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
