package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Postgres PostgresConfig
	Redis    RedisConfig
	Exchanges        []Exchange
	AggregatorWindow time.Duration
	APIAddr  string
	RedisTTL time.Duration
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host       string
	Port       int
	Password   string
	DB         int
	Expiration string
}

type Exchange struct {
	Name    string
	Address string
}

func Load() (*Config, error) {
	// First check all required variables
	required := map[string]string{
		"DB_USER":          os.Getenv("DB_USER"),
		"DB_PASSWORD":      os.Getenv("DB_PASSOWRD"),
		"DB_HOST":          os.Getenv("DB_HOST"),
		"DB_NAME":          os.Getenv("DB_NAME"),
		"DB_PORT":          os.Getenv("DB_PORT"),
		"DB_SSLMODE":       os.Getenv("DB_SSLMODE"),
		"REDIS_HOST":       os.Getenv("REDIS_HOST"),
		"REDIS_PORT":       os.Getenv("REDIS_PORT"),
		"REDIS_DB":         os.Getenv("REDIS_DB"),
		"REDIS_EXPIRATION": os.Getenv("REDIS_EXPIRATION"),
		"REDIS_TTL":        os.Getenv("REDIS_TTL"),
		"EXCHANGE1_ADDR":    os.Getenv("EXCHANGE1_ADDR"),
		"EXCHANGE2_ADDR":    os.Getenv("EXCHANGE2_ADDR"),
		"EXCHANGE3_ADDR":    os.Getenv("EXCHANGE3_ADDR"),
		"API_ADDR":          os.Getenv("API_ADDR"),
		"AGGREGATOR_WINDOW": os.Getenv("AGGREGATOR_WINDOW"),
	}

	for key, value := range required {
		if value == "" {
			return nil, fmt.Errorf("missing required env variable: %s", key)
		}
	}

	pgPort, err := strconv.Atoi(os.Getenv("PG_PORT"))
	if err != nil {
		return nil, fmt.Errorf("invalid PG_PORT: %w", err)
	}

	redisPort, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_PORT: %w", err)
	}

	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	redisTTL, err := time.ParseDuration(os.Getenv("REDIS_TTL"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_TTL: %w", err)
	}

	aggregatorWindow, err := time.ParseDuration(os.Getenv("AGGREGATOR_WINDOW"))
	if err != nil {
		return nil, fmt.Errorf("invalid AGGREGATOR_WINDOW: %w", err)
	}

	cfg := &Config{
		Postgres: PostgresConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     pgPort,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   os.Getenv("DB_NAME"),
			SSLMode:  os.Getenv("DB_SSLMODE"),
		},
		Redis: RedisConfig{
			Host:       os.Getenv("REDIS_HOST"),
			Port:       redisPort,
			Password:   os.Getenv("REDIS_PASSWORD"),
			DB:         redisDB,
			Expiration: os.Getenv("REDIS_EXPIRATION"),
		},
		Exchanges: []Exchange{
			{Name: "exchange1", Address: os.Getenv("EXCHANGE1_ADDR")},
			{Name: "exchange2", Address: os.Getenv("EXCHANGE2_ADDR")},
			{Name: "exchange3", Address: os.Getenv("EXCHANGE3_ADDR")},
		},
		APIAddr:          os.Getenv("API_ADDR"),
		AggregatorWindow: aggregatorWindow,
		RedisTTL: redisTTL,
	}

	return cfg, nil
}
