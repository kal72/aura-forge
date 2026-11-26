package service

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

const (
	CCPSDELatestMetaURL = "https://developers.eveonline.com/static-data/tranquility/latest.jsonl"
	CCPSDELatestZipURL  = "https://developers.eveonline.com/static-data/eve-online-static-data-latest-jsonl.zip"
)

// SDEImportStatus exposes live progress and installed metadata for the Official CCP SDE (JSONL) Importer.
type SDEImportStatus struct {
	IsRunning    bool                `json:"is_running"`
	Stage        string              `json:"stage"` // idle | downloading | parsing | saving | done | error
	ProgressText string              `json:"progress_text"`
	LastError    string              `json:"last_error,omitempty"`
	DurationMs   int64               `json:"duration_ms,omitempty"`
	Metadata     *domain.SDEMetadata `json:"metadata,omitempty"`
}

type sdeImportState struct {
	mu           sync.RWMutex
	running      bool
	stage        string
	progressText string
	lastError    string
	durationMs   int64
}

func newSDEImportState() *sdeImportState {
	return &sdeImportState{stage: "idle"}
}

func (st *sdeImportState) setStage(stage, text string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.stage = stage
	st.progressText = text
}

// GetSDEImportStatus returns the live SDE import state plus installed PostgreSQL blueprint/type counts.
func (s *CopilotService) GetSDEImportStatus(ctx context.Context) SDEImportStatus {
	out := SDEImportStatus{Stage: "idle"}
	if s.sdeImport != nil {
		s.sdeImport.mu.RLock()
		out.IsRunning = s.sdeImport.running
		out.Stage = s.sdeImport.stage
		out.ProgressText = s.sdeImport.progressText
		out.LastError = s.sdeImport.lastError
		out.DurationMs = s.sdeImport.durationMs
		s.sdeImport.mu.RUnlock()
	}

	if ext, ok := s.sde.(domain.SDEExtendedRepository); ok && ext != nil {
		if meta, err := ext.GetSDEMetadata(ctx); err == nil && meta != nil {
			out.Metadata = meta
		}
	} else if s.sde != nil {
		if bps, err := s.sde.SearchBlueprints(ctx, "", "", 200); err == nil {
			out.Metadata = &domain.SDEMetadata{
				Source:          "starter",
				BlueprintsCount: len(bps),
			}
		}
	}
	return out
}

// SearchBlueprintsPaged returns paginated blueprint search results + category list for the Blueprint Catalog Modal Popup.
func (s *CopilotService) SearchBlueprintsPaged(ctx context.Context, query, category string, techLevel, page, pageSize int) (*domain.BlueprintPageResult, error) {
	if s.sde == nil {
		return &domain.BlueprintPageResult{Items: []domain.SDEBlueprint{}, Page: 1, PageSize: 24, TotalPages: 1}, nil
	}
	if ext, ok := s.sde.(domain.SDEExtendedRepository); ok && ext != nil {
		return ext.SearchBlueprintsPaged(ctx, query, category, techLevel, page, pageSize)
	}
	// Fallback for basic mock SDERepository
	all, err := s.sde.SearchBlueprints(ctx, query, category, 200)
	if err != nil {
		return nil, err
	}
	return &domain.BlueprintPageResult{
		Items:      all,
		Total:      len(all),
		Page:       1,
		PageSize:   len(all),
		TotalPages: 1,
	}, nil
}

