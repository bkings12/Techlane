package apierrors

import (
	"encoding/json"
	"net/http"
)

type Body struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code          string         `json:"code"`
	Message       string         `json:"message"`
	Details       []FieldError   `json:"details,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func Write(w http.ResponseWriter, status int, code, message, correlationID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Body{Error: ErrorBody{
		Code:          code,
		Message:       message,
		CorrelationID: correlationID,
	}})
}

func WriteDetails(w http.ResponseWriter, status int, code, message, correlationID string, details []FieldError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Body{Error: ErrorBody{
		Code:          code,
		Message:       message,
		Details:       details,
		CorrelationID: correlationID,
	}})
}
