package objectstore

import (
	"bytes"
	"context"
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
	client *s3.Client
	bucket string
}

// Config from environment. When Endpoint/Bucket/AccessKey/SecretKey are set, storage is enabled.
type Config struct {
	Endpoint  string // e.g. https://<accountid>.r2.cloudflarestorage.com or http://localhost:9000
	Region    string // R2 often uses "auto"
	Bucket    string
	AccessKey string
	SecretKey string
	// PublicBase is optional CDN/public URL prefix for future use.
	PublicBase string
}

func ConfigFromEnv() Config {
	return Config{
		Endpoint:   strings.TrimSpace(os.Getenv("OBJECT_STORAGE_ENDPOINT")),
		Region:     envOr("OBJECT_STORAGE_REGION", "auto"),
		Bucket:     strings.TrimSpace(os.Getenv("OBJECT_STORAGE_BUCKET")),
		AccessKey:  strings.TrimSpace(os.Getenv("OBJECT_STORAGE_ACCESS_KEY")),
		SecretKey:  strings.TrimSpace(os.Getenv("OBJECT_STORAGE_SECRET_KEY")),
		PublicBase: strings.TrimSpace(os.Getenv("OBJECT_STORAGE_PUBLIC_BASE")),
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
	s := &Store{client: client, bucket: cfg.Bucket}
	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
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

func (s *Store) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
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
		Key:    aws.String(key),
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
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	presigner := s3.NewPresignClient(s.client)
	out, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
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
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	presigner := s3.NewPresignClient(s.client)
	out, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
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
