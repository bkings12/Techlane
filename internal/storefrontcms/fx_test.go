package storefrontcms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/packages/pkg/db"
)

// TestGetRates_FallsBackToCacheOnFetchFailure proves a currency display
// glitch never breaks the storefront: once a rate is cached, a broken
// upstream must not surface as an error.
func TestGetRates_FallsBackToCacheOnFetchFailure(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	defer pool.Close()
	if err := platform.EnsureSchemas(ctx, pool); err != nil {
		t.Fatalf("schemas: %v", err)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := dir
	for {
		if _, statErr := os.Stat(filepath.Join(repoRoot, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Fatal("repo root not found")
		}
		repoRoot = parent
	}
	if err := platform.RunMigrations(ctx, pool, repoRoot); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	base := "TESTBASE" + uuid.New().String()[:8]

	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"success","rates":{"USD":0.0077,"EUR":0.0071}}`))
	}))
	defer working.Close()

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	svc := NewService(pool)

	origBase := fxAPIBase
	defer func() { fxAPIBase = origBase }()

	// First call: upstream works, populates the cache.
	fxAPIBase = working.URL + "/"
	rates, err := svc.GetRates(ctx, base)
	if err != nil {
		t.Fatalf("GetRates (working upstream): %v", err)
	}
	if rates["USD"] != 0.0077 {
		t.Fatalf("expected USD rate 0.0077, got %v", rates)
	}

	// Force a refetch by clearing the cache timestamp, then point at a
	// broken upstream — must still return the last-cached rates, not error.
	if _, err := pool.Exec(ctx, `UPDATE platform.fx_rate_cache SET fetched_at = now() - interval '1 day' WHERE base_code = $1`, base); err != nil {
		t.Fatalf("age cache: %v", err)
	}
	fxAPIBase = broken.URL + "/"
	fallback, err := svc.GetRates(ctx, base)
	if err != nil {
		t.Fatalf("GetRates should fall back to cache, not error: %v", err)
	}
	if fallback["USD"] != 0.0077 {
		t.Fatalf("expected fallback to cached USD rate 0.0077, got %v", fallback)
	}
}

func TestParseEnabledCurrencies(t *testing.T) {
	cases := map[string][]string{
		"":            nil,
		"KES":         {"KES"},
		"KES,USD":     {"KES", "USD"},
		" kes , usd ": {"KES", "USD"},
		",,":          nil,
	}
	for input, want := range cases {
		got := ParseEnabledCurrencies(input)
		if len(got) != len(want) {
			t.Errorf("ParseEnabledCurrencies(%q) = %v, want %v", input, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("ParseEnabledCurrencies(%q) = %v, want %v", input, got, want)
				break
			}
		}
	}
}
