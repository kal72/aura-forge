package domain

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// AppraisalParsedItem holds one item name and quantity extracted from an EVE Online clipboard paste.
type AppraisalParsedItem struct {
	Name     string `json:"name"`
	Quantity int64  `json:"quantity"`
}

// AppraisalLine holds the live valuation for one recognized EVE item in an appraisal.
type AppraisalLine struct {
	TypeID        int32   `json:"type_id"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Quantity      int64   `json:"quantity"`
	UnitVolumeM3  float64 `json:"unit_volume_m3"`
	TotalVolumeM3 float64 `json:"total_volume_m3"`
	UnitBuy       float64 `json:"unit_buy"`
	UnitSell      float64 `json:"unit_sell"`
	TotalBuy      float64 `json:"total_buy"`
	TotalSell     float64 `json:"total_sell"`
}

// AppraisalResult summarizes a multi-item EVE Online bulk price check (Janice/Evepraisal style).
type AppraisalResult struct {
	Hub              TradeHub        `json:"hub"`
	Lines            []AppraisalLine `json:"lines"`
	UnmatchedLines   []string        `json:"unmatched_lines,omitempty"`
	TotalDistinct    int             `json:"total_distinct"`
	TotalUnits       int64           `json:"total_units"`
	TotalVolumeM3    float64         `json:"total_volume_m3"`
	ISKPerM3         float64         `json:"isk_per_m3"`
	TotalSell        float64         `json:"total_sell"`
	NetSellAfterFees float64         `json:"net_sell_after_fees"`
	TotalBuy         float64         `json:"total_buy"`
	NetBuyAfterFees  float64         `json:"net_buy_after_fees"`
	BuybackPercent   float64         `json:"buyback_percent"`
	BuybackValue     float64         `json:"buyback_value"`
	BrokerFeePercent float64         `json:"broker_fee_percent"`
	SalesTaxPercent  float64         `json:"sales_tax_percent"`
	GankRiskWarning  bool            `json:"gank_risk_warning"`
	MultibuyText     string          `json:"multibuy_text"`
}

var (
	rePrefixQty    = regexp.MustCompile(`^([0-9][0-9,\.]*)\s*[xX]\s+(.+)$`)
	reSuffixXQty   = regexp.MustCompile(`^(.+?)\s+[xX]\s*([0-9][0-9,\.]*)$`)
	reSuffixIntQty = regexp.MustCompile(`^(.+?)\s+([0-9][0-9,]*)$`)
	reEFTHeader    = regexp.MustCompile(`^\[([^,\]]+),[^\]]*\]$`)
)

func parseCleanInt64(raw string) (int64, bool) {
	cleaned := strings.NewReplacer(",", "", " ", "", "\u00a0", "", "x", "", "X", "").Replace(strings.TrimSpace(raw))
	if cleaned == "" {
		return 0, false
	}
	// Reject decimals like "12.50" unless pure integer
	if strings.Contains(cleaned, ".") {
		// Handle European thousands separator like "1.000" only if 3 digits after dot; otherwise reject
		parts := strings.Split(cleaned, ".")
		allThree := len(parts) > 1
		for i := 1; i < len(parts); i++ {
			if len(parts[i]) != 3 {
				allThree = false
				break
			}
		}
		if !allThree {
			return 0, false
		}
		cleaned = strings.ReplaceAll(cleaned, ".", "")
	}
	val, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil || val <= 0 {
		return 0, false
	}
	return val, true
}

// ParseEVEClipboard parses pasted text from EVE Online (Inventory/Hangar, Ship Cargo, Loot,
// Multibuy, Contract, or EFT Fitting) into deduplicated item names and quantities.
func ParseEVEClipboard(raw string) ([]AppraisalParsedItem, []string) {
	qtyByLower := make(map[string]int64)
	displayByLower := make(map[string]string)
	order := make([]string, 0)
	var skipped []string

	addItem := func(name string, qty int64) {
		name = strings.TrimSpace(name)
		// Strip trailing "*" often seen in EVE asset/fitting copies
		name = strings.TrimSuffix(name, "*")
		name = strings.TrimSpace(name)
		if name == "" || qty <= 0 {
			return
		}
		key := strings.ToLower(name)
		if _, seen := qtyByLower[key]; !seen {
			order = append(order, key)
			displayByLower[key] = name
		}
		qtyByLower[key] += qty
	}

	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lowerLine := strings.ToLower(line)
		// Skip empty EFT slot placeholders and pure separator lines
		if strings.HasPrefix(lowerLine, "[empty ") || strings.Trim(line, "-=_* ") == "" {
			continue
		}

		// 1. EFT Ship Header: "[Caracal, PvP Fit]" -> Caracal x1
		if m := reEFTHeader.FindStringSubmatch(line); len(m) == 2 {
			addItem(m[1], 1)
			continue
		}

		// 2. Tab-separated (EVE Inventory, Hangar, Ship Cargo, Multibuy, Contract)
		if strings.Contains(line, "\t") {
			cols := strings.Split(line, "\t")
			name := strings.TrimSpace(cols[0])
			if name == "" || strings.EqualFold(name, "name") || strings.EqualFold(name, "item") {
				continue
			}
			qty := int64(1)
			if len(cols) >= 2 {
				if parsedQty, ok := parseCleanInt64(cols[1]); ok {
					qty = parsedQty
				}
			}
			addItem(name, qty)
			continue
		}

		// 3. Prefix quantity: "10x Hobgoblin II" or "10 x Hobgoblin II"
		if m := rePrefixQty.FindStringSubmatch(line); len(m) == 3 {
			if qty, ok := parseCleanInt64(m[1]); ok {
				addItem(m[2], qty)
				continue
			}
		}

		// 4. Suffix "x" quantity: "Hobgoblin II x10" or "Hobgoblin II x 10"
		if m := reSuffixXQty.FindStringSubmatch(line); len(m) == 3 {
			if qty, ok := parseCleanInt64(m[2]); ok {
				addItem(m[1], qty)
				continue
			}
		}

		// 5. EFT module + loaded charge: "Rapid Light Missile Launcher II, Caldari Navy Scourge Light Missile"
		if strings.Contains(line, ", ") {
			parts := strings.SplitN(line, ", ", 2)
			if strings.TrimSpace(parts[0]) != "" {
				addItem(parts[0], 1)
				if strings.TrimSpace(parts[1]) != "" {
					addItem(parts[1], 1)
				}
				continue
			}
		}

		// 6. Space-separated trailing integer: "Tritanium 50,000" (be careful not to eat Roman numerals or "II"/"I")
		if m := reSuffixIntQty.FindStringSubmatch(line); len(m) == 3 {
			if qty, ok := parseCleanInt64(m[2]); ok {
				addItem(m[1], qty)
				continue
			}
		}

		// 7. Single bare item name (qty = 1)
		if len(line) >= 2 {
			addItem(line, 1)
		} else {
			skipped = append(skipped, line)
		}
	}

	out := make([]AppraisalParsedItem, 0, len(order))
	for _, k := range order {
		out = append(out, AppraisalParsedItem{
			Name:     displayByLower[k],
			Quantity: qtyByLower[k],
		})
	}
	return out, skipped
}

// BuildAppraisalResult computes total Sell/Buy valuation, post-tax net values, Buyback split,
// cargo volume, ISK/m3, and gank-risk status across all resolved items.
func BuildAppraisalResult(
	hub TradeHub,
	resolved []SDEType,
	qtyByID map[int32]int64,
	prices map[int32]JitaPrice,
	unmatched []string,
	brokerFeePct, salesTaxPct, buybackPct float64,
) AppraisalResult {
	if buybackPct <= 0 || buybackPct > 100 {
		buybackPct = 90.0
	}
	if brokerFeePct < 0 {
		brokerFeePct = 0
	}
	if salesTaxPct < 0 {
		salesTaxPct = 0
	}

	lines := make([]AppraisalLine, 0, len(resolved))
	var totalUnits int64
	var totalVol, totalSell, totalBuy float64

	for _, t := range resolved {
		qty := qtyByID[t.TypeID]
		if qty <= 0 {
			continue
		}
		jp := prices[t.TypeID]
		unitSell := RoundISK(jp.LowestSell)
		unitBuy := RoundISK(jp.HighestBuy)
		lineSell := RoundISK(unitSell * float64(qty))
		lineBuy := RoundISK(unitBuy * float64(qty))
		lineVol := RoundISK(t.VolumeM3 * float64(qty))

		lines = append(lines, AppraisalLine{
			TypeID:        t.TypeID,
			Name:          t.Name,
			Category:      t.Category,
			Quantity:      qty,
			UnitVolumeM3:  t.VolumeM3,
			TotalVolumeM3: lineVol,
			UnitBuy:       unitBuy,
			UnitSell:      unitSell,
			TotalBuy:      lineBuy,
			TotalSell:     lineSell,
		})

		totalUnits += qty
		totalVol += lineVol
		totalSell += lineSell
		totalBuy += lineBuy
	}

	sort.Slice(lines, func(i, j int) bool {
		if lines[i].TotalSell == lines[j].TotalSell {
			return lines[i].Name < lines[j].Name
		}
		return lines[i].TotalSell > lines[j].TotalSell
	})

	multibuyLines := make([]string, 0, len(lines))
	for _, ln := range lines {
		multibuyLines = append(multibuyLines, fmt.Sprintf("%s\t%d", ln.Name, ln.Quantity))
	}

	totalSell = RoundISK(totalSell)
	totalBuy = RoundISK(totalBuy)
	totalVol = RoundISK(totalVol)

	iskPerM3 := 0.0
	if totalVol > 0 {
		iskPerM3 = RoundISK(totalSell / totalVol)
	}

	netSell := RoundISK(totalSell * math.Max(0, 1.0-(brokerFeePct+salesTaxPct)/100.0))
	netBuy := RoundISK(totalBuy * math.Max(0, 1.0-salesTaxPct/100.0))
	buybackVal := RoundISK(totalBuy * (buybackPct / 100.0))

	return AppraisalResult{
		Hub:              hub,
		Lines:            lines,
		UnmatchedLines:   unmatched,
		TotalDistinct:    len(lines),
		TotalUnits:       totalUnits,
		TotalVolumeM3:    totalVol,
		ISKPerM3:         iskPerM3,
		TotalSell:        totalSell,
		NetSellAfterFees: netSell,
		TotalBuy:         totalBuy,
		NetBuyAfterFees:  netBuy,
		BuybackPercent:   RoundPercent(buybackPct),
		BuybackValue:     buybackVal,
		BrokerFeePercent: brokerFeePct,
		SalesTaxPercent:  salesTaxPct,
		GankRiskWarning:  totalSell >= HighSecGankSafeCapISK,
		MultibuyText:     strings.Join(multibuyLines, "\n"),
	}
}
