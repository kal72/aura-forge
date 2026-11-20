package service_test

import (
	"context"
	"testing"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
	"aura-forge/internal/service"
)

type memItemRepo struct {
	items []domain.MarketItem
}

func (m *memItemRepo) ListItems(ctx context.Context) ([]domain.MarketItem, error) {
	return m.items, nil
}
func (m *memItemRepo) GetItemByID(ctx context.Context, id int64) (*domain.MarketItem, error) {
	for i := range m.items {
		if m.items[i].ID == id {
			return &m.items[i], nil
		}
	}
	return nil, nil
}
func (m *memItemRepo) GetItemByTypeID(ctx context.Context, typeID int32) (*domain.MarketItem, error) {
	for i := range m.items {
		if m.items[i].TypeID == typeID {
			return &m.items[i], nil
		}
	}
	return nil, nil
}
func (m *memItemRepo) SaveItem(ctx context.Context, item *domain.MarketItem) error {
	item.ID = int64(len(m.items) + 1)
	m.items = append(m.items, *item)
	return nil
}
func (m *memItemRepo) DeleteItem(ctx context.Context, id int64) error { return nil }
func (m *memItemRepo) BatchUpsertItems(ctx context.Context, items []domain.MarketItem) error {
	m.items = make([]domain.MarketItem, len(items))
	for i, it := range items {
		it.ID = int64(i + 1)
		m.items[i] = it
	}
	return nil
}

type memPlanRepo struct {
	plans []domain.WeeklyPlan
}

func (m *memPlanRepo) ListPlans(ctx context.Context) ([]domain.WeeklyPlan, error) {
	return m.plans, nil
}
func (m *memPlanRepo) GetPlanByID(ctx context.Context, id int64) (*domain.WeeklyPlan, error) {
	for i := range m.plans {
		if m.plans[i].ID == id {
			return &m.plans[i], nil
		}
	}
	return nil, nil
}
func (m *memPlanRepo) GetActivePlan(ctx context.Context) (*domain.WeeklyPlan, error) {
	if len(m.plans) == 0 {
		return nil, nil
	}
	return &m.plans[len(m.plans)-1], nil
}
func (m *memPlanRepo) SavePlan(ctx context.Context, p *domain.WeeklyPlan) error {
	p.ID = int64(len(m.plans) + 1)
	m.plans = append(m.plans, *p)
	return nil
}
func (m *memPlanRepo) DeletePlan(ctx context.Context, id int64) error { return nil }

type memBatchRepo struct {
	batches []domain.BatchRecord
}

func (m *memBatchRepo) ListBatches(ctx context.Context, status *domain.BatchStatus) ([]domain.BatchRecord, error) {
	return m.batches, nil
}
func (m *memBatchRepo) GetBatchByID(ctx context.Context, id int64) (*domain.BatchRecord, error) {
	for i := range m.batches {
		if m.batches[i].ID == id {
			return &m.batches[i], nil
		}
	}
	return nil, nil
}
func (m *memBatchRepo) SaveBatch(ctx context.Context, b *domain.BatchRecord) error {
	b.ID = int64(len(m.batches) + 1)
	m.batches = append(m.batches, *b)
	return nil
}
func (m *memBatchRepo) UpdateBatchStatus(ctx context.Context, id int64, status domain.BatchStatus) error {
	return nil
}
func (m *memBatchRepo) DeleteBatch(ctx context.Context, id int64) error { return nil }
func (m *memBatchRepo) CreateBatchesFromPlan(ctx context.Context, planID int64, batches []domain.BatchRecord) error {
	for i := range batches {
		batches[i].ID = int64(len(m.batches) + 1)
		m.batches = append(m.batches, batches[i])
	}
	return nil
}

func TestCopilotService_SampleDataToPlannerToTrackerFlow(t *testing.T) {
	itemRepo := &memItemRepo{}
	planRepo := &memPlanRepo{}
	batchRepo := &memBatchRepo{}
	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)

	svc := service.NewCopilotService(
		itemRepo,
		batchRepo,
		planRepo,
		nil,
		nil,
		esi.NewPublicMarketClient(esi.NewClient("", "test", nil, nil)),
		esi.NewNoOpCharacterSyncProvider(),
		sso,
	)

	ctx := context.Background()

	// 1. Load Sample Data
	count, err := svc.LoadSampleData(ctx)
	if err != nil {
		t.Fatalf("LoadSampleData failed: %v", err)
	}
	if count < 5 {
		t.Fatalf("expected at least 5 sample items, got %d", count)
	}

	// Verify both LAYAK_PRODUKSI and JEBAKAN items are present
	dash, err := svc.GetDashboard(ctx)
	if err != nil {
		t.Fatalf("GetDashboard failed: %v", err)
	}
	if dash.LayakItemsCount == 0 || dash.JebakanItemsCount == 0 {
		t.Errorf("expected both layak (%d) and jebakan (%d) sample items", dash.LayakItemsCount, dash.JebakanItemsCount)
	}

	// 2. Allocate Capital -> Create Weekly Plan
	plan, err := svc.CreatePlanFromAllocation(ctx, "Minggu Uji #1", 1, domain.DefaultAllocationConfig())
	if err != nil {
		t.Fatalf("CreatePlanFromAllocation failed: %v", err)
	}
	if len(plan.Items) == 0 || len(plan.Days) != 7 {
		t.Fatalf("expected populated plan with 7 days, got %d items and %d days", len(plan.Items), len(plan.Days))
	}

	// 3. Planner View (Slot Report + Shopping List + Multibuy + 4-Week Growth)
	view, err := svc.GetPlannerView(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlannerView failed: %v", err)
	}
	if len(view.ShoppingList.Items) == 0 || view.ShoppingList.MultibuyText == "" {
		t.Errorf("expected consolidated shopping list and EVE Multibuy text")
	}
	if len(view.Growth4Weeks) != 4 {
		t.Errorf("expected 4-week growth projection, got %d", len(view.Growth4Weeks))
	}

	// 4. Convert Weekly Plan -> Tracker Draft Batches
	batchCount, err := svc.ConvertPlanToTrackerBatches(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ConvertPlanToTrackerBatches failed: %v", err)
	}
	if batchCount != len(plan.Items) {
		t.Errorf("expected %d batches created, got %d", len(plan.Items), batchCount)
	}
}
