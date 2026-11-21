package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/config"
	"aura-forge/internal/domain"
	"aura-forge/internal/store/postgres"
)

// cmd/sde-import imports official Fuzzwork/CCP EVE SDE CSV dumps into PostgreSQL:
//   - invTypes.csv (typeID, groupID, typeName, ..., volume, ..., portionSize, ...)
//   - industryActivityProducts.csv (typeID, activityID, productTypeID, quantity)
//   - industryActivityMaterials.csv (typeID, activityID, materialTypeID, quantity)
//   - industryActivity.csv (typeID, activityID, time)
//
// Usage:
//   go run ./cmd/sde-import -dir ./sde_csv
func main() {
	dir := flag.String("dir", "./sde_csv", "Directory containing Fuzzwork/CCP SDE CSV files")
	flag.Parse()

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to PostgreSQL: %v", err)
	}
	defer db.Close()

	typesByID, err := loadInvTypes(filepath.Join(*dir, "invTypes.csv"))
	if err != nil {
		log.Fatalf("loading invTypes.csv: %v", err)
	}

	buildTimes, err := loadIndustryTimes(filepath.Join(*dir, "industryActivity.csv"))
	if err != nil {
		log.Fatalf("loading industryActivity.csv: %v", err)
	}

	blueprints, bpSet, err := loadIndustryProducts(filepath.Join(*dir, "industryActivityProducts.csv"), typesByID, buildTimes)
	if err != nil {
		log.Fatalf("loading industryActivityProducts.csv: %v", err)
	}

	materials, referencedTypes, err := loadIndustryMaterials(filepath.Join(*dir, "industryActivityMaterials.csv"), bpSet)
	if err != nil {
		log.Fatalf("loading industryActivityMaterials.csv: %v", err)
	}

	// Filter sde_types to blueprints, products, and materials used in manufacturing
	typesList := make([]domain.SDEType, 0, len(referencedTypes))
	for id := range referencedTypes {
		if t, ok := typesByID[id]; ok {
			typesList = append(typesList, t)
		}
	}

	repo := postgres.NewSDERepo(db.Pool)
	if err := repo.UpsertSDEData(ctx, typesList, blueprints, materials); err != nil {
		log.Fatalf("upserting SDE data: %v", err)
	}

	fmt.Printf("SDE import complete: %d types, %d blueprints, %d blueprint BOMs upserted.\n", len(typesList), len(blueprints), len(materials))
}

func loadInvTypes(path string) (map[int32]domain.SDEType, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idx := indexColumns(header)
	out := make(map[int32]domain.SDEType)

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		id := parseInt32(getCol(rec, idx, "typeID"))
		name := strings.TrimSpace(getCol(rec, idx, "typeName"))
		if id <= 0 || name == "" {
			continue
		}
		vol, _ := strconv.ParseFloat(getCol(rec, idx, "volume"), 64)
		portion := int(parseInt32(getCol(rec, idx, "portionSize")))
		if portion <= 0 {
			portion = 1
		}
		out[id] = domain.SDEType{
			TypeID:      id,
			Name:        name,
			Category:    "Other",
			VolumeM3:    vol,
			PortionSize: portion,
		}
	}
	return out, nil
}

func loadIndustryTimes(path string) (map[int32]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idx := indexColumns(header)
	out := make(map[int32]int)

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		// ActivityID 1 = Manufacturing
		if parseInt32(getCol(rec, idx, "activityID")) != 1 {
			continue
		}
		bpID := parseInt32(getCol(rec, idx, "typeID"))
		sec := int(parseInt32(getCol(rec, idx, "time")))
		if bpID > 0 && sec > 0 {
			out[bpID] = sec
		}
	}
	return out, nil
}

func loadIndustryProducts(path string, types map[int32]domain.SDEType, times map[int32]int) ([]domain.SDEBlueprint, map[int32]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	idx := indexColumns(header)
	bps := make([]domain.SDEBlueprint, 0)
	bpSet := make(map[int32]struct{})

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if parseInt32(getCol(rec, idx, "activityID")) != 1 {
			continue
		}
		bpID := parseInt32(getCol(rec, idx, "typeID"))
		prodID := parseInt32(getCol(rec, idx, "productTypeID"))
		qty := int(parseInt32(getCol(rec, idx, "quantity")))
		if bpID <= 0 || prodID <= 0 {
			continue
		}
		if qty <= 0 {
			qty = 1
		}
		bpName := fmt.Sprintf("Blueprint #%d", bpID)
		if t, ok := types[bpID]; ok {
			bpName = t.Name
		}
		prodName := fmt.Sprintf("Type #%d", prodID)
		if t, ok := types[prodID]; ok {
			prodName = t.Name
		}
		sec := times[bpID]
		if sec <= 0 {
			sec = 3600
		}
		bps = append(bps, domain.SDEBlueprint{
			BlueprintTypeID:  bpID,
			Name:             bpName,
			ProductTypeID:    prodID,
			ProductName:      prodName,
			ProductQtyPerRun: qty,
			BaseTimeSeconds:  sec,
			TechLevel:        1,
			Category:         "SDE",
		})
		bpSet[bpID] = struct{}{}
	}
	return bps, bpSet, nil
}

func loadIndustryMaterials(path string, bpSet map[int32]struct{}) (map[int32][]domain.SDEMaterialRequirement, map[int32]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	idx := indexColumns(header)
	mats := make(map[int32][]domain.SDEMaterialRequirement)
	refTypes := make(map[int32]struct{})

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if parseInt32(getCol(rec, idx, "activityID")) != 1 {
			continue
		}
		bpID := parseInt32(getCol(rec, idx, "typeID"))
		if _, ok := bpSet[bpID]; !ok {
			continue
		}
		matID := parseInt32(getCol(rec, idx, "materialTypeID"))
		qty, _ := strconv.ParseInt(getCol(rec, idx, "quantity"), 10, 64)
		if matID <= 0 || qty <= 0 {
			continue
		}
		mats[bpID] = append(mats[bpID], domain.SDEMaterialRequirement{
			MaterialTypeID: matID,
			BaseQuantity:   qty,
		})
		refTypes[matID] = struct{}{}
	}
	return mats, refTypes, nil
}

func indexColumns(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

func getCol(rec []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

func parseInt32(s string) int32 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	return int32(v)
}
