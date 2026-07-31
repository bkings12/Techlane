package identity

import (
	"errors"
	"testing"
)

func TestValidateSignupInput(t *testing.T) {
	tests := []struct {
		name    string
		in      SignupInput
		wantErr bool
	}{
		{
			name: "valid input",
			in:   SignupInput{CompanyName: " Acme Repairs ", OwnerName: " Jane Doe ", Email: " Jane@Acme.com ", Password: "supersecret"},
		},
		{name: "missing company name", in: SignupInput{OwnerName: "Jane", Email: "jane@acme.com", Password: "supersecret"}, wantErr: true},
		{name: "missing owner name", in: SignupInput{CompanyName: "Acme", Email: "jane@acme.com", Password: "supersecret"}, wantErr: true},
		{name: "invalid email", in: SignupInput{CompanyName: "Acme", OwnerName: "Jane", Email: "not-an-email", Password: "supersecret"}, wantErr: true},
		{name: "short password", in: SignupInput{CompanyName: "Acme", OwnerName: "Jane", Email: "jane@acme.com", Password: "short"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			company, owner, email, err := validateSignupInput(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSignupInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
				return
			}
			if company != "Acme Repairs" {
				t.Errorf("expected trimmed company name, got %q", company)
			}
			if owner != "Jane Doe" {
				t.Errorf("expected trimmed owner name, got %q", owner)
			}
			if email != "jane@acme.com" {
				t.Errorf("expected lowercased/trimmed email, got %q", email)
			}
		})
	}
}

func TestErrEmailTakenWrapsErrDuplicateUser(t *testing.T) {
	if !errors.Is(ErrEmailTaken, ErrDuplicateUser) {
		t.Fatal("ErrEmailTaken should wrap ErrDuplicateUser so writeIdentityErr maps it to 409")
	}
}
