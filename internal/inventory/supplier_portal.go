package inventory

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
	"golang.org/x/crypto/bcrypt"
)

const (
	supplierSessionTTL  = 30 * 24 * time.Hour
	supplierInviteTTL   = 7 * 24 * time.Hour
	minSupplierPassword = 8
)

type SupplierContact struct {
	ID           uuid.UUID `json:"id"`
	SupplierID   uuid.UUID `json:"supplier_id"`
	Email        string    `json:"email"`
	Phone        *string   `json:"phone,omitempty"`
	DisplayName  string    `json:"display_name"`
	Status       string    `json:"status"`
	SupplierName string    `json:"supplier_name,omitempty"`
}

type SupplierSession struct {
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expires_at"`
	Contact   *SupplierContact `json:"contact"`
}

type InviteResult struct {
	Contact     *SupplierContact `json:"contact"`
	InviteToken string           `json:"invite_token"`
	ExpiresAt   time.Time        `json:"expires_at"`
}

type PartRequestQuote struct {
	ID            uuid.UUID  `json:"id"`
	PartRequestID uuid.UUID  `json:"part_request_id"`
	SupplierID    uuid.UUID  `json:"supplier_id"`
	UnitCost      float64    `json:"unit_cost"`
	Notes         *string    `json:"notes,omitempty"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
}

type SupplierPortalRequest struct {
	ID                 uuid.UUID          `json:"id"`
	RepairJobID        uuid.UUID          `json:"repair_job_id"`
	JobCode            string             `json:"job_code,omitempty"`
	Status             string             `json:"status"`
	Description        string             `json:"description"`
	Quantity           int                `json:"quantity"`
	AssignedSupplierID *uuid.UUID         `json:"assigned_supplier_id,omitempty"`
	QuoteStatus        *string            `json:"quote_status,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	Quotes             []PartRequestQuote `json:"quotes,omitempty"`
	Issue              *SupplierIssue     `json:"issue,omitempty"`
}

type SupplierIssueResult struct {
	Issue     *SupplierIssue `json:"issue"`
	AuthCode  string         `json:"auth_code"`
	QRPayload string         `json:"qr_payload"`
}

type SupplierCreditSummary struct {
	SupplierID        uuid.UUID     `json:"supplier_id"`
	SupplierName      string        `json:"supplier_name"`
	OutstandingCredit float64       `json:"outstanding_credit"`
	Entries           []CreditEntry `json:"entries"`
}

func hashOpaqueToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newOpaqueToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// QRPayloadForIssue builds the technician QR deep link.
func QRPayloadForIssue(issueID uuid.UUID, authCode string) string {
	return fmt.Sprintf("techlane://auth/%s/%s", issueID.String(), authCode)
}

// ContactMayAccessRequest reports whether a supplier contact may see a part request.
func ContactMayAccessRequest(assignedSupplierID *uuid.UUID, contactSupplierID uuid.UUID) bool {
	return assignedSupplierID != nil && *assignedSupplierID == contactSupplierID
}

// ValidateInviteAccept checks invite status, expiry, and password rules.
func ValidateInviteAccept(status string, expiresAt *time.Time, password string, now time.Time) error {
	if status != "invited" {
		return fmt.Errorf("invite is not pending")
	}
	if expiresAt == nil || !expiresAt.After(now) {
		return fmt.Errorf("invite expired")
	}
	if len(password) < minSupplierPassword {
		return fmt.Errorf("password must be at least %d characters", minSupplierPassword)
	}
	return nil
}

// ResolveIssueUnitCost picks unit cost for supplier issue creation.
// Prefer an accepted quote; otherwise auto-accept a pending quote from the same supplier.
func ResolveIssueUnitCost(accepted, pending *PartRequestQuote) (unitCost float64, autoAcceptPendingID *uuid.UUID, err error) {
	if accepted != nil && accepted.Status == "accepted" {
		return accepted.UnitCost, nil, nil
	}
	if pending != nil && pending.Status == "pending" {
		id := pending.ID
		return pending.UnitCost, &id, nil
	}
	return 0, nil, fmt.Errorf("no accepted or pending quote to issue")
}

