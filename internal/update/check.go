package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const checkInterval = 24 * time.Hour

// releaseURL is the GitHub API endpoint for the latest release.
// Overridden in tests.
var releaseURL = "https://api.github.com/repos/jhuiting/chargebee-cli/releases/latest"

// Info holds the result of an update check.
type Info struct {
	CurrentVersion   string
	LatestVersion    string
	UpdateAvailable  bool
	NotifiedRecently bool
}

type cache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	NotifiedAt    time.Time `json:"notified_at,omitempty"`
}

// CheckForUpdate checks if a newer CLI version is available.
// Uses a 24-hour cache to avoid excessive API calls. Returns nil if
// the check is skipped (e.g., dev build or disabled via env var), the
// cache file cannot be determined, or an error occurs while fetching the
// latest version. When a non-nil *Info is returned, callers should inspect
// Info.UpdateAvailable to determine whether an update is actually available.
func CheckForUpdate(ctx context.Context, currentVersion string) *Info {
	if currentVersion == "dev" || os.Getenv("CB_NO_UPDATE_CHECK") != "" {
		return nil
	}

	cachePath := cacheFilePath()
	if cachePath == "" {
		return nil
	}

	cached, err := readCache(cachePath)
	if err == nil && time.Since(cached.CheckedAt) < checkInterval {
		info := buildInfo(currentVersion, cached.LatestVersion)
		info.NotifiedRecently = !cached.NotifiedAt.IsZero() && time.Since(cached.NotifiedAt) < checkInterval
		return info
	}

	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		return nil
	}

	var notifiedAt time.Time
	if cached != nil {
		notifiedAt = cached.NotifiedAt
	}

	_ = writeCache(cachePath, &cache{
		CheckedAt:     time.Now(),
		LatestVersion: latest,
		NotifiedAt:    notifiedAt,
	})

	info := buildInfo(currentVersion, latest)
	info.NotifiedRecently = !notifiedAt.IsZero() && time.Since(notifiedAt) < checkInterval
	return info
}

// MarkNotified records that the update notification was shown, so it can be
// suppressed for the next 24 hours.
func MarkNotified() {
	path := cacheFilePath()
	if path == "" {
		return
	}
	cached, err := readCache(path)
	if err != nil {
		return
	}
	cached.NotifiedAt = time.Now()
	_ = writeCache(path, cached)
}

func cacheFilePath() string {
	dir := os.Getenv("CB_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config", "chargebee")
	}
	return filepath.Join(dir, "update-check.json")
}

func fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return "", fmt.Errorf("GitHub release tag_name is empty")
	}

	return release.TagName, nil
}

func readCache(path string) (*cache, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path derived from user home dir
	if err != nil {
		return nil, err
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeCache(path string, c *cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func buildInfo(current, latest string) *Info {
	return &Info{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: compareSemver(latest, current) > 0,
	}
}

// compareSemver compares two semver strings.
// Returns positive if a > b, negative if a < b, 0 if equal.
func compareSemver(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	if idx := strings.IndexByte(a, '-'); idx >= 0 {
		a = a[:idx]
	}
	if idx := strings.IndexByte(b, '-'); idx >= 0 {
		b = b[:idx]
	}

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := range max(len(aParts), len(bParts)) {
		var aNum, bNum int
		if i < len(aParts) {
			aNum, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bNum, _ = strconv.Atoi(bParts[i])
		}
		if aNum != bNum {
			return aNum - bNum
		}
	}
	return 0
}
