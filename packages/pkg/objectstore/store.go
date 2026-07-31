package objectstore

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store is an S3-compatible object store (Cloudflare R2 or MinIO).
type Store struct {
	client    *s3.Client
	bucket    string
	keyPrefix string
	gcm       cipher.AEAD // optional at-rest encryption for shared/public buckets
}

// Config from environment. When Endpoint/Bucket/AccessKey/SecretKey are set, storage is enabled.
type Config struct {
	Endpoint  string // e.g. https://<accountid>.r2.cloudflarestorage.com or http://localhost:9000
	Region    string // R2 often uses "auto"
	Bucket    string
	AccessKey string
	SecretKey string
	// KeyPrefix scopes all keys (e.g. "attachments" when sharing a bucket with public APKs).
	KeyPrefix string
	// PublicBase is optional CDN/public URL prefix for future use.
	PublicBase string
	// EncryptionKey is a 32-byte key (raw or 64-char hex). When set, Put/Get
	// encrypt with AES-GCM so objects remain opaque even if the bucket is public.
	EncryptionKey string
}

func ConfigFromEnv() Config {
	return Config{
		Endpoint:      strings.TrimSpace(os.Getenv("OBJECT_STORAGE_ENDPOINT")),
		Region:        envOr("OBJECT_STORAGE_REGION", "auto"),
		Bucket:        strings.TrimSpace(os.Getenv("OBJECT_STORAGE_BUCKET")),
		AccessKey:     strings.TrimSpace(os.Getenv("OBJECT_STORAGE_ACCESS_KEY")),
		SecretKey:     strings.TrimSpace(os.Getenv("OBJECT_STORAGE_SECRET_KEY")),
		KeyPrefix:     strings.Trim(strings.TrimSpace(os.Getenv("OBJECT_STORAGE_KEY_PREFIX")), "/"),
		PublicBase:    strings.TrimSpace(os.Getenv("OBJECT_STORAGE_PUBLIC_BASE")),
		EncryptionKey: strings.TrimSpace(os.Getenv("OBJECT_STORAGE_ENCRYPTION_KEY")),
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (c Config) Enabled() bool {
	return c.Endpoint != "" && c.Bucket != "" && c.AccessKey != "" && c.SecretKey != ""
}

func parseEncryptionKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(raw) == 64 {
		b, err := hex.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("OBJECT_STORAGE_ENCRYPTION_KEY hex: %w", err)
		}
		return b, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	// Derive a stable 32-byte key from longer passphrases.
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

// New creates an S3 client pointed at R2/MinIO. Returns nil,nil when not configured.
func New(cfg Config) (*Store, error) {
	if !cfg.Enabled() {
		return nil, nil
	}
	resolver := s3.EndpointResolverFromURL(cfg.Endpoint)
	client := s3.New(s3.Options{
		Region:                     cfg.Region,
		Credentials:                credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		EndpointResolver:           resolver,
		UsePathStyle:               true, // required for MinIO; R2 supports path-style too
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	s := &Store{client: client, bucket: cfg.Bucket, keyPrefix: cfg.KeyPrefix}
	if key, err := parseEncryptionKey(cfg.EncryptionKey); err != nil {
		return nil, err
	} else if key != nil {
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		s.gcm = gcm
	}
	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) fullKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	if s.keyPrefix == "" {
		return key
	}
	return s.keyPrefix + "/" + key
}

func (s *Store) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		return fmt.Errorf("create bucket %s: %w", s.bucket, err)
	}
	return nil
}

const encMagic = "TLENC1"

func (s *Store) seal(body []byte) []byte {
	if s.gcm == nil {
		return body
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic("objectstore: nonce: " + err.Error())
	}
	ct := s.gcm.Seal(nil, nonce, body, nil)
	out := make([]byte, 0, len(encMagic)+len(nonce)+len(ct))
	out = append(out, encMagic...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out
}

func (s *Store) open(body []byte) ([]byte, error) {
	if s.gcm == nil {
		return body, nil
	}
	if !bytes.HasPrefix(body, []byte(encMagic)) {
		// Plaintext object written before encryption was enabled.
		return body, nil
	}
	rest := body[len(encMagic):]
	ns := s.gcm.NonceSize()
	if len(rest) < ns {
		return nil, fmt.Errorf("encrypted object truncated")
	}
	nonce, ct := rest[:ns], rest[ns:]
	return s.gcm.Open(nil, nonce, ct, nil)
}

func (s *Store) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	payload := s.seal(body)
	if s.gcm != nil {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.fullKey(key)),
		Body:        bytes.NewReader(payload),
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	raw, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	return s.open(raw)
}

func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	})
	return err
}

// ObjectMeta is returned by Head for upload completion checks.
type ObjectMeta struct {
	Size        int64
	ContentType string
}

func (s *Store) Head(ctx context.Context, key string) (*ObjectMeta, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	})
	if err != nil {
		return nil, err
	}
	meta := &ObjectMeta{}
	if out.ContentLength != nil {
		meta.Size = *out.ContentLength
	}
	if out.ContentType != nil {
		meta.ContentType = *out.ContentType
	}
	return meta, nil
}

func (s *Store) PresignPut(ctx context.Context, key, contentType string, sizeBytes int64, expiry time.Duration) (string, error) {
	if s.gcm != nil {
		return "", fmt.Errorf("direct uploads are disabled when object encryption is enabled — POST the file through the API")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	presigner := s3.NewPresignClient(s.client)
	out, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.fullKey(key)),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(sizeBytes),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (s *Store) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if s.gcm != nil {
		return "", fmt.Errorf("presigned downloads are disabled when object encryption is enabled — use the authenticated content API")
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	presigner := s3.NewPresignClient(s.client)
	out, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// AttachmentKey builds a stable object key for a repair attachment.
func AttachmentKey(tenantID, repairID, attachmentID, fileName string) string {
	safe := strings.ReplaceAll(fileName, "/", "_")
	if safe == "" {
		safe = "file.bin"
	}
	return fmt.Sprintf("tenants/%s/repairs/%s/%s/%s", tenantID, repairID, attachmentID, safe)
}

// Encrypted reports whether at-rest encryption is active.
func (s *Store) Encrypted() bool { return s != nil && s.gcm != nil }
