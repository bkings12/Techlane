package repair

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/techlane/techlane/packages/pkg/objectstore"
)

const (
	MaxAttachmentBytes = 5 << 20
	presignUploadTTL   = 15 * time.Minute

	AttachmentUploadPending   = "pending"
	AttachmentUploadCompleted = "completed"
)

type PresignedAttachmentInit struct {
	AttachmentID    uuid.UUID         `json:"attachment_id"`
	StorageKey      string            `json:"storage_key"`
	UploadURL       string            `json:"upload_url"`
	ExpiresInSecond int               `json:"expires_in_seconds"`
	Headers         map[string]string `json:"headers"`
}

// ValidateAttachmentMeta checks file metadata before presign or direct upload.
func ValidateAttachmentMeta(fileName, contentType string, sizeBytes int) error {
	fileName = strings.TrimSpace(fileName)
	contentType = strings.TrimSpace(contentType)
	if fileName == "" {
		return fmt.Errorf("file_name required")
	}
	if len(fileName) > 255 {
		return fmt.Errorf("file_name too long")
	}
	if contentType == "" {
		return fmt.Errorf("content_type required")
	}
	if !allowedAttachmentContentType(contentType) {
		return fmt.Errorf("content_type not allowed")
	}
	if sizeBytes <= 0 {
		return fmt.Errorf("size_bytes must be positive")
	}
	if sizeBytes > MaxAttachmentBytes {
		return fmt.Errorf("attachment must be no larger than 5 MB")
	}
	return nil
}

func allowedAttachmentContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "application/pdf" {
		return true
	}
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	return false
}

func normalizeSHA256Hex(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	if len(raw) != 64 {
		return "", fmt.Errorf("sha256_hex must be 64 hex characters")
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return "", fmt.Errorf("sha256_hex must be valid hex")
	}
	return raw, nil
}

func attachmentStorageKey(tenantID, repairID, attachmentID uuid.UUID, fileName string) string {
	return objectstore.AttachmentKey(
		tenantID.String(), repairID.String(), attachmentID.String(), fileName,
	)
}

func storageKeyOwnedBy(storageKey string, tenantID, repairID, attachmentID uuid.UUID, fileName string) bool {
	return storageKey == attachmentStorageKey(tenantID, repairID, attachmentID, fileName)
}

func (s *Service) InitiatePresignedAttachment(
	ctx context.Context,
	tenantID, repairID uuid.UUID,
	fileName, contentType string,
	sizeBytes int,
	sha256Hex string,
	actorID, corrID uuid.UUID,
) (*PresignedAttachmentInit, error) {
	if s.store == nil {
		return nil, fmt.Errorf("object storage is not configured")
	}
	if err := ValidateAttachmentMeta(fileName, contentType, sizeBytes); err != nil {
		return nil, err
	}
	sha256Hex, err := normalizeSHA256Hex(sha256Hex)
	if err != nil {
		return nil, err
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2)`,
		tenantID, repairID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("repair not found")
	}

	id := uuid.New()
	now := time.Now().UTC()
	key := attachmentStorageKey(tenantID, repairID, id, fileName)

	uploadURL, err := s.store.PresignPut(ctx, key, contentType, int64(sizeBytes), presignUploadTTL)
	if err != nil {
		return nil, fmt.Errorf("presign upload: %w", err)
	}

	var sha256Ptr *string
	if sha256Hex != "" {
		sha256Ptr = &sha256Hex
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_attachments
			(id, tenant_id, repair_job_id, file_name, content_type, content, size_bytes,
			 storage_key, upload_status, sha256_hex, created_at, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, NULL, $6, $7, $8, $9, $10, $11, $12)`,
		id, tenantID, repairID, fileName, contentType, sizeBytes,
		key, AttachmentUploadPending, sha256Ptr, now, actorID, corrID)
	if err != nil {
		return nil, err
	}

	return &PresignedAttachmentInit{
		AttachmentID:    id,
		StorageKey:      key,
		UploadURL:       uploadURL,
		ExpiresInSecond: int(presignUploadTTL.Seconds()),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	}, nil
}