func (s *Service) ensureDefaultSupplierContact(ctx context.Context, tenantID, supplierID uuid.UUID) error {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inventory.supplier_contacts
		WHERE tenant_id = $1 AND lower(email) = lower($2)`, tenantID, "supplier@techlane.local").Scan(&n)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO inventory.supplier_contacts
			(id, tenant_id, supplier_id, email, display_name, password_hash, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')`,
		uuid.New(), tenantID, supplierID, "supplier@techlane.local", "Default Screens Contact", string(hash))
	return err
}

func (s *Service) CreateSupplier(ctx context.Context, tenantID uuid.UUID, name string, phone *string) (*Supplier, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.suppliers (id, tenant_id, name, phone)
		VALUES ($1, $2, $3, $4)`, id, tenantID, name, phone)
	if err != nil {
		return nil, err
	}
	return &Supplier{ID: id, Name: name, Phone: phone}, nil
}

func (s *Service) InviteSupplierContact(
	ctx context.Context,
	tenantID, supplierID uuid.UUID,
	email, displayName string,
	phone *string,
) (*InviteResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	if email == "" || !strings.Contains(email, "@") || displayName == "" {
		return nil, fmt.Errorf("email and display_name required")
	}
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM inventory.suppliers WHERE tenant_id = $1 AND id = $2)`,
		tenantID, supplierID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("supplier not found")
	}

	token, err := newOpaqueToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(supplierInviteTTL)
	id := uuid.New()
	var phoneVal any
	if phone != nil && strings.TrimSpace(*phone) != "" {
		p := strings.TrimSpace(*phone)
		phoneVal = p
		phone = &p
	} else {
		phone = nil
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO inventory.supplier_contacts
			(id, tenant_id, supplier_id, email, phone, display_name, status, invite_token_hash, invite_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'invited', $7, $8)`,
		id, tenantID, supplierID, email, phoneVal, displayName, hashOpaqueToken(token), expires)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("contact already exists for that email")
		}
		return nil, err
	}
	return &InviteResult{
		Contact: &SupplierContact{
			ID: id, SupplierID: supplierID, Email: email, Phone: phone,
			DisplayName: displayName, Status: "invited",
		},
		InviteToken: token,
		ExpiresAt:   expires,
	}, nil
}

func (s *Service) AcceptSupplierInvite(ctx context.Context, token, password string) (*SupplierSession, error) {
	token = strings.TrimSpace(token)
	now := time.Now().UTC()
	var contact SupplierContact
	var tenantID uuid.UUID
	var status string
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.supplier_id, c.email, c.phone, c.display_name, c.status, c.invite_expires_at
		FROM inventory.supplier_contacts c
		WHERE c.invite_token_hash = $1`, hashOpaqueToken(token)).
		Scan(&contact.ID, &tenantID, &contact.SupplierID, &contact.Email, &contact.Phone, &contact.DisplayName, &status, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid invite token")
	}
	if err != nil {
		return nil, err
	}
	if err := ValidateInviteAccept(status, expiresAt, password, now); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE inventory.supplier_contacts
		SET password_hash = $1, status = 'active', invite_token_hash = NULL, invite_expires_at = NULL, updated_at = now()
		WHERE id = $2`, string(hash), contact.ID)
	if err != nil {
		return nil, err
	}
	contact.Status = "active"
	return s.createSupplierSession(ctx, tenantID, &contact)
}

func (s *Service) LoginSupplier(ctx context.Context, email, password string) (*SupplierSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var contact SupplierContact
	var tenantID uuid.UUID
	var passwordHash string
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.supplier_id, c.email, c.phone, c.display_name, c.status, c.password_hash, s.name
		FROM inventory.supplier_contacts c
		JOIN inventory.suppliers s ON s.id = c.supplier_id AND s.tenant_id = c.tenant_id
		WHERE lower(c.email) = $1 AND c.status = 'active' AND c.password_hash IS NOT NULL
		ORDER BY c.created_at
		LIMIT 1`, email).
		Scan(&contact.ID, &tenantID, &contact.SupplierID, &contact.Email, &contact.Phone,
			&contact.DisplayName, &contact.Status, &passwordHash, &contact.SupplierName)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil) {
		return nil, fmt.Errorf("invalid email or password")
	}
	if err != nil {
		return nil, err
	}
	return s.createSupplierSession(ctx, tenantID, &contact)
}

