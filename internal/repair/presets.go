package repair

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	PresetKindConditionTag = "condition_tag"
	PresetKindIssue        = "issue"
)

var (
	ErrPresetNotFound = errors.New("intake preset not found")
	ErrPresetInvalid  = errors.New("invalid intake preset")
	ErrPresetConflict = errors.New("intake preset already exists")
)

// IntakePreset is a tenant-scoped suggestion for intake UI (not a FK on jobs).
type IntakePreset struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Label     string    `json:"label"`
	SortOrder int       `json:"sort_order"`
	IsSystem  bool      `json:"is_system"`
	IsActive  bool      `json:"is_active"`
}

type systemPresetSeed struct {
	Kind      string
	Label     string
	SortOrder int
}

func defaultSystemPresets() []systemPresetSeed {
	return []systemPresetSeed{
		{Kind: PresetKindConditionTag, Label: "Back cover missing", SortOrder: 10},
		{Kind: PresetKindConditionTag, Label: "Screen scratches", SortOrder: 20},
		{Kind: PresetKindConditionTag, Label: "Powers on", SortOrder: 30},
		{Kind: PresetKindConditionTag, Label: "Does not power on", SortOrder: 40},
		{Kind: PresetKindConditionTag, Label: "Liquid marks", SortOrder: 50},
		{Kind: PresetKindConditionTag, Label: "Bent frame", SortOrder: 60},
		{Kind: PresetKindConditionTag, Label: "Missing screws", SortOrder: 70},

		{Kind: PresetKindIssue, Label: "Screen cracked", SortOrder: 10},
		{Kind: PresetKindIssue, Label: "Won't charge", SortOrder: 20},
		{Kind: PresetKindIssue, Label: "No power", SortOrder: 30},
		{Kind: PresetKindIssue, Label: "Water damage", SortOrder: 40},
		{Kind: PresetKindIssue, Label: "Battery draining fast", SortOrder: 50},
		{Kind: PresetKindIssue, Label: "Speaker / mic issue", SortOrder: 60},
		{Kind: PresetKindIssue, Label: "Camera not working", SortOrder: 70},
		{Kind: PresetKindIssue, Label: "Software / boot loop", SortOrder: 80},
		{Kind: PresetKindIssue, Label: "Charging port damaged", SortOrder: 90},
	}
}

func validPresetKind(kind string) bool {
	return kind == PresetKindConditionTag || kind == PresetKindIssue
}

// SeedSystemIntakePresets inserts built-in condition/issue suggestions for every tenant.
func (s *Service) SeedSystemIntakePresets(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id FROM identity.tenants`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var tenants []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		tenants = append(tenants, id)
	}
	for _, tenantID := range tenants {
		if err := s.ensureSystemPresetsForTenant(ctx, tenantID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureSystemPresetsForTenant(ctx context.Context, tenantID uuid.UUID) error {
	for _, seed := range defaultSystemPresets() {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO repair.intake_presets (id, tenant_id, kind, label, sort_order, is_system, is_active)
			VALUES ($1, $2, $3, $4, $5, true, true)
			ON CONFLICT (tenant_id, kind, label) DO NOTHING`,
			uuid.New(), tenantID, seed.Kind, seed.Label, seed.SortOrder)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListIntakePresets returns presets for a kind. Active-only by default; set includeInactive for settings.
func (s *Service) ListIntakePresets(ctx context.Context, tenantID uuid.UUID, kind string, includeInactive bool) ([]IntakePreset, error) {
	if kind != "" && !validPresetKind(kind) {
		return nil, fmt.Errorf("%w: kind must be condition_tag or issue", ErrPresetInvalid)
	}
	if err := s.ensureSystemPresetsForTenant(ctx, tenantID); err != nil {
		return nil, err
	}

	q := `
		SELECT id, kind, label, sort_order, is_system, is_active
		FROM repair.intake_presets
		WHERE tenant_id = $1`
	args := []any{tenantID}
	if kind != "" {
		args = append(args, kind)
		q += fmt.Sprintf(` AND kind = $%d`, len(args))
	}
	if !includeInactive {
		q += ` AND is_active = true`
	}
	q += ` ORDER BY kind, sort_order, label`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]IntakePreset, 0)
	for rows.Next() {
		var p IntakePreset
		if err := rows.Scan(&p.ID, &p.Kind, &p.Label, &p.SortOrder, &p.IsSystem, &p.IsActive); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Service) getIntakePreset(ctx context.Context, tenantID, id uuid.UUID) (*IntakePreset, error) {
	var p IntakePreset
	err := s.pool.QueryRow(ctx, `
		SELECT id, kind, label, sort_order, is_system, is_active
		FROM repair.intake_presets
		WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&p.ID, &p.Kind, &p.Label, &p.SortOrder, &p.IsSystem, &p.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPresetNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) CreateIntakePreset(ctx context.Context, tenantID uuid.UUID, kind, label string) (*IntakePreset, error) {
	kind = strings.TrimSpace(kind)
	label = strings.TrimSpace(label)
	if !validPresetKind(kind) {
		return nil, fmt.Errorf("%w: kind must be condition_tag or issue", ErrPresetInvalid)
	}
	if label == "" {
		return nil, fmt.Errorf("%w: label is required", ErrPresetInvalid)
	}
	if err := s.ensureSystemPresetsForTenant(ctx, tenantID); err != nil {
		return nil, err
	}

	var maxSort int
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(sort_order), 0) FROM repair.intake_presets
		WHERE tenant_id = $1 AND kind = $2`, tenantID, kind).Scan(&maxSort)

	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.intake_presets (id, tenant_id, kind, label, sort_order, is_system, is_active)
		VALUES ($1, $2, $3, $4, $5, false, true)`,
		id, tenantID, kind, label, maxSort+10)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrPresetConflict
		}
		return nil, err
	}
	return s.getIntakePreset(ctx, tenantID, id)
}

func (s *Service) UpdateIntakePreset(ctx context.Context, tenantID, id uuid.UUID, label *string, isActive *bool) (*IntakePreset, error) {
	p, err := s.getIntakePreset(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if label == nil && isActive == nil {
		return p, nil
	}

	nextLabel := p.Label
	if label != nil {
		nextLabel = strings.TrimSpace(*label)
		if nextLabel == "" {
			return nil, fmt.Errorf("%w: label is required", ErrPresetInvalid)
		}
	}
	nextActive := p.IsActive
	if isActive != nil {
		nextActive = *isActive
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE repair.intake_presets
		SET label = $1, is_active = $2
		WHERE tenant_id = $3 AND id = $4`,
		nextLabel, nextActive, tenantID, id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrPresetConflict
		}
		return nil, err
	}
	return s.getIntakePreset(ctx, tenantID, id)
}

// DeleteIntakePreset hard-deletes tenant-created presets. System presets are deactivated instead.
func (s *Service) DeleteIntakePreset(ctx context.Context, tenantID, id uuid.UUID) error {
	p, err := s.getIntakePreset(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if p.IsSystem {
		_, err = s.pool.Exec(ctx, `
			UPDATE repair.intake_presets SET is_active = false
			WHERE tenant_id = $1 AND id = $2`, tenantID, id)
		return err
	}
	_, err = s.pool.Exec(ctx, `
		DELETE FROM repair.intake_presets WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return err
}
