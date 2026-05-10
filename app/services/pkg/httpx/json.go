// Package httpx hosts small HTTP helpers used by every service.
package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as JSON with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error":{"type":..., "message":...}} at the given status.
func Error(w http.ResponseWriter, status int, kind, message string) {
	JSON(w, status, map[string]any{
		"error": map[string]string{"type": kind, "message": message},
	})
}

// DecodeJSON reads the request body into v with a sane size limit.
// Returns false (and writes a 400) if decoding fails.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	const maxBody = 1 << 20 // 1 MiB
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return false
	}
	return true
}
