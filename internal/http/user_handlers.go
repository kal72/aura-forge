package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"

	"github.com/go-chi/chi/v5"
)

// UserHandlers manages application login, logout, and admin-only user management endpoints.
type UserHandlers struct {
	svc      *service.CopilotService
	authSvc  *service.AuthService
	renderer Renderer
}

func NewUserHandlers(svc *service.CopilotService, authSvc *service.AuthService, renderer Renderer) *UserHandlers {
	return &UserHandlers{
		svc:      svc,
		authSvc:  authSvc,
		renderer: renderer,
	}
}

// HandleLoginPage renders the luxury industrial login page (or redirects to / if already logged in).
func (h *UserHandlers) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	if h.authSvc != nil {
		if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
			if user, err := h.authSvc.ValidateSession(r.Context(), cookie.Value); err == nil && user != nil {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
	}

	if h.renderer == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("aura-forge login"))
		return
	}
	_ = h.renderer.RenderLogin(w, r, "", "")
}

// HandleLoginSubmit validates credentials and sets an HttpOnly session cookie.
func (h *UserHandlers) HandleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if h.authSvc == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	token, _, err := h.authSvc.Authenticate(r.Context(), username, password)
	if err != nil {
		if h.renderer != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = h.renderer.RenderLogin(w, r, err.Error(), username)
			return
		}
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		MaxAge:   7 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleLogout invalidates the current session and redirects to /login.
func (h *UserHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if h.authSvc != nil {
		if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
			_ = h.authSvc.Logout(r.Context(), cookie.Value)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// HandleUsersPage renders the admin-only User Management dashboard.
func (h *UserHandlers) HandleUsersPage(w http.ResponseWriter, r *http.Request) {
	if h.authSvc == nil {
		http.Error(w, "Auth service tidak tersedia", http.StatusServiceUnavailable)
		return
	}
	users, err := h.authSvc.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "Gagal memuat daftar user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	mode := h.svc.GetModeInfo(r.Context())
	if h.renderer == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	_ = h.renderer.RenderUsers(w, r, mode, users, r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// HandleCreateUser creates a new user account (admin-only).
func (h *UserHandlers) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	password := r.FormValue("password")
	role := domain.UserRole(strings.TrimSpace(r.FormValue("role")))

	u, err := h.authSvc.CreateUser(r.Context(), username, displayName, password, role)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/admin/users?err="+err.Error(), http.StatusFound)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Akun '"+u.Username+"' ("+string(u.Role)+") berhasil dibuat.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/admin/users?msg=Akun+berhasil+dibuat", http.StatusFound)
}

// HandleUpdateUser updates a user's display name, role, and active status (admin-only).
func (h *UserHandlers) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "ID user tidak valid", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}

	actor := CurrentUser(r)
	var actorID int64
	if actor != nil {
		actorID = actor.ID
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	role := domain.UserRole(strings.TrimSpace(r.FormValue("role")))
	isActive := r.FormValue("is_active") == "true" || r.FormValue("is_active") == "on" || r.FormValue("is_active") == "1"

	u, err := h.authSvc.UpdateUser(r.Context(), actorID, id, displayName, role, isActive)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/admin/users?err="+err.Error(), http.StatusFound)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Akun '"+u.Username+"' diperbarui.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/admin/users?msg=Akun+diperbarui", http.StatusFound)
}

// HandleResetUserPassword resets a user's password and revokes their active sessions (admin-only).
func (h *UserHandlers) HandleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "ID user tidak valid", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}

	newPassword := r.FormValue("new_password")
	if err := h.authSvc.ResetUserPassword(r.Context(), id, newPassword); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/admin/users?err="+err.Error(), http.StatusFound)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Password berhasil direset dan sesi lama dicabut.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/admin/users?msg=Password+berhasil+direset", http.StatusFound)
}

// HandleDeleteUser permanently deletes a user account (admin-only).
func (h *UserHandlers) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "ID user tidak valid", http.StatusBadRequest)
		return
	}

	actor := CurrentUser(r)
	var actorID int64
	if actor != nil {
		actorID = actor.ID
	}

	if err := h.authSvc.DeleteUser(r.Context(), actorID, id); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/admin/users?err="+err.Error(), http.StatusFound)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "info", "Akun user berhasil dihapus.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/admin/users?msg=Akun+dihapus", http.StatusFound)
}
