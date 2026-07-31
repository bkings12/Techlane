// Package appversion lets the Android apps ask "is a newer build available?"
// on launch, without depending on the Play Store. Ops publishes new rows via
// POST /app-releases whenever a build is cut; clients read GET /app-version.
package appversion

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Release struct {
	App                     string    `json:"app"`
	Platform                string    `json:"platform"`
	VersionCode             int       `json:"latest_version_code"`
	VersionName             string    `json:"latest_version_name"`
	MinSupportedVersionCode int       `json:"min_supported_version_code"`
	DownloadURL             string    `json:"download_url,omitempty"`
	Notes                   string    `json:"notes,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}

func (s *Service) Latest(ctx context.Context, app, platform string) (*Release, error) {
	var rel Release
	var downloadURL, notes *string
	err := s.pool.QueryRow(ctx, `
		SELECT app, platform, version_code, version_name, min_supported_version_code, download_url, notes, created_at
		FROM platform.app_releases
		WHERE app = $1 AND platform = $2
		ORDER BY version_code DESC
		LIMIT 1`, app, platform).
		Scan(&rel.App, &rel.Platform, &rel.VersionCode, &rel.VersionName, &rel.MinSupportedVersionCode, &downloadURL, &notes, &rel.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if downloadURL != nil {
		rel.DownloadURL = *downloadURL
	}
	if notes != nil {
		rel.Notes = *notes
	}
	return &rel, nil
}

type PublishInput struct {
	App                     string
	Platform                string
	VersionCode             int
	VersionName             string
	MinSupportedVersionCode int
	DownloadURL             string
	Notes                   string
	ActorID                 uuid.UUID
}

func (s *Service) Publish(ctx context.Context, in PublishInput) (*Release, error) {
	id := uuid.New()
	if in.MinSupportedVersionCode <= 0 {
		in.MinSupportedVersionCode = 1
	}
	var downloadURL, notes *string
	if in.DownloadURL != "" {
		downloadURL = &in.DownloadURL
	}
	if in.Notes != "" {
		notes = &in.Notes
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform.app_releases (id, app, platform, version_code, version_name, min_supported_version_code, download_url, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (app, platform, version_code) DO UPDATE SET
		  version_name = EXCLUDED.version_name,
		  min_supported_version_code = EXCLUDED.min_supported_version_code,
		  download_url = EXCLUDED.download_url,
		  notes = EXCLUDED.notes`,
		id, in.App, in.Platform, in.VersionCode, in.VersionName, in.MinSupportedVersionCode, downloadURL, notes, in.ActorID)
	if err != nil {
		return nil, err
	}
	return s.Latest(ctx, in.App, in.Platform)
}

func (s *Service) ListReleases(ctx context.Context, app string) ([]Release, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT app, platform, version_code, version_name, min_supported_version_code, download_url, notes, created_at
		FROM platform.app_releases
		WHERE ($1 = '' OR app = $1)
		ORDER BY app, platform, version_code DESC`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Release{}
	for rows.Next() {
		var rel Release
		var downloadURL, notes *string
		if err := rows.Scan(&rel.App, &rel.Platform, &rel.VersionCode, &rel.VersionName, &rel.MinSupportedVersionCode, &downloadURL, &notes, &rel.CreatedAt); err != nil {
			return nil, err
		}
		if downloadURL != nil {
			rel.DownloadURL = *downloadURL
		}
		if notes != nil {
			rel.Notes = *notes
		}
		items = append(items, rel)
	}
	return items, rows.Err()
}
