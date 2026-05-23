package handler

import (
	"net/http"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/response"

	"github.com/jmoiron/sqlx"
)

type HealthHandler struct {
	cfg config.Config
	db  *sqlx.DB
}

func NewHealthHandler(cfg config.Config, db *sqlx.DB) HealthHandler {
	return HealthHandler{cfg: cfg, db: db}
}

func (h HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, r, http.StatusOK, map[string]string{
		"status":  "ok",
		"name":    h.cfg.Name,
		"env":     h.cfg.Env,
		"version": h.cfg.Version,
	})
}

func (h HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.db != nil {
		if err := repository.Ping(r.Context(), h.db); err != nil {
			WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database ping failed")
			return
		}
	}

	WriteSuccess(w, r, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

func WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	response.WriteJSON(w, status, response.Success(r.Context(), data))
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	response.WriteJSON(w, status, response.Error(r.Context(), code, message, status))
}