func (s *Service) createSupplierSession(ctx context.Context, tenantID uuid.UUID, contact *SupplierContact) (*SupplierSession, error) {
	token, err := newOpaqueToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(supplierSessionTTL)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO inventory.supplier_sessions (id, tenant_id, contact_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), tenantID, contact.ID, hashOpaqueToken(token), expires)
	if err != nil {
		return nil, err
	}
	return &SupplierSession{Token: token, ExpiresAt: expires, Contact: contact}, nil
}

func (s *Service) LogoutSupplier(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM inventory.supplier_sessions WHERE token_hash = $1`, hashOpaqueToken(token))
	return err
}

func (s *Service) AuthenticateSupplier(ctx context.Context, token string) (tenantID uuid.UUID, contact *SupplierContact, err error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return uuid.Nil, nil, fmt.Errorf("supplier authentication required")
	}
	var c SupplierContact
	err = s.pool.QueryRow(ctx, `
		SELECT c.id, ss.tenant_id, c.supplier_id, c.email, c.phone, c.display_name, c.status, s.name
		FROM inventory.supplier_sessions ss
		JOIN inventory.supplier_contacts c ON c.id = ss.contact_id AND c.tenant_id = ss.tenant_id
		JOIN inventory.suppliers s ON s.id = c.supplier_id AND s.tenant_id = c.tenant_id
		WHERE ss.token_hash = $1 AND ss.expires_at > now() AND c.status = 'active'`,
		hashOpaqueToken(token)).
		Scan(&c.ID, &tenantID, &c.SupplierID, &c.Email, &c.Phone, &c.DisplayName, &c.Status, &c.SupplierName)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil, fmt.Errorf("invalid or expired supplier session")
	}
	if err != nil {
		return uuid.Nil, nil, err
	}
	return tenantID, &c, nil
}

func (s *Service) AssignPartRequest(ctx context.Context, tenantID, requestID, supplierID uuid.UUID) (*PartRequest, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM inventory.suppliers WHERE tenant_id = $1 AND id = $2)`,
		tenantID, supplierID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("supplier not found")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE inventory.part_requests
		SET assigned_supplier_id = $1, quote_status = COALESCE(quote_status, 'awaiting'), updated_at = now()
		WHERE tenant_id = $2 AND id = $3 AND status = 'pending'`,
		supplierID, tenantID, requestID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("part request not found or not pending")
	}
	return s.getPartRequest(ctx, tenantID, requestID)
}

func (s *Service) getPartRequest(ctx context.Context, tenantID, requestID uuid.UUID) (*PartRequest, error) {
	var pr PartRequest
	err := s.pool.QueryRow(ctx, `
		SELECT pr.id, pr.repair_job_id, COALESCE(rj.job_code, ''), pr.status, pr.description, pr.quantity,
		       pr.created_at, pr.assigned_supplier_id, pr.quote_status
		FROM inventory.part_requests pr
		LEFT JOIN repair.repair_jobs rj ON rj.id = pr.repair_job_id
		WHERE pr.tenant_id = $1 AND pr.id = $2`, tenantID, requestID).
		Scan(&pr.ID, &pr.RepairJobID, &pr.JobCode, &pr.Status, &pr.Description, &pr.Quantity,
			&pr.CreatedAt, &pr.AssignedSupplierID, &pr.QuoteStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("part request not found")
	}
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

func (s *Service) ListSupplierPortalRequests(ctx context.Context, tenantID, supplierID uuid.UUID, status string) ([]SupplierPortalRequest, error) {
	q := `
		SELECT pr.id, pr.repair_job_id, COALESCE(rj.job_code, ''), pr.status, pr.description, pr.quantity,
		       pr.assigned_supplier_id, pr.quote_status, pr.created_at
		FROM inventory.part_requests pr
		LEFT JOIN repair.repair_jobs rj ON rj.id = pr.repair_job_id
		WHERE pr.tenant_id = $1 AND pr.assigned_supplier_id = $2`
	args := []any{tenantID, supplierID}
	if status != "" {
		q += ` AND pr.status = $3`
		args = append(args, status)
	}
	q += ` ORDER BY pr.created_at DESC LIMIT 200`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SupplierPortalRequest
	for rows.Next() {
		var pr SupplierPortalRequest
		if err := rows.Scan(
			&pr.ID, &pr.RepairJobID, &pr.JobCode, &pr.Status, &pr.Description, &pr.Quantity,
			&pr.AssignedSupplierID, &pr.QuoteStatus, &pr.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, pr)
	}
	if items == nil {
		items = []SupplierPortalRequest{}
	}
	return items, rows.Err()
}

func (s *Service) GetSupplierPortalRequest(ctx context.Context, tenantID, supplierID, requestID uuid.UUID) (*SupplierPortalRequest, error) {
	var pr SupplierPortalRequest
	err := s.pool.QueryRow(ctx, `
		SELECT pr.id, pr.repair_job_id, COALESCE(rj.job_code, ''), pr.status, pr.description, pr.quantity,
		       pr.assigned_supplier_id, pr.quote_status, pr.created_at
		FROM inventory.part_requests pr
		LEFT JOIN repair.repair_jobs rj ON rj.id = pr.repair_job_id
		WHERE pr.tenant_id = $1 AND pr.id = $2`, tenantID, requestID).
		Scan(&pr.ID, &pr.RepairJobID, &pr.JobCode, &pr.Status, &pr.Description, &pr.Quantity,
			&pr.AssignedSupplierID, &pr.QuoteStatus, &pr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("part request not found")
	}
	if err != nil {
		return nil, err
	}
	if !ContactMayAccessRequest(pr.AssignedSupplierID, supplierID) {
		return nil, fmt.Errorf("part request not found")
	}

	quotes, err := s.listQuotesForRequest(ctx, tenantID, requestID, &supplierID)
	if err != nil {
		return nil, err
	}
	pr.Quotes = quotes

	var issue SupplierIssue
	var collectedAt *time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT id, part_request_id, repair_job_id, supplier_id, auth_code, status, unit_cost::float8,
		       collected_at, reconciliation_status
		FROM inventory.supplier_issues
		WHERE tenant_id = $1 AND part_request_id = $2 AND supplier_id = $3
		ORDER BY created_at DESC LIMIT 1`, tenantID, requestID, supplierID).
		Scan(&issue.ID, &issue.PartRequestID, &issue.RepairJobID, &issue.SupplierID, &issue.AuthCode,
			&issue.Status, &issue.UnitCost, &collectedAt, &issue.ReconciliationStatus)
	if err == nil {
		issue.CollectedAt = collectedAt
		pr.Issue = &issue
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return &pr, nil
}

