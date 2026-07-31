package commerce

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/techlane/techlane/internal/repair"
	"golang.org/x/crypto/bcrypt"
)

type CustomerAccount struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Phone    *string   `json:"phone,omitempty"`
}

type CustomerSession struct {
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expires_at"`
	Customer  *CustomerAccount `json:"customer"`
}

func (s *Service) RegisterCustomerAccount(
	ctx context.Context,
	tenantID uuid.UUID,
	fullName, email, phone, password string,
) (*CustomerSession, error) {
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	phone = strings.TrimSpace(phone)
	if fullName == "" || email == "" || !strings.Contains(email, "@") || len(password) < 8 {
		return nil, fmt.Errorf("full name, valid email, and password of at least 8 characters required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	var phoneValue any
	if phone != "" {
		phoneValue = phone
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.customers (id, tenant_id, full_name, phone, email, password_hash)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, tenantID, fullName, phoneValue, email, string(hash))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("an account already exists for that email")
		}
		return nil, err
	}
	account := &CustomerAccount{ID: id, FullName: fullName, Email: email}
	if phone != "" {
		account.Phone = &phone
	}
	return s.createCustomerSession(ctx, tenantID, account)
}

func (s *Service) LoginCustomerAccount(ctx context.Context, tenantID uuid.UUID, email, password string) (*CustomerSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var account CustomerAccount
	var passwordHash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, email, phone, password_hash
		FROM repair.customers
		WHERE tenant_id = $1 AND lower(email) = $2 AND password_hash IS NOT NULL`,
		tenantID, email).Scan(&account.ID, &account.FullName, &account.Email, &account.Phone, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil) {
		return nil, fmt.Errorf("invalid email or password")
	}
	if err != nil {
		return nil, err
	}
	return s.createCustomerSession(ctx, tenantID, &account)
}

func (s *Service) createCustomerSession(ctx context.Context, tenantID uuid.UUID, account *CustomerAccount) (*CustomerSession, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(raw)
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.customer_sessions (id, tenant_id, customer_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), tenantID, account.ID, hex.EncodeToString(tokenHash[:]), expiresAt)
	if err != nil {
		return nil, err
	}
	return &CustomerSession{Token: token, ExpiresAt: expiresAt, Customer: account}, nil
}

func (s *Service) AuthenticateCustomer(ctx context.Context, tenantID uuid.UUID, token string) (*CustomerAccount, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("customer authentication required")
	}
	tokenHash := sha256.Sum256([]byte(token))
	var account CustomerAccount
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.full_name, c.email, c.phone
		FROM repair.customer_sessions cs
		JOIN repair.customers c ON c.id = cs.customer_id AND c.tenant_id = cs.tenant_id
		WHERE cs.tenant_id = $1 AND cs.token_hash = $2 AND cs.expires_at > now()`,
		tenantID, hex.EncodeToString(tokenHash[:])).
		Scan(&account.ID, &account.FullName, &account.Email, &account.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid or expired customer session")
	}
	return &account, err
}

func (s *Service) ListCustomerOrders(ctx context.Context, tenantID, customerID uuid.UUID) ([]Order, error) {
	var phone *string
	_ = s.pool.QueryRow(ctx, `SELECT phone FROM repair.customers WHERE tenant_id = $1 AND id = $2`, tenantID, customerID).Scan(&phone)
	variants := []string{}
	if phone != nil && strings.TrimSpace(*phone) != "" {
		variants = phoneMatchVariants(*phone)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.status, o.total::float8, o.branch_id, o.fulfilment_type, o.created_at,
		       CASE WHEN o.status IN ('ready_for_pickup', 'collected', 'confirmed', 'delivered') THEN o.collection_code ELSE NULL END
		FROM sales.orders o
		WHERE o.tenant_id = $1 AND o.channel = 'online'
		  AND (
		    o.customer_id = $2
		    OR (
		      cardinality($3::text[]) > 0
		      AND regexp_replace(COALESCE(o.guest_phone, ''), '[^0-9]', '', 'g') = ANY($3::text[])
		    )
		  )
		ORDER BY o.created_at DESC LIMIT 100`, tenantID, customerID, variants)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Order, 0)
	for rows.Next() {
		var order Order
		var code *string
		if err := rows.Scan(
			&order.ID, &order.Status, &order.Total, &order.BranchID,
			&order.FulfilmentType, &order.CreatedAt, &code,
		); err != nil {
			return nil, err
		}
		if code != nil {
			order.CollectionCode = *code
		}
		items = append(items, order)
	}
	return items, rows.Err()
}

// AccountRepair is a slim repair row for the customer dashboard.
type AccountRepair struct {
	ID             uuid.UUID `json:"id"`
	JobCode        string    `json:"job_code"`
	JobNumber      int       `json:"job_number"`
	Status         string    `json:"status"`
	ProblemSummary string    `json:"problem_summary,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Service) ListAccountRepairs(ctx context.Context, tenantID, customerID uuid.UUID, phone string) ([]AccountRepair, error) {
	variants := phoneMatchVariants(phone)
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.job_code, j.job_number, j.status, j.problem_summary, j.created_at
		FROM repair.repair_jobs j
		LEFT JOIN repair.customers c ON c.id = j.customer_id
		WHERE j.tenant_id = $1
		  AND j.deleted_at IS NULL
		  AND (
		    j.customer_id = $2
		    OR (
		      cardinality($3::text[]) > 0
		      AND regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = ANY($3::text[])
		    )
		  )
		ORDER BY j.created_at DESC
		LIMIT 100`, tenantID, customerID, variants)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AccountRepair, 0)
	for rows.Next() {
		var item AccountRepair
		if err := rows.Scan(&item.ID, &item.JobCode, &item.JobNumber, &item.Status, &item.ProblemSummary, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func phoneMatchVariants(raw string) []string {
	return repair.PhoneMatchVariants(raw)
}