// StartOfficialCCPSDEImport triggers an asynchronous download and bulk PostgreSQL import of
// CCP's official EVE Online Static Data Export (JSONL ZIP) — 0 ESI API calls.
func (s *CopilotService) StartOfficialCCPSDEImport() (bool, error) {
	if s.sde == nil {
		return false, fmt.Errorf("PostgreSQL SDE repository unavailable")
	}
	if s.sdeImport == nil {
		s.sdeImport = newSDEImportState()
	}

	s.sdeImport.mu.Lock()
	if s.sdeImport.running {
		s.sdeImport.mu.Unlock()
		return false, nil
	}
	s.sdeImport.running = true
	s.sdeImport.stage = "downloading"
	s.sdeImport.progressText = "Checking latest official CCP SDE build..."
	s.sdeImport.lastError = ""
	s.sdeImport.durationMs = 0
	s.sdeImport.mu.Unlock()

	go func() {
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		err := s.runOfficialCCPSDEImport(ctx, CCPSDELatestMetaURL, CCPSDELatestZipURL)

		elapsed := time.Since(started).Milliseconds()
		s.sdeImport.mu.Lock()
		s.sdeImport.running = false
		s.sdeImport.durationMs = elapsed
		if err != nil {
			s.sdeImport.stage = "error"
			s.sdeImport.lastError = err.Error()
			s.sdeImport.progressText = "Import failed"
			log.Printf("[ERROR] Official CCP SDE import failed: %v", err)
		} else {
			s.sdeImport.stage = "done"
			s.sdeImport.lastError = ""
		}
		s.sdeImport.mu.Unlock()
	}()

	return true, nil
}

func (s *CopilotService) runOfficialCCPSDEImport(ctx context.Context, metaURL, zipURL string) error {
	httpClient := &http.Client{Timeout: 4 * time.Minute}

	// 1. Fetch latest build number from tranquility/latest.jsonl (best-effort)
	buildNum, releaseDate := fetchCCPSDELatestBuild(ctx, httpClient, metaURL)

	// 2. Download official JSONL ZIP to a temp file
	s.sdeImport.setStage("downloading", "Downloading official CCP SDE JSONL archive...")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return fmt.Errorf("creating SDE download request: %w", err)
	}
	req.Header.Set("User-Agent", "aura-forge/1.0 (EVE Market & Industry Copilot)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading CCP SDE archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CCP SDE download returned HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "aura-forge-ccp-sde-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp SDE file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	written, err := io.Copy(tmpFile, resp.Body)
	_ = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("saving CCP SDE archive: %w", err)
	}
	log.Printf("[INFO] Downloaded official CCP SDE JSONL archive (%.1f MB, build=%d).", float64(written)/(1024*1024), buildNum)

	// 3. Parse JSONL files directly from ZIP
	s.sdeImport.setStage("parsing", "Parsing official categories, groups, types, blueprints & T2 inventions JSONL...")
	types, blueprints, materials, matCount, inventions, invMaterials, err := ParseCCPSDEZipFull(tmpPath)
	if err != nil {
		return err
	}

	// 4. Bulk upsert into PostgreSQL via UNNEST
	s.sdeImport.setStage("saving", fmt.Sprintf("Saving %d blueprints, %d T2 inventions & %d types to PostgreSQL...", len(blueprints), len(inventions), len(types)))
	if err := s.sde.UpsertSDEData(ctx, types, blueprints, materials); err != nil {
		return fmt.Errorf("upserting CCP SDE into PostgreSQL: %w", err)
	}
	if invRepo, ok := s.sde.(domain.SDEInventionRepository); ok && invRepo != nil && len(inventions) > 0 {
		if err := invRepo.UpsertInventionData(ctx, inventions, invMaterials); err != nil {
			return fmt.Errorf("upserting CCP SDE inventions into PostgreSQL: %w", err)
		}
	}

	if ext, ok := s.sde.(domain.SDEExtendedRepository); ok && ext != nil {
		_ = ext.SaveSDEMetadata(ctx, &domain.SDEMetadata{
			BuildNumber:     buildNum,
			ReleaseDate:     releaseDate,
			Source:          "ccp_jsonl",
			TypesCount:      len(types),
			BlueprintsCount: len(blueprints),
			MaterialsCount:  matCount,
			InventionsCount: len(inventions),
		})
	}

	s.sdeImport.setStage("done", fmt.Sprintf("Imported %d blueprints, %d T2 inventions (%d materials) from official CCP SDE build #%d.", len(blueprints), len(inventions), matCount, buildNum))
	log.Printf("[INFO] Official CCP SDE import complete: %d types, %d blueprints, %d inventions, %d material lines (build #%d).", len(types), len(blueprints), len(inventions), matCount, buildNum)
	return nil
}

