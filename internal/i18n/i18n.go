package i18n

import (
	"context"
	"strings"
	"time"
)

const (
	// LangEN is the default UI language (English).
	LangEN = "en"
	// LangID is the Indonesian UI language option.
	LangID = "id"
	// CookieName is the HTTP cookie storing the selected UI language.
	CookieName = "af_lang"

	// TZWIB formats timestamps in UTC+7 (Western Indonesian Time / WIB).
	TZWIB = "wib"
	// TZUTC formats timestamps in UTC+0 (EVE Online Server Time).
	TZUTC = "utc"
	// TZCookieName is the HTTP cookie storing the selected display timezone.
	TZCookieName = "af_tz"
)

type ctxKey string

const (
	langCtxKey ctxKey = "af_lang"
	tzCtxKey   ctxKey = "af_tz"
)

var wibLocation = time.FixedZone("UTC+7", 7*60*60)

// Normalize normalizes a raw language string to LangEN or LangID (default LangEN).
func Normalize(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case LangID:
		return LangID
	default:
		return LangEN
	}
}

// WithLang attaches the normalized UI language to ctx.
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langCtxKey, Normalize(lang))
}

// FromContext returns the UI language from ctx (defaults to LangEN).
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return LangEN
	}
	if v, ok := ctx.Value(langCtxKey).(string); ok && v != "" {
		return Normalize(v)
	}
	return LangEN
}

// IsID reports whether the active UI language in ctx is Indonesian.
func IsID(ctx context.Context) bool {
	return FromContext(ctx) == LangID
}

// L returns the Indonesian string when ctx is set to LangID, otherwise the English string.
func L(ctx context.Context, en, id string) string {
	if IsID(ctx) && id != "" {
		return id
	}
	return en
}

// NormalizeTZ normalizes a raw timezone preference to TZWIB or TZUTC (default TZWIB).
func NormalizeTZ(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case TZUTC, "eve", "eve_utc":
		return TZUTC
	default:
		return TZWIB
	}
}

// WithTZ attaches the normalized display timezone preference to ctx.
func WithTZ(ctx context.Context, tz string) context.Context {
	return context.WithValue(ctx, tzCtxKey, NormalizeTZ(tz))
}

// TZFromContext returns the display timezone preference from ctx (defaults to TZWIB).
func TZFromContext(ctx context.Context) string {
	if ctx == nil {
		return TZWIB
	}
	if v, ok := ctx.Value(tzCtxKey).(string); ok && v != "" {
		return NormalizeTZ(v)
	}
	return TZWIB
}

// IsUTC reports whether the active display timezone is EVE Online UTC.
func IsUTC(ctx context.Context) bool {
	return TZFromContext(ctx) == TZUTC
}

// Location returns the *time.Location for the active display timezone in ctx.
func Location(ctx context.Context) *time.Location {
	if IsUTC(ctx) {
		return time.UTC
	}
	return wibLocation
}

// TZLabel returns a short human-readable timezone suffix ("UTC+7" or "EVE UTC").
func TZLabel(ctx context.Context) string {
	if IsUTC(ctx) {
		return "EVE UTC"
	}
	return "UTC+7"
}

// FormatTime formats t in the active display timezone using layout and appends the timezone label.
// Returns "-" when t is zero.
func FormatTime(ctx context.Context, t time.Time, layout string) string {
	if t.IsZero() {
		return "-"
	}
	if layout == "" {
		layout = "02 Jan 2006 15:04"
	}
	return t.In(Location(ctx)).Format(layout) + " " + TZLabel(ctx)
}
