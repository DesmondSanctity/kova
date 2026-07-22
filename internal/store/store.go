// Package store is the Postgres-backed persistence layer (users, sessions, workspaces, API keys, links, usage).
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrExists   = errors.New("account already exists")
	ErrInvalid  = errors.New("invalid email or password")
	ErrNotFound = errors.New("not found")
)

const sessionTTL = 30 * 24 * time.Hour

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatarUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

type Workspace struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	OrgName        string        `json:"orgName"`
	UseCase        string        `json:"useCase"`
	Plan           string        `json:"plan"`
	OwnerID        string        `json:"ownerId"`
	BrandName      string        `json:"brandName"`
	BrandColor     string        `json:"brandColor"`
	BrandTextColor string        `json:"brandTextColor"`
	SupportEmail   string        `json:"supportEmail"`
	LoanProducts   []LoanProduct `json:"loanProducts"`
	MinScore       int           `json:"minScore"`
	// Per-lender Monnify credentials. Secrets are stored encrypted; the API key
	// and secret are never serialized to clients.
	MonnifyBaseURL       string    `json:"monnifyBaseUrl"`
	MonnifyContractCode  string    `json:"monnifyContractCode"`
	MonnifyWalletAccount string    `json:"monnifyWalletAccount"`
	MonnifyAPIKeyEnc     string    `json:"-"`
	MonnifySecretKeyEnc  string    `json:"-"`
	MonnifyConnected     bool      `json:"monnifyConnected"`
	CreatedAt            time.Time `json:"createdAt"`
}

// MonnifyCreds carries a workspace's payment credentials for create/update.
// APIKeyEnc and SecretKeyEnc hold already-encrypted values.
type MonnifyCreds struct {
	BaseURL       string
	APIKeyEnc     string
	SecretKeyEnc  string
	ContractCode  string
	WalletAccount string
}

// LoanProduct is a reusable offer template a lender exposes to borrowers.
type LoanProduct struct {
	MaxAmount    int64   `json:"maxAmount"`
	InterestRate float64 `json:"interestRate"`
	TenorDays    int     `json:"tenorDays"`
}

type Key struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspaceId"`
	Name           string     `json:"name"`
	Publishable    string     `json:"publishable"`
	Secret         string     `json:"secret"`
	AllowedDomains []string   `json:"allowedDomains"`
	AllowedIPs     []string   `json:"allowedIps"`
	Calls          int64      `json:"calls"`
	CreatedAt      time.Time  `json:"createdAt"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
}

type KeyAuth struct {
	WorkspaceID    string
	KeyID          string
	IsSecret       bool
	AllowedDomains []string
	AllowedIPs     []string
}

type Request struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspaceId"`
	Note        string          `json:"note"`
	Status      string          `json:"status"`
	Report      json.RawMessage `json:"report,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`

	// Borrower intake (captured before upload).
	BorrowerName    string `json:"borrowerName"`
	BorrowerEmail   string `json:"borrowerEmail"`
	AmountRequested int64  `json:"amountRequested"`

	// Loan terms the lender sets on the link.
	MaxAmount    int64   `json:"maxAmount"`
	InterestRate float64 `json:"interestRate"`
	TenorDays    int     `json:"tenorDays"`

	// Decision produced after scoring.
	Decision    string `json:"decision"` // approved | counter | declined
	OfferAmount int64  `json:"offerAmount"`

	// Acceptance + disbursement.
	Accepted            bool       `json:"accepted"`
	BorrowerBVN         string     `json:"-"`
	AccountNumber       string     `json:"accountNumber"`
	BankCode            string     `json:"bankCode"`
	AccountName         string     `json:"accountName"`
	Disbursed           bool       `json:"disbursed"`
	DisbursementRef     string     `json:"disbursementRef"`
	DisbursedAt         *time.Time `json:"disbursedAt,omitempty"`
	DisbursementStatus  string     `json:"disbursementStatus"`
	RepaymentTotal      int64      `json:"repaymentTotal"`
	RepaymentDueAt      *time.Time `json:"repaymentDueAt,omitempty"`
	Repaid              bool       `json:"repaid"`
	RepaidAt            *time.Time `json:"repaidAt,omitempty"`
	RepaymentRemindedAt *time.Time `json:"repaymentRemindedAt,omitempty"`
}

