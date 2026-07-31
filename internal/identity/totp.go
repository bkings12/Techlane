package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Minimal RFC 4226 (HOTP) + RFC 6238 (TOTP) implementation. No external deps
// so we don't depend on network access for `go get` in restricted environments.

const (
	totpDigits    = 6
	totpStepSecs  = 30
	totpSkewSteps = 1 // accept one step before/after to tolerate clock drift
)

func generateTOTPSecret() (string, error) {
	raw := make([]byte, 20) // 160-bit secret, standard for TOTP
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func totpAuthURL(issuer, accountName, secretBase32 string) string {
	v := url.Values{}
	v.Set("secret", secretBase32)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", strconv.Itoa(totpDigits))
	v.Set("period", strconv.Itoa(totpStepSecs))
	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, accountName))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, v.Encode())
}

func hotp(secretBase32 string, counter uint64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secretBase32))
	if err != nil {
		return "", fmt.Errorf("invalid secret: %w", err)
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1])&0xff)<<16 |
		(uint32(sum[offset+2])&0xff)<<8 |
		(uint32(sum[offset+3]) & 0xff)
	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, code%mod), nil
}

// verifyTOTP checks a 6-digit code against the secret, allowing +/-1 time step of drift.
func verifyTOTP(secretBase32, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	counter := uint64(now.Unix() / totpStepSecs)
	for skew := -totpSkewSteps; skew <= totpSkewSteps; skew++ {
		c := counter + uint64(skew)
		expected, err := hotp(secretBase32, c)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func generateBackupCodes(n int) ([]string, error) {
	codes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		codes = append(codes, strings.ToUpper(hex.EncodeToString(b)))
	}
	return codes, nil
}

func hashBackupCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}
