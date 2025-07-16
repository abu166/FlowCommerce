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
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// InitRedis initializes the Redis client with configuration
// Change this function signature
func InitRedis(cfg *config.Config) (*redis.Client, error) {
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
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	logger.Info("Successfully connected to Redis")
	return client, nil
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

// GetRedisClient returns the Redis client
func GetRedisClient() *redis.Client {
	return client
}

// FetchDataFromEndpoint fetches price data and includes exchange
func FetchDataFromEndpoint(endpoint string) ([]models.Price, error) {
	const (
		maxRetries  = 3
		dialTimeout = 3 * time.Second
		readTimeout = 30 * time.Second
	)

	var (
		prices  []models.Price
		lastErr error
	)

	// Extract exchange name from endpoint (e.g., "exchange1:40101" -> "exchange1")
	exchange := strings.Split(endpoint, ":")[0]

	for i := 0; i < maxRetries; i++ {
		conn, err := net.DialTimeout("tcp", endpoint, dialTimeout)
		if err != nil {
			lastErr = fmt.Errorf("dial error: %v", err)
			continue
		}
		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(readTimeout))

		if _, err := conn.Write([]byte("GET_PRICES\n")); err != nil {
			lastErr = fmt.Errorf("write error: %v", err)
			continue
		}

		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		for scanner.Scan() {
			var price models.Price
			if err := json.Unmarshal(scanner.Bytes(), &price); err != nil {
				lastErr = fmt.Errorf("invalid JSON: %v", err)
				break
			}
			price.Exchange = exchange
			prices = append(prices, price)
		}

		if err := scanner.Err(); err != nil {
			lastErr = fmt.Errorf("read error: %v", err)
			continue
		}

		if len(prices) > 0 {
			return prices, nil
		}
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("failed after %d attempts. Last error: %v", maxRetries, lastErr)
	}
	return prices, nil
}

// CacheDataInRedis stores price updates in a hash map
func CacheDataInRedis(prices []models.Price) error {
	ctx := context.Background()
	expiration := 70 * time.Second // Keep data for 70s to ensure 60s availability

	pipe := client.Pipeline()
	for _, p := range prices {
		key := fmt.Sprintf("price:%s:%s:updates", p.Exchange, p.Symbol)
		field := strconv.FormatInt(p.Timestamp, 10)
		pipe.HSet(ctx, key, field, p.Price)
		pipe.Expire(ctx, key, expiration)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cache prices: %v", err)
	}

	logger.Infof("Cached %d price updates", len(prices))
	return nil
}

// GetPrice retrieves the latest price for a symbol and exchange
func GetPrice(exchange, symbol string) (*models.Price, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("price:%s:%s:updates", exchange, symbol)
	result, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis error: %v", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("price not found for %s on %s", symbol, exchange)
	}

	// Find the latest price by comparing timestamps
	var latestPrice float64
	var latestTimestamp int64
	for ts, priceStr := range result {
		tsInt, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			continue
		}
		if tsInt > latestTimestamp {
			latestTimestamp = tsInt
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				continue
			}
			latestPrice = price
		}
	}

	if latestTimestamp == 0 {
		return nil, fmt.Errorf("no valid price data found for %s on %s", symbol, exchange)
	}

	return &models.Price{
		Exchange:  exchange,
		Symbol:    symbol,
		Price:     latestPrice,
		Timestamp: latestTimestamp,
	}, nil
}

// GetAllPrices retrieves all cached prices
func GetAllPrices() ([]models.Price, error) {
	ctx := context.Background()
	var prices []models.Price

	keys, err := client.Keys(ctx, "price:*:*:updates").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get price keys: %v", err)
	}

	for _, key := range keys {
		// Extract exchange and symbol from key (e.g., price:exchange1:BTCUSDT:updates)
		parts := strings.Split(key, ":")
		if len(parts) != 4 {
			continue
		}
		exchange, symbol := parts[1], parts[2]
		price, err := GetPrice(exchange, symbol)
		if err != nil {
			logger.Warnf("Failed to get price for %s on %s: %v", symbol, exchange, err)
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