const requestCols = `id, COALESCE(workspace_id,''), note, status, report, created_at,
	borrower_name, borrower_email, amount_requested, max_amount, interest_rate, tenor_days,
	decision, offer_amount, accepted, borrower_bvn, account_number, bank_code, account_name,
	disbursed, disbursement_ref, disbursed_at, disbursement_status,
	repayment_total, repayment_due_at, repaid, repaid_at, repayment_reminded_at`

func scanRequest(row pgx.Row) (*Request, error) {
	var r Request
	err := row.Scan(&r.ID, &r.WorkspaceID, &r.Note, &r.Status, &r.Report, &r.CreatedAt,
		&r.BorrowerName, &r.BorrowerEmail, &r.AmountRequested, &r.MaxAmount, &r.InterestRate, &r.TenorDays,
		&r.Decision, &r.OfferAmount, &r.Accepted, &r.BorrowerBVN, &r.AccountNumber, &r.BankCode, &r.AccountName,
		&r.Disbursed, &r.DisbursementRef, &r.DisbursedAt, &r.DisbursementStatus,
		&r.RepaymentTotal, &r.RepaymentDueAt, &r.Repaid, &r.RepaidAt, &r.RepaymentRemindedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// --- users & sessions ---

func (s *Store) Signup(ctx context.Context, email, password, name string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 6 {
		return nil, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &User{ID: id("usr"), Email: email, Name: name, CreatedAt: time.Now().UTC()}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO users(id,email,name,password_hash,created_at) VALUES($1,$2,$3,$4,$5)`,
		u.ID, email, name, hash, u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, ErrExists
		}
		return nil, err
	}
	return u, nil
}

func (s *Store) Login(ctx context.Context, email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	var hash []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id,email,name,avatar_url,created_at,password_hash FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &hash)
	if err != nil || bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		return nil, ErrInvalid
	}
	return &u, nil
}

func (s *Store) UpsertGitHub(ctx context.Context, githubID, email, name, avatar string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id,email,name,avatar_url,created_at FROM users WHERE github_id=$1`, githubID).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	if err == nil {
		return &u, nil
	}
	nu := &User{ID: id("usr"), Email: email, Name: name, AvatarURL: avatar, CreatedAt: time.Now().UTC()}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO users(id,email,name,github_id,avatar_url,created_at) VALUES($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (email) DO UPDATE SET github_id=EXCLUDED.github_id, avatar_url=EXCLUDED.avatar_url`,
		nu.ID, email, name, githubID, avatar, nu.CreatedAt)
	if err != nil {
		return nil, err
	}
	// re-read (email conflict may have kept the original id)
	_ = s.pool.QueryRow(ctx, `SELECT id,email,name,avatar_url,created_at FROM users WHERE email=$1`, email).
		Scan(&nu.ID, &nu.Email, &nu.Name, &nu.AvatarURL, &nu.CreatedAt)
	return nu, nil
}

func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	tok := id("ses")
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions(token,user_id,expires_at) VALUES($1,$2,$3)`,
		tok, userID, time.Now().Add(sessionTTL))
	return tok, err
}

func (s *Store) UserBySession(ctx context.Context, token string) (*User, bool) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT u.id,u.email,u.name,u.avatar_url,u.created_at FROM sessions s
		 JOIN users u ON u.id=s.user_id WHERE s.token=$1 AND s.expires_at>now()`, token).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, false
	}
	return &u, true
}

func (s *Store) DeleteSession(ctx context.Context, token string) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM sessions WHERE token=$1`, token)
}

