package response

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Envelope struct {
	Success   bool        `json:"success"`
	RequestID string      `json:"request_id"`
	Data      any         `json:"data,omitempty"`
	Error     *ErrorBody  `json:"error,omitempty"`
	Timestamp string      `json:"timestamp"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func Success(ctx context.Context, data any) Envelope {
	return Envelope{
		Success:   true,
		RequestID: requestIDFromContext(ctx),
		Data:      data,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func Error(ctx context.Context, code, message string, status int) Envelope {
	return Envelope{
		Success:   false,
		RequestID: requestIDFromContext(ctx),
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Status:  status,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func WriteJSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, ok := ctx.Value("request_id").(string)
	if !ok || requestID == "" {
		return ""
	}

	return requestID
}
