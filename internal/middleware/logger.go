package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"

	"github.com/rs/zerolog/log"
)

func LifecycleLogger(logRepo repository.RequestLogRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			requestHash := hashRequestBody(r)
			requestID := RequestIDFromContext(r.Context())

			writeLog(r, logRepo, domain.RequestLog{
				RequestID:       requestID,
				ParentRequestID: optionalString(r.Header.Get("X-Parent-Request-Id")),
				UserID:          optionalString(r.Header.Get("X-User-Id")),
				SourceApp:       optionalString(r.Header.Get("X-Source-App")),
				TargetApp:       targetApp(r.URL.Path),
				Endpoint:        r.URL.Path,
				Method:          r.Method,
				IPAddress:       clientIP(r),
				RequestHash:     requestHash,
				Lifecycle:       "STARTED",
			})

			recorder := newResponseRecorder(w)
			defer func() {
				if recovered := recover(); recovered != nil {
					statusCode := http.StatusInternalServerError
					latencyMS := int(time.Since(startedAt).Milliseconds())
					errorMessage := "panic recovered"
					writeLog(r, logRepo, domain.RequestLog{
						RequestID:       requestID,
						ParentRequestID: optionalString(r.Header.Get("X-Parent-Request-Id")),
						UserID:          optionalString(r.Header.Get("X-User-Id")),
						SourceApp:       optionalString(r.Header.Get("X-Source-App")),
						TargetApp:       targetApp(r.URL.Path),
						Endpoint:        r.URL.Path,
						Method:          r.Method,
						StatusCode:      &statusCode,
						LatencyMS:       &latencyMS,
						IPAddress:       clientIP(r),
						RequestHash:     requestHash,
						ResponseHash:    hashBytes(recorder.body.Bytes()),
						Lifecycle:       "FAILED",
						ErrorMessage:    &errorMessage,
					})
					panic(recovered)
				}
			}()

			next.ServeHTTP(recorder, r)

			statusCode := recorder.statusCode
			latencyMS := int(time.Since(startedAt).Milliseconds())
			lifecycle := "COMPLETED"
			var errorMessage *string
			if statusCode >= http.StatusBadRequest {
				lifecycle = "FAILED"
				errorMessage = optionalString(http.StatusText(statusCode))
			}

			writeLog(r, logRepo, domain.RequestLog{
				RequestID:       requestID,
				ParentRequestID: optionalString(r.Header.Get("X-Parent-Request-Id")),
				UserID:          optionalString(r.Header.Get("X-User-Id")),
				SourceApp:       optionalString(r.Header.Get("X-Source-App")),
				TargetApp:       targetApp(r.URL.Path),
				Endpoint:        r.URL.Path,
				Method:          r.Method,
				StatusCode:      &statusCode,
				LatencyMS:       &latencyMS,
				IPAddress:       clientIP(r),
				RequestHash:     requestHash,
				ResponseHash:    hashBytes(recorder.body.Bytes()),
				Lifecycle:       lifecycle,
				ErrorMessage:    errorMessage,
			})
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	r.body.Write(body)
	return r.ResponseWriter.Write(body)
}

func hashRequestBody(r *http.Request) *string {
	if r.Body == nil {
		return nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Warn().Err(err).Msg("request body could not be read for hashing")
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return hashBytes(body)
}

func hashBytes(body []byte) *string {
	if len(body) == 0 {
		return nil
	}

	sum := sha256.Sum256(body)
	value := hex.EncodeToString(sum[:])
	return &value
}

func writeLog(r *http.Request, repo repository.RequestLogRepository, entry domain.RequestLog) {
	if repo == nil {
		return
	}

	if _, err := repo.Create(r.Context(), entry); err != nil {
		log.Warn().Err(err).Str("request_id", entry.RequestID).Msg("request lifecycle log insert failed")
	}
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func targetApp(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "gateway"
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" {
		return parts[2]
	}
	return parts[0]
}

func clientIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}
