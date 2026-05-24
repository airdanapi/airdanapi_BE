package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/handler"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()

	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	db := openOptionalDB(cfg)
	if db != nil {
		defer db.Close()
	}

	router := NewRouter(cfg, db)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Info().
		Str("env", cfg.Env).
		Str("port", cfg.Port).
		Str("name", cfg.Name).
		Msg("starting integrator gateway")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server stopped unexpectedly")
	}
}

func NewRouter(cfg config.Config, db *sqlx.DB) http.Handler {
	r := chi.NewRouter()
	r.Use(recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins))

	var logRepo repository.RequestLogRepository
	var blacklistRepo repository.JWTBlacklistRepository
	var routeRepo repository.RouteRepository
	var feeRepo repository.GatewayFeeRepository
	var operatorRepo repository.OperatorRepository
	var consoleRouteRepo handler.ConsoleRouteRepository
	var consoleLogRepo handler.DashboardLogRepository
	var consoleFeeRepo handler.ConsoleFeeRepository
	if db != nil {
		mysqlLogRepo := repository.NewRequestLogRepository(db)
		mysqlRouteRepo := repository.NewRouteRepository(db)
		mysqlFeeRepo := repository.NewGatewayFeeRepository(db)
		mysqlOperatorRepo := repository.NewOperatorRepository(db)

		logRepo = mysqlLogRepo
		blacklistRepo = repository.NewJWTBlacklistRepository(db)
		routeRepo = mysqlRouteRepo
		feeRepo = mysqlFeeRepo
		operatorRepo = mysqlOperatorRepo
		consoleRouteRepo = mysqlRouteRepo
		consoleLogRepo = mysqlLogRepo
		consoleFeeRepo = mysqlFeeRepo
	}

	r.Use(middleware.LifecycleLogger(logRepo))

	authService, err := service.NewAuthService(cfg.Auth, blacklistRepo)
	if err != nil {
		log.Fatal().Err(err).Msg("jwt auth service could not be initialized")
	}
	smartBankClient := service.NewSmartBankClient(cfg.SmartBank)
	feeService := service.NewFeeService(cfg.Fee, feeRepo, smartBankClient)
	feeService.StartRetryWorker(context.Background())
	protectionService := service.NewProtectionService(cfg.Protection)
	consoleSessions := service.NewConsoleSessionStore(8 * time.Hour)

	health := handler.NewHealthHandler(cfg, db)
	r.Get("/health", health.Health)
	r.Get("/ready", health.Ready)

	validation := handler.NewValidationHandler(routeRepo)
	routing := handler.NewRoutingHandler(routeRepo, &feeService, protectionService)
	fees := handler.NewFeeHandler(feeRepo, feeService)
	logging := handler.NewLoggingHandler(logRepo)
	consoleAuth := handler.NewConsoleAuthHandler(operatorRepo, consoleSessions)
	consoleDashboard := handler.NewConsoleDashboardHandler(consoleLogRepo, consoleFeeRepo)
	consoleRoutes := handler.NewConsoleRoutesHandler(consoleRouteRepo)
	consoleHealth := handler.NewConsoleHealthHandler(consoleRouteRepo)
	consoleFees := handler.NewConsoleFeesHandler(consoleFeeRepo)

	r.Post("/console/auth/login", consoleAuth.Login)
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.AuthRequired(authService))
		protected.Post("/integrator/validasi_request", validation.ValidateRequest)
		protected.Post("/integrator/routing_api", routing.Envelope)
		protected.Get("/integrator/logging", logging.List)
		protected.Get("/integrator/biaya_layanan_integrasi", fees.List)
		protected.Post("/integrator/biaya_layanan_integrasi/retry/{id}", fees.Retry)
		protected.MethodFunc(http.MethodGet, "/api/v1/{service}/{feature}", routing.Transparent)
		protected.MethodFunc(http.MethodPost, "/api/v1/{service}/{feature}", routing.Transparent)
		protected.MethodFunc(http.MethodPut, "/api/v1/{service}/{feature}", routing.Transparent)
		protected.MethodFunc(http.MethodPatch, "/api/v1/{service}/{feature}", routing.Transparent)
		protected.MethodFunc(http.MethodDelete, "/api/v1/{service}/{feature}", routing.Transparent)
	})
	r.Group(func(console chi.Router) {
		console.Use(middleware.ConsoleAuthRequired(consoleSessions))
		console.Post("/console/auth/logout", consoleAuth.Logout)
		console.Get("/console/auth/me", consoleAuth.Me)
		console.Get("/console/dashboard/summary", consoleDashboard.Summary)
		console.Get("/console/dashboard/throughput", consoleDashboard.Throughput)
		console.Get("/console/routes", consoleRoutes.List)
		console.Post("/console/routes", consoleRoutes.Create)
		console.Put("/console/routes/{id}", consoleRoutes.Update)
		console.Patch("/console/routes/{id}/toggle", consoleRoutes.Toggle)
		console.Get("/console/services/health", consoleHealth.List)
		console.Get("/console/fees/summary", consoleFees.Summary)
		console.Get("/console/fees/pending", consoleFees.Pending)
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handler.WriteError(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", fmt.Sprintf("route %s %s was not found", r.Method, r.URL.Path))
	})

	return r
}

func openOptionalDB(cfg config.Config) *sqlx.DB {
	if !cfg.DB.Configured() {
		log.Warn().Msg("database config incomplete; readiness will not check database")
		return nil
	}

	db, err := repository.OpenMySQL(cfg.DB)
	if err != nil {
		log.Warn().Err(err).Msg("database connection not opened; readiness will not check database")
		return nil
	}

	if err := repository.Ping(context.Background(), db); err != nil {
		_ = db.Close()
		log.Warn().Err(err).Msg("database ping failed; readiness will not check database")
		return nil
	}

	return db
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error().Interface("panic", recovered).Msg("panic recovered")
				handler.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
