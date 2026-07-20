package objectstore

import (
	"strings"
	"testing"
)

func TestAttachmentKeySanitizesFileName(t *testing.T) {
	key := AttachmentKey("t1", "r1", "a1", "photos/front.jpg")
	want := "tenants/t1/repairs/r1/a1/photos_front.jpg"
	if key != want {
		t.Fatalf("got %q want %q", key, want)
	}
}

func TestAttachmentKeyEmptyFileName(t *testing.T) {
	key := AttachmentKey("t1", "r1", "a1", "")
	if !strings.HasSuffix(key, "file.bin") {
		t.Fatalf("expected file.bin suffix, got %q", key)
	}
}
