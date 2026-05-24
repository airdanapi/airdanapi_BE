package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

func integrationDB(t *testing.T) *sql.DB {
	t.Helper()

	if os.Getenv("INTEGRATION_DB_TEST") != "1" {
		t.Skip("set INTEGRATION_DB_TEST=1 to run MySQL integration tests")
	}

	cfg := config.DatabaseConfig{
		Host:      envOr("DB_HOST", "localhost"),
		Port:      envOr("DB_PORT", "3306"),
		User:      envOr("DB_USER", "root"),
		Password:  envOr("DB_PASS", ""),
		Name:      envOr("DB_NAME", "airdanapi_gateway_test"),
		ParseTime: envOr("DB_PARSE_TIME", "true"),
		Loc:       envOr("DB_LOC", "Local"),
	}

	db, err := OpenMySQL(cfg)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}

	if err := Ping(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("ping mysql: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db.DB
}

func TestRouteRepositoryIntegration(t *testing.T) {
	db := integrationDB(t)
	repo := NewRouteRepository(sqlxFromDB(db))

	route, err := repo.FindActiveByServiceFeatureMethod(context.Background(), "marketplace", "checkout", "POST")
	if err != nil {
		t.Fatalf("find seeded route: %v", err)
	}

	if route.ServiceName != "marketplace" || !route.Transactional {
		t.Fatalf("unexpected route: %+v", route)
	}

	routes, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("list active routes: %v", err)
	}
	if len(routes) < 6 {
		t.Fatalf("expected seeded routes, got %d", len(routes))
	}

	_, err = repo.FindActiveByServiceFeatureMethod(context.Background(), "marketplace", "browse_produk", "GET")
	if err != nil {
		t.Fatalf("find csv marketplace browse route: %v", err)
	}

	_, err = repo.FindActiveByServiceFeatureMethod(context.Background(), "missing", "route", "GET")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRequestLogRepositoryIntegration(t *testing.T) {
	db := integrationDB(t)
	repo := NewRequestLogRepository(sqlxFromDB(db))

	statusCode := 200
	latencyMS := 12
	requestHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	responseHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	log := domain.RequestLog{
		RequestID:    fmt.Sprintf("req-%d", time.Now().UnixNano()),
		TargetApp:    "marketplace",
		Endpoint:     "/marketplace/checkout",
		Method:       "POST",
		StatusCode:   &statusCode,
		LatencyMS:    &latencyMS,
		IPAddress:    "127.0.0.1",
		RequestHash:  &requestHash,
		ResponseHash: &responseHash,
		Lifecycle:    "COMPLETED",
	}

	id, err := repo.Create(context.Background(), log)
	if err != nil {
		t.Fatalf("create request log: %v", err)
	}
	if id == 0 {
		t.Fatal("expected inserted request log id")
	}

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM request_logs WHERE id = ?", id)
	})
}

func TestGatewayFeeRepositoryIntegration(t *testing.T) {
	db := integrationDB(t)
	repo := NewGatewayFeeRepository(sqlxFromDB(db))
	requestID := fmt.Sprintf("fee-%d", time.Now().UnixNano())

	id, err := repo.Create(context.Background(), domain.GatewayFee{
		RequestID:         requestID,
		UserID:            "user_test",
		SourceApp:         "marketplace",
		TransactionAmount: 100000,
		FeeAmount:         500,
		FeeRate:           0.005,
		Status:            "SUCCESS",
		RetryCount:        0,
		MaxRetries:        5,
	})
	if err != nil {
		t.Fatalf("create gateway fee: %v", err)
	}
	if id == 0 {
		t.Fatal("expected inserted gateway fee id")
	}

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM gateway_fees WHERE id = ?", id)
	})

	fee, err := repo.FindByRequestID(context.Background(), requestID)
	if err != nil {
		t.Fatalf("find gateway fee: %v", err)
	}
	if fee.FeeAmount != 500 || fee.Status != "SUCCESS" {
		t.Fatalf("unexpected fee: %+v", fee)
	}
}

func TestJWTBlacklistRepositoryIntegration(t *testing.T) {
	db := integrationDB(t)
	repo := NewJWTBlacklistRepository(sqlxFromDB(db))
	jti := fmt.Sprintf("jti-%d", time.Now().UnixNano())

	result, err := db.Exec(
		"INSERT INTO jwt_blacklist (jti, user_id, reason, expires_at) VALUES (?, ?, ?, DATE_ADD(NOW(), INTERVAL 1 HOUR))",
		jti,
		"user_test",
		"integration test",
	)
	if err != nil {
		t.Fatalf("insert blacklist: %v", err)
	}
	id, _ := result.LastInsertId()
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM jwt_blacklist WHERE id = ?", id)
	})

	exists, err := repo.ExistsActiveJTI(context.Background(), jti)
	if err != nil {
		t.Fatalf("check blacklist: %v", err)
	}
	if !exists {
		t.Fatal("expected active jti to exist")
	}
}

func TestOperatorRepositoryIntegration(t *testing.T) {
	db := integrationDB(t)
	repo := NewOperatorRepository(sqlxFromDB(db))
	email := fmt.Sprintf("operator-%d@example.test", time.Now().UnixNano())

	result, err := db.Exec(
		"INSERT INTO operators (email, password_hash, name, role) VALUES (?, ?, ?, ?)",
		email,
		"hashed-password",
		"Operator Test",
		"Operator",
	)
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	id, _ := result.LastInsertId()
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM operators WHERE id = ?", id)
	})

	operator, err := repo.FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("find operator: %v", err)
	}
	if operator.Email != email || !operator.IsActive {
		t.Fatalf("unexpected operator: %+v", operator)
	}
}

func envOr(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func sqlxFromDB(db *sql.DB) *sqlx.DB {
	return sqlx.NewDb(db, "mysql")
}
