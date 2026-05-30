package handler

import (
	"net/http"

	"airdanapi-be/internal/config"
)

type ConsoleAdminHandler struct {
	cfg config.Config
}

func NewConsoleAdminHandler(cfg config.Config) ConsoleAdminHandler {
	return ConsoleAdminHandler{cfg: cfg}
}

func (h ConsoleAdminHandler) SecuritySummary(w http.ResponseWriter, r *http.Request) {
	keyMode := "missing"
	if h.cfg.Auth.PublicKeyPEM != "" {
		keyMode = "configured"
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"issuer":             h.cfg.Auth.Issuer,
		"audience":           h.cfg.Auth.Audience,
		"clock_skew_seconds": h.cfg.Auth.ClockSkewSeconds,
		"signing_key_mode":   keyMode,
		"key_rotation_owner": "SmartBank",
		"roles": []map[string]string{
			{"name": "AdminFull", "description": "Full console access for routing, finance, and security operations."},
			{"name": "Operator", "description": "Operational monitoring and route maintenance."},
			{"name": "FinanceAuditor", "description": "Read access for fees, revenue, and reconciliation."},
			{"name": "ReadOnlyViewer", "description": "Read-only access to dashboard and monitoring data."},
		},
		"scopes": []map[string]string{
			{"name": "admin:read", "description": "Read operational logs, fees, routes, health, and console summaries."},
			{"name": "admin:write", "description": "Retry gateway fees and perform administrative mutations."},
			{"name": "marketplace:write", "description": "Forward write requests to Marketplace transactional routes."},
			{"name": "pos:write", "description": "Forward write requests to POS transactional routes."},
			{"name": "supplierhub:write", "description": "Forward write requests to SupplierHub transactional routes."},
			{"name": "logistikita:write", "description": "Forward write requests to LogistiKita transactional routes."},
			{"name": "umkm_insight:read", "description": "Forward read requests to UMKM Insight analytics routes."},
		},
		"blacklist": map[string]interface{}{
			"mode":        "mysql",
			"management":  "read_only",
			"description": "JWT blacklist is enforced by the backend; console CRUD is outside Sprint 7C.",
		},
	})
}

func (h ConsoleAdminHandler) ConfigDefaults(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"app": map[string]interface{}{
			"env":     h.cfg.Env,
			"name":    h.cfg.Name,
			"version": h.cfg.Version,
			"port":    h.cfg.Port,
		},
		"smartbank": map[string]interface{}{
			"base_url":   h.cfg.SmartBank.BaseURL,
			"mode":       h.cfg.SmartBank.Mode,
			"timeout_ms": h.cfg.SmartBank.TimeoutMS,
		},
		"fee": map[string]interface{}{
			"rate":         h.cfg.Fee.Rate,
			"revenue_user": h.cfg.Fee.RevenueUser,
		},
		"protection": map[string]interface{}{
			"read_rate_limit_per_minute":          h.cfg.Protection.ReadRateLimitPerMinute,
			"transactional_rate_limit_per_minute": h.cfg.Protection.TransactionalRateLimitPerMinute,
			"transaction_cooldown_seconds":        h.cfg.Protection.TransactionCooldownSeconds,
			"transaction_daily_limit":             h.cfg.Protection.TransactionDailyLimit,
			"idempotency_ttl_hours":               h.cfg.Protection.IdempotencyTTLHours,
			"circuit_open_seconds":                h.cfg.Protection.CircuitOpenSeconds,
		},
		"logging": map[string]interface{}{
			"request_lifecycle": []string{"STARTED", "COMPLETED", "FAILED"},
			"body_storage":      "sha256_hash",
			"persistence":       "mysql_best_effort",
		},
		"cors": map[string]interface{}{
			"allowed_origins": h.cfg.CORS.AllowedOrigins,
		},
	})
}
