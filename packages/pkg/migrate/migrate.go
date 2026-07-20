package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunDir(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("migrate %s: %w", f, err)
		}
	}
	return nil
}

func RunSQL(ctx context.Context, pool *pgxpool.Pool, sqls ...string) error {
	for i, s := range sqls {
		if _, err := pool.Exec(ctx, s); err != nil {
			return fmt.Errorf("sql[%d]: %w", i, err)
		}
	}
	return nil
}