// UserByID loads a user by id (used to resolve the workspace owner's email).
func (s *Store) UserByID(ctx context.Context, userID string) (*User, bool) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id,email,name,avatar_url,created_at FROM users WHERE id=$1`, userID).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, false
	}
	return &u, true
}

// --- password reset ---

// CreateReset issues a fresh 6-digit OTP for the account, invalidating any
// prior unused codes. Returns ("", false) if no account exists for the email.
func (s *Store) CreateReset(ctx context.Context, email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	var uid string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&uid); err != nil {
		return "", false
	}
	_, _ = s.pool.Exec(ctx, `UPDATE password_resets SET used=true WHERE user_id=$1 AND used=false`, uid)
	code := otpCode()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO password_resets(token,user_id,code,expires_at) VALUES($1,$2,$3,$4)`,
		id("rst"), uid, code, time.Now().Add(15*time.Minute))
	return code, err == nil
}

// ResetPassword verifies the OTP for the given email and sets a new password.
func (s *Store) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	if len(newPassword) < 6 {
		return ErrInvalid
	}
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	var tok, uid string
	err := s.pool.QueryRow(ctx,
		`SELECT pr.token, pr.user_id FROM password_resets pr
		 JOIN users u ON u.id=pr.user_id
		 WHERE u.email=$1 AND pr.code=$2 AND pr.used=false AND pr.expires_at>now()`,
		email, code).Scan(&tok, &uid)
	if err != nil {
		return ErrNotFound
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, hash, uid)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `UPDATE password_resets SET used=true WHERE token=$1`, tok)
	}
	return err
}

// --- workspaces ---

func (s *Store) CreateWorkspace(ctx context.Context, ownerID, name, orgName, useCase string, mc MonnifyCreds) (*Workspace, error) {
	if useCase != "individual" {
		useCase = "fintech"
	}
	ws := &Workspace{ID: id("ws"), Name: name, OrgName: orgName, UseCase: useCase, Plan: "pilot", OwnerID: ownerID,
		MonnifyBaseURL: mc.BaseURL, MonnifyContractCode: mc.ContractCode, MonnifyWalletAccount: mc.WalletAccount,
		MonnifyAPIKeyEnc: mc.APIKeyEnc, MonnifySecretKeyEnc: mc.SecretKeyEnc, MonnifyConnected: mc.SecretKeyEnc != "",
		CreatedAt: time.Now().UTC()}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspaces(id,name,org_name,use_case,plan,owner_id,created_at,
			monnify_base_url,monnify_api_key_enc,monnify_secret_key_enc,monnify_contract_code,monnify_wallet_account)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		ws.ID, ws.Name, ws.OrgName, ws.UseCase, ws.Plan, ws.OwnerID, ws.CreatedAt,
		mc.BaseURL, mc.APIKeyEnc, mc.SecretKeyEnc, mc.ContractCode, mc.WalletAccount); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner')`, ws.ID, ownerID); err != nil {
		return nil, err
	}
	return ws, tx.Commit(ctx)
}

