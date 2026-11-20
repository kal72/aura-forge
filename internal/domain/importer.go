package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ImportRowError describes a parsing error on a specific input line.
type ImportRowError struct {
	LineNumber int    `json:"line_number"`
	RawLine    string `json:"raw_line"`
	Reason     string `json:"reason"`
}

// ImportResult summarizes a bulk paste / CSV import operation.
type ImportResult struct {
	Items        []MarketItem     `json:"items"`
	Errors       []ImportRowError `json:"errors"`
	TotalLines   int              `json:"total_lines"`
	SuccessCount int              `json:"success_count"`
	ErrorCount   int              `json:"error_count"`
}

// ParseISKNotation parses EVE numeric formats including K, M, B, T suffixes, commas, and ISK labels.
func ParseISKNotation(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-" || strings.EqualFold(s, "n/a") {
		return 0, nil
	}

	s = strings.ReplaceAll(s, "ISK", "")
	s = strings.ReplaceAll(s, "isk", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	if s == "" {
		return 0, fmt.Errorf("empty numeric value")
	}

	multiplier := 1.0
	lastChar := s[len(s)-1]
	switch lastChar {
	case 'k', 'K':
		multiplier = 1_000.0
		s = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1_000_000.0
		s = s[:len(s)-1]
	case 'b', 'B':
		multiplier = 1_000_000_000.0
		s = s[:len(s)-1]
	case 't', 'T':
		multiplier = 1_000_000_000_000.0
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ISK number %q: %w", raw, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("negative value not allowed: %q", raw)
	}

	return RoundISK(val * multiplier), nil
}

// ParsePercentNotation parses percentage strings such as "15.4%" or "15.4".
func ParsePercentNotation(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, "%")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid percentage %q: %w", raw, err)
	}
	return RoundPercent(val), nil
}