func (s *Service) listQuotesForRequest(ctx context.Context, tenantID, requestID uuid.UUID, supplierID *uuid.UUID) ([]PartRequestQuote, error) {
	q := `
		SELECT id, part_request_id, supplier_id, unit_cost::float8, notes, status, created_at, decided_at
		FROM inventory.part_request_quotes
		WHERE tenant_id = $1 AND part_request_id = $2`
	args := []any{tenantID, requestID}
	if supplierID != nil {
		q += ` AND supplier_id = $3`
		args = append(args, *supplierID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PartRequestQuote
	for rows.Next() {
		var quote PartRequestQuote
		if err := rows.Scan(
			&quote.ID, &quote.PartRequestID, &quote.SupplierID, &quote.UnitCost, &quote.Notes,
			&quote.Status, &quote.CreatedAt, &quote.DecidedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, quote)
	}
	if items == nil {
		items = []PartRequestQuote{}
	}
	return items, rows.Err()
}

// SubmitSupplierQuote records the supplier's price and immediately authorizes the
// part: the quote is auto-accepted and a supplier issue with an auth code is
// created in the same transaction, so no separate ops approval step is needed.
func (s *Service) SubmitSupplierQuote(
	ctx context.Context,
	tenantID, supplierID, requestID uuid.UUID,
	unitCost float64,
	notes *string,
	actorContactID uuid.UUID,
) (*PartRequestQuote, error) {
	if unitCost < 0 {
		return nil, fmt.Errorf("unit_cost must be non-negative")
	}
	pr, err := s.requireAssignedPendingRequest(ctx, tenantID, supplierID, requestID)
	if err != nil {
		return nil, err
	}

	var existing uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.supplier_issues
		WHERE tenant_id = $1 AND part_request_id = $2 LIMIT 1`, tenantID, requestID).Scan(&existing)
	if err == nil {
		return nil, fmt.Errorf("issue already exists for this request")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	authCode, err := GenerateAuthCode()
	if err != nil {
		return nil, err
	}
	issueID := uuid.New()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_request_quotes
		SET status = 'superseded', decided_at = now()
		WHERE tenant_id = $1 AND part_request_id = $2 AND supplier_id = $3 AND status = 'pending'`,
		tenantID, requestID, supplierID)
	if err != nil {
		return nil, err
	}

	id := uuid.New()
	now := time.Now().UTC()
	var notesVal any
	if notes != nil && strings.TrimSpace(*notes) != "" {
		n := strings.TrimSpace(*notes)
		notesVal = n
		notes = &n
	} else {
		notes = nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.part_request_quotes
			(id, tenant_id, part_request_id, supplier_id, unit_cost, notes, status, decided_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'accepted', $7)`,
		id, tenantID, requestID, supplierID, unitCost, notesVal, now)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_requests
		SET status = 'approved', quote_status = 'accepted', updated_at = now()
		WHERE id = $1`, requestID)
	if err != nil {
		return nil, err
	}

	var branchID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT branch_id FROM inventory.part_requests WHERE id = $1`, requestID).Scan(&branchID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_issues
			(id, tenant_id, branch_id, supplier_id, part_request_id, repair_job_id, auth_code, unit_cost,
			 status, requested_by, approved_by, reconciliation_status, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'approved', $9, $9, 'pending', $10)`,
		issueID, tenantID, branchID, supplierID, requestID, pr.RepairJobID, authCode, unitCost,
		actorContactID, uuid.New())
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_credit_entries
			(id, tenant_id, supplier_id, supplier_issue_id, amount, entry_type, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, 'issue', $6, $7)`,
		uuid.New(), tenantID, supplierID, issueID, unitCost, actorContactID, uuid.New())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &PartRequestQuote{
		ID: id, PartRequestID: requestID, SupplierID: supplierID,
		UnitCost: unitCost, Notes: notes, Status: "accepted", CreatedAt: now, DecidedAt: &now,
	}, nil
}

func (s *Service) DeclineSupplierRequest(ctx context.Context, tenantID, supplierID, requestID uuid.UUID, notes *string) (*PartRequestQuote, error) {
	if _, err := s.requireAssignedPendingRequest(ctx, tenantID, supplierID, requestID); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_request_quotes
		SET status = 'superseded', decided_at = now()
		WHERE tenant_id = $1 AND part_request_id = $2 AND supplier_id = $3 AND status = 'pending'`,
		tenantID, requestID, supplierID)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	var notesVal any
	if notes != nil && strings.TrimSpace(*notes) != "" {
		n := strings.TrimSpace(*notes)
		notesVal = n
		notes = &n
	} else {
		notes = nil
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.part_request_quotes
			(id, tenant_id, part_request_id, supplier_id, unit_cost, notes, status, decided_at)
		VALUES ($1, $2, $3, $4, 0, $5, 'declined', $6)`,
		id, tenantID, requestID, supplierID, notesVal, now)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_requests SET quote_status = 'declined', updated_at = now()
		WHERE id = $1`, requestID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &PartRequestQuote{
		ID: id, PartRequestID: requestID, SupplierID: supplierID,
		UnitCost: 0, Notes: notes, Status: "declined", CreatedAt: now, DecidedAt: &now,
	}, nil
}