// SetMonnifyCreds updates a workspace's payment credentials (used from Settings).
func (s *Store) SetMonnifyCreds(ctx context.Context, wsID string, mc MonnifyCreds) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE workspaces SET monnify_base_url=$1, monnify_api_key_enc=$2, monnify_secret_key_enc=$3,
			monnify_contract_code=$4, monnify_wallet_account=$5 WHERE id=$6`,
		mc.BaseURL, mc.APIKeyEnc, mc.SecretKeyEnc, mc.ContractCode, mc.WalletAccount, wsID)
	return err == nil && ct.RowsAffected() > 0
}

func (s *Store) WorkspaceForUser(ctx context.Context, userID string) (*Workspace, bool) {
	var w Workspace
	var products []byte
	err := s.pool.QueryRow(ctx,
		`SELECT w.id,w.name,w.org_name,w.use_case,w.plan,w.owner_id,w.brand_name,w.brand_color,w.brand_text_color,w.support_email,w.loan_products,w.min_score,w.monnify_base_url,w.monnify_contract_code,w.monnify_wallet_account,w.monnify_api_key_enc,w.monnify_secret_key_enc,w.created_at
		 FROM workspace_members m JOIN workspaces w ON w.id=m.workspace_id
		 WHERE m.user_id=$1 ORDER BY w.created_at LIMIT 1`, userID).
		Scan(&w.ID, &w.Name, &w.OrgName, &w.UseCase, &w.Plan, &w.OwnerID, &w.BrandName, &w.BrandColor, &w.BrandTextColor, &w.SupportEmail, &products, &w.MinScore, &w.MonnifyBaseURL, &w.MonnifyContractCode, &w.MonnifyWalletAccount, &w.MonnifyAPIKeyEnc, &w.MonnifySecretKeyEnc, &w.CreatedAt)
	if err != nil {
		return nil, false
	}
	w.MonnifyConnected = w.MonnifySecretKeyEnc != ""
	_ = json.Unmarshal(products, &w.LoanProducts)
	return &w, true
}

// WorkspaceByID loads a workspace by its id (used for link-page branding).
func (s *Store) WorkspaceByID(ctx context.Context, wsID string) (*Workspace, bool) {
	var w Workspace
	var products []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id,name,org_name,use_case,plan,owner_id,brand_name,brand_color,brand_text_color,support_email,loan_products,min_score,monnify_base_url,monnify_contract_code,monnify_wallet_account,monnify_api_key_enc,monnify_secret_key_enc,created_at
		 FROM workspaces WHERE id=$1`, wsID).
		Scan(&w.ID, &w.Name, &w.OrgName, &w.UseCase, &w.Plan, &w.OwnerID, &w.BrandName, &w.BrandColor, &w.BrandTextColor, &w.SupportEmail, &products, &w.MinScore, &w.MonnifyBaseURL, &w.MonnifyContractCode, &w.MonnifyWalletAccount, &w.MonnifyAPIKeyEnc, &w.MonnifySecretKeyEnc, &w.CreatedAt)
	if err != nil {
		return nil, false
	}
	w.MonnifyConnected = w.MonnifySecretKeyEnc != ""
	_ = json.Unmarshal(products, &w.LoanProducts)
	return &w, true
}

// UpdateWorkspaceSettings updates the editable org + branding + loan products.
func (s *Store) UpdateWorkspaceSettings(ctx context.Context, wsID, orgName, brandName, brandColor, brandTextColor, supportEmail string, products []LoanProduct, minScore int) (*Workspace, bool) {
	if products == nil {
		products = []LoanProduct{}
	}
	if minScore < 0 {
		minScore = 0
	}
	if minScore > 100 {
		minScore = 100
	}
	pj, _ := json.Marshal(products)
	ct, err := s.pool.Exec(ctx,
		`UPDATE workspaces SET org_name=$1, brand_name=$2, brand_color=$3, brand_text_color=$4, support_email=$5, loan_products=$6, min_score=$7 WHERE id=$8`,
		orgName, brandName, brandColor, brandTextColor, supportEmail, pj, minScore, wsID)
	if err != nil || ct.RowsAffected() == 0 {
		return nil, false
	}
	return s.WorkspaceByID(ctx, wsID)
}

// --- API keys ---

