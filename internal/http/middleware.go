package http

import (
	"context"
	"encoding/json"
	"net/http"
)

type ctxKey string

const htmxContextKey ctxKey = "is_htmx"

// IsHTMX returns true if the incoming HTTP request was initiated by HTMX.
func IsHTMX(r *http.Request) bool {
	if v, ok := r.Context().Value(htmxContextKey).(bool); ok {
		return v
	}
	return r.Header.Get("HX-Request") == "true"
}

// HTMXMiddleware detects HX-Request headers and sets Vary: HX-Request for safe caching.
func HTMXMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isHX := r.Header.Get("HX-Request") == "true"
		w.Header().Add("Vary", "HX-Request")
		ctx := context.WithValue(r.Context(), htmxContextKey, isHX)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SecurityHeadersMiddleware adds baseline HTTP security headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// TriggerToast sets an HX-Trigger header so Alpine.js displays a toast notification on HTMX responses.
func TriggerToast(w http.ResponseWriter, level, message string) {
	payload := map[string]any{
		"showToast": map[string]string{
			"level":   level,
			"message": message,
		},
	}
	if b, err := json.Marshal(payload); err == nil {
		w.Header().Set("HX-Trigger", string(b))
	}
}
