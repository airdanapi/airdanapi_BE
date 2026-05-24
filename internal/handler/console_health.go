package handler

import (
	"context"
	"net/http"

	"airdanapi-be/internal/domain"
)

type ConsoleHealthRouteRepository interface {
	ListAll(ctx context.Context) ([]domain.Route, error)
}

type ConsoleHealthHandler struct {
	routes ConsoleHealthRouteRepository
}

func NewConsoleHealthHandler(routes ConsoleHealthRouteRepository) ConsoleHealthHandler {
	return ConsoleHealthHandler{routes: routes}
}

func (h ConsoleHealthHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route repository is unavailable")
		return
	}

	routes, err := h.routes.ListAll(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "service health could not be queried")
		return
	}

	services := map[string]map[string]interface{}{}
	for _, route := range routes {
		if _, exists := services[route.ServiceName]; !exists {
			services[route.ServiceName] = map[string]interface{}{
				"service_name":  route.ServiceName,
				"status":        "CONFIGURED",
				"circuit_state": "UNKNOWN",
				"routes":        0,
			}
		}
		services[route.ServiceName]["routes"] = services[route.ServiceName]["routes"].(int) + 1
	}

	items := make([]map[string]interface{}, 0, len(services))
	for _, service := range services {
		items = append(items, service)
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{"items": items})
}