type ccpLatestRow struct {
	Key         string `json:"_key"`
	BuildNumber int64  `json:"buildNumber"`
	ReleaseDate string `json:"releaseDate"`
}

func fetchCCPSDELatestBuild(ctx context.Context, client *http.Client, metaURL string) (int64, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return 0, ""
	}
	req.Header.Set("User-Agent", "aura-forge/1.0 (EVE Market & Industry Copilot)")
	resp, err := client.Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, ""
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var row ccpLatestRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err == nil {
			if row.Key == "sde" || row.BuildNumber > 0 {
				return row.BuildNumber, row.ReleaseDate
			}
		}
	}
	return 0, ""
}

// localizedString decodes CCP JSONL name fields whether they are {"en": "..."} or a plain string.
type localizedString string

func (l *localizedString) UnmarshalJSON(data []byte) error {
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		*l = localizedString(plain)
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		if en, ok := m["en"]; ok {
			*l = localizedString(en)
			return nil
		}
		for _, v := range m {
			*l = localizedString(v)
			return nil
		}
	}
	return nil
}

type ccpCategoryRow struct {
	Key  int32           `json:"_key"`
	Name localizedString `json:"name"`
}

type ccpGroupRow struct {
	Key        int32           `json:"_key"`
	CategoryID int32           `json:"categoryID"`
	Name       localizedString `json:"name"`
}

type ccpTypeRow struct {
	Key           int32           `json:"_key"`
	GroupID       int32           `json:"groupID"`
	MarketGroupID int32           `json:"marketGroupID"`
	Name          localizedString `json:"name"`
	Volume        float64         `json:"volume"`
	PortionSize   int             `json:"portionSize"`
	MetaGroupID   int             `json:"metaGroupID"`
	Published     bool            `json:"published"`
}

type ccpBlueprintMaterial struct {
	TypeID   int32 `json:"typeID"`
	Quantity int64 `json:"quantity"`
}

type ccpBlueprintProduct struct {
	TypeID      int32   `json:"typeID"`
	Quantity    int     `json:"quantity"`
	Probability float64 `json:"probability"`
}

type ccpBlueprintActivity struct {
	Time      int                    `json:"time"`
	Materials []ccpBlueprintMaterial `json:"materials"`
	Products  []ccpBlueprintProduct  `json:"products"`
}

type ccpBlueprintRow struct {
	Key        int32 `json:"_key"`
	Activities struct {
		Manufacturing *ccpBlueprintActivity `json:"manufacturing"`
		Reaction      *ccpBlueprintActivity `json:"reaction"`
		Invention     *ccpBlueprintActivity `json:"invention"`
	} `json:"activities"`
}

// ParseCCPSDEZip is a backward-compatible wrapper around ParseCCPSDEZipFull.
func ParseCCPSDEZip(zipPath string) ([]domain.SDEType, []domain.SDEBlueprint, map[int32][]domain.SDEMaterialRequirement, int, error) {
	types, blueprints, materials, totalMatLines, _, _, err := ParseCCPSDEZipFull(zipPath)
	return types, blueprints, materials, totalMatLines, err
}

