package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
)

// LedgerSyncResult summarizes counts imported from Private ESI.
type LedgerSyncResult struct {
	TransactionsInserted int `json:"transactions_inserted"`
	TransactionsSkipped  int `json:"transactions_skipped"`
	FeesInserted         int `json:"fees_inserted"`
	ActiveOrdersSynced   int `json:"active_orders_synced"`
	OrderHistorySynced   int `json:"order_history_synced"`
}

// TradeLedgerViewData bundles all data required to render Tab 2 (Buy & Sell Ledger)
// and Tab 3 (Realized P&L & Orders) inside /tracker.
type TradeLedgerViewData struct {
	PeriodDays   int                            `json:"period_days"`
	SideFilter   string                         `json:"side_filter"`
	SearchQuery  string                         `json:"search_query"`
	Transactions []domain.MarketTransaction     `json:"transactions"`
	Report       domain.TradeLedgerReport       `json:"report"`
	ActiveOrders []domain.CharacterOrder        `json:"active_orders"`
	OrderHistory []domain.CharacterOrderHistory `json:"order_history"`
}

// SyncPrivateESILedger pulls wallet transactions, tax/fee journal entries, active orders,
// and 90-day order history from Private ESI using the already-granted OAuth scopes.
func (s *CopilotService) SyncPrivateESILedger(ctx context.Context) (*LedgerSyncResult, error) {
	if s.sso == nil || !s.sso.IsEnabled() {
		return nil, esi.ErrSSODisabled
	}
	if s.chars == nil || s.charSync == nil || s.ledger == nil {
		return nil, fmt.Errorf("trade ledger sync dependencies unavailable")
	}
	ledgerProvider, ok := s.charSync.(domain.CharacterLedgerSyncProvider)
	if !ok {
		return nil, esi.ErrSSODisabled
	}

	tok, err := s.chars.GetPrimaryToken(ctx)
	if err != nil || tok == nil {
		return nil, fmt.Errorf("no connected EVE SSO character found")
	}

	res := &LedgerSyncResult{}

	// 1. Wallet Transactions (BUY / SELL)
	txs, txErr := ledgerProvider.SyncWalletTransactions(ctx, tok)
	if txErr != nil {
		return nil, txErr
	}
	if len(txs) > 0 {
		s.resolveLedgerTypeNames(ctx, txs, nil, nil)
		ins, skip, err := s.ledger.UpsertTransactions(ctx, txs)
		if err != nil {
			return nil, err
		}
		res.TransactionsInserted = ins
		res.TransactionsSkipped = skip
	}

	// 2. Wallet Journal Fees (brokers_fee, transaction_tax, industry_job_tax, PI taxes)
	if fees, err := ledgerProvider.SyncWalletJournalFees(ctx, tok); err == nil && len(fees) > 0 {
		if ins, err := s.ledger.UpsertFeeEntries(ctx, fees); err == nil {
			res.FeesInserted = ins
		}
	}

	// 3. Active Orders
	if activeOrders, err := s.charSync.SyncOrders(ctx, tok); err == nil {
		if len(activeOrders) > 0 {
			s.resolveLedgerTypeNames(ctx, nil, activeOrders, nil)
			_ = s.chars.UpsertOrders(ctx, tok.CharacterID, activeOrders)
		}
		res.ActiveOrdersSynced = len(activeOrders)
	}

	// 4. 90-Day Order History
	if hist, err := ledgerProvider.SyncOrderHistory(ctx, tok); err == nil && len(hist) > 0 {
		s.resolveLedgerTypeNames(ctx, nil, nil, hist)
		if n, err := s.ledger.UpsertOrderHistory(ctx, hist); err == nil {
			res.OrderHistorySynced = n
		}
	}

	return res, nil
}

