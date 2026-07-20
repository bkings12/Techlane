package platform

import (
	"fmt"
	"os"
	"strings"
)

const DevJWTSecret = "dev-change-me-in-production-32chars"

// ValidateProductionSecrets refuses insecure defaults when APP_ENV=production.
func ValidateProductionSecrets(jwtSecret string) error {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env != "production" {
		return nil
	}
	secret := strings.TrimSpace(jwtSecret)
	if secret == "" || secret == DevJWTSecret {
		return fmt.Errorf("JWT_SECRET must be set to a non-default value when APP_ENV=production")
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters when APP_ENV=production")
	}
	origins := strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	if origins == "" || origins == "*" {
		return fmt.Errorf("CORS_ORIGINS must be an explicit allowlist when APP_ENV=production")
	}
	return nil
}
