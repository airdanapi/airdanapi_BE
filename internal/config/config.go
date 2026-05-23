package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env     string
	Port    string
	Name    string
	Version string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Env:     envOrDefault("APP_ENV", "development"),
		Port:    envOrDefault("APP_PORT", "8080"),
		Name:    envOrDefault("APP_NAME", "airdanapi-integrator"),
		Version: envOrDefault("APP_VERSION", "0.1.0"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
