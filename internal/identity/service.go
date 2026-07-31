package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/packages/pkg/authz"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultTenantName = "TechLane"
	refreshTokenTTL   = 7 * 24 * time.Hour
	accessTokenTTL    = 15 * time.Minute

	maxFailedLoginAttempts = 5
	baseLockoutDuration    = 5 * time.Minute
	maxLockoutDuration     = 60 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
)

// ErrAccountLocked is returned when a user has exceeded the failed-login
// threshold. Callers should surface Until so the client can show a retry time.
type ErrAccountLocked struct {
	Until time.Time
}

func (e *ErrAccountLocked) Error() string {
	return fmt.Sprintf("account locked until %s", e.Until.Format(time.RFC3339))
}

// dummyPasswordHash is compared against on unknown-email login attempts so the
// response time doesn't leak whether an account exists (timing side channel).
var dummyPasswordHash = mustBcryptHash("dummy-password-for-timing-normalization")

func mustBcryptHash(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}

// lockoutDuration returns how long to lock the account given the number of
// consecutive failures, escalating (5m, 10m, 20m, ... capped at 60m).
func lockoutDuration(failedCount int) time.Duration {
	if failedCount < maxFailedLoginAttempts {
		return 0
	}
	over := failedCount - maxFailedLoginAttempts
	if over > 4 {
		over = 4
	}
	d := baseLockoutDuration * time.Duration(1<<uint(over))
	if d > maxLockoutDuration {
		d = maxLockoutDuration
	}
	return d
}

// AuditSink records security-relevant events. Implemented by audit.Service.
type AuditSink interface {
	Record(ctx context.Context, tenantID uuid.UUID, actorID *uuid.UUID, action, entityType string, entityID *uuid.UUID, payload map[string]any, corrID uuid.UUID) error
}

type Service struct {
	pool      *pgxpool.Pool
	jwtSecret string
	auditSink AuditSink
}

func NewService(pool *pgxpool.Pool, jwtSecret string) *Service {
	return &Service{pool: pool, jwtSecret: jwtSecret}
}

// SetAuditSink wires an audit trail for login/MFA/password-reset events.
func (s *Service) SetAuditSink(sink AuditSink) {
	s.auditSink = sink
}

func (s *Service) audit(ctx context.Context, tenantID uuid.UUID, actorID *uuid.UUID, action string, payload map[string]any) {
	if s.auditSink == nil {
		return
	}
	_ = s.auditSink.Record(ctx, tenantID, actorID, action, "auth", nil, payload, uuid.New())
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	BranchIDs   []string  `json:"branch_ids"`
}

type seedUser struct {
	email       string
	password    string
	displayName string
	role        string
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.SeedPermissionCatalog(ctx); err != nil {
		return fmt.Errorf("permission catalog: %w", err)
	}
	if err := s.seed(ctx); err != nil {
		return err
	}
	if err := s.SeedSystemRoles(ctx); err != nil {
		return fmt.Errorf("system roles: %w", err)
	}
	return s.ensureEmployeeProfiles(ctx)
}

