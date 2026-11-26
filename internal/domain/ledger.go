package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TransactionSide identifies whether a market transaction is a purchase (BUY) or sale (SELL).
type TransactionSide string

const (
	TransactionSideBuy  TransactionSide = "BUY"
	TransactionSideSell TransactionSide = "SELL"
)

// TransactionSource identifies how a ledger entry entered aura-forge.
type TransactionSource string

const (
	TransactionSourceESI    TransactionSource = "ESI"
	TransactionSourcePaste  TransactionSource = "PASTE"
	TransactionSourceManual TransactionSource = "MANUAL"
)

// MarketTransaction represents a single realized buy or sell execution.
type MarketTransaction struct {
	ID           int64             `json:"id"`
	CharacterID  int64             `json:"character_id"`
	ExternalTxID *int64            `json:"external_tx_id,omitempty"`
	DedupHash    string            `json:"dedup_hash"`
	TransactedAt time.Time         `json:"transacted_at"`
	Side         TransactionSide   `json:"side"`
	TypeID       int32             `json:"type_id"`
	ItemName     string            `json:"item_name"`
	Quantity     int               `json:"quantity"`
	UnitPrice    float64           `json:"unit_price"`
	TotalISK     float64           `json:"total_isk"`
	EstimatedFee float64           `json:"estimated_fee"`
	LocationID   int64             `json:"location_id"`
	HubName      string            `json:"hub_name"`
	Source       TransactionSource `json:"source"`
	Notes        string            `json:"notes"`
	IsExcluded   bool              `json:"is_excluded"`
	CreatedAt    time.Time         `json:"created_at"`
}

