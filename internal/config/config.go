package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env        string
	Port       string
	Name       string
	Version    string
	DB         DatabaseConfig
	Auth       AuthConfig
	CORS       CORSConfig
	SmartBank  SmartBankConfig
	Fee        FeeConfig
	Protection ProtectionConfig
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

type AuthConfig struct {
	Issuer           string
	Audience         string
	PublicKeyPEM     string
	ClockSkewSeconds int
}

type CORSConfig struct {
	AllowedOrigins string
}

type SmartBankConfig struct {
	BaseURL   string
	Mode      string
	TimeoutMS int
}

type FeeConfig struct {
	RevenueUser string
	Rate        float64
}

type ProtectionConfig struct {
	ReadRateLimitPerMinute          int
	TransactionalRateLimitPerMinute int
	TransactionCooldownSeconds      int
	TransactionDailyLimit           int
	IdempotencyTTLHours             int
	CircuitOpenSeconds              int
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
		Auth: AuthConfig{
			Issuer:           envOrDefault("JWT_ISSUER", "smartbank"),
			Audience:         envOrDefault("JWT_AUDIENCE", "ecosystem"),
			PublicKeyPEM:     envOrDefault("JWT_PUBLIC_KEY_PEM", ""),
			ClockSkewSeconds: intEnvOrDefault("JWT_CLOCK_SKEW_SECONDS", 30),
		},
		CORS: CORSConfig{
			AllowedOrigins: envOrDefault("CORS_ALLOWED_ORIGINS", "*"),
		},
		SmartBank: SmartBankConfig{
			BaseURL:   envOrDefault("SMARTBANK_BASE_URL", "http://localhost:8101"),
			Mode:      envOrDefault("SMARTBANK_MODE", "mock_success"),
			TimeoutMS: intEnvOrDefault("SMARTBANK_TIMEOUT_MS", 5000),
		},
		Fee: FeeConfig{
			RevenueUser: envOrDefault("GATEWAY_REVENUE_USER", "GATEWAY_REVENUE"),
			Rate:        floatEnvOrDefault("GATEWAY_FEE_RATE", 0.005),
		},
		Protection: ProtectionConfig{
			ReadRateLimitPerMinute:          intEnvOrDefault("RATE_LIMIT_READ_PER_MINUTE", 60),
			TransactionalRateLimitPerMinute: intEnvOrDefault("RATE_LIMIT_TRANSACTIONAL_PER_MINUTE", 10),
			TransactionCooldownSeconds:      intEnvOrDefault("TRANSACTION_COOLDOWN_SECONDS", 10),
			TransactionDailyLimit:           intEnvOrDefault("TRANSACTION_DAILY_LIMIT", 10),
			IdempotencyTTLHours:             intEnvOrDefault("IDEMPOTENCY_TTL_HOURS", 24),
			CircuitOpenSeconds:              intEnvOrDefault("CIRCUIT_OPEN_SECONDS", 60),
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

func intEnvOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}

	return parsed
}

func floatEnvOrDefault(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return fallback
	}

	return parsed
}