func (s *Service) ensureEmployeeProfiles(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity.employee_profiles (user_id, tenant_id, is_technician, commission_enabled, commission_type)
		SELECT u.id, u.tenant_id,
		       EXISTS (SELECT 1 FROM identity.user_roles ur WHERE ur.user_id = u.id AND ur.role = 'technician'),
		       false, 'none'
		FROM identity.users u
		WHERE NOT EXISTS (SELECT 1 FROM identity.employee_profiles p WHERE p.user_id = u.id)`)
	return err
}

func (s *Service) seed(ctx context.Context) error {
	var tenantCount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity.tenants`).Scan(&tenantCount); err != nil {
		return err
	}
	if tenantCount > 0 {
		return nil
	}

	tenantID := uuid.New()
	mainBranchID := uuid.New()
	branch2ID := uuid.New()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO identity.tenants (id, name) VALUES ($1, $2)`, tenantID, defaultTenantName); err != nil {
		return err
	}
	for _, b := range []struct {
		id   uuid.UUID
		name string
		code string
	}{
		{mainBranchID, "Main", "main"},
		{branch2ID, "Branch 2", "branch-2"},
	} {
		if _, err := tx.Exec(ctx, `INSERT INTO identity.branches (id, tenant_id, name, code) VALUES ($1, $2, $3, $4)`,
			b.id, tenantID, b.name, b.code); err != nil {
			return err
		}
	}

	users := []seedUser{
		{"owner@techlane.local", "password", "Owner", "owner"},
		{"tech@techlane.local", "password", "Technician", "technician"},
		{"cashier@techlane.local", "password", "Cashier", "cashier"},
	}
	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		userID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity.users (id, tenant_id, email, password_hash, display_name)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, tenantID, u.email, string(hash), u.displayName); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO identity.user_roles (user_id, role) VALUES ($1, $2)`, userID, u.role); err != nil {
			return err
		}
		branchIDs := []uuid.UUID{mainBranchID, branch2ID}
		if u.role == "technician" {
			branchIDs = []uuid.UUID{mainBranchID}
		}
		for _, branchID := range branchIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO identity.branch_memberships (user_id, branch_id) VALUES ($1, $2)`, userID, branchID); err != nil {
				return err
			}
		}
		isTech := u.role == "technician"
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity.employee_profiles (user_id, tenant_id, is_technician, commission_enabled, commission_type)
			VALUES ($1, $2, $3, false, 'none')`, userID, tenantID, isTech); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// LoginOutcome is either a fully issued token pair, or a signal that the
// caller must complete an MFA challenge before tokens are issued.
type LoginOutcome struct {
	Tokens         *TokenPair `json:"tokens,omitempty"`
	MFARequired    bool       `json:"mfa_required,omitempty"`
	ChallengeToken string     `json:"mfa_challenge,omitempty"`
}

func (s *Service) Login(ctx context.Context, email, password, ip string) (*LoginOutcome, error) {
	var userID, tenantID uuid.UUID
	var hash, displayName string
	var failedCount int
	var lockedUntil *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, password_hash, display_name, failed_login_count, locked_until
		FROM identity.users WHERE email = $1 AND status = 'active'`, email).
		Scan(&userID, &tenantID, &hash, &displayName, &failedCount, &lockedUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Burn roughly the same CPU time as a real bcrypt compare so
			// unknown-email vs wrong-password responses aren't distinguishable by timing.
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	now := time.Now().UTC()
	if lockedUntil != nil && lockedUntil.After(now) {
		s.audit(ctx, tenantID, &userID, "auth.login.blocked", map[string]any{"ip": ip})
		return nil, &ErrAccountLocked{Until: *lockedUntil}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		failedCount++
		var newLock *time.Time
		if d := lockoutDuration(failedCount); d > 0 {
			t := now.Add(d)
			newLock = &t
		}
		if _, execErr := s.pool.Exec(ctx, `
			UPDATE identity.users SET failed_login_count = $1, locked_until = $2 WHERE id = $3`,
			failedCount, newLock, userID); execErr != nil {
			return nil, execErr
		}
		action := "auth.login.failed"
		if newLock != nil {
			action = "auth.login.locked"
		}
		s.audit(ctx, tenantID, &userID, action, map[string]any{"ip": ip, "failed_count": failedCount})
		if newLock != nil {
			return nil, &ErrAccountLocked{Until: *newLock}
		}
		return nil, ErrInvalidCredentials
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE identity.users SET failed_login_count = 0, locked_until = NULL, last_login_at = now() WHERE id = $1`, userID); err != nil {
		return nil, err
	}
	s.audit(ctx, tenantID, &userID, "auth.login.success", map[string]any{"ip": ip})

	mfaEnabled, err := s.isMFAEnabled(ctx, userID)
	if err != nil {
		return nil, err
	}
	if mfaEnabled {
		challengeID := uuid.New()
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO identity.mfa_challenges (id, user_id, expires_at) VALUES ($1, $2, $3)`,
			challengeID, userID, now.Add(5*time.Minute)); err != nil {
			return nil, err
		}
		return &LoginOutcome{MFARequired: true, ChallengeToken: challengeID.String()}, nil
	}

	pair, err := s.issueTokens(ctx, userID, tenantID, email, displayName)
	if err != nil {
		return nil, err
	}
	return &LoginOutcome{Tokens: pair}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	hash := hashToken(refreshToken)
	var userID uuid.UUID
	var tokenID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT rt.id, rt.user_id FROM identity.refresh_tokens rt
		WHERE rt.token_hash = $1 AND rt.revoked_at IS NULL AND rt.expires_at > now()`, hash).
		Scan(&tokenID, &userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}

	_, _ = s.pool.Exec(ctx, `UPDATE identity.refresh_tokens SET revoked_at = now() WHERE id = $1`, tokenID)

	var tenantID uuid.UUID
	var email, displayName string
	err = s.pool.QueryRow(ctx, `SELECT tenant_id, email, display_name FROM identity.users WHERE id = $1`, userID).
		Scan(&tenantID, &email, &displayName)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, userID, tenantID, email, displayName)
}

