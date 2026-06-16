package config

import (
	"os"
)

type Config struct {
	Port         string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	RedisHost    string
	RedisPort    string
	KafkaBrokers []string
	JaegerEndpoint string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8002"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "urluser"),
		DBPassword:     getEnv("DB_PASSWORD", "urlpassword"),
		DBName:         getEnv("DB_NAME", "urldb"),
		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      getEnv("REDIS_PORT", "6379"),
		KafkaBrokers:   []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:4318/v1/traces"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
