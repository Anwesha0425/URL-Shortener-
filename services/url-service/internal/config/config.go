package config

import (
	"os"
)

// Config holds all configuration for the URL Service
type Config struct {
	// Server
	Port string

	// Database (PostgreSQL)
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Cache (Redis)
	RedisHost string
	RedisPort string

	// Message Queue (Kafka)
	KafkaBrokers string

	// Observability
	JaegerEndpoint string

	// App
	BaseURL string
}

// Load reads config from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8001"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "urluser"),
		DBPassword:     getEnv("DB_PASSWORD", "urlpassword"),
		DBName:         getEnv("DB_NAME", "urldb"),
		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      getEnv("REDIS_PORT", "6379"),
		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9092"),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:4318/v1/traces"),
		BaseURL:        getEnv("BASE_URL", "http://localhost:8002"),
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
