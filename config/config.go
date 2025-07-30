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
		RedisTTL: redisTTL,
	}

	return cfg, nil
}
