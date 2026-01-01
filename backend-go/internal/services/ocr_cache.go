package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ocrCacheEntry struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

var ocrCacheMu sync.Mutex

func ocrCacheTTL() time.Duration {
	// 0 means never expire.
	v := strings.TrimSpace(os.Getenv("SBM_OCR_CACHE_TTL_HOURS"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Hour
}

func ocrCacheDir() string {
	base := strings.TrimSpace(os.Getenv("SBM_OCR_CACHE_DIR"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("DATA_DIR"))
	}
	if base == "" {
		base = "./data"
	}
	return filepath.Join(base, "ocr_cache")
}

func ensureOCRCacheDir() string {
	dir := ocrCacheDir()
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func stableArgsKey(extraArgs []string) string {
	if len(extraArgs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(extraArgs))
	for _, a := range extraArgs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func fileStatKey(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return strings.TrimSpace(path)
	}
	return strings.TrimSpace(path) + "|" + strconv.FormatInt(info.Size(), 10) + "|" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

func buildOCRCacheKey(kind string, fileSHA256 *string, filePath string, engine string, extraArgs []string) string {
	sha := ""
	if fileSHA256 != nil {
		sha = strings.TrimSpace(*fileSHA256)
	}
	if sha == "" {
		sha = fileStatKey(filePath)
	}
	raw := strings.Join([]string{
		"v1",
		strings.TrimSpace(kind),
		strings.TrimSpace(engine),
		strings.TrimSpace(sha),
		stableArgsKey(extraArgs),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func loadOCRTextCache(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	dir := ensureOCRCacheDir()
	p := filepath.Join(dir, key+".json")

	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}

	var e ocrCacheEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return "", false
	}

	ttl := ocrCacheTTL()
	if ttl > 0 && !e.CreatedAt.IsZero() && time.Since(e.CreatedAt) > ttl {
		_ = os.Remove(p)
		return "", false
	}

	if strings.TrimSpace(e.Text) == "" {
		return "", false
	}
	return e.Text, true
}

func saveOCRTextCache(key string, text string) {
	key = strings.TrimSpace(key)
	text = strings.TrimSpace(text)
	if key == "" || text == "" {
		return
	}

	ocrCacheMu.Lock()
	defer ocrCacheMu.Unlock()

	dir := ensureOCRCacheDir()
	p := filepath.Join(dir, key+".json")

	e := ocrCacheEntry{Text: text, CreatedAt: time.Now()}
	if b, err := json.Marshal(e); err == nil {
		_ = os.WriteFile(p, b, 0644)
	}
}

