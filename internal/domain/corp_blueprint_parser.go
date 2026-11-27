package domain

import (
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reInlineME   = regexp.MustCompile(`(?i)\bME[:\s]*(\d{1,2})%?\b`)
	reInlineTE   = regexp.MustCompile(`(?i)\bTE[:\s]*(\d{1,2})%?\b`)
	reInlineRuns = regexp.MustCompile(`(?i)\b(\d+)\s*runs?\b|\bruns?[:\s]*(\d+)\b`)
	rePctOnly    = regexp.MustCompile(`^(\d{1,2})%$`)
)

// ParsePastedCorpBlueprints parses EVE Online in-game copy-paste text (Ctrl+C from Corp Hangar,
// Assets, Industry Blueprints tab, or plain 1-per-line lists) into CharacterBlueprint entries.
func ParsePastedCorpBlueprints(characterID int64, raw, defaultHangar string, defaultME, defaultTE int) []CharacterBlueprint {
	if defaultME < 0 {
		defaultME = 0
	}
	if defaultME > 10 {
		defaultME = 10
	}
	if defaultTE < 0 {
		defaultTE = 0
	}
	if defaultTE > 20 {
		defaultTE = 20
	}
	defaultHangar = strings.TrimSpace(defaultHangar)
	if defaultHangar == "" {
		defaultHangar = "Corp Hangar 1"
	}

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	now := time.Now()
	out := make([]CharacterBlueprint, 0, len(lines))
	seen := make(map[int64]struct{}, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lowerLine := strings.ToLower(line)
		// Skip common EVE table header rows
		if strings.HasPrefix(lowerLine, "name\t") || strings.HasPrefix(lowerLine, "blueprint\t") ||
			strings.HasPrefix(lowerLine, "item\t") || lowerLine == "name" || lowerLine == "blueprint" {
			continue
		}

		cols := strings.Split(line, "\t")
		name := strings.TrimSpace(cols[0])
		if name == "" {
			continue
		}

		me := defaultME
		te := defaultTE
		runs := -1
		isCopy := false
		hangar := defaultHangar
		pctFound := 0

		// Inspect extra tab-separated columns (EVE Hangar / Industry / Assets format)
		for _, col := range cols[1:] {
			c := strings.TrimSpace(col)
			if c == "" {
				continue
			}
			cl := strings.ToLower(c)
			switch {
			case cl == "copy" || cl == "bpc" || strings.Contains(cl, "blueprint copy"):
				isCopy = true
				if runs <= 0 {
					runs = 1
				}
			case cl == "original" || cl == "bpo" || strings.Contains(cl, "blueprint original"):
				isCopy = false
				runs = -1
			case strings.HasPrefix(cl, "corpsag") || strings.HasPrefix(cl, "corp hangar") || cl == "corpdeliveries":
				hangar = c
			default:
				if m := reInlineME.FindStringSubmatch(c); len(m) == 2 {
					if v, err := strconv.Atoi(m[1]); err == nil && v >= 0 && v <= 10 {
						me = v
					}
				}
				if m := reInlineTE.FindStringSubmatch(c); len(m) == 2 {
					if v, err := strconv.Atoi(m[1]); err == nil && v >= 0 && v <= 20 {
						te = v
					}
				}
				if m := reInlineRuns.FindStringSubmatch(c); len(m) >= 2 {
					numStr := m[1]
					if numStr == "" && len(m) >= 3 {
						numStr = m[2]
					}
					if v, err := strconv.Atoi(numStr); err == nil && v > 0 {
						runs = v
						isCopy = true
					}
				}
				if m := rePctOnly.FindStringSubmatch(c); len(m) == 2 {
					if v, err := strconv.Atoi(m[1]); err == nil {
						if pctFound == 0 && v <= 10 {
							me = v
							pctFound++
						} else if pctFound == 1 && v <= 20 {
							te = v
							pctFound++
						}
					}
				}
			}
		}

		// Also support inline annotations in single-column lines (e.g. "Hobgoblin I Blueprint BPC ME2 TE4 10 runs")
		if m := reInlineME.FindStringSubmatch(name); len(m) == 2 {
			if v, err := strconv.Atoi(m[1]); err == nil && v >= 0 && v <= 10 {
				me = v
			}
			name = reInlineME.ReplaceAllString(name, "")
		}
		if m := reInlineTE.FindStringSubmatch(name); len(m) == 2 {
			if v, err := strconv.Atoi(m[1]); err == nil && v >= 0 && v <= 20 {
				te = v
			}
			name = reInlineTE.ReplaceAllString(name, "")
		}
		if m := reInlineRuns.FindStringSubmatch(name); len(m) >= 2 {
			numStr := m[1]
			if numStr == "" && len(m) >= 3 {
				numStr = m[2]
			}
			if v, err := strconv.Atoi(numStr); err == nil && v > 0 {
				runs = v
				isCopy = true
			}
			name = reInlineRuns.ReplaceAllString(name, "")
		}

		// Check trailing BPC / BPO / Copy / Original tokens in name
		words := strings.Fields(name)
		cleanWords := make([]string, 0, len(words))
		for _, w := range words {
			wl := strings.ToLower(strings.Trim(w, "()[]"))
			switch wl {
			case "bpc", "(copy)":
				isCopy = true
				if runs <= 0 {
					runs = 1
				}
			case "bpo", "(original)":
				isCopy = false
				runs = -1
			default:
				cleanWords = append(cleanWords, w)
			}
		}
		name = strings.TrimSpace(strings.Join(cleanWords, " "))
		// Strip trailing " x1" or " ×1" quantity suffixes
		if idx := strings.LastIndex(name, "×"); idx > 0 {
			if _, err := strconv.Atoi(strings.TrimSpace(name[idx+len("×"):])); err == nil {
				name = strings.TrimSpace(name[:idx])
			}
		}
		if name == "" {
			continue
		}

		if !isCopy {
			runs = -1
		}

		// Deterministic negative synthetic item_id so pasted corp blueprints never clash with real positive EVE item_ids
		h := fnv.New64a()
		_, _ = h.Write([]byte(strconv.FormatInt(characterID, 10) + "|" + strings.ToLower(name) + "|" + hangar + "|" + strconv.Itoa(me) + "|" + strconv.Itoa(te) + "|" + strconv.Itoa(runs)))
		synthID := -int64(h.Sum64()&0x7fffffffffffffff) - 1
		if _, dup := seen[synthID]; dup {
			continue
		}
		seen[synthID] = struct{}{}

		qty := -1
		if isCopy {
			qty = -2
		}

		out = append(out, CharacterBlueprint{
			ItemID:             synthID,
			CharacterID:        characterID,
			TypeName:           name,
			LocationFlag:       hangar,
			Quantity:           qty,
			MaterialEfficiency: me,
			TimeEfficiency:     te,
			Runs:               runs,
			IsCopy:             isCopy,
			IsCorp:             true,
			UpdatedAt:          now,
		})
	}

	return out
}
