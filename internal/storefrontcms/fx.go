package storefrontcms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const fxCacheTTL = 12 * time.Hour

// fxAPIBase is a free, no-API-key exchange-rate endpoint
// (https://www.exchangerate-api.com/docs/free). The storefront currency
// switcher is display-only — checkout always settles in KES via M-Pesa — so
// staleness here is cosmetic, not a money-handling risk.
var fxAPIBase = "https://open.er-api.com/v6/latest/"

type fxAPIResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

// GetRates returns cached rates for base, refreshing from the vendor when
// the cache is missing or stale. A fetch failure never surfaces as an
// error as long as *any* cached value exists — a currency display glitch
// must not take down the storefront.
func (s *Service) GetRates(ctx context.Context, base string) (map[string]float64, error) {
	cached, fetchedAt, err := s.cachedRates(ctx, base)
	if err == nil && time.Since(fetchedAt) < fxCacheTTL {
		return cached, nil
	}

	fresh, fetchErr := s.fetchRates(ctx, base)
	if fetchErr != nil {
		if cached != nil {
			return cached, nil
		}
		return nil, fetchErr
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO platform.fx_rate_cache (base_code, rates, fetched_at)
		VALUES ($1, $2, now())
		ON CONFLICT (base_code) DO UPDATE SET rates = $2, fetched_at = now()`,
		base, fresh)
	return fresh, nil
}

func (s *Service) cachedRates(ctx context.Context, base string) (map[string]float64, time.Time, error) {
	var raw []byte
	var fetchedAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT rates, fetched_at FROM platform.fx_rate_cache WHERE base_code = $1`, base).
		Scan(&raw, &fetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, err
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	var rates map[string]float64
	if err := json.Unmarshal(raw, &rates); err != nil {
		return nil, time.Time{}, err
	}
	return rates, fetchedAt, nil
}

func (s *Service) fetchRates(ctx context.Context, base string) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fxAPIBase+base, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fx rate provider returned status %d", resp.StatusCode)
	}
	var out fxAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Result != "success" || len(out.Rates) == 0 {
		return nil, fmt.Errorf("fx rate provider returned no rates")
	}
	return out.Rates, nil
}

// ParseEnabledCurrencies parses the comma-separated ISO code list an owner
// set on storefront_settings.enabled_currencies. Empty/unset means the
// switcher is hidden entirely.
func ParseEnabledCurrencies(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		code := strings.ToUpper(strings.TrimSpace(part))
		if code != "" {
			out = append(out, code)
		}
	}
	return out
}
