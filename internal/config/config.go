package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env     string
	Port    string
	Name    string
	Version string
	DB      DatabaseConfig
}

type DatabaseConfig struct {
	Host      string
	Port      string
	User      string
	Password  string
	Name      string
	ParseTime string
	Loc       string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Env:     envOrDefault("APP_ENV", "development"),
		Port:    envOrDefault("APP_PORT", "8080"),
		Name:    envOrDefault("APP_NAME", "airdanapi-integrator"),
		Version: envOrDefault("APP_VERSION", "0.1.0"),
		DB: DatabaseConfig{
			Host:      envOrDefault("DB_HOST", "localhost"),
			Port:      envOrDefault("DB_PORT", "3306"),
			User:      envOrDefault("DB_USER", "root"),
			Password:  envOrDefault("DB_PASS", ""),
			Name:      envOrDefault("DB_NAME", "airdanapi_gateway"),
			ParseTime: envOrDefault("DB_PARSE_TIME", "true"),
			Loc:       envOrDefault("DB_LOC", "Local"),
		},
	}
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=%s&loc=%s&multiStatements=true",
		d.User,
		d.Password,
		d.Host,
		d.Port,
		d.Name,
		d.ParseTime,
		url.QueryEscape(d.Loc),
	)
}

func (d DatabaseConfig) Configured() bool {
	return d.Host != "" && d.Port != "" && d.User != "" && d.Name != ""
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
