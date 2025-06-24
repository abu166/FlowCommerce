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

	AppEnv string
}

func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{}

	// DB config

	cfg.DB.User = mustGet("DB_USER")
	cfg.DB.Password = mustGet("DB_PASSWORD")
	cfg.DB.Host = mustGet("DB_HOST")
	cfg.DB.Name = mustGet("DB_Name")
	cfg.DB.Port = mustGetInt("DB_PORT")
	cfg.DB.SSLMode = mustGet("DB_SSLMODE")


	cfg.AppEnv = getOrDefault("APP_ENV", "development")

	return cfg
}

func mustGet(key string) string {
	val := os.Getenv(key)

	if val == "" {
		log.Fatalf("Missing required env var: %s", val)
	}

	return val
}

func mustGetInt(key string) int {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Missing required env var: %s", val)
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