func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	var profile UserProfile
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name, tenant_id FROM identity.users WHERE id = $1`, userID).
		Scan(&profile.ID, &profile.Email, &profile.DisplayName, &profile.TenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT role FROM identity.user_roles WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		profile.Roles = append(profile.Roles, role)
	}
	perms := map[string]struct{}{}
	dbPerms, err := s.PermissionsForRoleKeys(ctx, profile.TenantID, profile.Roles)
	if err != nil {
		return nil, err
	}
	for _, p := range dbPerms {
		perms[p] = struct{}{}
	}
	for p := range perms {
		profile.Permissions = append(profile.Permissions, p)
	}
	brows, err := s.pool.Query(ctx, `SELECT branch_id FROM identity.branch_memberships WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer brows.Close()
	for brows.Next() {
		var bid uuid.UUID
		if err := brows.Scan(&bid); err != nil {
			return nil, err
		}
		profile.BranchIDs = append(profile.BranchIDs, bid.String())
	}
	return &profile, nil
}

func (s *Service) issueTokens(ctx context.Context, userID, tenantID uuid.UUID, email, displayName string) (*TokenPair, error) {
	roles, perms, branches, err := s.loadAuthz(ctx, userID)
	if err != nil {
		return nil, err
	}
	access, err := authz.IssueAccessToken(s.jwtSecret, authz.Claims{
		UserID:      userID,
		TenantID:    tenantID,
		Email:       email,
		Roles:       roles,
		Permissions: perms,
		BranchIDs:   branches,
	}, accessTokenTTL)
	if err != nil {
		return nil, err
	}
	refreshRaw, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshID := uuid.New()
	expires := time.Now().UTC().Add(refreshTokenTTL)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO identity.refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, refreshID, userID, hashToken(refreshRaw), expires); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refreshRaw,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
	}, nil
}

func (s *Service) loadAuthz(ctx context.Context, userID uuid.UUID) (roles, perms, branches []string, err error) {
	var tenantID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT tenant_id FROM identity.users WHERE id = $1`, userID).Scan(&tenantID); err != nil {
		return nil, nil, nil, err
	}
	rrows, err := s.pool.Query(ctx, `SELECT role FROM identity.user_roles WHERE user_id = $1`, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var role string
		if err := rrows.Scan(&role); err != nil {
			return nil, nil, nil, err
		}
		roles = append(roles, role)
	}
	perms, err = s.PermissionsForRoleKeys(ctx, tenantID, roles)
	if err != nil {
		return nil, nil, nil, err
	}
	brows, err := s.pool.Query(ctx, `SELECT branch_id FROM identity.branch_memberships WHERE user_id = $1`, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer brows.Close()
	for brows.Next() {
		var bid uuid.UUID
		if err := brows.Scan(&bid); err != nil {
			return nil, nil, nil, err
		}
		branches = append(branches, bid.String())
	}
	return roles, perms, branches, nil
}

func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ParseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid: %w", err)
	}
	return id, nil
}