// WalletFeeEntry records an actual market/industry/PI fee entry from Private ESI wallet/journal.
type WalletFeeEntry struct {
	ID          int64     `json:"id"`
	CharacterID int64     `json:"character_id"`
	JournalID   int64     `json:"journal_id"`
	RecordedAt  time.Time `json:"recorded_at"`
	RefType     string    `json:"ref_type"` // brokers_fee, transaction_tax, industry_job_tax, planetary_import_tax, planetary_export_tax
	AmountISK   float64   `json:"amount_isk"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// CharacterOrderHistory records a completed, expired, or cancelled market order from Private ESI orders/history.
type CharacterOrderHistory struct {
	OrderID      int64     `json:"order_id"`
	CharacterID  int64     `json:"character_id"`
	TypeID       int32     `json:"type_id"`
	TypeName     string    `json:"type_name"`
	IsBuyOrder   bool      `json:"is_buy_order"`
	Price        float64   `json:"price"`
	VolumeTotal  int       `json:"volume_total"`
	VolumeRemain int       `json:"volume_remain"`
	State        string    `json:"state"` // expired | cancelled
	RegionID     int32     `json:"region_id"`
	LocationID   int64     `json:"location_id"`
	IssuedAt     time.Time `json:"issued_at"`
	DurationDays int       `json:"duration_days"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FilledUnits returns how many units were filled before the order ended.
func (h CharacterOrderHistory) FilledUnits() int {
	filled := h.VolumeTotal - h.VolumeRemain
	if filled < 0 {
		return 0
	}
	return filled
}

// FillPercent returns the fill completion percentage (0..100).
func (h CharacterOrderHistory) FillPercent() float64 {
	if h.VolumeTotal <= 0 {
		return 0
	}
	return RoundPercent((float64(h.FilledUnits()) / float64(h.VolumeTotal)) * 100.0)
}

// ItemPnLClass classifies realized item trading/manufacturing performance.
type ItemPnLClass string

const (
	ItemPnLCashCow     ItemPnLClass = "CASH_COW"
	ItemPnLHealthy     ItemPnLClass = "HEALTHY"
	ItemPnLThinMargin  ItemPnLClass = "THIN_MARGIN"
	ItemPnLLossMaker   ItemPnLClass = "LOSS_MAKER"
	ItemPnLAccumulate  ItemPnLClass = "ACCUMULATING"
)

// ItemPnLSummary holds realized profit & weighted-average cost basis metrics for a single item.
type ItemPnLSummary struct {
	TypeID            int32        `json:"type_id"`
	ItemName          string       `json:"item_name"`
	QtyBought         int          `json:"qty_bought"`
	TotalBuyISK       float64      `json:"total_buy_isk"`
	AvgBuyPrice       float64      `json:"avg_buy_price"`
	QtySold           int          `json:"qty_sold"`
	TotalSellISK      float64      `json:"total_sell_isk"`
	AvgSellPrice      float64      `json:"avg_sell_price"`
	RealizedCostISK   float64      `json:"realized_cost_isk"`
	RealizedFeesISK   float64      `json:"realized_fees_isk"`
	RealizedProfitISK float64      `json:"realized_profit_isk"`
	RealizedROIPct    float64      `json:"realized_roi_pct"`
	OpenUnits         int          `json:"open_units"`
	OpenCostBasisISK  float64      `json:"open_cost_basis_isk"`
	Classification    ItemPnLClass `json:"classification"`
	LastActivityAt    time.Time    `json:"last_activity_at"`
}

// FeeLeakSummary summarizes actual or estimated taxes & fees paid in the selected period.
type FeeLeakSummary struct {
	BrokerAndRelistISK float64 `json:"broker_and_relist_isk"`
	SalesTaxISK        float64 `json:"sales_tax_isk"`
	IndustryJobTaxISK  float64 `json:"industry_job_tax_isk"`
	PITaxISK           float64 `json:"pi_tax_isk"`
	EstimatedTxFeesISK float64 `json:"estimated_tx_fees_isk"`
	TotalFeesISK       float64 `json:"total_fees_isk"`
	FeeToSalesPct      float64 `json:"fee_to_sales_pct"`
	UsingJournalData   bool    `json:"using_journal_data"`
	LeakWarning        string  `json:"leak_warning,omitempty"`
}

// OrderExecutionSummary aggregates live active orders and historical order fill rates.
type OrderExecutionSummary struct {
	ActiveOrdersCount    int     `json:"active_orders_count"`
	ActiveSellOrders     int     `json:"active_sell_orders"`
	ActiveBuyOrders      int     `json:"active_buy_orders"`
	ActiveSellValueISK   float64 `json:"active_sell_value_isk"`
	ActiveBuyEscrowISK   float64 `json:"active_buy_escrow_isk"`
	UndercutActiveCount  int     `json:"undercut_active_count"`
	HistoryTotalCount    int     `json:"history_total_count"`
	HistoryAvgFillPct    float64 `json:"history_avg_fill_pct"`
	CancelledCount       int     `json:"cancelled_count"`
	ExpiredUnfilledCount int     `json:"expired_unfilled_count"`
}

// TradeLedgerReport is the complete closed-loop accounting & execution analytics summary.
type TradeLedgerReport struct {
	PeriodDays        int                   `json:"period_days"`
	TotalBuyCount     int                   `json:"total_buy_count"`
	TotalSellCount    int                   `json:"total_sell_count"`
	TotalCashOutISK   float64               `json:"total_cash_out_isk"`
	TotalCashInISK    float64               `json:"total_cash_in_isk"`
	NetCashFlowISK    float64               `json:"net_cash_flow_isk"`
	RealizedProfitISK float64               `json:"realized_profit_isk"`
	RealizedROIPct    float64               `json:"realized_roi_pct"`
	OpenInventoryISK  float64               `json:"open_inventory_isk"`
	FeeLeak           FeeLeakSummary        `json:"fee_leak"`
	Orders            OrderExecutionSummary `json:"orders"`
	Items             []ItemPnLSummary      `json:"items"`
	TopCashCows       []ItemPnLSummary      `json:"top_cash_cows"`
	CapitalDrains     []ItemPnLSummary      `json:"capital_drains"`
}

// ComputeTransactionDedupHash produces a deterministic deduplication key for a market transaction.
// When ExternalTxID is present (from Private ESI), it keys directly on the unique CCP transaction_id.
func ComputeTransactionDedupHash(tx MarketTransaction) string {
	if tx.ExternalTxID != nil && *tx.ExternalTxID > 0 {
		raw := fmt.Sprintf("esi:%d:%d", tx.CharacterID, *tx.ExternalTxID)
		sum := sha256.Sum256([]byte(raw))
		return hex.EncodeToString(sum[:16])
	}
	ts := tx.TransactedAt.UTC().Truncate(time.Minute).Format("2006-01-02T15:04")
	raw := fmt.Sprintf("%s|%s|%s|%d|%.2f",
		ts,
		strings.ToUpper(string(tx.Side)),
		strings.ToLower(strings.TrimSpace(tx.ItemName)),
		tx.Quantity,
		RoundISK(tx.UnitPrice),
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

// ParseEVEWalletTransactionsPaste parses copied rows from EVE Online's Wallet -> Transactions window
// (tab-separated or pipe-separated) into normalized MarketTransaction records.
func ParseEVEWalletTransactionsPaste(raw string, defaultHubName string) ([]MarketTransaction, []string) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var out []MarketTransaction
	var skipped []string

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		// Skip header lines
		if strings.HasPrefix(lower, "date\t") || strings.HasPrefix(lower, "time\t") || strings.Contains(lower, "unit price") && strings.Contains(lower, "quantity") {
			continue
		}

		var cols []string
		if strings.Contains(line, "\t") {
			cols = strings.Split(line, "\t")
		} else if strings.Contains(line, "|") {
			cols = strings.Split(line, "|")
		} else {
			skipped = append(skipped, line)
			continue
		}

		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}

		tx, ok := parseWalletTxColumns(cols, defaultHubName)
		if !ok {
			skipped = append(skipped, line)
			continue
		}
		tx.Source = TransactionSourcePaste
		tx.DedupHash = ComputeTransactionDedupHash(tx)
		out = append(out, tx)
	}

	return out, skipped
}

