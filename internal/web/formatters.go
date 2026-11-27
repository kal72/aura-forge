package web

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"aura-forge/internal/domain"
	"aura-forge/internal/i18n"
)

// FormatISK formats an ISK value into compact human-readable form (e.g. 125.50M ISK, 450.0K ISK, 1.25B ISK).
func FormatISK(val float64) string {
	abs := math.Abs(val)
	sign := ""
	if val < 0 {
		sign = "-"
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%s%.2fB ISK", sign, abs/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%s%.2fM ISK", sign, abs/1_000_000)
	case abs >= 10_000:
		return fmt.Sprintf("%s%.1fK ISK", sign, abs/1_000)
	default:
		return fmt.Sprintf("%s%.2f ISK", sign, abs)
	}
}

// FormatISKExact formats an ISK value with thousand separators.
func FormatISKExact(val float64) string {
	sign := ""
	if val < 0 {
		sign = "-"
		val = -val
	}
	whole := int64(val)
	frac := int64(math.Round((val - float64(whole)) * 100))
	if frac == 100 {
		whole++
		frac = 0
	}
	s := fmt.Sprintf("%d", whole)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return fmt.Sprintf("%s%s.%02d ISK", sign, strings.Join(parts, ","), frac)
}

// FormatVolume formats daily sales volume clearly without cryptic abbreviations.
func FormatVolume(val float64) string {
	switch {
	case val >= 1_000_000:
		return fmt.Sprintf("%.2fM units/day", val/1_000_000)
	case val >= 1_000:
		return fmt.Sprintf("%.1fK units/day", val/1_000)
	default:
		return fmt.Sprintf("%.0f units/day", val)
	}
}

// FormatVolumeCtx formats daily sales volume according to the active UI language (units/day or unit/hari).
func FormatVolumeCtx(ctx context.Context, val float64) string {
	unitSuffix := i18n.L(ctx, "units/day", "unit/hari")
	switch {
	case val >= 1_000_000:
		return fmt.Sprintf("%.2fM %s", val/1_000_000, unitSuffix)
	case val >= 1_000:
		return fmt.Sprintf("%.1fK %s", val/1_000, unitSuffix)
	default:
		return fmt.Sprintf("%.0f %s", val, unitSuffix)
	}
}

// FormatTimeCtx formats a timestamp in the user's selected timezone (UTC+7 or EVE UTC).
func FormatTimeCtx(ctx context.Context, t time.Time, layout string) string {
	return i18n.FormatTime(ctx, t, layout)
}

// FormatPercent formats percentage with 2 decimal places.
func FormatPercent(val float64) string {
	return fmt.Sprintf("%.2f%%", val)
}

// TrapBadgeClass returns Tailwind badge styles for TrapStatus.
func TrapBadgeClass(status domain.TrapStatus) string {
	switch status {
	case domain.TrapStatusLayak:
		return "bg-emerald-500/15 text-emerald-300 border border-emerald-500/40"
	case domain.TrapStatusWaspadaModal:
		return "bg-amber-500/15 text-amber-300 border border-amber-500/40"
	case domain.TrapStatusWaspadaNiche:
		return "bg-violet-500/15 text-sky-300 border border-violet-500/40"
	case domain.TrapStatusJebakan:
		return "bg-rose-500/20 text-rose-300 border border-rose-500/50"
	default:
		return "bg-slate-700/40 text-slate-300 border border-slate-600/40"
	}
}

// TrapStatusLabel returns a concise English UI label for TrapStatus while preserving database enum values.
func TrapStatusLabel(status domain.TrapStatus) string {
	switch status {
	case domain.TrapStatusLayak:
		return "VIABLE"
	case domain.TrapStatusWaspadaModal:
		return "CAPITAL RISK"
	case domain.TrapStatusWaspadaNiche:
		return "WATCHLIST"
	case domain.TrapStatusJebakan:
		return "TRAP"
	default:
		return string(status)
	}
}

// BatchStatusBadgeClass returns Tailwind badge styles for BatchStatus.
func BatchStatusBadgeClass(status domain.BatchStatus) string {
	switch status {
	case domain.BatchStatusDraft:
		return "bg-slate-700/50 text-slate-200 border border-slate-500/40"
	case domain.BatchStatusInProduction:
		return "bg-amber-500/15 text-amber-300 border border-amber-500/40"
	case domain.BatchStatusListed:
		return "bg-violet-500/15 text-sky-300 border border-violet-500/40"
	case domain.BatchStatusSold:
		return "bg-emerald-500/15 text-emerald-300 border border-emerald-500/40"
	case domain.BatchStatusSlowMover:
		return "bg-rose-500/20 text-rose-300 border border-rose-500/50"
	default:
		return "bg-slate-700/50 text-slate-300"
	}
}

// NavLinkUpperClass highlights sidebar links inside the Royal Industrial Faction Banner.
func NavLinkUpperClass(activeNav, target string) string {
	if activeNav == target {
		return "flex items-center gap-3 px-2.5 py-2 rounded-md text-xs font-semibold tracking-wide bg-white/15 text-white border border-amber-300/30 border-l-4 border-l-amber-300 shadow-xs"
	}
	return "flex items-center gap-3 px-2.5 py-2 rounded-md text-xs font-medium tracking-wide text-violet-100/85 hover:text-white hover:bg-white/10 border border-transparent transition"
}

// NavLinkClass highlights sidebar links inside the lower 45% neutral zone (Day & Night adaptive).
func NavLinkClass(activeNav, target string) string {
	if activeNav == target {
		return "flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold tracking-wide bg-violet-500/12 text-violet-700 dark:text-violet-300 border border-violet-500/30 border-l-4 border-l-violet-600 dark:border-l-violet-400 shadow-xs"
	}
	return "flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-medium tracking-wide text-slate-700 dark:text-slate-300 hover:text-slate-950 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/70 border border-transparent transition"
}

// EVEItemIconURL returns the official CCP EVE Image Server icon URL for an inventory type_id.
func EVEItemIconURL(typeID int32, size int) string {
	if typeID <= 0 {
		return ""
	}
	if size <= 0 {
		size = 64
	}
	return fmt.Sprintf("https://images.evetech.net/types/%d/icon?size=%d", typeID, size)
}

// EVEBlueprintIconURL returns the official CCP EVE Image Server blueprint icon URL.
func EVEBlueprintIconURL(bpTypeID int32, size int) string {
	if bpTypeID <= 0 {
		return ""
	}
	if size <= 0 {
		size = 64
	}
	return fmt.Sprintf("https://images.evetech.net/types/%d/bp?size=%d", bpTypeID, size)
}
