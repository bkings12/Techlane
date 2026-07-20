package platform

import "testing"

func TestValidateProductionSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ORIGINS", "https://ops.example.com")
	if err := ValidateProductionSecrets(DevJWTSecret); err == nil {
		t.Fatal("expected error for default JWT")
	}
	if err := ValidateProductionSecrets("short"); err == nil {
		t.Fatal("expected error for short JWT")
	}
	if err := ValidateProductionSecrets("this-is-a-sufficiently-long-secret-key"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORS_ORIGINS", "*")
	if err := ValidateProductionSecrets("this-is-a-sufficiently-long-secret-key"); err == nil {
		t.Fatal("expected CORS error")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ORIGINS", "*")
	if err := ValidateProductionSecrets(DevJWTSecret); err != nil {
		t.Fatal(err)
	}
}