func parseWalletTxColumns(cols []string, defaultHubName string) (MarketTransaction, bool) {
	// Supported layouts:
	// A) Official EVE Wallet Transactions copy (tab-separated):
	//    [0] Date (2026.11.22 14:30) | [1] Type (Item Name) | [2] Price (14,500.00 ISK) | [3] Qty (100) | [4] Credit (-1,450,000.00 ISK) | [5] Client | [6] Where (Station)
	// B) Compact format:
	//    [0] Date | [1] BUY/SELL | [2] Item Name | [3] Qty | [4] Unit Price | [5] Optional Hub
	// C) Minimal 4-col format:
	//    [0] BUY/SELL | [1] Item Name | [2] Qty | [3] Unit Price
	if len(cols) < 4 {
		return MarketTransaction{}, false
	}

	// Layout C: [0]=BUY/SELL
	if side, ok := parseSideToken(cols[0]); ok {
		item := cols[1]
		qty, okQ := parseIntLoose(cols[2])
		price, okP := parseISKLoose(cols[3])
		if item == "" || !okQ || qty <= 0 || !okP || price <= 0 {
			return MarketTransaction{}, false
		}
		hub := defaultHubName
		if len(cols) >= 5 && cols[4] != "" {
			hub = cols[4]
		}
		return MarketTransaction{
			TransactedAt: time.Now().UTC(),
			Side:         side,
			ItemName:     item,
			Quantity:     qty,
			UnitPrice:    RoundISK(price),
			TotalISK:     RoundISK(price * float64(qty)),
			HubName:      hub,
		}, true
	}

	// Layout B: [0]=Date, [1]=BUY/SELL, [2]=Item, [3]=Qty, [4]=UnitPrice
	if len(cols) >= 5 {
		if side, ok := parseSideToken(cols[1]); ok {
			ts := parseEVEDateLoose(cols[0])
			item := cols[2]
			qty, okQ := parseIntLoose(cols[3])
			price, okP := parseISKLoose(cols[4])
			if item != "" && okQ && qty > 0 && okP && price > 0 {
				hub := defaultHubName
				if len(cols) >= 6 && cols[5] != "" {
					hub = cols[5]
				}
				return MarketTransaction{
					TransactedAt: ts,
					Side:         side,
					ItemName:     item,
					Quantity:     qty,
					UnitPrice:    RoundISK(price),
					TotalISK:     RoundISK(price * float64(qty)),
					HubName:      hub,
				}, true
			}
		}
	}

	// Layout A: Official EVE Wallet Transactions
	// [0] Date | [1] Item Name | [2] Unit Price | [3] Qty | [4] Credit (+/- ISK) | [5] Client | [6] Station
	if len(cols) >= 5 {
		ts := parseEVEDateLoose(cols[0])
		item := cols[1]
		unitPrice, okP := parseISKLoose(cols[2])
		qty, okQ := parseIntLoose(cols[3])
		creditRaw := cols[4]
		creditVal, okC := parseSignedISKLoose(creditRaw)
		if item != "" && okP && unitPrice > 0 && okQ && qty > 0 && okC {
			side := TransactionSideSell
			if creditVal < 0 || strings.HasPrefix(strings.TrimSpace(creditRaw), "-") {
				side = TransactionSideBuy
			}
			total := math.Abs(creditVal)
			if total <= 0 {
				total = unitPrice * float64(qty)
			}
			hub := defaultHubName
			if len(cols) >= 7 && cols[6] != "" {
				hub = shortenStationToHub(cols[6])
			} else if len(cols) == 6 && cols[5] != "" && strings.Contains(cols[5], "-") {
				hub = shortenStationToHub(cols[5])
			}
			return MarketTransaction{
				TransactedAt: ts,
				Side:         side,
				ItemName:     item,
				Quantity:     qty,
				UnitPrice:    RoundISK(unitPrice),
				TotalISK:     RoundISK(total),
				HubName:      hub,
			}, true
		}

		// Variant A2 where Qty is col[2] and Price is col[3]
		qty2, okQ2 := parseIntLoose(cols[2])
		price2, okP2 := parseISKLoose(cols[3])
		if item != "" && okQ2 && qty2 > 0 && okP2 && price2 > 0 && okC {
			side := TransactionSideSell
			if creditVal < 0 || strings.HasPrefix(strings.TrimSpace(creditRaw), "-") {
				side = TransactionSideBuy
			}
			hub := defaultHubName
			if len(cols) >= 7 && cols[6] != "" {
				hub = shortenStationToHub(cols[6])
			}
			return MarketTransaction{
				TransactedAt: ts,
				Side:         side,
				ItemName:     item,
				Quantity:     qty2,
				UnitPrice:    RoundISK(price2),
				TotalISK:     RoundISK(math.Abs(creditVal)),
				HubName:      hub,
			}, true
		}
	}

	return MarketTransaction{}, false
}

