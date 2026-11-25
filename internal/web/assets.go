package web

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	staticAssetMu    sync.RWMutex
	staticAssetCache = make(map[string]string)
)

// StaticAssetURL appends a short content-hash query parameter (?v=<8-hex>) to /static/... URLs
// so browsers automatically fetch updated CSS/JS bundles after every rebuild without hard-refreshing.
func StaticAssetURL(publicPath string) string {
	staticAssetMu.RLock()
	if cached, ok := staticAssetCache[publicPath]; ok {
		staticAssetMu.RUnlock()
		return cached
	}
	staticAssetMu.RUnlock()

	rel := strings.TrimPrefix(publicPath, "/static/")
	if rel == publicPath || rel == "" {
		return publicPath
	}

	diskPath := filepath.Join("web", "static", filepath.FromSlash(rel))
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return publicPath
	}

	sum := sha256.Sum256(data)
	versioned := publicPath + "?v=" + hex.EncodeToString(sum[:4])

	staticAssetMu.Lock()
	staticAssetCache[publicPath] = versioned
	staticAssetMu.Unlock()

	return versioned
}