// ImportWalletTransactionsPaste parses pasted EVE Wallet Transactions rows, resolves TypeIDs locally,
// computes estimated market fees for SELL rows when absent, and upserts with deduplication.
func (s *CopilotService) ImportWalletTransactionsPaste(ctx context.Context, rawText, defaultHubName string) (inserted int, skippedDup int, unparsed int, err error) {
	if s.ledger == nil {
		return 0, 0, 0, fmt.Errorf("trade ledger repository unavailable")
	}
	if strings.TrimSpace(defaultHubName) == "" {
		defaultHubName = s.ActiveCatalogHub(ctx).Name
	}

	parsed, badLines := domain.ParseEVEWalletTransactionsPaste(rawText, defaultHubName)
	if len(parsed) == 0 {
		return 0, 0, len(badLines), fmt.Errorf("no valid EVE wallet transaction rows found (paste tab-separated rows from EVE Wallet -> Transactions)")
	}

	settings, _ := s.GetSettings(ctx)
	sellFeeRate := (settings.BrokerFeePercent + settings.SalesTaxPercent) / 100.0

	for i := range parsed {
		s.resolveSingleItemTypeID(ctx, &parsed[i])
		if parsed[i].Side == domain.TransactionSideSell && parsed[i].EstimatedFee <= 0 {
			parsed[i].EstimatedFee = domain.RoundISK(parsed[i].TotalISK * sellFeeRate)
		}
	}

	ins, skip, err := s.ledger.UpsertTransactions(ctx, parsed)
	return ins, skip, len(badLines), err
}

// AddManualTransaction records a single manual BUY or SELL transaction into the ledger.
func (s *CopilotService) AddManualTransaction(ctx context.Context, tx *domain.MarketTransaction) error {
	if s.ledger == nil {
		return fmt.Errorf("trade ledger repository unavailable")
	}
	if tx == nil || strings.TrimSpace(tx.ItemName) == "" {
		return fmt.Errorf("item name is required")
	}
	if tx.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than zero")
	}
	if tx.UnitPrice <= 0 {
		return fmt.Errorf("unit price must be greater than zero")
	}
	if tx.Side != domain.TransactionSideBuy && tx.Side != domain.TransactionSideSell {
		tx.Side = domain.TransactionSideBuy
	}
	if tx.TransactedAt.IsZero() {
		tx.TransactedAt = time.Now().UTC()
	}
	if strings.TrimSpace(tx.HubName) == "" {
		tx.HubName = s.ActiveCatalogHub(ctx).Name
	}
	tx.ItemName = strings.TrimSpace(tx.ItemName)
	tx.TotalISK = domain.RoundISK(tx.UnitPrice * float64(tx.Quantity))
	tx.Source = domain.TransactionSourceManual

	if tx.Side == domain.TransactionSideSell && tx.EstimatedFee <= 0 {
		settings, _ := s.GetSettings(ctx)
		tx.EstimatedFee = domain.RoundISK(tx.TotalISK * ((settings.BrokerFeePercent + settings.SalesTaxPercent) / 100.0))
	}
	s.resolveSingleItemTypeID(ctx, tx)
	tx.DedupHash = domain.ComputeTransactionDedupHash(*tx)

	_, _, err := s.ledger.UpsertTransactions(ctx, []domain.MarketTransaction{*tx})
	return err
}

// DeleteLedgerTransaction removes a transaction row by ID.
func (s *CopilotService) DeleteLedgerTransaction(ctx context.Context, id int64) error {
	if s.ledger == nil {
		return fmt.Errorf("trade ledger repository unavailable")
	}
	return s.ledger.DeleteTransaction(ctx, id)
}