func (s *Service) CompletePresignedAttachment(
	ctx context.Context,
	tenantID, repairID, attachmentID uuid.UUID,
	storageKey, sha256Hex string,
) (*RepairAttachment, error) {
	if s.store == nil {
		return nil, fmt.Errorf("object storage is not configured")
	}
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil, fmt.Errorf("storage_key required")
	}
	sha256Hex, err := normalizeSHA256Hex(sha256Hex)
	if err != nil {
		return nil, err
	}

	var fileName, contentType, uploadStatus string
	var expectedSize int
	var dbStorageKey *string
	var expectedSHA *string
	var createdAt time.Time
	var createdBy *uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT file_name, content_type, size_bytes, storage_key, upload_status, sha256_hex, created_at, created_by
		FROM repair.repair_attachments
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, attachmentID).Scan(
		&fileName, &contentType, &expectedSize, &dbStorageKey, &uploadStatus,
		&expectedSHA, &createdAt, &createdBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("attachment not found")
	}
	if err != nil {
		return nil, err
	}
	if dbStorageKey == nil || *dbStorageKey == "" {
		return nil, fmt.Errorf("attachment is not a presigned upload")
	}
	if storageKey != *dbStorageKey {
		return nil, fmt.Errorf("storage_key mismatch")
	}
	if !storageKeyOwnedBy(storageKey, tenantID, repairID, attachmentID, fileName) {
		return nil, fmt.Errorf("storage_key ownership invalid")
	}
	if uploadStatus == AttachmentUploadCompleted {
		return &RepairAttachment{
			ID: attachmentID, RepairJobID: repairID, FileName: fileName,
			ContentType: contentType, SizeBytes: expectedSize,
			CreatedAt: createdAt, CreatedBy: createdBy,
		}, nil
	}
	if uploadStatus != AttachmentUploadPending {
		return nil, fmt.Errorf("attachment upload is not pending")
	}

	meta, err := s.store.Head(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("object storage head: %w", err)
	}
	if meta.Size != int64(expectedSize) {
		return nil, fmt.Errorf("uploaded size does not match expected size")
	}
	if ct := strings.TrimSpace(meta.ContentType); ct != "" && !strings.EqualFold(ct, contentType) {
		return nil, fmt.Errorf("uploaded content_type does not match")
	}

	verifiedSHA := sha256Hex
	if expectedSHA != nil && *expectedSHA != "" {
		if sha256Hex != "" && sha256Hex != *expectedSHA {
			return nil, fmt.Errorf("sha256_hex does not match initiate value")
		}
		verifiedSHA = *expectedSHA
	}
	if verifiedSHA != "" {
		body, gerr := s.store.Get(ctx, storageKey)
		if gerr != nil {
			return nil, fmt.Errorf("object storage download for checksum: %w", gerr)
		}
		sum := sha256.Sum256(body)
		actual := hex.EncodeToString(sum[:])
		if actual != verifiedSHA {
			return nil, fmt.Errorf("sha256 checksum mismatch")
		}
	}

	var sha256Ptr *string
	if verifiedSHA != "" {
		sha256Ptr = &verifiedSHA
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE repair.repair_attachments
		SET upload_status = $1, sha256_hex = COALESCE($2, sha256_hex), content = NULL
		WHERE tenant_id = $3 AND repair_job_id = $4 AND id = $5 AND upload_status = $6`,
		AttachmentUploadCompleted, sha256Ptr,
		tenantID, repairID, attachmentID, AttachmentUploadPending)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("attachment upload is not pending")
	}

	return &RepairAttachment{
		ID: attachmentID, RepairJobID: repairID, FileName: fileName,
		ContentType: contentType, SizeBytes: expectedSize,
		CreatedAt: createdAt, CreatedBy: createdBy,
	}, nil
}
