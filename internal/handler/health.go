package handler

import (
	"net/http"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/response"
)

type HealthHandler struct {
	cfg config.Config
}

func NewHealthHandler(cfg config.Config) HealthHandler {
	return HealthHandler{cfg: cfg}
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
