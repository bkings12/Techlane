package repair

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateAttachmentMeta(t *testing.T) {
	if err := ValidateAttachmentMeta("photo.jpg", "image/jpeg", 1024); err != nil {
		t.Fatalf("valid meta: %v", err)
	}
	if err := ValidateAttachmentMeta("", "image/jpeg", 1024); err == nil {
		t.Fatal("expected file_name error")
	}
	if err := ValidateAttachmentMeta("photo.jpg", "text/plain", 1024); err == nil {
		t.Fatal("expected content_type error")
	}
	if err := ValidateAttachmentMeta("photo.jpg", "image/jpeg", 0); err == nil {
		t.Fatal("expected size error")
	}
	if err := ValidateAttachmentMeta("photo.jpg", "image/jpeg", MaxAttachmentBytes+1); err == nil {
		t.Fatal("expected max size error")
	}
}

func TestNormalizeSHA256Hex(t *testing.T) {
	valid := "a" + strings.Repeat("b", 63)
	got, err := normalizeSHA256Hex(valid)
	if err != nil || got != valid {
		t.Fatalf("unexpected: got=%q err=%v", got, err)
	}
	if _, err := normalizeSHA256Hex("not-hex"); err == nil {
		t.Fatal("expected invalid hex error")
	}
	if _, err := normalizeSHA256Hex(strings.Repeat("a", 63)); err == nil {
		t.Fatal("expected length error")
	}
	if got, err := normalizeSHA256Hex(""); err != nil || got != "" {
		t.Fatalf("empty should be ok: got=%q err=%v", got, err)
	}
}

func TestStorageKeyOwnedBy(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repairID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	attachmentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	fileName := "photo.jpg"
	key := attachmentStorageKey(tenantID, repairID, attachmentID, fileName)
	if !storageKeyOwnedBy(key, tenantID, repairID, attachmentID, fileName) {
		t.Fatal("expected owned key")
	}
	if storageKeyOwnedBy(key, tenantID, repairID, uuid.New(), fileName) {
		t.Fatal("wrong attachment id must not own key")
	}
	if storageKeyOwnedBy("tenants/other/repairs/x/y/z", tenantID, repairID, attachmentID, fileName) {
		t.Fatal("foreign key must not match")
	}
}
