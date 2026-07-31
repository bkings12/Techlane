package identity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/techlane/techlane/packages/pkg/authz"
)

var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)

type Role struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateRoleInput struct {
	Key         string
	Name        string
	Description string
	Permissions []string
}

type UpdateRoleInput struct {
	Name        *string
	Description *string
	Permissions []string // if non-nil, replace full set
}

func (s *Service) SeedPermissionCatalog(ctx context.Context) error {
	for _, p := range authz.SystemPermissionCatalog() {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO identity.permission_catalog (code, description, category, is_system)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description, category = EXCLUDED.category`,
			p.Code, p.Description, p.Category)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SeedSystemRoles(ctx context.Context) error {
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
		if err := s.ensureSystemRolesForTenant(ctx, tenantID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureSystemRolesForTenant(ctx context.Context, tenantID uuid.UUID) error {
	for _, key := range authz.AllRoles() {
		var roleID uuid.UUID
		err := s.pool.QueryRow(ctx, `
			SELECT id FROM identity.roles WHERE tenant_id = $1 AND key = $2`, tenantID, key).Scan(&roleID)
		if errors.Is(err, pgx.ErrNoRows) {
			roleID = uuid.New()
			_, err = s.pool.Exec(ctx, `
				INSERT INTO identity.roles (id, tenant_id, key, name, description, is_system)
				VALUES ($1, $2, $3, $4, $5, true)`,
				roleID, tenantID, key, key, "System role: "+key)
			if err != nil {
				return err
			}
			perms := authz.DefaultPermissions(key)
			if key == "owner" {
				perms = []string{"*"}
			}
			for _, p := range perms {
				if p == "*" {
					continue // owner handled as wildcard in HasPermission via role key
				}
				if _, err := s.pool.Exec(ctx, `
					INSERT INTO identity.role_permissions (role_id, permission_code) VALUES ($1, $2)
					ON CONFLICT DO NOTHING`, roleID, p); err != nil {
					return err
				}
			}
			continue
		}
		if err != nil {
			return err
		}
		// Top up missing defaults for existing system roles (new catalog entries).
		if key == "owner" {
			continue
		}
		for _, p := range authz.DefaultPermissions(key) {
			if p == "*" {
				continue
			}
			if _, err := s.pool.Exec(ctx, `
				INSERT INTO identity.role_permissions (role_id, permission_code) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, roleID, p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ListPermissionCatalog(ctx context.Context) ([]authz.PermissionDef, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT code, description, category FROM identity.permission_catalog ORDER BY category, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []authz.PermissionDef
	for rows.Next() {
		var p authz.PermissionDef
		if err := rows.Scan(&p.Code, &p.Description, &p.Category); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if len(items) == 0 {
		return authz.SystemPermissionCatalog(), nil
	}
	return items, nil
}

func (s *Service) CreatePermission(ctx context.Context, code, description, category string) (*authz.PermissionDef, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if !roleKeyPattern.MatchString(strings.ReplaceAll(code, ".", "_")) && !regexp.MustCompile(`^[a-z][a-z0-9_.]{1,62}$`).MatchString(code) {
		return nil, fmt.Errorf("%w: invalid permission code", ErrInvalidInput)
	}
	if category == "" {
		category = "custom"
	}
	if description == "" {
		description = code
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity.permission_catalog (code, description, category, is_system)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (code) DO NOTHING`, code, description, category)
	if err != nil {
		return nil, err
	}
	return &authz.PermissionDef{Code: code, Description: description, Category: category}, nil
}

func (s *Service) ListRolesDB(ctx context.Context, tenantID uuid.UUID) ([]Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, key, name, description, is_system, created_at
		FROM identity.roles WHERE tenant_id = $1 ORDER BY is_system DESC, name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Key, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt); err != nil {
			return nil, err
		}
		perms, err := s.rolePermissions(ctx, r.ID, r.Key)
		if err != nil {
			return nil, err
		}
		r.Permissions = perms
		items = append(items, r)
	}
	return items, nil
}

func (s *Service) rolePermissions(ctx context.Context, roleID uuid.UUID, key string) ([]string, error) {
	if key == "owner" {
		return []string{"*"}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT permission_code FROM identity.role_permissions WHERE role_id = $1 ORDER BY permission_code`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func (s *Service) GetRole(ctx context.Context, tenantID, roleID uuid.UUID) (*Role, error) {
	var r Role
	err := s.pool.QueryRow(ctx, `
		SELECT id, key, name, description, is_system, created_at
		FROM identity.roles WHERE tenant_id = $1 AND id = $2`, tenantID, roleID).
		Scan(&r.ID, &r.Key, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	perms, err := s.rolePermissions(ctx, r.ID, r.Key)
	if err != nil {
		return nil, err
	}
	r.Permissions = perms
	return &r, nil
}

func (s *Service) CreateRole(ctx context.Context, tenantID, actorID uuid.UUID, in CreateRoleInput) (*Role, error) {
	key := strings.ToLower(strings.TrimSpace(in.Key))
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = key
	}
	if !roleKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("%w: key must be lowercase letters, numbers, underscore", ErrInvalidInput)
	}
	if authz.ValidRole(key) {
		return nil, fmt.Errorf("%w: key collides with system role", ErrInvalidInput)
	}
	if err := s.validatePermissionCodes(ctx, in.Permissions); err != nil {
		return nil, err
	}

	id := uuid.New()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO identity.roles (id, tenant_id, key, name, description, is_system, created_by)
		VALUES ($1, $2, $3, $4, $5, false, $6)`,
		id, tenantID, key, name, in.Description, actorID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("%w: role key already exists", ErrDuplicateUser)
		}
		return nil, err
	}
	for _, p := range in.Permissions {
		if _, err := tx.Exec(ctx, `INSERT INTO identity.role_permissions (role_id, permission_code) VALUES ($1, $2)`, id, p); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetRole(ctx, tenantID, id)
}

func (s *Service) UpdateRole(ctx context.Context, tenantID, roleID, actorID uuid.UUID, in UpdateRoleInput) (*Role, error) {
	r, err := s.GetRole(ctx, tenantID, roleID)
	if err != nil {
		return nil, err
	}
	if r.Key == "owner" && in.Permissions != nil {
		return nil, fmt.Errorf("%w: owner permissions cannot be changed", ErrForbidden)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if in.Name != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.roles SET name = $1, updated_at = now() WHERE id = $2`, *in.Name, roleID); err != nil {
			return nil, err
		}
	}
	if in.Description != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.roles SET description = $1, updated_at = now() WHERE id = $2`, *in.Description, roleID); err != nil {
			return nil, err
		}
	}
	if in.Permissions != nil {
		if err := s.validatePermissionCodes(ctx, in.Permissions); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM identity.role_permissions WHERE role_id = $1`, roleID); err != nil {
			return nil, err
		}
		for _, p := range in.Permissions {
			if p == "*" {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO identity.role_permissions (role_id, permission_code) VALUES ($1, $2)`, roleID, p); err != nil {
				return nil, err
			}
		}
	}
	_ = actorID
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetRole(ctx, tenantID, roleID)
}

func (s *Service) DeleteRole(ctx context.Context, tenantID, roleID uuid.UUID) error {
	r, err := s.GetRole(ctx, tenantID, roleID)
	if err != nil {
		return err
	}
	if r.IsSystem {
		return fmt.Errorf("%w: cannot delete system role", ErrForbidden)
	}
	var inUse int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM identity.user_roles WHERE role = $1`, r.Key).Scan(&inUse); err != nil {
		return err
	}
	if inUse > 0 {
		return fmt.Errorf("%w: role is assigned to %d user(s)", ErrInvalidInput, inUse)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM identity.roles WHERE tenant_id = $1 AND id = $2`, tenantID, roleID)
	return err
}

func (s *Service) validatePermissionCodes(ctx context.Context, codes []string) error {
	for _, code := range codes {
		if code == "*" {
			return fmt.Errorf("%w: cannot assign wildcard permission to custom roles", ErrInvalidInput)
		}
		var exists bool
		err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity.permission_catalog WHERE code = $1)`, code).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: unknown permission %s", ErrInvalidInput, code)
		}
	}
	return nil
}

func (s *Service) RoleKeyExists(ctx context.Context, tenantID uuid.UUID, key string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM identity.roles WHERE tenant_id = $1 AND key = $2)`, tenantID, key).Scan(&exists)
	return exists, err
}

func (s *Service) PermissionsForRoleKeys(ctx context.Context, tenantID uuid.UUID, keys []string) ([]string, error) {
	pm := map[string]struct{}{}
	for _, key := range keys {
		if key == "owner" {
			return []string{"*"}, nil
		}
		var roleID uuid.UUID
		err := s.pool.QueryRow(ctx, `
			SELECT id FROM identity.roles WHERE tenant_id = $1 AND key = $2`, tenantID, key).Scan(&roleID)
		if errors.Is(err, pgx.ErrNoRows) {
			// Fallback to hardcoded defaults for not-yet-seeded tenants
			for _, p := range authz.DefaultPermissions(key) {
				pm[p] = struct{}{}
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		perms, err := s.rolePermissions(ctx, roleID, key)
		if err != nil {
			return nil, err
		}
		for _, p := range perms {
			pm[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(pm))
	for p := range pm {
		out = append(out, p)
	}
	return out, nil
}