// ParseCCPSDEZipFull reads categories.jsonl, groups.jsonl, types.jsonl, and blueprints.jsonl from
// an official CCP SDE JSONL ZIP archive and builds clean domain slices including manufacturing,
// reaction formulas (activity_type), and T1 -> T2 invention pathways + datacore materials.
func ParseCCPSDEZipFull(zipPath string) (
	[]domain.SDEType,
	[]domain.SDEBlueprint,
	map[int32][]domain.SDEMaterialRequirement,
	int,
	[]domain.SDEInvention,
	map[int32][]domain.SDEInventionMaterial,
	error,
) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, nil, 0, nil, nil, fmt.Errorf("opening CCP SDE zip: %w", err)
	}
	defer zr.Close()

	filesByBase := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		filesByBase[base] = f
	}

	readLines := func(baseName string, fn func(line []byte)) error {
		zf, ok := filesByBase[baseName]
		if !ok {
			return fmt.Errorf("required file %q not found in CCP SDE zip", baseName)
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
		for sc.Scan() {
			b := sc.Bytes()
			if len(b) == 0 {
				continue
			}
			fn(b)
		}
		return sc.Err()
	}

	// 1. Categories (optional fallback if missing)
	rawCatNames := make(map[int32]string, 64)
	catNames := make(map[int32]string, 64)
	_ = readLines("categories.jsonl", func(line []byte) {
		var row ccpCategoryRow
		if json.Unmarshal(line, &row) == nil && row.Key > 0 {
			rawCatNames[row.Key] = strings.TrimSpace(string(row.Name))
			catNames[row.Key] = normalizeSDECategory(string(row.Name))
		}
	})

	// 2. Groups (optional fallback if missing)
	type groupMeta struct {
		Name       string
		CategoryID int32
	}
	groups := make(map[int32]groupMeta, 1500)
	_ = readLines("groups.jsonl", func(line []byte) {
		var row ccpGroupRow
		if json.Unmarshal(line, &row) == nil && row.Key > 0 {
			groups[row.Key] = groupMeta{Name: string(row.Name), CategoryID: row.CategoryID}
		}
	})

	// 3. Types (required)
	typeInfo := make(map[int32]ccpTypeRow, 45000)
	if err := readLines("types.jsonl", func(line []byte) {
		var row ccpTypeRow
		if json.Unmarshal(line, &row) == nil && row.Key > 0 {
			typeInfo[row.Key] = row
		}
	}); err != nil {
		return nil, nil, nil, 0, nil, nil, err
	}

	// 4. Blueprints (required) — manufacturing, reactions, and T1 -> T2 invention pathways
	blueprints := make([]domain.SDEBlueprint, 0, 6000)
	bpByID := make(map[int32]domain.SDEBlueprint, 6000)
	materials := make(map[int32][]domain.SDEMaterialRequirement, 6000)
	neededTypeIDs := make(map[int32]struct{}, 15000)
	totalMatLines := 0

	type rawInvention struct {
		t1BPID   int32
		t2BPID   int32
		prob     float64
		runs     int
		timeSec  int
		mats     []domain.SDEInventionMaterial
	}
	rawInventions := make([]rawInvention, 0, 2000)

	if err := readLines("blueprints.jsonl", func(line []byte) {
		var row ccpBlueprintRow
		if json.Unmarshal(line, &row) != nil || row.Key <= 0 {
			return
		}

		// Capture T1 -> T2 invention activity if present
		if row.Activities.Invention != nil && len(row.Activities.Invention.Products) > 0 && len(row.Activities.Invention.Materials) > 0 {
			invAct := row.Activities.Invention
			invMats := make([]domain.SDEInventionMaterial, 0, len(invAct.Materials))
			for _, m := range invAct.Materials {
				if m.TypeID <= 0 || m.Quantity <= 0 {
					continue
				}
				if _, ok := typeInfo[m.TypeID]; !ok {
					continue
				}
				neededTypeIDs[m.TypeID] = struct{}{}
				invMats = append(invMats, domain.SDEInventionMaterial{
					T1BlueprintTypeID: row.Key,
					MaterialTypeID:    m.TypeID,
					Quantity:          int(m.Quantity),
				})
			}
			if len(invMats) > 0 {
				invTime := invAct.Time
				if invTime <= 0 {
					invTime = 3600
				}
				for _, p := range invAct.Products {
					if p.TypeID <= 0 {
						continue
					}
					prob := p.Probability
					if prob <= 0 {
						prob = 0.26
					}
					runs := p.Quantity
					if runs <= 0 {
						runs = 1
					}
					rawInventions = append(rawInventions, rawInvention{
						t1BPID:  row.Key,
						t2BPID:  p.TypeID,
						prob:    prob,
						runs:    runs,
						timeSec: invTime,
						mats:    invMats,
					})
				}
			}
		}

		var act *ccpBlueprintActivity
		isReaction := false
		if row.Activities.Manufacturing != nil && len(row.Activities.Manufacturing.Products) > 0 && len(row.Activities.Manufacturing.Materials) > 0 {
			act = row.Activities.Manufacturing
		} else if row.Activities.Reaction != nil && len(row.Activities.Reaction.Products) > 0 && len(row.Activities.Reaction.Materials) > 0 {
			act = row.Activities.Reaction
			isReaction = true
		} else {
			return
		}

		prod := act.Products[0]
		prodType, hasProd := typeInfo[prod.TypeID]
		if !hasProd || !prodType.Published || strings.TrimSpace(string(prodType.Name)) == "" {
			return
		}

		bpName := fmt.Sprintf("%s Blueprint", string(prodType.Name))
		if bpType, ok := typeInfo[row.Key]; ok && strings.TrimSpace(string(bpType.Name)) != "" {
			bpName = string(bpType.Name)
		}

		category := "Other"
		if isReaction {
			category = "Reactions"
		} else if gm, ok := groups[prodType.GroupID]; ok {
			if cn, ok := catNames[gm.CategoryID]; ok && cn != "" {
				category = cn
			}
			if strings.Contains(strings.ToLower(gm.Name), "rig") {
				category = "Rigs"
			} else if strings.Contains(strings.ToLower(gm.Name), "component") {
				category = "Components"
			}
		}

		techLevel := inferTechLevel(prodType.MetaGroupID, string(prodType.Name))
		qtyPerRun := prod.Quantity
		if qtyPerRun <= 0 {
			qtyPerRun = 1
		}
		baseTime := act.Time
		if baseTime <= 0 {
			baseTime = 600
		}

		// Collect valid materials
		matList := make([]domain.SDEMaterialRequirement, 0, len(act.Materials))
		for _, m := range act.Materials {
			if m.TypeID <= 0 || m.Quantity <= 0 {
				continue
			}
			if _, ok := typeInfo[m.TypeID]; !ok {
				continue
			}
			neededTypeIDs[m.TypeID] = struct{}{}
			matList = append(matList, domain.SDEMaterialRequirement{
				MaterialTypeID: m.TypeID,
				BaseQuantity:   m.Quantity,
			})
		}
		if len(matList) == 0 {
			return
		}

		activityType := "manufacturing"
		if isReaction {
			activityType = "reaction"
		}
		neededTypeIDs[prod.TypeID] = struct{}{}
		bp := domain.SDEBlueprint{
			BlueprintTypeID:  row.Key,
			Name:             bpName,
			ProductTypeID:    prod.TypeID,
			ProductName:      string(prodType.Name),
			ProductQtyPerRun: qtyPerRun,
			BaseTimeSeconds:  baseTime,
			TechLevel:        techLevel,
			Category:         category,
			ActivityType:     activityType,
		}
		blueprints = append(blueprints, bp)
		bpByID[row.Key] = bp
		materials[row.Key] = matList
		totalMatLines += len(matList)
	}); err != nil {
		return nil, nil, nil, 0, nil, nil, err
	}

	// Resolve T1 -> T2 invention rows against parsed T2 blueprints so every invention carries its T2 ProductTypeID & Name
	inventions := make([]domain.SDEInvention, 0, len(rawInventions))
	invMaterials := make(map[int32][]domain.SDEInventionMaterial, len(rawInventions))
	for _, ri := range rawInventions {
		t2BP, ok := bpByID[ri.t2BPID]
		if !ok || t2BP.ProductTypeID <= 0 {
			continue
		}
		inventions = append(inventions, domain.SDEInvention{
			T1BlueprintTypeID: ri.t1BPID,
			T2BlueprintTypeID: ri.t2BPID,
			T2ProductTypeID:   t2BP.ProductTypeID,
			T2ProductName:     t2BP.ProductName,
			BaseProbability:   ri.prob,
			BaseRuns:          ri.runs,
			BaseTimeSeconds:   ri.timeSec,
			Category:          t2BP.Category,
		})
		invMaterials[ri.t1BPID] = ri.mats
	}

	// Include 100% of published tradable EVE Market items (marketGroupID > 0) + published blueprints
	// so Marketplace Autocomplete and Browse by Type cover every single market item in New Eden.
	for tid, t := range typeInfo {
		if !t.Published || strings.TrimSpace(string(t.Name)) == "" {
			continue
		}
		if t.MarketGroupID > 0 {
			neededTypeIDs[tid] = struct{}{}
			continue
		}
		if gm, ok := groups[t.GroupID]; ok {
			if strings.EqualFold(rawCatNames[gm.CategoryID], "Blueprint") {
				neededTypeIDs[tid] = struct{}{}
			}
		}
	}

	// Build sde_types slice for all blueprint products/materials + all published market items
	types := make([]domain.SDEType, 0, len(neededTypeIDs))
	for tid := range neededTypeIDs {
		t := typeInfo[tid]
		gm := groups[t.GroupID]
		cat := normalizeMarketTypeCategory(rawCatNames[gm.CategoryID], gm.Name)
		portion := t.PortionSize
		if portion <= 0 {
			portion = 1
		}
		types = append(types, domain.SDEType{
			TypeID:      t.Key,
			Name:        string(t.Name),
			GroupName:   gm.Name,
			Category:    cat,
			VolumeM3:    t.Volume,
			PortionSize: portion,
		})
	}

	return types, blueprints, materials, totalMatLines, inventions, invMaterials, nil
}