func (s *Service) requireAssignedPendingRequest(ctx context.Context, tenantID, supplierID, requestID uuid.UUID) (*PartRequest, error) {
	pr, err := s.getPartRequest(ctx, tenantID, requestID)
	if err != nil {
		return nil, err
	}
	if !ContactMayAccessRequest(pr.AssignedSupplierID, supplierID) {
		return nil, fmt.Errorf("part request not found")
	}
	if pr.Status != "pending" {
		return nil, fmt.Errorf("part request not pending")
	}
	return pr, nil
}

// MarkSupplierRequestReady flags an accepted/quoted request as ready for shop collection.
func (s *Service) MarkSupplierRequestReady(ctx context.Context, tenantID, supplierID, requestID uuid.UUID) (*PartRequest, error) {
	pr, err := s.getPartRequest(ctx, tenantID, requestID)
	if err != nil {
		return nil, err
	}
	if !ContactMayAccessRequest(pr.AssignedSupplierID, supplierID) {
		return nil, fmt.Errorf("part request not found")
	}
	qs := ""
	if pr.QuoteStatus != nil {
		qs = *pr.QuoteStatus
	}
	if qs != "quoted" && qs != "accepted" && qs != "ready" {
		return nil, fmt.Errorf("request must be quoted or accepted before marking ready")
	}
	ready := "ready"
	_, err = s.pool.Exec(ctx, `
		UPDATE inventory.part_requests
		SET quote_status = $1, updated_at = now()
		WHERE tenant_id = $2 AND id = $3`, ready, tenantID, requestID)
	if err != nil {
		return nil, err
	}
	pr.QuoteStatus = &ready
	return pr, nil
}

