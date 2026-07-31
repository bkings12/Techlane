package identity

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SignupInput captures a self-serve tenant onboarding request: a brand new
// shop owner registering their business for the first time (no invite, no
// existing tenant).
type SignupInput struct {
	CompanyName string
	OwnerName   string
	Email       string
	Password    string
}

var ErrEmailTaken = fmt.Errorf("%w: email already in use", ErrDuplicateUser)

// validateSignupInput normalizes and checks a signup request, independent of
// any database access, so it can be unit tested without a live Postgres pool.
func validateSignupInput(in SignupInput) (companyName, ownerName, email string, err error) {
	companyName = strings.TrimSpace(in.CompanyName)
	ownerName = strings.TrimSpace(in.OwnerName)
	email = strings.ToLower(strings.TrimSpace(in.Email))

	if companyName == "" || ownerName == "" {
		return "", "", "", fmt.Errorf("%w: company name and owner name are required", ErrInvalidInput)
	}
	if _, addrErr := mail.ParseAddress(email); addrErr != nil {
		return "", "", "", fmt.Errorf("%w: a valid email is required", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return "", "", "", fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	return companyName, ownerName, email, nil
}

// Signup provisions a brand new tenant (shop), a "Main" branch, and an owner
// user in one transaction, then logs the owner straight in. Email uniqueness
// is enforced globally (not just per-tenant) even though the schema only
// constrains (tenant_id, email) — this keeps unscoped-by-tenant login lookups
// unambiguous across the whole platform.
func (s *Service) Signup(ctx context.Context, in SignupInput) (*LoginOutcome, error) {
	companyName, ownerName, email, err := validateSignupInput(in)
	if err != nil {
		return nil, err
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity.users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	tenantID := uuid.New()
	branchID := uuid.New()
	userID := uuid.New()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO identity.tenants (id, name) VALUES ($1, $2)`, tenantID, companyName); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity.branches (id, tenant_id, name, code) VALUES ($1, $2, 'Main', 'main')`,
		branchID, tenantID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO identity.users (id, tenant_id, email, password_hash, display_name)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, tenantID, email, string(hash), ownerName); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity.user_roles (user_id, role) VALUES ($1, 'owner')`, userID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity.branch_memberships (user_id, branch_id) VALUES ($1, $2)`, userID, branchID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO identity.employee_profiles (user_id, tenant_id, is_technician, commission_enabled, commission_type)
		VALUES ($1, $2, false, false, 'none')`, userID, tenantID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Roles/permissions are seeded per-tenant on demand — the permission
	// catalog itself is global and already populated at process start.
	if err := s.ensureSystemRolesForTenant(ctx, tenantID); err != nil {
		return nil, err
	}

	s.audit(ctx, tenantID, &userID, "tenant.signup", map[string]any{"company": companyName, "email": email})

	pair, err := s.issueTokens(ctx, userID, tenantID, email, ownerName)
	if err != nil {
		return nil, err
	}
	return &LoginOutcome{Tokens: pair}, nil
}
