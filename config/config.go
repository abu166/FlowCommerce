package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port int

	DB struct {
		User     string
		Password string	
		Host     string
		Name     string
		Port     int
		SSLMode  string
	}

	Redis struct {
		Host       string
		Port       int
		Password   string
		DB         int
		Expiration int
	}

	AppEnv string
}

func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{}

	// First check all required variables
	required := map[string]string{
		"DB_USER":     "Database user",
		"DB_PASSWORD": "Database password",
		"DB_HOST":     "Database host",
		"DB_NAME":     "Database name",
		"DB_PORT":     "Database port",
	}

	for envVar, desc := range required {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Missing required environment variable: %s (%s)", envVar, desc)
		}
	}

	// DB config
	cfg.DB.User = os.Getenv("DB_USER")
	cfg.DB.Password = os.Getenv("DB_PASSWORD")
	cfg.DB.Host = os.Getenv("DB_HOST")
	cfg.DB.Name = os.Getenv("DB_NAME") 
	cfg.DB.Port = mustGetInt("DB_PORT")
	cfg.DB.SSLMode = getOrDefault("DB_SSLMODE", "disable")

	// Redis config
	cfg.Redis.Host = getOrDefault("REDIS_HOST", "redis")
	cfg.Redis.Port = getOrDefaultInt("REDIS_PORT", 6379)
	cfg.Redis.Password = getOrDefault("REDIS_PASSWORD", "")
	cfg.Redis.DB = getOrDefaultInt("REDIS_DB", 0)
	cfg.Redis.Expiration = getOrDefaultInt("REDIS_EXPIRATION", 3600)

	cfg.AppEnv = getOrDefault("APP_ENV", "development")

	return cfg
}

func mustGetInt(key string) int {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Missing required env var: %s", key) // Fixed to show key
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("Invalid integer value for %s: %s", key, val)
	}
	return n
}

func getOrDefault(key string, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func getOrDefaultInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Invalid integer value for %s: %s, using default %d", key, val, def)
		return def
	}
	return n
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"`)
		os.Setenv(key, val)
	}
}