package service

import (
	"context"
	"fmt"
	"strings"

	"aura-forge/internal/domain"
)

// AppraiseEVEClipboard parses pasted EVE Online inventory/cargo/contract/fit text, resolves every item
// via local PostgreSQL sde_types (with Public ESI fallback), fetches live prices at the chosen Trade Hub,
// and computes Sell/Buy totals, post-tax net values, Buyback split, cargo volume, and Gank Risk.
func (s *CopilotService) AppraiseEVEClipboard(
	ctx context.Context,
	rawText string,
	hubKey string,
	buybackPct float64,
) (*domain.AppraisalResult, error) {
	parsed, skipped := domain.ParseEVEClipboard(rawText)
	if len(parsed) == 0 {
		return nil, fmt.Errorf("no items detected — paste lines from your EVE Online Inventory, Cargo, Multibuy, or Fit")
	}

	settings, _ := s.GetSettings(ctx)
	hub := domain.LookupTradeHub("jita")
	brokerPct := 3.0
	salesTaxPct := 3.6
	if settings != nil {
		hub = settings.ActiveTradeHub()
		brokerPct = settings.BrokerFeePercent
		salesTaxPct = settings.SalesTaxPercent
	}
	if strings.TrimSpace(hubKey) != "" {
		hub = domain.LookupTradeHub(hubKey)
	}

	resolvedByID := make(map[int32]domain.SDEType, len(parsed))
	qtyByID := make(map[int32]int64, len(parsed))
	var unresolved []domain.AppraisalParsedItem

	// 1. Fast local PostgreSQL sde_types resolution (0 ESI calls, includes VolumeM3 + Category)
	var typeSearcher domain.SDETypeSearchRepository
	if s.sde != nil {
		if ts, ok := s.sde.(domain.SDETypeSearchRepository); ok {
			typeSearcher = ts
		}
	}

	for _, it := range parsed {
		var matched *domain.SDEType
		if typeSearcher != nil {
			if candidates, err := typeSearcher.SearchTypes(ctx, it.Name, 8); err == nil {
				for i := range candidates {
					if strings.EqualFold(candidates[i].Name, it.Name) {
						c := candidates[i]
						matched = &c
						break
					}
				}
			}
		}
		if matched != nil {
			resolvedByID[matched.TypeID] = *matched
			qtyByID[matched.TypeID] += it.Quantity
		} else {
			unresolved = append(unresolved, it)
		}
	}

	// 2. Fallback: batch resolve any remaining item names via Public ESI (/universe/ids/)
	unmatched := append([]string{}, skipped...)
	if len(unresolved) > 0 && s.market != nil {
		names := make([]string, 0, len(unresolved))
		for _, u := range unresolved {
			names = append(names, u.Name)
		}
		if idMap, err := s.market.ResolveTypeIDs(ctx, names); err == nil {
			for _, u := range unresolved {
				var tid int32
				for k, v := range idMap {
					if strings.EqualFold(k, u.Name) {
						tid = v
						break
					}
				}
				if tid > 0 {
					if _, exists := resolvedByID[tid]; !exists {
						resolvedByID[tid] = domain.SDEType{
							TypeID:   tid,
							Name:     u.Name,
							Category: "Market",
							VolumeM3: 1.0,
						}
					}
					qtyByID[tid] += u.Quantity
				} else {
					unmatched = append(unmatched, u.Name)
				}
			}
		} else {
			for _, u := range unresolved {
				unmatched = append(unmatched, u.Name)
			}
		}
	} else {
		for _, u := range unresolved {
			unmatched = append(unmatched, u.Name)
		}
	}

	if len(resolvedByID) == 0 {
		return nil, fmt.Errorf("none of the pasted lines matched a known EVE Online item")
	}

	resolvedList := make([]domain.SDEType, 0, len(resolvedByID))
	typeIDs := make([]int32, 0, len(resolvedByID))
	for id, t := range resolvedByID {
		resolvedList = append(resolvedList, t)
		typeIDs = append(typeIDs, id)
	}

	prices := make(map[int32]domain.JitaPrice, len(typeIDs))
	if s.market != nil {
		fetched, err := s.fetchHubPrices(ctx, hub, typeIDs)
		if err != nil {
			return nil, fmt.Errorf("fetching live %s prices for appraisal: %w", hub.Name, err)
		}
		prices = fetched
	}

	res := domain.BuildAppraisalResult(hub, resolvedList, qtyByID, prices, unmatched, brokerPct, salesTaxPct, buybackPct)
	return &res, nil
}
