package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Category is a POS catalog folder. Roots have ParentID == nil; children nest under a parent.
type Category struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Path     string     `json:"path"`
	Depth    int        `json:"depth"`
	Children []Category `json:"children,omitempty"`
}

func (s *Service) ListCategories(ctx context.Context, tenantID uuid.UUID) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE tree AS (
			SELECT id, name, parent_id, name::text AS path, 0 AS depth,
				lower(name) AS sort_key
			FROM inventory.categories
			WHERE tenant_id = $1 AND parent_id IS NULL
			UNION ALL
			SELECT c.id, c.name, c.parent_id,
				tree.path || ' › ' || c.name,
				tree.depth + 1,
				tree.sort_key || '/' || lower(c.name)
			FROM inventory.categories c
			JOIN tree ON c.parent_id = tree.id
			WHERE c.tenant_id = $1
		)
		SELECT id, name, parent_id, path, depth
		FROM tree
		ORDER BY sort_key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Path, &c.Depth); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// ListOnlineCategories returns categories that have at least one
// online-visible product, plus their ancestors so the storefront can nest
// leaves under parents even when products live only on children.
func (s *Service) ListOnlineCategories(ctx context.Context, tenantID uuid.UUID) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE leaf AS (
			SELECT DISTINCT c.id, c.name, c.parent_id
			FROM inventory.categories c
			JOIN inventory.products p ON p.category_id = c.id
			WHERE c.tenant_id = $1 AND p.tenant_id = $1 AND COALESCE(p.online_visible, false) = true
		),
		walk AS (
			SELECT id, name, parent_id FROM leaf
			UNION
			SELECT c.id, c.name, c.parent_id
			FROM inventory.categories c
			JOIN walk w ON c.id = w.parent_id
			WHERE c.tenant_id = $1
		),
		tree AS (
			SELECT id, name, parent_id, name::text AS path, 0 AS depth,
				lower(name) AS sort_key
			FROM walk
			WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, c.name, c.parent_id,
				tree.path || ' › ' || c.name,
				tree.depth + 1,
				tree.sort_key || '/' || lower(c.name)
			FROM walk c
			JOIN tree ON c.parent_id = tree.id
		)
		SELECT id, name, parent_id, path, depth
		FROM tree
		ORDER BY sort_key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Path, &c.Depth); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// ListCategoryTree nests ListCategories into a parent → children tree.
func (s *Service) ListCategoryTree(ctx context.Context, tenantID uuid.UUID) ([]Category, error) {
	flat, err := s.ListCategories(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byParent := map[string][]Category{}
	for _, c := range flat {
		key := ""
		if c.ParentID != nil {
			key = c.ParentID.String()
		}
		byParent[key] = append(byParent[key], c)
	}
	var attach func(parentKey string) []Category
	attach = func(parentKey string) []Category {
		nodes := byParent[parentKey]
		out := make([]Category, 0, len(nodes))
		for _, n := range nodes {
			n.Children = attach(n.ID.String())
			out = append(out, n)
		}
		return out
	}
	return attach(""), nil
}

func (s *Service) CreateCategory(ctx context.Context, tenantID uuid.UUID, name string, parentID *uuid.UUID) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	if parentID != nil {
		var ok bool
		err := s.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM inventory.categories WHERE tenant_id = $1 AND id = $2)`,
			tenantID, *parentID).Scan(&ok)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("parent category not found")
		}
	}
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.categories (id, tenant_id, name, parent_id)
		VALUES ($1, $2, $3, $4)`, id, tenantID, name, parentID)
	if err != nil {
		if strings.Contains(err.Error(), "idx_categories_tenant_parent_name") ||
			strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("a category with that name already exists here")
		}
		return nil, err
	}
	return s.getCategory(ctx, tenantID, id)
}

func (s *Service) UpdateCategory(ctx context.Context, tenantID, id uuid.UUID, name *string, parentID **uuid.UUID) (*Category, error) {
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, fmt.Errorf("name required")
		}
		*name = n
	}
	var parentArg *uuid.UUID
	setParent := false
	if parentID != nil {
		setParent = true
		parentArg = *parentID
		if parentArg != nil {
			if *parentArg == id {
				return nil, fmt.Errorf("category cannot be its own parent")
			}
			desc, err := s.categoryDescendantIDs(ctx, tenantID, id)
			if err != nil {
				return nil, err
			}
			for _, d := range desc {
				if d == *parentArg {
					return nil, fmt.Errorf("cannot move a category under its own child")
				}
			}
			var ok bool
			err = s.pool.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM inventory.categories WHERE tenant_id = $1 AND id = $2)`,
				tenantID, *parentArg).Scan(&ok)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("parent category not found")
			}
		}
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE inventory.categories SET
			name = COALESCE($1, name),
			parent_id = CASE WHEN $2::boolean THEN $3 ELSE parent_id END
		WHERE tenant_id = $4 AND id = $5`,
		name, setParent, parentArg, tenantID, id)
	if err != nil {
		if strings.Contains(err.Error(), "idx_categories_tenant_parent_name") ||
			strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("a category with that name already exists here")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("category not found")
	}
	if name != nil {
		_, _ = s.pool.Exec(ctx, `
			UPDATE inventory.products SET category = $1, updated_at = now()
			WHERE tenant_id = $2 AND category_id = $3`, *name, tenantID, id)
	}
	return s.getCategory(ctx, tenantID, id)
}

func (s *Service) DeleteCategory(ctx context.Context, tenantID, id uuid.UUID) error {
	var childCount, productCount int
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM inventory.categories WHERE tenant_id = $1 AND parent_id = $2),
			(SELECT COUNT(*) FROM inventory.products WHERE tenant_id = $1 AND category_id = $2)`,
		tenantID, id).Scan(&childCount, &productCount)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return fmt.Errorf("remove or move child categories first")
	}
	if productCount > 0 {
		return fmt.Errorf("move or uncategorize %d product(s) first", productCount)
	}
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM inventory.categories WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

func (s *Service) getCategory(ctx context.Context, tenantID, id uuid.UUID) (*Category, error) {
	flat, err := s.ListCategories(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range flat {
		if flat[i].ID == id {
			return &flat[i], nil
		}
	}
	return nil, fmt.Errorf("category not found")
}

func (s *Service) categoryDescendantIDs(ctx context.Context, tenantID, root uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE tree AS (
			SELECT id FROM inventory.categories WHERE tenant_id = $1 AND parent_id = $2
			UNION ALL
			SELECT c.id FROM inventory.categories c
			JOIN tree t ON c.parent_id = t.id
			WHERE c.tenant_id = $1
		)
		SELECT id FROM tree`, tenantID, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) categoryName(ctx context.Context, tenantID, id uuid.UUID) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `
		SELECT name FROM inventory.categories WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("category not found")
	}
	return name, err
}
