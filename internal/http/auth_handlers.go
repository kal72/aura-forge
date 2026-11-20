package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"aura-forge/internal/service"
)

const ssoStateCookieName = "aura_forge_sso_state"

// AuthHandlers manages optional EVE SSO OAuth2 login, callback, disconnect, and manual sync endpoints.
type AuthHandlers struct {
	svc *service.CopilotService
}

func NewAuthHandlers(svc *service.CopilotService) *AuthHandlers {
	return &AuthHandlers{svc: svc}
}

// HandleLogin redirects to EVE SSO if configured, or returns a clear guidance message if running in Standard Public ESI Mode.
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	if !mode.SSOEnabled {
		msg := "EVE SSO belum dikonfigurasi. Aplikasi sedang berjalan dalam Standard Mode (Public ESI + Manual). Untuk mengaktifkan Auto-Sync Karakter, isi ESI_CLIENT_ID dan ESI_CLIENT_SECRET di file .env lalu restart server."
		if IsHTMX(r) {
			TriggerToast(w, "info", msg)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<div class="rounded-lg border border-amber-500/40 bg-amber-950/40 p-4 text-sm text-amber-200">` + msg + `</div>`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sso_enabled": false,
			"mode":        "PUBLIC_ESI_STANDARD",
			"message":     msg,
		})
		return
	}

	state := generateRandomState()
	authURL, err := h.svc.GetSSOAuthURL(state)
	if err != nil {
		http.Error(w, "Gagal membuat URL login EVE SSO: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     ssoStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback validates OAuth2 state, exchanges the code, and redirects to Settings.
func (h *AuthHandlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	if !mode.SSOEnabled {
		http.Redirect(w, r, "/settings?sso=disabled", http.StatusFound)
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	cookie, err := r.Cookie(ssoStateCookieName)
	if err != nil || cookie.Value == "" || cookie.Value != state {
		http.Error(w, "OAuth2 state tidak valid atau kedaluwarsa. Silakan ulangi login EVE SSO.", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     ssoStateCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	})

	if _, err := h.svc.HandleSSOCallback(r.Context(), code); err != nil {
		http.Error(w, "Gagal menyelesaikan login EVE SSO: "+err.Error(), http.StatusBadGateway)
		return
	}

	http.Redirect(w, r, "/settings?sso=connected", http.StatusFound)
}

// HandleDisconnect removes the stored EVE SSO token and returns to Standard Mode.
func (h *AuthHandlers) HandleDisconnect(w http.ResponseWriter, r *http.Request) {
	_ = h.svc.DisconnectSSO(r.Context())
	if IsHTMX(r) {
		TriggerToast(w, "info", "EVE SSO diputus. Kembali ke Standard Mode (Public ESI + Manual).")
		w.Header().Set("HX-Redirect", "/settings")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings?sso=disconnected", http.StatusFound)
}

// HandleSync triggers an on-demand character wallet, orders, jobs, blueprints, and assets sync.
func (h *AuthHandlers) HandleSync(w http.ResponseWriter, r *http.Request) {
	profile, err := h.svc.SyncCharacterSSO(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Sync Karakter gagal: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Karakter "+profile.CharacterName+" berhasil disinkronkan dari EVE ESI.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

func generateRandomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "aura-forge-state"
	}
	return hex.EncodeToString(b)
}
