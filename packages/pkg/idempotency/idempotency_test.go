package idempotency_test

import (
	"context"
	"testing"

	"github.com/techlane/techlane/packages/pkg/idempotency"
)

func TestIdempotentReplay(t *testing.T) {
	s := idempotency.NewStore()
	ctx := context.Background()
	hash, err := idempotency.HashBody(map[string]string{"a": "1"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, began, err := s.BeginOrReplay(ctx, "k1", hash)
	if err != nil || !began {
		t.Fatalf("begin: began=%v err=%v", began, err)
	}
	if err := s.Complete(ctx, "k1", 201, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	body, status, began, err := s.BeginOrReplay(ctx, "k1", hash)
	if err != nil || began || status != 201 || string(body) != `{"ok":true}` {
		t.Fatalf("replay: began=%v status=%d body=%s err=%v", began, status, body, err)
	}
}

func TestPayloadMismatch(t *testing.T) {
	s := idempotency.NewStore()
	ctx := context.Background()
	h1, _ := idempotency.HashBody(map[string]string{"a": "1"})
	h2, _ := idempotency.HashBody(map[string]string{"a": "2"})
	_, _, _, err := s.BeginOrReplay(ctx, "k1", h1)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Complete(ctx, "k1", 200, []byte(`{}`))
	_, _, _, err = s.BeginOrReplay(ctx, "k1", h2)
	if err != idempotency.ErrMismatch {
		t.Fatalf("expected mismatch, got %v", err)
	}
}
