package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMFAAlreadyEnabled   = errors.New("mfa already enabled")
	ErrMFANotSetup         = errors.New("mfa setup not started")
	ErrMFAInvalidCode      = errors.New("invalid authentication code")
	ErrMFAInvalidChallenge = errors.New("invalid or expired mfa challenge")
)

const mfaIssuer = "TechLane"

type MFASetupResult struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type MFAEnableResult struct {
	BackupCodes []string `json:"backup_codes"`
}

type MFAStatus struct {
	Enabled bool `json:"enabled"`
}

func (s *Service) isMFAEnabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT enabled FROM identity.mfa_totp WHERE user_id = $1`, userID).Scan(&enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return enabled, nil
}

func (s *Service) GetMFAStatus(ctx context.Context, userID uuid.UUID) (*MFAStatus, error) {
	enabled, err := s.isMFAEnabled(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &MFAStatus{Enabled: enabled}, nil
}

// SetupMFA generates (or regenerates, while unconfirmed) a TOTP secret for the
// user and returns enough info to enroll in an authenticator app.
func (s *Service) SetupMFA(ctx context.Context, userID uuid.UUID, email string) (*MFASetupResult, error) {
	enabled, err := s.isMFAEnabled(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enabled {
		return nil, ErrMFAAlreadyEnabled
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO identity.mfa_totp (user_id, secret_base32, enabled, backup_codes)
		VALUES ($1, $2, false, '{}')
		ON CONFLICT (user_id) DO UPDATE SET secret_base32 = EXCLUDED.secret_base32, enabled = false, backup_codes = '{}'`,
		userID, secret); err != nil {
		return nil, err
	}
	return &MFASetupResult{Secret: secret, OTPAuthURL: totpAuthURL(mfaIssuer, email, secret)}, nil
}

// EnableMFA confirms enrollment by validating one live code, then turns MFA on
// and returns one-time backup codes (shown once).
func (s *Service) EnableMFA(ctx context.Context, userID, tenantID uuid.UUID, code string) (*MFAEnableResult, error) {
	var secret string
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT secret_base32, enabled FROM identity.mfa_totp WHERE user_id = $1`, userID).
		Scan(&secret, &enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMFANotSetup
		}
		return nil, err
	}
	if enabled {
		return nil, ErrMFAAlreadyEnabled
	}
	if !verifyTOTP(secret, code, time.Now().UTC()) {
		return nil, ErrMFAInvalidCode
	}
	codes, err := generateBackupCodes(8)
	if err != nil {
		return nil, err
	}
	hashed := make([]string, len(codes))
	for i, c := range codes {
		hashed[i] = hashBackupCode(c)
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE identity.mfa_totp SET enabled = true, enabled_at = now(), backup_codes = $1 WHERE user_id = $2`,
		hashed, userID); err != nil {
		return nil, err
	}
	s.audit(ctx, tenantID, &userID, "auth.mfa.enabled", map[string]any{})
	return &MFAEnableResult{BackupCodes: codes}, nil
}

// DisableMFA requires the current password as a step-up check.
func (s *Service) DisableMFA(ctx context.Context, userID, tenantID uuid.UUID, password string) error {
	var hash string
	if err := s.pool.QueryRow(ctx, `SELECT password_hash FROM identity.users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM identity.mfa_totp WHERE user_id = $1`, userID); err != nil {
		return err
	}
	s.audit(ctx, tenantID, &userID, "auth.mfa.disabled", map[string]any{})
	return nil
}

// VerifyMFAChallenge completes a login started by Login() when MFA is enabled.
func (s *Service) VerifyMFAChallenge(ctx context.Context, challengeToken, code, ip string) (*TokenPair, error) {
	challengeID, err := uuid.Parse(challengeToken)
	if err != nil {
		return nil, ErrMFAInvalidChallenge
	}
	var userID uuid.UUID
	var expiresAt time.Time
	var consumedAt *time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT user_id, expires_at, consumed_at FROM identity.mfa_challenges WHERE id = $1`, challengeID).
		Scan(&userID, &expiresAt, &consumedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMFAInvalidChallenge
		}
		return nil, err
	}
	if consumedAt != nil || time.Now().UTC().After(expiresAt) {
		return nil, ErrMFAInvalidChallenge
	}

	var secret string
	var backupCodes []string
	if err := s.pool.QueryRow(ctx, `
		SELECT secret_base32, backup_codes FROM identity.mfa_totp WHERE user_id = $1 AND enabled = true`, userID).
		Scan(&secret, &backupCodes); err != nil {
		return nil, ErrMFAInvalidChallenge
	}

	valid := verifyTOTP(secret, code, time.Now().UTC())
	usedBackupIdx := -1
	if !valid {
		hashed := hashBackupCode(code)
		for i, bc := range backupCodes {
			if bc == hashed {
				valid = true
				usedBackupIdx = i
				break
			}
		}
	}
	if !valid {
		return nil, ErrMFAInvalidCode
	}

	if _, err := s.pool.Exec(ctx, `UPDATE identity.mfa_challenges SET consumed_at = now() WHERE id = $1`, challengeID); err != nil {
		return nil, err
	}
	if usedBackupIdx >= 0 {
		remaining := make([]string, 0, len(backupCodes)-1)
		remaining = append(remaining, backupCodes[:usedBackupIdx]...)
		remaining = append(remaining, backupCodes[usedBackupIdx+1:]...)
		if _, err := s.pool.Exec(ctx, `UPDATE identity.mfa_totp SET backup_codes = $1 WHERE user_id = $2`, remaining, userID); err != nil {
			return nil, err
		}
	}

	var tenantID uuid.UUID
	var email, displayName string
	if err := s.pool.QueryRow(ctx, `SELECT tenant_id, email, display_name FROM identity.users WHERE id = $1`, userID).
		Scan(&tenantID, &email, &displayName); err != nil {
		return nil, err
	}
	s.audit(ctx, tenantID, &userID, "auth.mfa.verified", map[string]any{"ip": ip, "used_backup_code": usedBackupIdx >= 0})
	return s.issueTokens(ctx, userID, tenantID, email, displayName)
}