func parseSideToken(s string) (TransactionSide, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "BUY", "BOUGHT", "PURCHASE", "BELI":
		return TransactionSideBuy, true
	case "SELL", "SOLD", "SALE", "JUAL":
		return TransactionSideSell, true
	default:
		return "", false
	}
}

func parseEVEDateLoose(s string) time.Time {
	s = strings.TrimSpace(s)
	layouts := []string{
		"2006.01.02 15:04:05",
		"2006.01.02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		time.RFC3339,
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

func parseIntLoose(s string) (int, bool) {
	clean := strings.TrimSpace(s)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.TrimSuffix(clean, "x")
	clean = strings.TrimPrefix(clean, "x")
	v, err := strconv.Atoi(clean)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseISKLoose(s string) (float64, bool) {
	v, ok := parseSignedISKLoose(s)
	if !ok {
		return 0, false
	}
	return math.Abs(v), true
}

func parseSignedISKLoose(s string) (float64, bool) {
	clean := strings.TrimSpace(s)
	clean = strings.ReplaceAll(clean, "ISK", "")
	clean = strings.ReplaceAll(clean, "isk", "")
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, "\u00a0", "")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func shortenStationToHub(station string) string {
	low := strings.ToLower(station)
	switch {
	case strings.Contains(low, "jita"):
		return "Jita 4-4"
	case strings.Contains(low, "amarr"):
		return "Amarr VIII"
	case strings.Contains(low, "dodixie"):
		return "Dodixie IX"
	case strings.Contains(low, "rens"):
		return "Rens VI"
	case strings.Contains(low, "hek"):
		return "Hek VIII"
	default:
		if len(station) > 48 {
			return station[:48]
		}
		return station
	}
}

// ComputeRealizedPnLReport computes chronological weighted-average cost basis, realized P&L per item,
// fee/tax leak analysis, and active/historical order execution metrics.
func ComputeRealizedPnLReport(
	txs []MarketTransaction,
	fees []WalletFeeEntry,
	activeOrders []CharacterOrder,
	orderHist []CharacterOrderHistory,
	periodDays int,
) TradeLedgerReport {
	report := TradeLedgerReport{
		PeriodDays: periodDays,
	}

	// Sort transactions chronologically (oldest -> newest) for accurate weighted-average cost basis
	sorted := make([]MarketTransaction, len(txs))
	copy(sorted, txs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TransactedAt.Equal(sorted[j].TransactedAt) {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].TransactedAt.Before(sorted[j].TransactedAt)
	})

	type itemAcc struct {
		summary      ItemPnLSummary
		invQty       int
		invTotalCost float64
	}

	byItem := make(map[string]*itemAcc)
	itemKey := func(tx MarketTransaction) string {
		if tx.TypeID > 0 {
			return fmt.Sprintf("tid:%d", tx.TypeID)
		}
		return "name:" + strings.ToLower(strings.TrimSpace(tx.ItemName))
	}

	var totalRealizedCost float64
	var totalEstimatedFees float64

	for _, tx := range sorted {
		if tx.IsExcluded {
			continue
		}
		k := itemKey(tx)
		acc, ok := byItem[k]
		if !ok {
			acc = &itemAcc{
				summary: ItemPnLSummary{
					TypeID:   tx.TypeID,
					ItemName: tx.ItemName,
				},
			}
			byItem[k] = acc
		}
		if acc.summary.TypeID == 0 && tx.TypeID > 0 {
			acc.summary.TypeID = tx.TypeID
		}
		if tx.TransactedAt.After(acc.summary.LastActivityAt) {
			acc.summary.LastActivityAt = tx.TransactedAt
		}

		total := tx.TotalISK
		if total <= 0 {
			total = tx.UnitPrice * float64(tx.Quantity)
		}
		totalEstimatedFees += tx.EstimatedFee

		if tx.Side == TransactionSideBuy {
			report.TotalBuyCount++
			report.TotalCashOutISK += total
			acc.summary.QtyBought += tx.Quantity
			acc.summary.TotalBuyISK += total
			acc.invQty += tx.Quantity
			acc.invTotalCost += total
		} else {
			report.TotalSellCount++
			report.TotalCashInISK += total
			acc.summary.QtySold += tx.Quantity
			acc.summary.TotalSellISK += total
			acc.summary.RealizedFeesISK += tx.EstimatedFee

			// Match sold units against weighted-average inventory cost basis
			var cogs float64
			if acc.invQty > 0 {
				avgCost := acc.invTotalCost / float64(acc.invQty)
				matched := tx.Quantity
				if matched > acc.invQty {
					matched = acc.invQty
				}
				cogs = avgCost * float64(matched)
				acc.invQty -= matched
				acc.invTotalCost -= cogs
				if acc.invQty == 0 || acc.invTotalCost < 0 {
					acc.invTotalCost = 0
				}
			}
			acc.summary.RealizedCostISK += cogs
			acc.summary.RealizedProfitISK += total - cogs - tx.EstimatedFee
			totalRealizedCost += cogs
		}
	}

	report.TotalCashOutISK = RoundISK(report.TotalCashOutISK)
	report.TotalCashInISK = RoundISK(report.TotalCashInISK)
	report.NetCashFlowISK = RoundISK(report.TotalCashInISK - report.TotalCashOutISK)

	items := make([]ItemPnLSummary, 0, len(byItem))
	for _, acc := range byItem {
		s := acc.summary
		if s.QtyBought > 0 {
			s.AvgBuyPrice = RoundISK(s.TotalBuyISK / float64(s.QtyBought))
		}
		if s.QtySold > 0 {
			s.AvgSellPrice = RoundISK(s.TotalSellISK / float64(s.QtySold))
		}
		s.TotalBuyISK = RoundISK(s.TotalBuyISK)
		s.TotalSellISK = RoundISK(s.TotalSellISK)
		s.RealizedCostISK = RoundISK(s.RealizedCostISK)
		s.RealizedFeesISK = RoundISK(s.RealizedFeesISK)
		s.RealizedProfitISK = RoundISK(s.RealizedProfitISK)
		s.OpenUnits = acc.invQty
		s.OpenCostBasisISK = RoundISK(acc.invTotalCost)
		report.OpenInventoryISK += s.OpenCostBasisISK

		if s.RealizedCostISK > 0 {
			s.RealizedROIPct = RoundPercent((s.RealizedProfitISK / s.RealizedCostISK) * 100.0)
		} else if s.QtySold > 0 && s.TotalSellISK > 0 {
			// Sold items without prior BUY in window (e.g. self-mined, PI, or loot)
			s.RealizedROIPct = 100.0
		}

		// Classify performance
		switch {
		case s.QtySold == 0:
			s.Classification = ItemPnLAccumulate
		case s.RealizedProfitISK < 0:
			s.Classification = ItemPnLLossMaker
		case s.RealizedROIPct >= 15.0 && s.RealizedProfitISK >= 500_000:
			s.Classification = ItemPnLCashCow
		case s.RealizedROIPct >= 6.0:
			s.Classification = ItemPnLHealthy
		default:
			s.Classification = ItemPnLThinMargin
		}

		report.RealizedProfitISK += s.RealizedProfitISK
		items = append(items, s)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].RealizedProfitISK > items[j].RealizedProfitISK
	})
	report.Items = items
	report.OpenInventoryISK = RoundISK(report.OpenInventoryISK)
	report.RealizedProfitISK = RoundISK(report.RealizedProfitISK)
	if totalRealizedCost > 0 {
		report.RealizedROIPct = RoundPercent((report.RealizedProfitISK / totalRealizedCost) * 100.0)
	}

	// Top Cash Cows & Capital Drains
	for _, it := range items {
		if it.QtySold > 0 && it.RealizedProfitISK > 0 && len(report.TopCashCows) < 5 {
			report.TopCashCows = append(report.TopCashCows, it)
		}
	}
	for i := len(items) - 1; i >= 0 && len(report.CapitalDrains) < 5; i-- {
		it := items[i]
		if it.Classification == ItemPnLLossMaker || it.Classification == ItemPnLThinMargin {
			report.CapitalDrains = append(report.CapitalDrains, it)
		}
	}

	// Fee & Tax Leak Breakdown (prefer real Private ESI wallet/journal fees if available)
	var feeLeak FeeLeakSummary
	feeLeak.EstimatedTxFeesISK = RoundISK(totalEstimatedFees)
	if len(fees) > 0 {
		feeLeak.UsingJournalData = true
		for _, f := range fees {
			amt := math.Abs(f.AmountISK)
			switch f.RefType {
			case "brokers_fee":
				feeLeak.BrokerAndRelistISK += amt
			case "transaction_tax":
				feeLeak.SalesTaxISK += amt
			case "industry_job_tax":
				feeLeak.IndustryJobTaxISK += amt
			case "planetary_import_tax", "planetary_export_tax":
				feeLeak.PITaxISK += amt
			}
		}
		feeLeak.BrokerAndRelistISK = RoundISK(feeLeak.BrokerAndRelistISK)
		feeLeak.SalesTaxISK = RoundISK(feeLeak.SalesTaxISK)
		feeLeak.IndustryJobTaxISK = RoundISK(feeLeak.IndustryJobTaxISK)
		feeLeak.PITaxISK = RoundISK(feeLeak.PITaxISK)
		feeLeak.TotalFeesISK = RoundISK(feeLeak.BrokerAndRelistISK + feeLeak.SalesTaxISK + feeLeak.IndustryJobTaxISK + feeLeak.PITaxISK)
	} else {
		feeLeak.TotalFeesISK = feeLeak.EstimatedTxFeesISK
	}
	if report.TotalCashInISK > 0 {
		feeLeak.FeeToSalesPct = RoundPercent((feeLeak.TotalFeesISK / report.TotalCashInISK) * 100.0)
	}
	if feeLeak.UsingJournalData && feeLeak.BrokerAndRelistISK > feeLeak.SalesTaxISK*1.35 && feeLeak.BrokerAndRelistISK > 500_000 {
		feeLeak.LeakWarning = "High Broker/Relist Fee leak detected! Avoid aggressive price wars — check Safe Relist Floor before modifying orders."
	} else if feeLeak.FeeToSalesPct > 10.0 && feeLeak.TotalFeesISK > 500_000 {
		feeLeak.LeakWarning = fmt.Sprintf("Fees consume %.1f%% of gross sales. Consider training Accounting / Broker Relations or reducing relists.", feeLeak.FeeToSalesPct)
	}
	report.FeeLeak = feeLeak

	// Active & Historical Order Execution Summary
	var ordSum OrderExecutionSummary
	ordSum.ActiveOrdersCount = len(activeOrders)
	for _, o := range activeOrders {
		val := o.Price * float64(o.VolumeRemain)
		if o.IsBuyOrder {
			ordSum.ActiveBuyOrders++
			ordSum.ActiveBuyEscrowISK += val
		} else {
			ordSum.ActiveSellOrders++
			ordSum.ActiveSellValueISK += val
			if o.IsUndercut {
				ordSum.UndercutActiveCount++
			}
		}
	}
	ordSum.ActiveSellValueISK = RoundISK(ordSum.ActiveSellValueISK)
	ordSum.ActiveBuyEscrowISK = RoundISK(ordSum.ActiveBuyEscrowISK)

	ordSum.HistoryTotalCount = len(orderHist)
	var fillPctSum float64
	for _, h := range orderHist {
		fillPctSum += h.FillPercent()
		if strings.EqualFold(h.State, "cancelled") {
			ordSum.CancelledCount++
		} else if h.VolumeRemain > 0 {
			ordSum.ExpiredUnfilledCount++
		}
	}
	if len(orderHist) > 0 {
		ordSum.HistoryAvgFillPct = RoundPercent(fillPctSum / float64(len(orderHist)))
	}
	report.Orders = ordSum

	return report
}
