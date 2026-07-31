package objectstore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	encStore, err := newEncryptedTestStore("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("repair photo bytes")
	sealed := encStore.seal(plain)
	if bytes.Equal(sealed, plain) {
		t.Fatal("expected ciphertext")
	}
	if !bytes.HasPrefix(sealed, []byte(encMagic)) {
		t.Fatal("missing magic")
	}
	got, err := encStore.open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q", got)
	}
	legacy, err := encStore.open(plain)
	if err != nil || !bytes.Equal(legacy, plain) {
		t.Fatalf("legacy: %v %q", err, legacy)
	}
}

func TestFullKeyPrefix(t *testing.T) {
	s := &Store{keyPrefix: "attachments"}
	if got := s.fullKey("tenants/a/b"); got != "attachments/tenants/a/b" {
		t.Fatalf("got %q", got)
	}
}

func newEncryptedTestStore(hexKey string) (*Store, error) {
	key, err := parseEncryptionKey(hexKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{gcm: gcm}, nil
}