func normalizeMarketTypeCategory(rawCat, groupName string) string {
	gLower := strings.ToLower(strings.TrimSpace(groupName))
	switch strings.ToLower(strings.TrimSpace(rawCat)) {
	case "ship":
		return "Ships"
	case "module":
		if strings.Contains(gLower, "rig") {
			return "Rigs"
		}
		return "Modules"
	case "charge":
		return "Ammunition"
	case "drone":
		return "Drones"
	case "blueprint":
		return "Blueprints"
	case "material":
		return "Minerals & Materials"
	case "asteroid":
		return "Ore & Ice"
	case "commodity", "planetary commodities", "planetary resources", "planetary industry":
		return "Commodities & PI"
	case "implant":
		return "Implants & Boosters"
	case "skill":
		return "Skills"
	case "structure", "structure module":
		return "Structures"
	case "skins", "apparel":
		return "SKINs & Apparel"
	case "deployable":
		return "Deployables"
	case "fighter":
		return "Fighters"
	case "subsystem":
		return "Subsystems"
	case "accessories", "special edition assets", "plex":
		return "Commodities & PI"
	case "":
		return "Other"
	default:
		return strings.TrimSpace(rawCat)
	}
}

func normalizeSDECategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ship":
		return "Ships"
	case "module":
		return "Modules"
	case "drone":
		return "Drones"
	case "charge":
		return "Ammunition"
	case "structure", "structure module":
		return "Structures"
	case "commodity", "material":
		return "Components"
	case "deployable":
		return "Deployables"
	case "implant":
		return "Implants"
	case "fighter":
		return "Fighters"
	case "subsystem":
		return "Subsystems"
	case "":
		return "Other"
	default:
		return strings.TrimSpace(raw)
	}
}

func inferTechLevel(metaGroupID int, name string) int {
	switch metaGroupID {
	case 2:
		return 2
	case 14:
		return 3
	}
	if strings.HasSuffix(name, " II") {
		return 2
	}
	return 1
}
