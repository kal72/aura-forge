package http

import (
	"net/http"
	"net/url"
	"strings"

	"aura-forge/internal/i18n"
)

const prefsCookieMaxAge = 365 * 24 * 60 * 60 // 1 year

// HandleSetLanguage persists the selected UI language (en/id) in a cookie and redirects back safely.
func (h *Handlers) HandleSetLanguage(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	lang := i18n.Normalize(r.FormValue("lang"))
	http.SetCookie(w, &http.Cookie{
		Name:     i18n.CookieName,
		Value:    lang,
		Path:     "/",
		MaxAge:   prefsCookieMaxAge,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, safeLocalRedirect(r), http.StatusSeeOther)
}

// HandleSetTimezone persists the selected display timezone (wib/utc) in a cookie and redirects back safely.
func (h *Handlers) HandleSetTimezone(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	tz := i18n.NormalizeTZ(r.FormValue("tz"))
	http.SetCookie(w, &http.Cookie{
		Name:     i18n.TZCookieName,
		Value:    tz,
		Path:     "/",
		MaxAge:   prefsCookieMaxAge,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, safeLocalRedirect(r), http.StatusSeeOther)
}

// safeLocalRedirect resolves a safe same-origin redirect path from next form field or HTTP Referer.
func safeLocalRedirect(r *http.Request) string {
	if next := strings.TrimSpace(r.FormValue("next")); strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	if ref := strings.TrimSpace(r.Referer()); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Path != "" && strings.HasPrefix(u.Path, "/") {
			if u.RawQuery != "" {
				return u.Path + "?" + u.RawQuery
			}
			return u.Path
		}
	}
	return "/"
}