func (s *Store) CreateKey(ctx context.Context, workspaceID, name string) (*Key, error) {
	if name == "" {
		name = "Default"
	}
	k := &Key{ID: id("key"), WorkspaceID: workspaceID, Name: name,
		Publishable: "pk_" + token(18), Secret: "sk_" + token(24),
		AllowedDomains: []string{}, AllowedIPs: []string{}, CreatedAt: time.Now().UTC()}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO api_keys(id,workspace_id,name,publishable,secret,created_at) VALUES($1,$2,$3,$4,$5,$6)`,
		k.ID, workspaceID, name, k.Publishable, k.Secret, k.CreatedAt)
	return k, err
}

func (s *Store) ListKeys(ctx context.Context, workspaceID string) ([]*Key, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,workspace_id,name,publishable,secret,allowed_domains,allowed_ips,calls,created_at,revoked_at
		 FROM api_keys WHERE workspace_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Key
	for rows.Next() {
		var k Key
		if err := rows.Scan(&k.ID, &k.WorkspaceID, &k.Name, &k.Publishable, &k.Secret,
			&k.AllowedDomains, &k.AllowedIPs, &k.Calls, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, &k)
	}
	return out, rows.Err()
}

func (s *Store) RevokeKey(ctx context.Context, workspaceID, keyID string) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at=now() WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, keyID, workspaceID)
	return err == nil && ct.RowsAffected() > 0
}

func (s *Store) UpdateKeyAllowlist(ctx context.Context, workspaceID, keyID string, domains, ips []string) bool {
	if domains == nil {
		domains = []string{}
	}
	if ips == nil {
		ips = []string{}
	}
	ct, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET allowed_domains=$1, allowed_ips=$2 WHERE id=$3 AND workspace_id=$4`,
		domains, ips, keyID, workspaceID)
	return err == nil && ct.RowsAffected() > 0
}

// Authorize validates a key value, records the call, and returns auth context.
func (s *Store) Authorize(ctx context.Context, value string, needSecret bool) (KeyAuth, bool) {
	var a KeyAuth
	err := s.pool.QueryRow(ctx,
		`SELECT id, workspace_id, (secret=$1) AS is_secret, allowed_domains, allowed_ips
		 FROM api_keys WHERE (publishable=$1 OR secret=$1) AND revoked_at IS NULL`, value).
		Scan(&a.KeyID, &a.WorkspaceID, &a.IsSecret, &a.AllowedDomains, &a.AllowedIPs)
	if err != nil {
		return KeyAuth{}, false
	}
	if needSecret && !a.IsSecret {
		return KeyAuth{}, false
	}
	_, _ = s.pool.Exec(ctx, `UPDATE api_keys SET calls=calls+1 WHERE id=$1`, a.KeyID)
	return a, true
}

func (s *Store) AnyKeys(ctx context.Context) bool {
	var n int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE revoked_at IS NULL`).Scan(&n)
	return n > 0
}

// --- lender links (requests) ---

func (s *Store) CreateRequest(ctx context.Context, workspaceID, note string, maxAmount int64, interestRate float64, tenorDays int) (*Request, error) {
	r := &Request{ID: randomID(), WorkspaceID: workspaceID, Note: note, Status: "pending",
		MaxAmount: maxAmount, InterestRate: interestRate, TenorDays: tenorDays, CreatedAt: time.Now().UTC()}
	var wsID any
	if workspaceID != "" {
		wsID = workspaceID
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO requests(id,workspace_id,note,status,max_amount,interest_rate,tenor_days,created_at)
		 VALUES($1,$2,$3,'pending',$4,$5,$6,$7)`,
		r.ID, wsID, note, maxAmount, interestRate, tenorDays, r.CreatedAt)
	return r, err
}

// SetIntake records the borrower details captured before upload.
func (s *Store) SetIntake(ctx context.Context, reqID, name, email string, amount, maxAmount int64, interestRate float64, tenorDays int) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET borrower_name=$1, borrower_email=$2, amount_requested=$3, max_amount=$4, interest_rate=$5, tenor_days=$6
		 WHERE id=$7 AND status='pending'`,
		name, email, amount, maxAmount, interestRate, tenorDays, reqID)
	return err == nil && ct.RowsAffected() > 0
}

func (s *Store) RequestByID(ctx context.Context, reqID string) (*Request, bool) {
	r, err := scanRequest(s.pool.QueryRow(ctx, `SELECT `+requestCols+` FROM requests WHERE id=$1`, reqID))
	if err != nil {
		return nil, false
	}
	return r, true
}

// CompleteRequest stores the scored report plus the lender decision, offer, and
// the resolved repayment tenor (lender/borrower choice or Kova's recommendation).
func (s *Store) CompleteRequest(ctx context.Context, reqID string, report json.RawMessage, decision string, offer int64, tenorDays int) {
	status := "scored"
	if decision == "declined" {
		status = "declined"
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE requests SET status=$1, report=$2, decision=$3, offer_amount=$4, tenor_days=$5 WHERE id=$6`,
		status, report, decision, offer, tenorDays, reqID)
}

