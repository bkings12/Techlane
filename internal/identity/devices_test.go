package identity_test

import (
	"errors"
	"testing"

	"github.com/techlane/techlane/internal/identity"
)

func TestErrDeviceRevoked(t *testing.T) {
	if !errors.Is(identity.ErrDeviceRevoked, identity.ErrDeviceRevoked) {
		t.Fatal("expected ErrDeviceRevoked sentinel")
	}
}
