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

	// Regular user can access / (Research Dashboard — NO admin menus, NO financial KPIs, NO Sync Jita Prices button)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(userCookies[0])
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected user GET / to succeed, got %d", rec.Code)
	}
	userDashBody := rec.Body.String()
	for _, forbidden := range []string{"/admin/users", "/settings", "/catalog", "/wealth", "/allocator", "Sync Jita Prices", "Available Capital"} {
		if strings.Contains(userDashBody, forbidden) {
			t.Fatalf("regular user dashboard must NOT contain %q", forbidden)
		}
	}

	// Regular user CAN access research tools
	for _, allowed := range []string{"/marketplace", "/scanner", "/calculator", "/invention", "/pi", "/wiki", "/esi/minerals"} {
		req = httptest.NewRequest(http.MethodGet, allowed, nil)
		req.AddCookie(userCookies[0])
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected regular user on %s to get 200, got %d", allowed, rec.Code)
		}
	}

	// Regular user attempting admin-only pages or write actions gets 403 Forbidden
	forbiddenRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/users"},
		{http.MethodGet, "/settings"},
		{http.MethodGet, "/catalog"},
		{http.MethodGet, "/allocator"},
		{http.MethodGet, "/planner"},
		{http.MethodGet, "/tracker"},
		{http.MethodGet, "/wealth"},
		{http.MethodPost, "/esi/sync-prices"},
		{http.MethodPost, "/settings/sde/import"},
		{http.MethodPost, "/marketplace/add-catalog"},
		{http.MethodPost, "/calculator/blueprint/save-catalog"},
		{http.MethodPost, "/pi/colonies"},
	}
	for _, fr := range forbiddenRoutes {
		req = httptest.NewRequest(fr.method, fr.path, nil)
		req.AddCookie(userCookies[0])
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected regular user on %s %s to get 403, got %d", fr.method, fr.path, rec.Code)
		}
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

	// Admin CAN access /admin/users, /settings, /catalog, /wealth and sees full Dashboard
	for _, adminPath := range []string{"/", "/admin/users", "/settings", "/catalog", "/allocator", "/planner", "/tracker", "/wealth"} {
		req = httptest.NewRequest(http.MethodGet, adminPath, nil)
		req.AddCookie(adminCookies[0])
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected admin GET %s to render 200, got %d", adminPath, rec.Code)
		}
	}
}
