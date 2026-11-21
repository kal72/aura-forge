package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aura-forge/internal/domain"
	apphttp "aura-forge/internal/http"
	"aura-forge/internal/service"
	"aura-forge/internal/web"
)

func TestRouterWithAuth_LoginAndRBAC(t *testing.T) {
	ctx := context.Background()
	svc := service.NewCopilotService(nil, nil, nil, nil, nil, nil, nil, nil)
	authSvc := service.NewAuthService(nil)
	_, _ = authSvc.EnsureDefaultAdmin(ctx, "admin", "admin123")
	_, _ = authSvc.CreateUser(ctx, "operator", "Indy Operator", "user123", domain.RoleUser)

	router := apphttp.NewRouterWithAuth(svc, authSvc, web.NewRenderer())

	// 1. Unauthenticated GET / redirects to /login
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Fatalf("expected unauthenticated / to redirect to /login, got %d loc=%q", rec.Code, rec.Header().Get("Location"))
	}

	// 2. GET /login renders luxury industrial login page (no public registration)
	req = httptest.NewRequest(http.MethodGet, "/login", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Account Sign In") {
		t.Fatalf("expected /login to render login page, got %d", rec.Code)
	}

	// 3. Login as regular 'user'
	form := url.Values{"username": {"operator"}, "password": {"user123"}}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("expected user login redirect to /, got %d", rec.Code)
	}
	userCookies := rec.Result().Cookies()
	if len(userCookies) == 0 {
		t.Fatalf("expected session cookie after login")
	}

	// Regular user can access / (and does NOT see User Management menu)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(userCookies[0])
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected user GET / to succeed, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "/admin/users") {
		t.Fatalf("regular user must NOT see User Management menu link")
	}

	// Regular user attempting GET /admin/users gets 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(userCookies[0])
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected regular user on /admin/users to get 403, got %d", rec.Code)
	}

	// 4. Login as 'admin'
	form = url.Values{"username": {"admin"}, "password": {"admin123"}}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected admin login redirect, got %d", rec.Code)
	}
	adminCookies := rec.Result().Cookies()

	// Admin CAN access /admin/users and sees User Management page
	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(adminCookies[0])
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "User Management") {
		t.Fatalf("expected admin GET /admin/users to render 200, got %d", rec.Code)
	}
}