// ParseBulkTable parses pasted TSV/CSV/pipe-delimited tables into MarketItem candidates with per-row error reporting.
// Expected column order: Name | SellPrice | MaterialCost | DailyVolume | BatchOutputQty (optional) | Category (optional)
func ParseBulkTable(rawText string) ImportResult {
	result := ImportResult{
		Items:  make([]MarketItem, 0),
		Errors: make([]ImportRowError, 0),
	}

	lines := strings.Split(strings.ReplaceAll(rawText, "\r\n", "\n"), "\n")
	now := time.Now()

	for idx, line := range lines {
		lineNum := idx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		cols := splitTableColumns(trimmed)
		if len(cols) == 0 {
			continue
		}

		// Skip header row
		if isHeaderRow(cols[0]) {
			continue
		}

		result.TotalLines++

		name := strings.TrimSpace(cols[0])
		if name == "" {
			result.Errors = append(result.Errors, ImportRowError{
				LineNumber: lineNum,
				RawLine:    trimmed,
				Reason:     "nama item kosong",
			})
			result.ErrorCount++
			continue
		}

		var sellPrice, matCost, dailyVol float64
		batchQty := 1
		var err error

		if len(cols) >= 2 && strings.TrimSpace(cols[1]) != "" {
			sellPrice, err = ParseISKNotation(cols[1])
			if err != nil {
				result.Errors = append(result.Errors, ImportRowError{
					LineNumber: lineNum,
					RawLine:    trimmed,
					Reason:     fmt.Sprintf("kolom Harga Jual tidak valid (%v)", err),
				})
				result.ErrorCount++
				continue
			}
		}

		if len(cols) >= 3 && strings.TrimSpace(cols[2]) != "" {
			matCost, err = ParseISKNotation(cols[2])
			if err != nil {
				result.Errors = append(result.Errors, ImportRowError{
					LineNumber: lineNum,
					RawLine:    trimmed,
					Reason:     fmt.Sprintf("kolom Biaya Bahan tidak valid (%v)", err),
				})
				result.ErrorCount++
				continue
			}
		}

		if len(cols) >= 4 && strings.TrimSpace(cols[3]) != "" {
			// If column 4 has a '%' sign, ignore as margin column or parse volume
			if !strings.HasSuffix(strings.TrimSpace(cols[3]), "%") {
				dailyVol, err = ParseISKNotation(cols[3])
				if err != nil {
					result.Errors = append(result.Errors, ImportRowError{
						LineNumber: lineNum,
						RawLine:    trimmed,
						Reason:     fmt.Sprintf("kolom Volume Harian tidak valid (%v)", err),
					})
					result.ErrorCount++
					continue
				}
			}
		}

		if len(cols) >= 5 && strings.TrimSpace(cols[4]) != "" {
			qtyFloat, qErr := ParseISKNotation(cols[4])
			if qErr != nil || qtyFloat < 1 {
				result.Errors = append(result.Errors, ImportRowError{
					LineNumber: lineNum,
					RawLine:    trimmed,
					Reason:     fmt.Sprintf("kolom Output Qty harus >= 1 (%q)", cols[4]),
				})
				result.ErrorCount++
				continue
			}
			batchQty = int(qtyFloat)
		}

		category := inferItemCategory(name)
		if len(cols) >= 6 && strings.TrimSpace(cols[5]) != "" {
			category = normalizeCategory(cols[5])
		}

		item := MarketItem{
			Name:             name,
			Category:         category,
			EstimatedJitaSell: sellPrice,
			MaterialCost:     matCost,
			DailyVolume:      dailyVol,
			BatchOutputQty:   batchQty,
			IsManufacturable: !strings.Contains(strings.ToLower(name), "compressed "),
			RequiresRareBPC:  DetectRareBPC(MarketItem{Name: name}),
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		result.Items = append(result.Items, item)
		result.SuccessCount++
	}

	return result
}

func splitTableColumns(line string) []string {
	var sep string
	switch {
	case strings.Contains(line, "\t"):
		sep = "\t"
	case strings.Contains(line, "|"):
		sep = "|"
	case strings.Contains(line, ";"):
		sep = ";"
	default:
		sep = ","
	}
	rawCols := strings.Split(line, sep)
	cols := make([]string, 0, len(rawCols))
	for _, c := range rawCols {
		cols = append(cols, strings.TrimSpace(c))
	}
	return cols
}

func isHeaderRow(firstCol string) bool {
	lower := strings.ToLower(strings.TrimSpace(firstCol))
	return lower == "name" || lower == "item" || lower == "item name" || lower == "type" || lower == "nama" || lower == "nama item"
}

func inferItemCategory(name string) ItemCategory {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "compressed ") || strings.Contains(lower, "ice") || strings.Contains(lower, "ore"):
		return CategoryCommodities
	case strings.Contains(lower, "hobgoblin") || strings.Contains(lower, "hammerhead") || strings.Contains(lower, "warrior") || strings.Contains(lower, "hornet") || strings.Contains(lower, "drone"):
		return CategoryDrones
	case strings.Contains(lower, "rig") || strings.Contains(lower, "extender") || strings.Contains(lower, "trimark") || strings.Contains(lower, "bulkhead"):
		return CategoryRigs
	case strings.Contains(lower, "charge") || strings.Contains(lower, "missile") || strings.Contains(lower, "torpedo") || strings.Contains(lower, "ammo") || strings.Contains(lower, "antimatter") || strings.Contains(lower, "scourge") || strings.Contains(lower, "mjolnir"):
		return CategoryAmmunition
	case strings.Contains(lower, "tristan") || strings.Contains(lower, "kestrel") || strings.Contains(lower, "rifter") || strings.Contains(lower, "punisher") || strings.Contains(lower, "vexor") || strings.Contains(lower, "caracal") || strings.Contains(lower, "catalyst") || strings.Contains(lower, "thrasher"):
		return CategoryShips
	default:
		return CategoryModules
	}
}

func normalizeCategory(raw string) ItemCategory {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ammo", "ammunition", "charges":
		return CategoryAmmunition
	case "drone", "drones":
		return CategoryDrones
	case "ship", "ships":
		return CategoryShips
	case "module", "modules":
		return CategoryModules
	case "rig", "rigs":
		return CategoryRigs
	case "commodity", "commodities":
		return CategoryCommodities
	case "component", "components":
		return CategoryComponents
	default:
		return CategoryOther
	}
}
