package i18n

import (
	"context"
	"testing"
	"time"
)

func TestLanguageContextAndHelper(t *testing.T) {
	if got := Normalize(""); got != LangEN {
		t.Fatalf("expected default %q, got %q", LangEN, got)
	}
	if got := Normalize("ID"); got != LangID {
		t.Fatalf("expected %q, got %q", LangID, got)
	}
	if got := Normalize("fr"); got != LangEN {
		t.Fatalf("expected fallback %q, got %q", LangEN, got)
	}

	ctx := context.Background()
	if IsID(ctx) {
		t.Fatalf("expected default context to be EN")
	}
	if got := L(ctx, "Units", "Unit"); got != "Units" {
		t.Fatalf("expected EN string, got %q", got)
	}

	ctxID := WithLang(ctx, "id")
	if !IsID(ctxID) {
		t.Fatalf("expected ID context")
	}
	if got := L(ctxID, "Units", "Unit"); got != "Unit" {
		t.Fatalf("expected ID string, got %q", got)
	}
	if got := L(ctxID, "Units", ""); got != "Units" {
		t.Fatalf("expected fallback to EN when ID string is empty, got %q", got)
	}
}

func TestTimezoneContextAndFormat(t *testing.T) {
	if got := NormalizeTZ(""); got != TZWIB {
		t.Fatalf("expected default %q, got %q", TZWIB, got)
	}
	if got := NormalizeTZ("utc"); got != TZUTC {
		t.Fatalf("expected %q, got %q", TZUTC, got)
	}
	if got := NormalizeTZ("EVE"); got != TZUTC {
		t.Fatalf("expected %q for EVE alias, got %q", TZUTC, got)
	}

	sample := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC)

	ctxWIB := WithTZ(context.Background(), TZWIB)
	if got := FormatTime(ctxWIB, sample, "02 Jan 2006 15:04"); got != "27 Nov 2026 17:30 UTC+7" {
		t.Fatalf("unexpected WIB formatted time: %q", got)
	}

	ctxUTC := WithTZ(context.Background(), TZUTC)
	if got := FormatTime(ctxUTC, sample, "02 Jan 2006 15:04"); got != "27 Nov 2026 10:30 EVE UTC" {
		t.Fatalf("unexpected UTC formatted time: %q", got)
	}

	if got := FormatTime(ctxWIB, time.Time{}, ""); got != "-" {
		t.Fatalf("expected '-' for zero time, got %q", got)
	}
}
