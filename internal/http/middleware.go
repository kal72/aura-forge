package http

import (
	"context"
	"encoding/json"
	"net/http"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"
)

type ctxKey string

const (
	htmxContextKey    ctxKey = "is_htmx"
	SessionCookieName        = "aura_forge_session"
)

// CurrentUser returns the authenticated AppUser attached to the request context, if any.
func CurrentUser(r *http.Request) *domain.AppUser {
	return domain.AppUserFromContext(r.Context())
}

// RequireAuth enforces an authenticated session when authSvc is configured.
// Unauthenticated browser requests redirect to /login; HTMX requests receive HX-Redirect: /login.
func RequireAuth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authSvc == nil {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(SessionCookieName)
			if err == nil && cookie.Value != "" {
				if user, err := authSvc.ValidateSession(r.Context(), cookie.Value); err == nil && user != nil {
					ctx := domain.WithAppUser(r.Context(), user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			if IsHTMX(r) {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
		})
	}
}

// RequireAdmin restricts a route to authenticated users with the admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := CurrentUser(r)
		if user == nil || !user.IsAdmin() {
			if IsHTMX(r) {
				TriggerToast(w, "error", "Access denied: Administrator privileges required.")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Error(w, "403 Forbidden — Administrator access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