// AcceptOffer records the borrower accepting, with verified payout details.
func (s *Store) AcceptOffer(ctx context.Context, reqID, bvn, accountNumber, bankCode, accountName string) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET accepted=true, status='accepted', borrower_bvn=$1, account_number=$2, bank_code=$3, account_name=$4
		 WHERE id=$5 AND decision IN ('approved','counter') AND accepted=false AND disbursed=false AND status<>'rejected'`,
		bvn, accountNumber, bankCode, accountName, reqID)
	return err == nil && ct.RowsAffected() > 0
}

// RejectRequest lets the lender decline a scored/accepted request (before payout).
func (s *Store) RejectRequest(ctx context.Context, reqID string) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET status='rejected', decision='declined' WHERE id=$1 AND disbursed=false AND status<>'rejected'`,
		reqID)
	return err == nil && ct.RowsAffected() > 0
}

// SetDisbursementRef records the reference used for a (possibly pending) payout,
// so a retry can use a fresh reference and the OTP step can find the right one.
func (s *Store) SetDisbursementRef(ctx context.Context, reqID, ref string) {
	_, _ = s.pool.Exec(ctx, `UPDATE requests SET disbursement_ref=$1 WHERE id=$2`, ref, reqID)
}

// SetDisbursementPending records a payout that is awaiting OTP authorization,
// so a repeat disburse click reuses it instead of creating a duplicate transfer.
func (s *Store) SetDisbursementPending(ctx context.Context, reqID, ref string) {
	_, _ = s.pool.Exec(ctx,
		`UPDATE requests SET disbursement_ref=$1, disbursement_status='PENDING_AUTHORIZATION' WHERE id=$2`,
		ref, reqID)
}

// SetRepayment records the amount owed and due date after disbursement.
func (s *Store) SetRepayment(ctx context.Context, reqID string, total int64, dueAt time.Time) {
	_, _ = s.pool.Exec(ctx,
		`UPDATE requests SET repayment_total=$1, repayment_due_at=$2 WHERE id=$3`,
		total, dueAt, reqID)
}

// MarkRepaid marks a disbursed loan as fully repaid (lender-confirmed).
func (s *Store) MarkRepaid(ctx context.Context, workspaceID, reqID string) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET repaid=true, repaid_at=now() WHERE id=$1 AND workspace_id=$2 AND disbursed=true AND repaid=false`,
		reqID, workspaceID)
	return err == nil && ct.RowsAffected() > 0
}

// MarkRepaidByPayment marks a disbursed loan repaid without a workspace session
// (used by the borrower-paid Monnify collection flow).
func (s *Store) MarkRepaidByPayment(ctx context.Context, reqID string) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET repaid=true, repaid_at=now() WHERE id=$1 AND disbursed=true AND repaid=false`,
		reqID)
	return err == nil && ct.RowsAffected() > 0
}

// SetRepaymentReminded records that a repayment reminder email was sent.
func (s *Store) SetRepaymentReminded(ctx context.Context, reqID string) {
	_, _ = s.pool.Exec(ctx, `UPDATE requests SET repayment_reminded_at=now() WHERE id=$1`, reqID)
}

// DueRepayments returns disbursed, unpaid loans whose due date has arrived and
// which have not yet been reminded — the scheduler's work list.
func (s *Store) DueRepayments(ctx context.Context) ([]*Request, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+requestCols+` FROM requests
		WHERE disbursed=true AND repaid=false AND repayment_due_at IS NOT NULL
		  AND repayment_due_at <= now() AND repayment_reminded_at IS NULL AND borrower_email <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkDisbursed records a completed payout.
func (s *Store) MarkDisbursed(ctx context.Context, reqID, ref string) bool {
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET disbursed=true, status='disbursed', disbursement_ref=$1, disbursed_at=now() WHERE id=$2 AND accepted=true AND disbursed=false`,
		ref, reqID)
	return err == nil && ct.RowsAffected() > 0
}

