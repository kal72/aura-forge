package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// SetNetWorthRepository wires the PostgreSQL NetWorthRepository into CopilotService.
func (s *CopilotService) SetNetWorthRepository(repo domain.NetWorthRepository) {
	s.wealth = repo
}

// GetWealthSummary computes live asset breakdown, 0 -> 100B+ ISK milestones, Beginner Capital-Phase guidance,
// historical equity curve snapshots, and top realized profit contributors.
func (s *CopilotService) GetWealthSummary(ctx context.Context) (*domain.WealthSummary, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	_, availWallet, _ := s.resolveProfileAndCapital(ctx, *settings)

	var (
		activeJobsISK float64
		sellOrdersISK float64
		batches       []domain.BatchRecord
	)
	if s.batches != nil {
		if list, bErr := s.batches.ListBatches(ctx, nil); bErr == nil {
			batches = list
			for _, b := range batches {
				switch b.Status {
				case domain.BatchStatusInProduction:
					activeJobsISK += b.ActualProductionCost
				case domain.BatchStatusListed, domain.BatchStatusSlowMover:
					sellOrdersISK += b.TotalRevenue
				}
			}
		}
	}

	var snapshots []domain.NetWorthSnapshot
	if s.wealth != nil {
		if list, sErr := s.wealth.ListSnapshots(ctx, 90); sErr == nil {
			snapshots = list
		}
	}

	summary := &domain.WealthSummary{
		WalletISK:      domain.RoundISK(availWallet),
		SellOrdersISK:  domain.RoundISK(sellOrdersISK),
		ActiveJobsISK:  domain.RoundISK(activeJobsISK),
		Snapshots:      snapshots,
		TopProfitItems: domain.AggregateTopProfitItems(batches, 10),
	}

	// If historical snapshots exist, use the latest snapshot as the authoritative current net worth;
	// otherwise use live estimated capital (Wallet + Listed Orders + Active Jobs).
	if len(snapshots) > 0 {
		latest := snapshots[len(snapshots)-1]
		summary.CurrentNetWorth = latest.TotalNetWorth
		summary.WalletISK = latest.WalletISK
		summary.SellOrdersISK = latest.SellOrdersISK
		summary.EscrowISK = latest.EscrowISK
		summary.InventoryISK = latest.InventoryISK
		summary.ActiveJobsISK = latest.ActiveJobsISK

		if len(snapshots) >= 2 {
			prev := snapshots[len(snapshots)-2]
			summary.DailyChangeISK = domain.RoundISK(latest.TotalNetWorth - prev.TotalNetWorth)
			if prev.TotalNetWorth > 0 {
				summary.DailyChangePct = domain.RoundPercent((summary.DailyChangeISK / prev.TotalNetWorth) * 100.0)
			}
		}
	} else {
		summary.CurrentNetWorth = domain.RoundISK(summary.WalletISK + summary.SellOrdersISK + summary.ActiveJobsISK)
	}

	summary.CurrentPhase = domain.ResolveCapitalPhase(summary.CurrentNetWorth)
	summary.Milestones, summary.NextMilestone = domain.BuildWealthMilestones(summary.CurrentNetWorth)
	return summary, nil
}

// RecordNetWorthSnapshot persists a manual or edited daily Net Worth snapshot.
func (s *CopilotService) RecordNetWorthSnapshot(ctx context.Context, snap *domain.NetWorthSnapshot) (*domain.NetWorthSnapshot, error) {
	if s.wealth == nil {
		return nil, fmt.Errorf("net worth repository is not available")
	}
	if snap == nil {
		return nil, fmt.Errorf("snapshot cannot be nil")
	}
	snap.SnapshotDate = strings.TrimSpace(snap.SnapshotDate)
	if snap.SnapshotDate == "" {
		snap.SnapshotDate = time.Now().Format("2006-01-02")
	}
	snap.ComputeTotal()
	if err := s.wealth.UpsertSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// CaptureAutoNetWorthSnapshot records today's snapshot automatically from live Wallet + Tracker batches.
func (s *CopilotService) CaptureAutoNetWorthSnapshot(ctx context.Context) (*domain.NetWorthSnapshot, error) {
	if s.wealth == nil {
		return nil, fmt.Errorf("net worth repository is not available")
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	_, availWallet, capSource := s.resolveProfileAndCapital(ctx, *settings)

	var activeJobsISK, sellOrdersISK float64
	if s.batches != nil {
		if batches, bErr := s.batches.ListBatches(ctx, nil); bErr == nil {
			for _, b := range batches {
				switch b.Status {
				case domain.BatchStatusInProduction:
					activeJobsISK += b.ActualProductionCost
				case domain.BatchStatusListed, domain.BatchStatusSlowMover:
					sellOrdersISK += b.TotalRevenue
				}
			}
		}
	}

	snap := &domain.NetWorthSnapshot{
		SnapshotDate:  time.Now().Format("2006-01-02"),
		WalletISK:     domain.RoundISK(availWallet),
		SellOrdersISK: domain.RoundISK(sellOrdersISK),
		ActiveJobsISK: domain.RoundISK(activeJobsISK),
		Note:          "Auto-captured (" + capSource + ")",
	}
	snap.ComputeTotal()
	if err := s.wealth.UpsertSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// DeleteNetWorthSnapshot deletes a recorded snapshot by ID.
func (s *CopilotService) DeleteNetWorthSnapshot(ctx context.Context, id int64) error {
	if s.wealth == nil {
		return nil
	}
	return s.wealth.DeleteSnapshot(ctx, id)
}
