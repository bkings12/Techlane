package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/config"
	"github.com/techlane/techlane/packages/pkg/db"
	"github.com/techlane/techlane/packages/pkg/objectstore"
)

type attachmentRow struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	RepairJobID uuid.UUID
	FileName    string
	ContentType string
	Content     []byte
	SizeBytes   int
}

func main() {
	dryRun := flag.Bool("dry-run", false, "log actions without writing to object storage or database")
	limit := flag.Int("limit", 0, "max rows to migrate (0 = all)")
	flag.Parse()

	ctx := context.Background()
	databaseURL := config.Env("DATABASE_URL", "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable")
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	objCfg := objectstore.ConfigFromEnv()
	store, err := objectstore.New(objCfg)
	if err != nil {
		log.Fatalf("object storage: %v", err)
	}
	if store == nil {
		log.Fatal("object storage is not configured (set OBJECT_STORAGE_* env vars)")
	}

	query := `
		SELECT id, tenant_id, repair_job_id, file_name, content_type, content, size_bytes
		FROM repair.repair_attachments
		WHERE content IS NOT NULL AND octet_length(content) > 0
		  AND (storage_key IS NULL OR storage_key = '')
		ORDER BY created_at ASC`
	args := []any{}
	if *limit > 0 {
		query += " LIMIT $1"
		args = append(args, *limit)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		log.Fatalf("query attachments: %v", err)
	}
	defer rows.Close()

	var pending []attachmentRow
	for rows.Next() {
		var row attachmentRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.RepairJobID,
			&row.FileName, &row.ContentType, &row.Content, &row.SizeBytes,
		); err != nil {
			log.Fatalf("scan row: %v", err)
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}

	if len(pending) == 0 {
		log.Printf("no attachments to migrate")
		return
	}

	log.Printf("found %d attachment(s) with BYTEA content to migrate (dry_run=%v)", len(pending), *dryRun)

	migrated, failed := 0, 0
	for _, row := range pending {
		key := objectstore.AttachmentKey(
			row.TenantID.String(), row.RepairJobID.String(), row.ID.String(), row.FileName,
		)
		if *dryRun {
			log.Printf("[dry-run] would migrate attachment %s (%d bytes) -> %s", row.ID, len(row.Content), key)
			migrated++
			continue
		}

		if err := store.Put(ctx, key, row.Content, row.ContentType); err != nil {
			log.Printf("FAIL %s put: %v", row.ID, err)
			failed++
			continue
		}
		got, err := store.Get(ctx, key)
		if err != nil {
			log.Printf("FAIL %s verify get: %v", row.ID, err)
			_ = store.Delete(ctx, key)
			failed++
			continue
		}
		if !bytes.Equal(got, row.Content) {
			log.Printf("FAIL %s verify bytes mismatch", row.ID)
			_ = store.Delete(ctx, key)
			failed++
			continue
		}

		tag, err := pool.Exec(ctx, `
			UPDATE repair.repair_attachments
			SET storage_key = $1, content = NULL, upload_status = 'completed'
			WHERE id = $2 AND tenant_id = $3
			  AND content IS NOT NULL AND (storage_key IS NULL OR storage_key = '')`,
			key, row.ID, row.TenantID)
		if err != nil {
			log.Printf("FAIL %s database update: %v", row.ID, err)
			_ = store.Delete(ctx, key)
			failed++
			continue
		}
		if tag.RowsAffected() == 0 {
			log.Printf("FAIL %s database update: row changed concurrently", row.ID)
			_ = store.Delete(ctx, key)
			failed++
			continue
		}
		log.Printf("OK migrated attachment %s -> %s", row.ID, key)
		migrated++
	}

	log.Printf("done migrated=%d failed=%d", migrated, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