// SetDisbursementStatus applies a Monnify webhook result to a payout by reference.
// FAILED/REVERSED reverts the loan to 'accepted' so the lender can retry.
func (s *Store) SetDisbursementStatus(ctx context.Context, ref, status string) bool {
	failed := status == "FAILED" || status == "REVERSED"
	ct, err := s.pool.Exec(ctx,
		`UPDATE requests SET disbursement_status=$1,
		   disbursed = CASE WHEN $2 THEN false ELSE disbursed END,
		   status    = CASE WHEN $2 THEN 'accepted' ELSE status END
		 WHERE disbursement_ref=$3`,
		status, failed, ref)
	return err == nil && ct.RowsAffected() > 0
}

func (s *Store) ListRequests(ctx context.Context, workspaceID string) ([]*Request, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+requestCols+` FROM requests WHERE workspace_id=$1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- usage ---

func (s *Store) RecordUsage(ctx context.Context, workspaceID, keyID, kind string) {
	if workspaceID == "" {
		return
	}
	var kid any
	if keyID != "" {
		kid = keyID
	}
	_, _ = s.pool.Exec(ctx, `INSERT INTO usage_events(workspace_id,key_id,kind) VALUES($1,$2,$3)`, workspaceID, kid, kind)
}

type UsageSummary struct {
	Total  int            `json:"total"`
	Last30 int            `json:"last30"`
	ByKind map[string]int `json:"byKind"`
	Daily  []DailyCount   `json:"daily"`
}

type DailyCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

func (s *Store) UsageSummary(ctx context.Context, workspaceID string) UsageSummary {
	sum := UsageSummary{ByKind: map[string]int{}}
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM usage_events WHERE workspace_id=$1`, workspaceID).Scan(&sum.Total)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM usage_events WHERE workspace_id=$1 AND created_at > now()-interval '30 days'`, workspaceID).Scan(&sum.Last30)

	if rows, err := s.pool.Query(ctx, `SELECT kind, count(*) FROM usage_events WHERE workspace_id=$1 GROUP BY kind`, workspaceID); err == nil {
		defer rows.Close()
		for rows.Next() {
			var k string
			var n int
			if rows.Scan(&k, &n) == nil {
				sum.ByKind[k] = n
			}
		}
	}
	if rows, err := s.pool.Query(ctx,
		`SELECT to_char(date_trunc('day',created_at),'YYYY-MM-DD') d, count(*)
		 FROM usage_events WHERE workspace_id=$1 AND created_at > now()-interval '365 days'
		 GROUP BY d ORDER BY d`, workspaceID); err == nil {
		defer rows.Close()
		for rows.Next() {
			var dc DailyCount
			if rows.Scan(&dc.Day, &dc.Count) == nil {
				sum.Daily = append(sum.Daily, dc)
			}
		}
	}
	return sum
}

// --- audit log ---

type AuditEvent struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

// RecordAudit appends an immutable audit entry. Best-effort (never blocks the caller).
func (s *Store) RecordAudit(ctx context.Context, workspaceID, actor, action, target, detail string) {
	if workspaceID == "" {
		return
	}
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO audit_events(id,workspace_id,actor,action,target,detail) VALUES($1,$2,$3,$4,$5,$6)`,
		id("aud"), workspaceID, actor, action, target, detail)
}

// ListAudit returns the most recent audit events for a workspace.
func (s *Store) ListAudit(ctx context.Context, workspaceID string, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,actor,action,target,detail,created_at FROM audit_events
		 WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Target, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

var _ = pgx.ErrNoRows

func id(prefix string) string { return prefix + "_" + token(12) }

func token(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// otpCode returns a random 6-digit numeric code, zero-padded.
func otpCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func randomID() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