func (s *Service) IssueAsSupplier(ctx context.Context, tenantID, supplierID, requestID, actorContactID uuid.UUID) (*SupplierIssueResult, error) {
	pr, err := s.requireAssignedPendingRequest(ctx, tenantID, supplierID, requestID)
	if err != nil {
		return nil, err
	}

	var existing uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.supplier_issues
		WHERE tenant_id = $1 AND part_request_id = $2 LIMIT 1`, tenantID, requestID).Scan(&existing)
	if err == nil {
		return nil, fmt.Errorf("issue already exists for this request")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	quotes, err := s.listQuotesForRequest(ctx, tenantID, requestID, &supplierID)
	if err != nil {
		return nil, err
	}
	var accepted, pending *PartRequestQuote
	for i := range quotes {
		q := &quotes[i]
		switch q.Status {
		case "accepted":
			if accepted == nil {
				accepted = q
			}
		case "pending":
			if pending == nil {
				pending = q
			}
		}
	}
	unitCost, autoAcceptID, err := ResolveIssueUnitCost(accepted, pending)
	if err != nil {
		return nil, err
	}

	authCode, err := GenerateAuthCode()
	if err != nil {
		return nil, err
	}
	issueID := uuid.New()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if autoAcceptID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE inventory.part_request_quotes
			SET status = 'accepted', decided_at = now()
			WHERE id = $1`, *autoAcceptID)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE inventory.part_request_quotes
			SET status = 'superseded', decided_at = now()
			WHERE tenant_id = $1 AND part_request_id = $2 AND id <> $3 AND status = 'pending'`,
			tenantID, requestID, *autoAcceptID)
		if err != nil {
			return nil, err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_requests
		SET status = 'approved', quote_status = 'accepted', updated_at = now()
		WHERE id = $1`, requestID)
	if err != nil {
		return nil, err
	}

	var branchID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT branch_id FROM inventory.part_requests WHERE id = $1`, requestID).Scan(&branchID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_issues
			(id, tenant_id, branch_id, supplier_id, part_request_id, repair_job_id, auth_code, unit_cost,
			 status, requested_by, approved_by, reconciliation_status, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'approved', $9, $9, 'pending', $10)`,
		issueID, tenantID, branchID, supplierID, requestID, pr.RepairJobID, authCode, unitCost,
		actorContactID, uuid.New())
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_credit_entries
			(id, tenant_id, supplier_id, supplier_issue_id, amount, entry_type, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, 'issue', $6, $7)`,
		uuid.New(), tenantID, supplierID, issueID, unitCost, actorContactID, uuid.New())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	issue := &SupplierIssue{
		ID: issueID, PartRequestID: requestID, RepairJobID: pr.RepairJobID, SupplierID: supplierID,
		AuthCode: authCode, Status: "approved", UnitCost: unitCost, ReconciliationStatus: "pending",
	}
	return &SupplierIssueResult{
		Issue: issue, AuthCode: authCode, QRPayload: QRPayloadForIssue(issueID, authCode),
	}, nil
}

func (s *Service) AcceptPartRequestQuote(
	ctx context.Context,
	tenantID, requestID, quoteID, approvedBy, corrID uuid.UUID,
) (*SupplierIssueResult, error) {
	var quote PartRequestQuote
	var status string
	var branchID, repairJobID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT q.id, q.part_request_id, q.supplier_id, q.unit_cost::float8, q.notes, q.status, q.created_at, q.decided_at,
		       pr.status, pr.branch_id, pr.repair_job_id
		FROM inventory.part_request_quotes q
		JOIN inventory.part_requests pr ON pr.id = q.part_request_id AND pr.tenant_id = q.tenant_id
		WHERE q.tenant_id = $1 AND q.part_request_id = $2 AND q.id = $3`,
		tenantID, requestID, quoteID).
		Scan(&quote.ID, &quote.PartRequestID, &quote.SupplierID, &quote.UnitCost, &quote.Notes, &quote.Status,
			&quote.CreatedAt, &quote.DecidedAt, &status, &branchID, &repairJobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("quote not found")
	}
	if err != nil {
		return nil, err
	}
	if quote.Status != "pending" {
		return nil, fmt.Errorf("quote not pending")
	}
	if status != "pending" {
		return nil, fmt.Errorf("part request not pending")
	}

	authCode, err := GenerateAuthCode()
	if err != nil {
		return nil, err
	}
	issueID := uuid.New()
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_request_quotes
		SET status = 'accepted', decided_at = $1
		WHERE id = $2`, now, quoteID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_request_quotes
		SET status = 'superseded', decided_at = $1
		WHERE tenant_id = $2 AND part_request_id = $3 AND id <> $4 AND status = 'pending'`,
		now, tenantID, requestID, quoteID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_requests
		SET status = 'approved', assigned_supplier_id = $1, quote_status = 'accepted', updated_at = now()
		WHERE id = $2`, quote.SupplierID, requestID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_issues
			(id, tenant_id, branch_id, supplier_id, part_request_id, repair_job_id, auth_code, unit_cost,
			 status, requested_by, approved_by, reconciliation_status, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'approved', $9, $9, 'pending', $10)`,
		issueID, tenantID, branchID, quote.SupplierID, requestID, repairJobID, authCode, quote.UnitCost,
		approvedBy, corrID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_credit_entries
			(id, tenant_id, supplier_id, supplier_issue_id, amount, entry_type, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, 'issue', $6, $7)`,
		uuid.New(), tenantID, quote.SupplierID, issueID, quote.UnitCost, approvedBy, corrID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	issue := &SupplierIssue{
		ID: issueID, PartRequestID: requestID, RepairJobID: repairJobID, SupplierID: quote.SupplierID,
		AuthCode: authCode, Status: "approved", UnitCost: quote.UnitCost, ReconciliationStatus: "pending",
	}
	return &SupplierIssueResult{
		Issue: issue, AuthCode: authCode, QRPayload: QRPayloadForIssue(issueID, authCode),
	}, nil
}

func (s *Service) ListSupplierPortalIssues(ctx context.Context, tenantID, supplierID uuid.UUID) ([]SupplierIssue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT si.id, si.part_request_id, si.repair_job_id, COALESCE(rj.job_code, ''), si.supplier_id,
		       si.auth_code, si.status, si.unit_cost::float8, si.collected_at, si.reconciliation_status
		FROM inventory.supplier_issues si
		LEFT JOIN repair.repair_jobs rj ON rj.id = si.repair_job_id
		WHERE si.tenant_id = $1 AND si.supplier_id = $2
		ORDER BY si.created_at DESC
		LIMIT 200`, tenantID, supplierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SupplierIssue
	for rows.Next() {
		var si SupplierIssue
		if err := rows.Scan(
			&si.ID, &si.PartRequestID, &si.RepairJobID, &si.JobCode, &si.SupplierID,
			&si.AuthCode, &si.Status, &si.UnitCost, &si.CollectedAt, &si.ReconciliationStatus,
		); err != nil {
			return nil, err
		}
		items = append(items, si)
	}
	if items == nil {
		items = []SupplierIssue{}
	}
	return items, rows.Err()
}

func (s *Service) GetSupplierPortalCredit(ctx context.Context, tenantID, supplierID uuid.UUID) (*SupplierCreditSummary, error) {
	var name string
	err := s.pool.QueryRow(ctx, `
		SELECT name FROM inventory.suppliers WHERE tenant_id = $1 AND id = $2`, tenantID, supplierID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("supplier not found")
	}
	if err != nil {
		return nil, err
	}
	entries, err := s.ListCreditEntries(ctx, tenantID, supplierID)
	if err != nil {
		return nil, err
	}
	var outstanding float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE
			WHEN entry_type = 'issue' THEN amount
			WHEN entry_type IN ('settlement', 'payment', 'adjustment') THEN -amount
			ELSE 0 END), 0)::float8
		FROM inventory.supplier_credit_entries
		WHERE tenant_id = $1 AND supplier_id = $2`, tenantID, supplierID).Scan(&outstanding)
	if err != nil {
		return nil, err
	}
	return &SupplierCreditSummary{
		SupplierID: supplierID, SupplierName: name,
		OutstandingCredit: outstanding, Entries: entries,
	}, nil
}
