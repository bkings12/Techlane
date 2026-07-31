package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/techlane/techlane/packages/pkg/authz"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrForbidden     = errors.New("forbidden")
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrDuplicateUser = errors.New("user already exists")
)

type Branch struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Code     string    `json:"code"`
	Location string    `json:"location,omitempty"`
	Phone    string    `json:"phone,omitempty"`
	Hours    string    `json:"hours,omitempty"`
	MapURL   string    `json:"map_url,omitempty"`
}

type EmployeeProfile struct {
	UserID            uuid.UUID `json:"user_id"`
	EmployeeCode      *string   `json:"employee_code,omitempty"`
	Phone             *string   `json:"phone,omitempty"`
	IsTechnician      bool      `json:"is_technician"`
	CommissionEnabled bool      `json:"commission_enabled"`
	CommissionType    string    `json:"commission_type"`
	PercentBPS        *int      `json:"percent_bps,omitempty"`
	FixedAmount       *float64  `json:"fixed_amount,omitempty"`
}

type StaffUser struct {
	ID          uuid.UUID        `json:"id"`
	Email       string           `json:"email"`
	DisplayName string           `json:"display_name"`
	Status      string           `json:"status"`
	Roles       []string         `json:"roles"`
	BranchIDs   []string         `json:"branch_ids"`
	Profile     *EmployeeProfile `json:"profile,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}

type CreateUserInput struct {
	Email        string
	DisplayName  string
	Password     string
	Roles        []string
	BranchIDs    []uuid.UUID
	Phone        *string
	EmployeeCode *string
	IsTechnician bool
}

type UpdateUserInput struct {
	DisplayName  *string
	Status       *string
	Roles        []string
	BranchIDs    []uuid.UUID
	Phone        *string
	EmployeeCode *string
	IsTechnician *bool
	Password     *string
}

func hasRole(roles []string, wanted string) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

func validateTechnicianBranchAccess(isTechnician bool, branchIDs []uuid.UUID) error {
	if isTechnician && len(branchIDs) != 1 {
		return fmt.Errorf("%w: a technician must be assigned to exactly one branch", ErrInvalidInput)
	}
	return nil
}

func (s *Service) ListBranches(ctx context.Context, tenantID uuid.UUID) ([]Branch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, code, COALESCE(location, ''), COALESCE(phone, ''), COALESCE(hours, ''), COALESCE(map_url, '')
		FROM identity.branches WHERE tenant_id = $1 ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Branch
	for rows.Next() {
		var b Branch
		if err := rows.Scan(&b.ID, &b.Name, &b.Code, &b.Location, &b.Phone, &b.Hours, &b.MapURL); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, nil
}

// BranchContactInput carries the optional store-locator fields — nil means
// "leave unchanged" on update, or "blank" on create.
type BranchContactInput struct {
	Phone  *string
	Hours  *string
	MapURL *string
}

func (s *Service) CreateBranch(ctx context.Context, tenantID uuid.UUID, name, code, location string, contact BranchContactInput) (*Branch, error) {
	name = strings.TrimSpace(name)
	code = strings.ToUpper(strings.TrimSpace(code))
	location = strings.TrimSpace(location)
	if name == "" || code == "" {
		return nil, fmt.Errorf("%w: branch name and code required", ErrInvalidInput)
	}
	branch := &Branch{
		ID: uuid.New(), Name: name, Code: code, Location: location,
		Phone: trimmedOrEmpty(contact.Phone), Hours: trimmedOrEmpty(contact.Hours), MapURL: trimmedOrEmpty(contact.MapURL),
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity.branches (id, tenant_id, name, code, location, phone, hours, map_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		branch.ID, tenantID, branch.Name, branch.Code, branch.Location,
		nullIfEmpty(branch.Phone), nullIfEmpty(branch.Hours), nullIfEmpty(branch.MapURL))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("%w: branch code already exists", ErrInvalidInput)
		}
		return nil, err
	}
	return branch, nil
}

func (s *Service) UpdateBranch(ctx context.Context, tenantID, branchID uuid.UUID, name, code, location *string, contact BranchContactInput) (*Branch, error) {
	var branch Branch
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, code, COALESCE(location, ''), COALESCE(phone, ''), COALESCE(hours, ''), COALESCE(map_url, '')
		FROM identity.branches WHERE tenant_id = $1 AND id = $2`,
		tenantID, branchID).Scan(&branch.ID, &branch.Name, &branch.Code, &branch.Location, &branch.Phone, &branch.Hours, &branch.MapURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if name != nil {
		branch.Name = strings.TrimSpace(*name)
	}
	if code != nil {
		branch.Code = strings.ToUpper(strings.TrimSpace(*code))
	}
	if location != nil {
		branch.Location = strings.TrimSpace(*location)
	}
	if contact.Phone != nil {
		branch.Phone = strings.TrimSpace(*contact.Phone)
	}
	if contact.Hours != nil {
		branch.Hours = strings.TrimSpace(*contact.Hours)
	}
	if contact.MapURL != nil {
		branch.MapURL = strings.TrimSpace(*contact.MapURL)
	}
	if branch.Name == "" || branch.Code == "" {
		return nil, fmt.Errorf("%w: branch name and code required", ErrInvalidInput)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE identity.branches SET name = $1, code = $2, location = $3, phone = $4, hours = $5, map_url = $6
		WHERE tenant_id = $7 AND id = $8`,
		branch.Name, branch.Code, branch.Location, nullIfEmpty(branch.Phone), nullIfEmpty(branch.Hours), nullIfEmpty(branch.MapURL),
		tenantID, branchID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("%w: branch code already exists", ErrInvalidInput)
		}
		return nil, err
	}
	return &branch, nil
}

func trimmedOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func (s *Service) DeleteBranch(ctx context.Context, tenantID, branchID uuid.UUID) error {
	var assigned int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM identity.branch_memberships WHERE branch_id = $1`, branchID).Scan(&assigned); err != nil {
		return err
	}
	if assigned > 0 {
		return fmt.Errorf("%w: branch still has assigned users — reassign staff first", ErrInvalidInput)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM identity.branches WHERE tenant_id = $1 AND id = $2`, tenantID, branchID)
	if err != nil {
		return fmt.Errorf("%w: branch has dependent records", ErrInvalidInput)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListRoles() []map[string]any {
	return authz.RoleCatalog()
}

// ListRolesForTenant returns DB-backed roles (system + custom).
func (s *Service) ListRolesForTenant(ctx context.Context, tenantID uuid.UUID) ([]Role, error) {
	return s.ListRolesDB(ctx, tenantID)
}

func (s *Service) validateRoleKeys(ctx context.Context, tenantID uuid.UUID, roles []string) error {
	for _, role := range roles {
		ok, err := s.RoleKeyExists(ctx, tenantID, role)
		if err != nil {
			return err
		}
		if !ok && !authz.ValidRole(role) {
			return fmt.Errorf("%w: unknown role %s", ErrInvalidInput, role)
		}
	}
	return nil
}

func (s *Service) ListUsers(ctx context.Context, tenantID uuid.UUID, role, status string) ([]StaffUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.created_at
		FROM identity.users u
		WHERE u.tenant_id = $1
		  AND ($2 = '' OR u.status = $2)
		  AND ($3 = '' OR EXISTS (
		        SELECT 1 FROM identity.user_roles ur WHERE ur.user_id = u.id AND ur.role = $3
		      ))
		ORDER BY u.display_name`, tenantID, status, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StaffUser
	for rows.Next() {
		var u StaffUser
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		if err := s.hydrateStaff(ctx, &u); err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	return items, nil
}

func (s *Service) ListTechnicians(ctx context.Context, tenantID uuid.UUID) ([]StaffUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.created_at
		FROM identity.users u
		LEFT JOIN identity.employee_profiles p ON p.user_id = u.id
		WHERE u.tenant_id = $1 AND u.status = 'active'
		  AND (
		    COALESCE(p.is_technician, false) = true
		    OR EXISTS (SELECT 1 FROM identity.user_roles ur WHERE ur.user_id = u.id AND ur.role = 'technician')
		  )
		ORDER BY u.display_name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StaffUser
	for rows.Next() {
		var u StaffUser
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		if err := s.hydrateStaff(ctx, &u); err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	return items, nil
}

func (s *Service) GetUser(ctx context.Context, tenantID, userID uuid.UUID) (*StaffUser, error) {
	var u StaffUser
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name, status, created_at
		FROM identity.users WHERE tenant_id = $1 AND id = $2`, tenantID, userID).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.hydrateStaff(ctx, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) CreateUser(ctx context.Context, tenantID, actorID uuid.UUID, in CreateUserInput) (*StaffUser, error) {
	if in.Email == "" || in.DisplayName == "" || in.Password == "" {
		return nil, fmt.Errorf("%w: email, display_name, password required", ErrInvalidInput)
	}
	if len(in.Roles) == 0 {
		return nil, fmt.Errorf("%w: at least one role required", ErrInvalidInput)
	}
	if err := s.validateRoleKeys(ctx, tenantID, in.Roles); err != nil {
		return nil, err
	}
	isTech := in.IsTechnician
	if hasRole(in.Roles, "technician") {
		isTech = true
	}
	if err := validateTechnicianBranchAccess(isTech, in.BranchIDs); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	userID := uuid.New()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO identity.users (id, tenant_id, email, password_hash, display_name)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, tenantID, strings.ToLower(strings.TrimSpace(in.Email)), string(hash), in.DisplayName)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateUser
		}
		return nil, err
	}
	for _, role := range in.Roles {
		if _, err := tx.Exec(ctx, `INSERT INTO identity.user_roles (user_id, role) VALUES ($1, $2)`, userID, role); err != nil {
			return nil, err
		}
	}
	for _, bid := range in.BranchIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO identity.branch_memberships (user_id, branch_id) VALUES ($1, $2)`, userID, bid); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO identity.employee_profiles (
			user_id, tenant_id, employee_code, phone, is_technician, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, tenantID, in.EmployeeCode, in.Phone, isTech, actorID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetUser(ctx, tenantID, userID)
}

func (s *Service) UpdateUser(ctx context.Context, tenantID, userID, actorID uuid.UUID, in UpdateUserInput) (*StaffUser, error) {
	current, err := s.GetUser(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	resultRoles := current.Roles
	if in.Roles != nil {
		resultRoles = in.Roles
	}
	resultBranchIDs := make([]uuid.UUID, 0, len(current.BranchIDs))
	if in.BranchIDs != nil {
		resultBranchIDs = in.BranchIDs
	} else {
		for _, raw := range current.BranchIDs {
			branchID, parseErr := uuid.Parse(raw)
			if parseErr != nil {
				return nil, parseErr
			}
			resultBranchIDs = append(resultBranchIDs, branchID)
		}
	}
	profileTechnician := current.Profile != nil && current.Profile.IsTechnician
	if in.Roles != nil && in.IsTechnician == nil {
		profileTechnician = hasRole(resultRoles, "technician")
	}
	if in.IsTechnician != nil {
		profileTechnician = *in.IsTechnician
	}
	if err := validateTechnicianBranchAccess(
		profileTechnician || hasRole(resultRoles, "technician"),
		resultBranchIDs,
	); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exists uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM identity.users WHERE tenant_id = $1 AND id = $2`, tenantID, userID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if in.DisplayName != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.users SET display_name = $1, updated_at = now() WHERE id = $2`, *in.DisplayName, userID); err != nil {
			return nil, err
		}
	}
	if in.Status != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.users SET status = $1, updated_at = now() WHERE id = $2`, *in.Status, userID); err != nil {
			return nil, err
		}
	}
	if in.Password != nil && *in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE identity.users SET password_hash = $1, updated_at = now() WHERE id = $2`, string(hash), userID); err != nil {
			return nil, err
		}
	}
	if in.BranchIDs != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM identity.branch_memberships WHERE user_id = $1`, userID); err != nil {
			return nil, err
		}
		for _, bid := range in.BranchIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO identity.branch_memberships (user_id, branch_id) VALUES ($1, $2)`, userID, bid); err != nil {
				return nil, err
			}
		}
	}
	if in.Roles != nil {
		if err := s.validateRoleKeys(ctx, tenantID, in.Roles); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM identity.user_roles WHERE user_id = $1`, userID); err != nil {
			return nil, err
		}
		for _, role := range in.Roles {
			if _, err := tx.Exec(ctx, `INSERT INTO identity.user_roles (user_id, role) VALUES ($1, $2)`, userID, role); err != nil {
				return nil, err
			}
		}
	}

	_, _ = tx.Exec(ctx, `
		INSERT INTO identity.employee_profiles (user_id, tenant_id, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO NOTHING`, userID, tenantID, actorID)

	if in.Phone != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.employee_profiles SET phone = $1, updated_at = now(), updated_by = $2 WHERE user_id = $3`, *in.Phone, actorID, userID); err != nil {
			return nil, err
		}
	}
	if in.EmployeeCode != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.employee_profiles SET employee_code = $1, updated_at = now(), updated_by = $2 WHERE user_id = $3`, *in.EmployeeCode, actorID, userID); err != nil {
			return nil, err
		}
	}
	if in.IsTechnician != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity.employee_profiles SET is_technician = $1, updated_at = now(), updated_by = $2 WHERE user_id = $3`, *in.IsTechnician, actorID, userID); err != nil {
			return nil, err
		}
	} else if in.Roles != nil {
		isTech := false
		for _, role := range in.Roles {
			if role == "technician" {
				isTech = true
				break
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE identity.employee_profiles SET is_technician = $1, updated_at = now(), updated_by = $2 WHERE user_id = $3`, isTech, actorID, userID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetUser(ctx, tenantID, userID)
}

func (s *Service) hydrateStaff(ctx context.Context, u *StaffUser) error {
	roles, _, branches, err := s.loadAuthz(ctx, u.ID)
	if err != nil {
		return err
	}
	u.Roles = roles
	u.BranchIDs = branches

	var p EmployeeProfile
	p.UserID = u.ID
	err = s.pool.QueryRow(ctx, `
		SELECT employee_code, phone, is_technician, commission_enabled, commission_type, percent_bps, fixed_amount
		FROM identity.employee_profiles WHERE user_id = $1`, u.ID).
		Scan(&p.EmployeeCode, &p.Phone, &p.IsTechnician, &p.CommissionEnabled, &p.CommissionType, &p.PercentBPS, &p.FixedAmount)
	if errors.Is(err, pgx.ErrNoRows) {
		u.Profile = &EmployeeProfile{UserID: u.ID, CommissionType: "none"}
		return nil
	}
	if err != nil {
		return err
	}
	u.Profile = &p
	return nil
}