// GetTradeLedgerView assembles transactions, Realized P&L report, Fee Leak summary,
// and active/historical market orders for /tracker tabs and Aura AI.
func (s *CopilotService) GetTradeLedgerView(ctx context.Context, periodDays int, sideFilter, searchQuery string) (*TradeLedgerViewData, error) {
	if periodDays < 0 {
		periodDays = 30
	}
	data := &TradeLedgerViewData{
		PeriodDays:  periodDays,
		SideFilter:  strings.ToUpper(strings.TrimSpace(sideFilter)),
		SearchQuery: strings.TrimSpace(searchQuery),
	}

	var allPeriodTxs []domain.MarketTransaction
	var fees []domain.WalletFeeEntry
	var activeOrders []domain.CharacterOrder
	var orderHist []domain.CharacterOrderHistory

	if s.ledger != nil {
		// Transactions for table (respects side & search filter)
		filtered, err := s.ledger.ListTransactions(ctx, periodDays, sideFilter, searchQuery, 500)
		if err != nil {
			return nil, err
		}
		data.Transactions = filtered

		// Full transactions in period for accurate Weighted-Average Cost Basis P&L
		if sideFilter == "" && searchQuery == "" {
			allPeriodTxs = filtered
		} else {
			allPeriodTxs, _ = s.ledger.ListTransactions(ctx, periodDays, "", "", 1500)
		}

		fees, _ = s.ledger.ListFeeEntries(ctx, periodDays)

		var charID int64
		if s.chars != nil {
			if tok, err := s.chars.GetPrimaryToken(ctx); err == nil && tok != nil {
				charID = tok.CharacterID
				activeOrders, _ = s.chars.ListOrders(ctx, charID)
			}
		}
		orderHist, _ = s.ledger.ListOrderHistory(ctx, charID, 120)
	}

	data.ActiveOrders = activeOrders
	data.OrderHistory = orderHist
	data.Report = domain.ComputeRealizedPnLReport(allPeriodTxs, fees, activeOrders, orderHist, periodDays)
	return data, nil
}

func (s *CopilotService) resolveSingleItemTypeID(ctx context.Context, tx *domain.MarketTransaction) {
	if tx == nil || tx.TypeID > 0 || strings.TrimSpace(tx.ItemName) == "" {
		return
	}
	name := strings.TrimSpace(tx.ItemName)
	if s.sde != nil {
		if ts, ok := s.sde.(domain.SDETypeSearchRepository); ok {
			if matches, err := ts.SearchTypes(ctx, name, 5); err == nil {
				for _, m := range matches {
					if strings.EqualFold(m.Name, name) {
						tx.TypeID = m.TypeID
						tx.ItemName = m.Name
						return
					}
				}
			}
		}
	}
}

func (s *CopilotService) resolveLedgerTypeNames(
	ctx context.Context,
	txs []domain.MarketTransaction,
	activeOrders []domain.CharacterOrder,
	orderHist []domain.CharacterOrderHistory,
) {
	// Collect unique type IDs that still have placeholder "Type #..." names
	needed := make(map[int32]struct{})
	for _, t := range txs {
		if t.TypeID > 0 && (t.ItemName == "" || strings.HasPrefix(t.ItemName, "Type #")) {
			needed[t.TypeID] = struct{}{}
		}
	}
	for _, o := range activeOrders {
		if o.TypeID > 0 && (o.TypeName == "" || strings.HasPrefix(o.TypeName, "Type #")) {
			needed[o.TypeID] = struct{}{}
		}
	}
	for _, h := range orderHist {
		if h.TypeID > 0 && (h.TypeName == "" || strings.HasPrefix(h.TypeName, "Type #")) {
			needed[h.TypeID] = struct{}{}
		}
	}
	if len(needed) == 0 {
		return
	}

	ids := make([]int32, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}

	resolved := make(map[int32]string, len(ids))
	if gp, ok := s.market.(domain.GlobalMarketPriceProvider); ok && gp != nil {
		esiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if names, err := gp.ResolveTypeNames(esiCtx, ids); err == nil {
			for k, v := range names {
				resolved[k] = v
			}
		}
		cancel()
	}

	for i := range txs {
		if name, ok := resolved[txs[i].TypeID]; ok && name != "" {
			txs[i].ItemName = name
		}
	}
	for i := range activeOrders {
		if name, ok := resolved[activeOrders[i].TypeID]; ok && name != "" {
			activeOrders[i].TypeName = name
		}
	}
	for i := range orderHist {
		if name, ok := resolved[orderHist[i].TypeID]; ok && name != "" {
			orderHist[i].TypeName = name
		}
	}
}
