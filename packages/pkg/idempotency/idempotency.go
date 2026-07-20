package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	ErrMismatch = errors.New("idempotency payload mismatch")
	ErrInFlight = errors.New("idempotency key in flight")
)

type Record struct {
	Key        string
	BodyHash   string
	StatusCode int
	Response   []byte
	CreatedAt  time.Time
}

// Store is an in-memory idempotency store suitable for single-process tests and MVP.
// Production should use Redis via RedisStore.
type Store struct {
	mu   sync.Mutex
	data map[string]Record
}

func NewStore() *Store {
	return &Store{data: map[string]Record{}}
}

func HashBody(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Store) Get(ctx context.Context, key string) (Record, bool) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data[key]
	return r, ok
}

func (s *Store) Put(ctx context.Context, rec Record) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data[rec.Key]; ok {
		if existing.BodyHash != rec.BodyHash {
			return ErrMismatch
		}
		return nil
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	s.data[rec.Key] = rec
	return nil
}

// BeginOrReplay returns a cached response if the key exists; otherwise reserves the key.
func (s *Store) BeginOrReplay(ctx context.Context, key, bodyHash string) (replay []byte, status int, began bool, err error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data[key]; ok {
		if existing.BodyHash != bodyHash {
			return nil, 0, false, ErrMismatch
		}
		if len(existing.Response) == 0 {
			return nil, 0, false, ErrInFlight
		}
		return existing.Response, existing.StatusCode, false, nil
	}
	s.data[key] = Record{Key: key, BodyHash: bodyHash, CreatedAt: time.Now().UTC()}
	return nil, 0, true, nil
}

func (s *Store) Complete(ctx context.Context, key string, status int, response []byte) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data[key]
	if !ok {
		return errors.New("unknown idempotency key")
	}
	rec.StatusCode = status
	rec.Response = response
	s.data[key] = rec
	return nil
}
