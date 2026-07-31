package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidResetToken = errors.New("invalid or expired reset token")

const passwordResetTTL = 30 * time.Minute

// PasswordResetRequest carries what the caller (handler) needs to send an email.
// Token is only populated when the account exists; callers must still return a
// generic "check your email" response either way to avoid leaking which emails
// are registered.
type PasswordResetRequest struct {
	UserID      uuid.UUID
	Email       string
	DisplayName string
	Token       string
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) (*PasswordResetRequest, error) {
	var userID uuid.UUID
	var displayName string
	err := s.pool.QueryRow(ctx, `
		SELECT id, display_name FROM identity.users WHERE email = $1 AND status = 'active'`, email).
		Scan(&userID, &displayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	raw, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	expires := time.Now().UTC().Add(passwordResetTTL)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO identity.password_reset_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, id, userID, hashToken(raw), expires); err != nil {
		return nil, err
	}
	return &PasswordResetRequest{UserID: userID, Email: email, DisplayName: displayName, Token: raw}, nil
}

func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrInvalidInput
	}
	hash := hashToken(rawToken)
	var tokenID, userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at FROM identity.password_reset_tokens WHERE token_hash = $1`, hash).
		Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidResetToken
		}
		return err
	}
	if usedAt != nil || time.Now().UTC().After(expiresAt) {
		return ErrInvalidResetToken
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE identity.users SET password_hash = $1, failed_login_count = 0, locked_until = NULL WHERE id = $2`,
		string(newHash), userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE identity.password_reset_tokens SET used_at = now() WHERE id = $1`, tokenID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE identity.refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	var tenantID uuid.UUID
	_ = s.pool.QueryRow(ctx, `SELECT tenant_id FROM identity.users WHERE id = $1`, userID).Scan(&tenantID)
	s.audit(ctx, tenantID, &userID, "auth.password.reset", map[string]any{})
	return nil
}
