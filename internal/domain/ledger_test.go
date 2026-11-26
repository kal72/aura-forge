package domain_test

import (
	"testing"
	"time"

	"aura-forge/internal/domain"
)

func TestParseEVEWalletTransactionsPaste(t *testing.T) {
	raw := "Date\tType\tPrice\tQuantity\tCredit\tClient\tWhere\n" +
		"2026.11.20 10:15\tHobgoblin I\t10,000.00 ISK\t100\t-1,000,000.00 ISK\tSeller A\tAmarr VIII (Oris) - Emperor Family Academy\n" +
		"2026.11.21 14:30\tHobgoblin I\t14,000.00 ISK\t60\t840,000.00 ISK\tBuyer B\tAmarr VIII (Oris) - Emperor Family Academy\n" +
		"BUY | Warrior II | 10 | 600,000\n"

	txs, skipped := domain.ParseEVEWalletTransactionsPaste(raw, "Amarr VIII")
	if len(skipped) != 0 {
		t.Fatalf("expected 0 skipped lines, got %v", skipped)
	}
	if len(txs) != 3 {
		t.Fatalf("expected 3 parsed transactions, got %d", len(txs))
	}

	if txs[0].Side != domain.TransactionSideBuy || txs[0].Quantity != 100 || txs[0].TotalISK != 1_000_000 {
		t.Fatalf("unexpected tx[0]: %+v", txs[0])
	}
	if txs[0].HubName != "Amarr VIII" {
		t.Fatalf("expected hub Amarr VIII, got %q", txs[0].HubName)
	}
	if txs[1].Side != domain.TransactionSideSell || txs[1].Quantity != 60 || txs[1].TotalISK != 840_000 {
		t.Fatalf("unexpected tx[1]: %+v", txs[1])
	}
	if txs[2].Side != domain.TransactionSideBuy || txs[2].ItemName != "Warrior II" || txs[2].Quantity != 10 {
		t.Fatalf("unexpected tx[2]: %+v", txs[2])
	}

	// Verify deterministic dedup hash prevents duplicate re-pastes
	txsAgain, _ := domain.ParseEVEWalletTransactionsPaste(raw, "Amarr VIII")
	if txs[0].DedupHash == "" || txs[0].DedupHash != txsAgain[0].DedupHash {
		t.Fatalf("expected identical dedup hash across repeated pastes: %q vs %q", txs[0].DedupHash, txsAgain[0].DedupHash)
	}
}

func TestComputeRealizedPnLReport_WeightedAverageAndFeeLeak(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	txs := []domain.MarketTransaction{
		// Buy 100 @ 10,000 = 1,000,000
		{ID: 1, TransactedAt: t0, Side: domain.TransactionSideBuy, TypeID: 2454, ItemName: "Hobgoblin I", Quantity: 100, UnitPrice: 10_000, TotalISK: 1_000_000},
		// Buy 100 @ 12,000 = 1,200,000 -> weighted avg = 11,000/u for 200u
		{ID: 2, TransactedAt: t0.Add(time.Hour), Side: domain.TransactionSideBuy, TypeID: 2454, ItemName: "Hobgoblin I", Quantity: 100, UnitPrice: 12_000, TotalISK: 1_200_000},
		// Sell 150 @ 16,000 = 2,400,000, fee 100,000 -> COGS = 150*11,000 = 1,650,000 -> Net = 650,000 (ROI 39.39% => CASH_COW)
		{ID: 3, TransactedAt: t0.Add(2 * time.Hour), Side: domain.TransactionSideSell, TypeID: 2454, ItemName: "Hobgoblin I", Quantity: 150, UnitPrice: 16_000, TotalISK: 2_400_000, EstimatedFee: 100_000},
		// Loss maker: Buy 10 @ 100,000 = 1,000,000, Sell 10 @ 90,000 = 900,000 -> Net = -100,000
		{ID: 4, TransactedAt: t0.Add(3 * time.Hour), Side: domain.TransactionSideBuy, TypeID: 587, ItemName: "Rifter", Quantity: 10, UnitPrice: 100_000, TotalISK: 1_000_000},
		{ID: 5, TransactedAt: t0.Add(4 * time.Hour), Side: domain.TransactionSideSell, TypeID: 587, ItemName: "Rifter", Quantity: 10, UnitPrice: 90_000, TotalISK: 900_000},
		// Excluded personal purchase (must be completely ignored by P&L & Cash Out!)
		{ID: 6, TransactedAt: t0.Add(5 * time.Hour), Side: domain.TransactionSideBuy, TypeID: 29668, ItemName: "Personal PvP Ship", Quantity: 1, UnitPrice: 50_000_000, TotalISK: 50_000_000, IsExcluded: true},
	}

	fees := []domain.WalletFeeEntry{
		{JournalID: 101, RefType: "brokers_fee", AmountISK: 800_000},
		{JournalID: 102, RefType: "transaction_tax", AmountISK: 200_000},
	}
	orderHist := []domain.CharacterOrderHistory{
		{OrderID: 1, VolumeTotal: 100, VolumeRemain: 0, State: "expired"},
		{OrderID: 2, VolumeTotal: 100, VolumeRemain: 50, State: "cancelled"},
	}

	rep := domain.ComputeRealizedPnLReport(txs, fees, nil, orderHist, 30)

	if rep.TotalCashOutISK != 3_200_000 || rep.TotalCashInISK != 3_300_000 || rep.NetCashFlowISK != 100_000 {
		t.Fatalf("unexpected cash flow: out=%.2f in=%.2f net=%.2f", rep.TotalCashOutISK, rep.TotalCashInISK, rep.NetCashFlowISK)
	}
	// Realized profit: 650,000 - 100,000 = 550,000; Open inventory: 50 * 11,000 = 550,000
	if rep.RealizedProfitISK != 550_000 {
		t.Fatalf("expected realized profit 550000, got %.2f", rep.RealizedProfitISK)
	}
	if rep.OpenInventoryISK != 550_000 {
		t.Fatalf("expected open inventory 550000, got %.2f", rep.OpenInventoryISK)
	}
	if len(rep.TopCashCows) != 1 || rep.TopCashCows[0].ItemName != "Hobgoblin I" || rep.TopCashCows[0].Classification != domain.ItemPnLCashCow {
		t.Fatalf("expected Hobgoblin I as CASH_COW, got %+v", rep.TopCashCows)
	}
	if len(rep.CapitalDrains) != 1 || rep.CapitalDrains[0].ItemName != "Rifter" || rep.CapitalDrains[0].Classification != domain.ItemPnLLossMaker {
		t.Fatalf("expected Rifter as LOSS_MAKER, got %+v", rep.CapitalDrains)
	}
	if !rep.FeeLeak.UsingJournalData || rep.FeeLeak.LeakWarning == "" {
		t.Fatalf("expected broker fee leak warning, got %+v", rep.FeeLeak)
	}
	if rep.Orders.HistoryAvgFillPct != 75.0 || rep.Orders.CancelledCount != 1 {
		t.Fatalf("unexpected order history summary: %+v", rep.Orders)
	}
}
