package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// Envelope is a small convenience map for ad-hoc JSON responses,
// e.g. httpx.Error uses it to produce {"error": "..."}.
type Envelope map[string]any

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("httpx: failed to encode response: %v", err)
	}
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, Envelope{"error": msg})
}

// DecodeJSON decodes the request body into dst and rejects unknown fields,
// which catches typos in client payloads early instead of silently
// ignoring them.
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
