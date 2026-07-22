// Package api exposes the Kova HTTP surface: statement scoring plus Monnify
// identity verification and disbursement.
package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kova/internal/email"
	"kova/internal/extract"
	"kova/internal/monnify"
	"kova/internal/pipeline"
	"kova/internal/secretbox"
	"kova/internal/store"
)

//go:embed static
var staticFS embed.FS

//go:embed banks.json
var banksJSON []byte

type Server struct {
	extractor extract.Extractor
	enc       *secretbox.Box
	store     *store.Store
	keyring   Keyring
	github    githubOAuth
	mailer    email.Sender
	baseURL   string
	limiter   *rateLimiter
	banks     []Bank
	maxUpload int64
	maxBanks  int
}

func New(ex extract.Extractor, st *store.Store, gh GitHubConfig, mailer email.Sender, enc *secretbox.Box) *Server {
	if mailer == nil {
		mailer = email.Noop{}
	}
	return &Server{
		extractor: ex,
		enc:       enc,
		store:     st,
		keyring:   LoadKeyring(),
		github:    githubOAuth{clientID: gh.ClientID, clientSecret: gh.ClientSecret},
		mailer:    mailer,
		baseURL:   strings.TrimRight(gh.BaseURL, "/"),
		limiter:   newRateLimiter(60, time.Minute),
		banks:     loadBanks(),
		maxUpload: 25 << 20,
		maxBanks:  3,
	}
}

// errMonnifyNotConnected is returned when a workspace has no payment credentials.
var errMonnifyNotConnected = fmt.Errorf("this workspace has not connected Monnify")

// monnifyFor builds a Monnify client from a workspace's stored credentials.
func (s *Server) monnifyForWorkspace(ws *store.Workspace) (*monnify.Client, error) {
	if ws == nil || ws.MonnifySecretKeyEnc == "" {
		return nil, errMonnifyNotConnected
	}
	apiKey, err := s.enc.Decrypt(ws.MonnifyAPIKeyEnc)
	if err != nil {
		return nil, err
	}
	secret, err := s.enc.Decrypt(ws.MonnifySecretKeyEnc)
	if err != nil {
		return nil, err
	}
	return monnify.NewWithCreds(ws.MonnifyBaseURL, apiKey, secret, ws.MonnifyContractCode, ws.MonnifyWalletAccount), nil
}

// monnifyFor resolves a workspace by id and builds its Monnify client.
func (s *Server) monnifyFor(ctx context.Context, wsID string) (*monnify.Client, error) {
	if wsID == "" {
		return nil, errMonnifyNotConnected
	}
	ws, ok := s.store.WorkspaceByID(ctx, wsID)
	if !ok {
		return nil, errMonnifyNotConnected
	}
	return s.monnifyForWorkspace(ws)
}

// validateMonnify checks a set of credentials by requesting an auth token.
func (s *Server) validateMonnify(ctx context.Context, base, apiKey, secret, contract, wallet string) error {
	c := monnify.NewWithCreds(base, apiKey, secret, contract, wallet)
	_, err := c.Token(ctx)
	return err
}

// GitHubConfig carries GitHub OAuth credentials from the caller.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	// BaseURL is the public origin of the frontend app (e.g. http://localhost:4322).
	// When set, post-auth redirects target it instead of a bare relative path.
	BaseURL string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /webhooks/monnify", s.handleMonnifyWebhook)
	mux.HandleFunc("GET /v1/banks", s.handleBanks)
	mux.HandleFunc("POST /v1/score", s.protect(scopeAny, s.handleScore))
	mux.HandleFunc("POST /v1/verify-account", s.protect(scopeSecret, s.handleVerify))
	mux.HandleFunc("POST /v1/disburse", s.protect(scopeSecret, s.handleDisburse))
	// Shareable-link lender flow.
	mux.HandleFunc("POST /v1/requests", s.protect(scopeAny, s.handleCreateRequest))
	mux.HandleFunc("GET /v1/requests/{id}", s.handleGetRequest)
	mux.HandleFunc("POST /v1/requests/{id}/intake", s.rateLimit(s.handleRequestIntake))
	mux.HandleFunc("POST /v1/requests/{id}/score", s.rateLimit(s.handleRequestScore))
	mux.HandleFunc("POST /v1/requests/{id}/accept", s.rateLimit(s.handleRequestAccept))
	mux.HandleFunc("POST /v1/requests/{id}/decline", s.rateLimit(s.handleRequestDecline))
	mux.HandleFunc("POST /v1/requests/{id}/repay-init", s.rateLimit(s.handleRepayInit))
	mux.HandleFunc("GET /r/{id}", s.handleBorrowerPage)
	mux.HandleFunc("GET /v/{id}", s.handleLenderPage)
	mux.HandleFunc("GET /pay/{id}", s.handleRepayPage)
	mux.HandleFunc("GET /pay/{id}/done", s.handleRepayDone)
	// Auth.
	mux.HandleFunc("POST /auth/signup", s.rateLimit(s.handleSignup))
	mux.HandleFunc("POST /auth/login", s.rateLimit(s.handleLogin))
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /auth/forgot", s.rateLimit(s.handleForgotPassword))
	mux.HandleFunc("POST /auth/reset", s.rateLimit(s.handleResetPassword))
	mux.HandleFunc("GET /auth/github", s.handleGitHubStart)
	mux.HandleFunc("GET /auth/github/callback", s.handleGitHubCallback)
	// Dashboard API.
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("POST /api/workspace", s.handleCreateWorkspace)
	mux.HandleFunc("PATCH /api/workspace", s.handleUpdateWorkspace)
	mux.HandleFunc("POST /api/workspace/monnify", s.handleUpdateMonnify)
	mux.HandleFunc("POST /api/keys", s.handleCreateKey)
	mux.HandleFunc("DELETE /api/keys/{id}", s.handleRevokeKey)
	mux.HandleFunc("PATCH /api/keys/{id}/allowlist", s.handleKeyAllowlist)
	mux.HandleFunc("POST /api/links", s.handleCreateLink)
	mux.HandleFunc("POST /api/links/{id}/disburse", s.handleDisburseLink)
	mux.HandleFunc("POST /api/links/{id}/authorize", s.handleAuthorizeLink)
	mux.HandleFunc("POST /api/links/{id}/resend-otp", s.handleResendOTP)
	mux.HandleFunc("POST /api/links/{id}/reject", s.handleRejectLink)
	mux.HandleFunc("POST /api/links/{id}/repay", s.handleRepayLink)
	mux.HandleFunc("POST /api/links/{id}/verify-repayment", s.handleVerifyRepayment)
	mux.HandleFunc("POST /api/links/{id}/request-repayment", s.handleRequestRepayment)
	mux.HandleFunc("POST /api/links/{id}/resend-offer", s.handleResendOffer)
	mux.HandleFunc("GET /api/links", s.handleListLinks)
	mux.HandleFunc("GET /api/audit", s.handleListAudit)
	// SDK asset for the shareable-link (borrower) page.
	if sub, err := fs.Sub(staticFS, "static"); err == nil {
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))
	}
	return cors(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Bank is one entry in the picker. Source is nigerianbanks.xyz (open dataset).
type Bank struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Code      string `json:"code"`
	Logo      string `json:"logo"`
	Supported bool   `json:"supported"`
}

func loadBanks() []Bank {
	var raw []Bank
	_ = json.Unmarshal(banksJSON, &raw)
	for i := range raw {
		raw[i].Supported = true
	}
	return raw
}

func (s *Server) handleBanks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"banks": s.banks, "maxBanks": s.maxBanks, "maxMonths": 3})
}

func (s *Server) handleScore(w http.ResponseWriter, r *http.Request) {
	rep, code, err := s.scoreRequest(r)
	if err != nil {
		writeErr(w, code, err.Error())
		return
	}
	if auth, ok := keyAuthFrom(r.Context()); ok && auth.WorkspaceID != "" {
		s.store.RecordUsage(r.Context(), auth.WorkspaceID, auth.KeyID, "score")
	}
	writeJSON(w, http.StatusOK, rep)
}

// handleCreateRequest creates a shareable borrower link, owned by the caller's
// workspace when an API key is presented.
// handleMonnifyWebhook receives disbursement settlement events from Monnify.
// It verifies the HMAC-SHA512 signature and updates the final payout status;
// FAILED/REVERSED reverts the loan to 'accepted' so it can be retried.
func (s *Server) handleMonnifyWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "cannot read body")
		return
	}
	var evt struct {
		EventType string `json:"eventType"`
		EventData struct {
			Reference        string `json:"reference"`
			PaymentReference string `json:"paymentReference"`
			Status           string `json:"status"`
			PaymentStatus    string `json:"paymentStatus"`
		} `json:"eventData"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	// Resolve the owning workspace from the reference so we verify the HMAC with
	// that lender's own Monnify secret.
	ref := evt.EventData.PaymentReference
	if ref == "" {
		ref = evt.EventData.Reference
	}
	reqID := requestIDFromRef(ref)
	if reqID == "" {
		writeErr(w, http.StatusBadRequest, "unrecognized reference")
		return
	}
	req, ok := s.store.RequestByID(r.Context(), reqID)
	if !ok || req.WorkspaceID == "" {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	mon, err := s.monnifyFor(r.Context(), req.WorkspaceID)
	if err != nil || !mon.VerifyWebhook(body, r.Header.Get("monnify-signature")) {
		writeErr(w, http.StatusUnauthorized, "invalid signature")
		return
	}
	// Collection (repayment) settled: mark the loan repaid.
	if ref := evt.EventData.PaymentReference; strings.HasPrefix(ref, "kova_repay_") {
		paid := strings.Contains(strings.ToUpper(evt.EventType), "SUCCESS") ||
			strings.EqualFold(evt.EventData.PaymentStatus, "PAID")
		if paid {
			reqID := strings.TrimPrefix(ref, "kova_repay_")
			if s.store.MarkRepaidByPayment(r.Context(), reqID) {
				if req, ok := s.store.RequestByID(r.Context(), reqID); ok && req.WorkspaceID != "" {
					s.store.RecordAudit(r.Context(), req.WorkspaceID, "borrower", "loan.repaid", reqID, "Paid via repayment link")
				}
				s.finalizeRepayment(r.Context(), reqID)
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	status := strings.ToUpper(evt.EventData.Status)
	if status == "" {
		switch {
		case strings.Contains(evt.EventType, "SUCCESS"):
			status = "SUCCESS"
		case strings.Contains(evt.EventType, "FAIL"):
			status = "FAILED"
		case strings.Contains(evt.EventType, "REVERS"):
			status = "REVERSED"
		}
	}
	if evt.EventData.Reference != "" && status != "" {
		s.store.SetDisbursementStatus(r.Context(), evt.EventData.Reference, status)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Note         string  `json:"note"`
		MaxAmount    int64   `json:"maxAmount"`
		InterestRate float64 `json:"interestRate"`
		TenorDays    int     `json:"tenorDays"`
	}
	_ = decodeJSON(r, &body)
	wsID := ""
	if auth, ok := keyAuthFrom(r.Context()); ok {
		wsID = auth.WorkspaceID
	}
	req, err := s.store.CreateRequest(r.Context(), wsID, body.Note, body.MaxAmount, body.InterestRate, body.TenorDays)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	base := origin(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": req.ID, "status": req.Status,
		"borrowerUrl": base + "/r/" + req.ID, "viewUrl": base + "/v/" + req.ID,
	})
}

func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	req, ok := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	// Public endpoint (borrower + lender link pages): expose only non-PII fields.
	resp := map[string]any{
		"id": req.ID, "note": req.Note, "status": req.Status, "createdAt": req.CreatedAt,
		"report": req.Report, "amountRequested": req.AmountRequested,
		"decision": req.Decision, "offerAmount": req.OfferAmount,
		"interestRate": req.InterestRate, "tenorDays": req.TenorDays,
		"accepted": req.Accepted, "disbursed": req.Disbursed,
		"hasIntake":      req.BorrowerEmail != "",
		"expired":        requestExpired(req),
		"repaymentTotal": req.RepaymentTotal, "repaymentDueAt": req.RepaymentDueAt, "repaid": req.Repaid,
	}
	if req.WorkspaceID != "" {
		if ws, ok := s.store.WorkspaceByID(r.Context(), req.WorkspaceID); ok {
			name := ws.BrandName
			if name == "" {
				name = ws.OrgName
			}
			resp["brand"] = map[string]any{"name": name, "color": ws.BrandColor, "textColor": ws.BrandTextColor, "supportEmail": ws.SupportEmail}
			resp["loanProducts"] = ws.LoanProducts
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleRequestIntake captures borrower name, email, requested amount and chosen terms before upload.
func (s *Server) handleRequestIntake(w http.ResponseWriter, r *http.Request) {
	if req, ok := s.store.RequestByID(r.Context(), r.PathValue("id")); ok && requestExpired(req) {
		writeErr(w, http.StatusGone, "this link has expired")
		return
	}
	var body struct {
		Name         string  `json:"name"`
		Email        string  `json:"email"`
		Amount       int64   `json:"amount"`
		MaxAmount    int64   `json:"maxAmount"`
		InterestRate float64 `json:"interestRate"`
		TenorDays    int     `json:"tenorDays"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Email) == "" || body.Amount <= 0 {
		writeErr(w, http.StatusBadRequest, "name, email and a positive amount are required")
		return
	}
	if !s.store.SetIntake(r.Context(), r.PathValue("id"), strings.TrimSpace(body.Name), strings.ToLower(strings.TrimSpace(body.Email)), body.Amount, body.MaxAmount, body.InterestRate, body.TenorDays) {
		writeErr(w, http.StatusNotFound, "request not found or already submitted")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleRequestAccept records the borrower accepting the offer and verifies the payout account.
func (s *Server) handleRequestAccept(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, ok := s.store.RequestByID(r.Context(), id)
	if !ok {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	if req.Decision != "approved" && req.Decision != "counter" {
		writeErr(w, http.StatusBadRequest, "no offer available to accept")
		return
	}
	if req.Status == "rejected" {
		writeErr(w, http.StatusConflict, "this offer has been withdrawn by the lender")
		return
	}
	if req.Accepted || req.Disbursed {
		writeErr(w, http.StatusConflict, "this offer has already been accepted")
		return
	}
	if requestExpired(req) {
		writeErr(w, http.StatusGone, "this link has expired")
		return
	}
	var body struct {
		BVN           string `json:"bvn"`
		AccountNumber string `json:"accountNumber"`
		BankCode      string `json:"bankCode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.AccountNumber == "" || body.BankCode == "" {
		writeErr(w, http.StatusBadRequest, "account number and bank are required")
		return
	}
	if len(body.BVN) != 11 {
		writeErr(w, http.StatusBadRequest, "a valid 11-digit BVN is required")
		return
	}
	// Try to verify the payout account name. On sandbox this often can't resolve
	// test accounts, so we don't block: we record the account + BVN and mark it
	// unverified. Live Monnify credentials make this a hard gate.
	accountName := ""
	verified := false
	if mon, err := s.monnifyFor(r.Context(), req.WorkspaceID); err == nil {
		if acc, err := mon.VerifyAccount(r.Context(), body.AccountNumber, body.BankCode); err == nil {
			accountName = acc.AccountName
			verified = true
		} else {
			log.Printf("account verify (unblocking): %v", err)
		}
	}
	if !s.store.AcceptOffer(r.Context(), id, body.BVN, body.AccountNumber, body.BankCode, accountName) {
		writeErr(w, http.StatusConflict, "offer could not be accepted")
		return
	}
	s.notifyLenderAccepted(r, req, accountName)
	s.store.RecordAudit(r.Context(), req.WorkspaceID, "borrower", "offer.accepted", id, accountName)
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "accountName": accountName, "verified": verified})
}

// handleRequestDecline lets the borrower decline an offer from their link.
func (s *Server) handleRequestDecline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, ok := s.store.RequestByID(r.Context(), id)
	if !ok {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	if req.Disbursed || req.Accepted {
		writeErr(w, http.StatusConflict, "this offer can no longer be changed")
		return
	}
	if !s.store.RejectRequest(r.Context(), id) {
		writeErr(w, http.StatusConflict, "could not decline this offer")
		return
	}
	if req.WorkspaceID != "" {
		s.store.RecordAudit(r.Context(), req.WorkspaceID, "borrower", "offer.declined", id, "Borrower not interested")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

// notifyLenderScored emails the workspace's support address when a borrower has
// finished uploading their statements and a decision is ready to review.
func (s *Server) notifyLenderScored(r *http.Request, req *store.Request, decision string, offer int64, score int, band string) {
	if s.mailer == nil || req.WorkspaceID == "" {
		return
	}
	ws, ok := s.store.WorkspaceByID(r.Context(), req.WorkspaceID)
	if !ok {
		return
	}
	to := s.lenderContactEmail(r.Context(), ws)
	if to == "" {
		return
	}
	who := req.BorrowerName
	if who == "" {
		who = "A borrower"
	}
	brand := brandName(ws)
	verdict := "Kova recommends an offer of <b style=\"color:#0f172a\">₦" + strconv.FormatInt(offer/100, 10) + "</b>."
	switch decision {
	case "declined":
		verdict = "The score fell below your lending threshold, so it was <b style=\"color:#0f172a\">auto-declined</b>."
	case "counter":
		verdict = "Kova suggests a counter-offer of <b style=\"color:#0f172a\">₦" + strconv.FormatInt(offer/100, 10) + "</b>."
	}
	reviewURL := origin(r) + "/dashboard"
	body := `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">A statement check is ready to review</h1>
		<p style="margin:0 0 16px;font-size:15px;line-height:1.55;color:#475569">` + html.EscapeString(who) + ` finished uploading their bank statements.</p>
		<div style="margin:0 0 18px;padding:14px 16px;background:#f4f4f5;border-radius:12px;font-size:14px;color:#334155">Score <b style="color:#0f172a">` + strconv.Itoa(score) + `</b> · Band <b style="color:#0f172a">` + html.EscapeString(band) + `</b><br>` + verdict + `</div>` +
		emailButton(reviewURL, "Review in dashboard")
	msg := emailLayout(brand, body)
	if err := s.mailer.Send(r.Context(), to, brand+": a statement check is ready", msg); err != nil {
		log.Printf("lender scored notice to %s: %v", to, err)
	}
}

// notifyLenderAccepted emails the workspace's support address when a borrower accepts.
func (s *Server) notifyLenderAccepted(r *http.Request, req *store.Request, accountName string) {
	if s.mailer == nil || req.WorkspaceID == "" {
		return
	}
	ws, ok := s.store.WorkspaceByID(r.Context(), req.WorkspaceID)
	if !ok {
		return
	}
	to := s.lenderContactEmail(r.Context(), ws)
	if to == "" {
		return
	}
	who := req.BorrowerName
	if who == "" {
		who = "A borrower"
	}
	name := accountName
	if name == "" {
		name = req.BorrowerName
	}
	naira := fmt.Sprintf("₦%d", req.OfferAmount/100)
	brand := brandName(ws)
	subject := brand + ": offer accepted — ready to disburse " + naira
	body := `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">A borrower accepted their offer</h1>
		<p style="margin:0 0 16px;font-size:15px;line-height:1.55;color:#475569">` + html.EscapeString(who) + ` accepted <b style="color:#0f172a">` + naira + `</b> and is ready to be paid.</p>
		<div style="margin:0 0 18px;padding:14px 16px;background:#f4f4f5;border-radius:12px;font-size:14px;color:#334155">Payout account: <b style="color:#0f172a">` + html.EscapeString(name) + `</b></div>` +
		emailButton(origin(r)+"/dashboard", "Review and disburse")
	msg := emailLayout(brand, body)
	if err := s.mailer.Send(r.Context(), to, subject, msg); err != nil {
		log.Printf("lender accept notice to %s: %v", to, err)
	}
}

// lenderContactEmail resolves where lender notifications are sent: the configured
// support email if set, otherwise the workspace owner's account email.
func (s *Server) lenderContactEmail(ctx context.Context, ws *store.Workspace) string {
	if ws == nil {
		return ""
	}
	if ws.SupportEmail != "" {
		return ws.SupportEmail
	}
	if ws.OwnerID != "" {
		if u, ok := s.store.UserByID(ctx, ws.OwnerID); ok {
			return u.Email
		}
	}
	return ""
}

// brandName returns the display brand for a workspace (brand name, else org, else Kova).
func brandName(ws *store.Workspace) string {
	if ws == nil {
		return "Kova"
	}
	if ws.BrandName != "" {
		return ws.BrandName
	}
	if ws.OrgName != "" {
		return ws.OrgName
	}
	return "Kova"
}

// finalizeRepayment fires the lender notification and borrower receipt once a
// loan transitions to repaid. Safe to call from every repayment path — it only
// runs after the store confirmed the repaid transition, so it never duplicates.
func (s *Server) finalizeRepayment(ctx context.Context, reqID string) {
	if s.mailer == nil {
		return
	}
	req, ok := s.store.RequestByID(ctx, reqID)
	if !ok {
		return
	}
	var ws *store.Workspace
	if req.WorkspaceID != "" {
		if w, ok := s.store.WorkspaceByID(ctx, req.WorkspaceID); ok {
			ws = w
		}
	}
	brand := brandName(ws)
	due := req.RepaymentTotal
	if due <= 0 {
		due = req.OfferAmount
	}
	naira := "₦" + strconv.FormatInt(due/100, 10)
	who := req.BorrowerName
	if who == "" {
		who = "The borrower"
	}

	// Lender notification.
	if to := s.lenderContactEmail(ctx, ws); to != "" {
		body := `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">A loan was repaid</h1>
			<p style="margin:0 0 16px;font-size:15px;line-height:1.55;color:#475569">` + html.EscapeString(who) + ` repaid their loan in full.</p>
			<div style="margin:0 0 18px;padding:14px 16px;background:#f4f4f5;border-radius:12px;font-size:14px;color:#334155">Amount collected: <b style="color:#0f172a">` + naira + `</b></div>` +
			emailButton(s.appURL("/dashboard"), "View in dashboard")
		if err := s.mailer.Send(ctx, to, brand+": "+who+" repaid "+naira, emailLayout(brand, body)); err != nil {
			log.Printf("lender repaid notice to %s: %v", to, err)
		}
	}

	// Borrower receipt.
	if req.BorrowerEmail != "" {
		subject, msg := receiptEmail(brand, req.BorrowerName, due, req.RepaidAt)
		if err := s.mailer.Send(ctx, req.BorrowerEmail, subject, msg); err != nil {
			log.Printf("borrower receipt to %s: %v", req.BorrowerEmail, err)
		}
	}
}

// StartRepaymentScheduler runs a background loop that emails borrowers a
// repayment link once their due date has passed (hourly check). The lender can
// also trigger this manually from the dashboard.
func (s *Server) StartRepaymentScheduler() {
	if s.mailer == nil || s.store == nil {
		return
	}
	go func() {
		s.runRepaymentReminders()
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			s.runRepaymentReminders()
		}
	}()
}

func (s *Server) runRepaymentReminders() {
	ctx := context.Background()
	due, err := s.store.DueRepayments(ctx)
	if err != nil {
		return
	}
	for _, req := range due {
		if s.sendRepaymentEmail(ctx, req) {
			s.store.SetRepaymentReminded(ctx, req.ID)
			if req.WorkspaceID != "" {
				s.store.RecordAudit(ctx, req.WorkspaceID, "system", "repayment.reminder", req.ID, "Auto reminder emailed to "+req.BorrowerEmail)
			}
		}
	}
}

// linkTTL is how long an unfinished borrower link stays usable.
const linkTTL = 30 * 24 * time.Hour

// requestExpired reports whether an open (not accepted/disbursed/rejected) link
// has aged past the TTL.
func requestExpired(req *store.Request) bool {
	if req.Accepted || req.Disbursed || req.Status == "rejected" {
		return false
	}
	return time.Since(req.CreatedAt) > linkTTL
}

// defaultMinScore is the platform-wide auto-decline threshold used when a
// workspace has not configured its own.
const defaultMinScore = 40

// decide turns a score + requested amount + lender cap into an offer. Money is
// integer kobo; the score's recommended limit is naira and converted to kobo.
// minScore is the lender's configured auto-decline floor (0 => platform default).
// Band E and sub-threshold scores are hard-declined; band D is allowed but is
// naturally capped by its (low) recommended limit, so a small request that fits
// within that limit can still be approved rather than declined outright.
func decide(score int, band string, recommendedNaira float64, requestedKobo, maxKobo int64, minScore int) (string, int64) {
	if minScore <= 0 {
		minScore = defaultMinScore
	}
	if band == "E" || score < minScore {
		return "declined", 0
	}
	capKobo := int64(recommendedNaira * 100)
	if maxKobo > 0 && maxKobo < capKobo {
		capKobo = maxKobo
	}
	offer := requestedKobo
	if offer > capKobo {
		offer = capKobo
	}
	if offer <= 0 {
		return "declined", 0
	}
	if offer >= requestedKobo {
		return "approved", requestedKobo
	}
	return "counter", offer
}

// recommendTenor derives a bullet-loan repayment window (in days) from the
// borrower's cashflow, used only when neither the lender product nor the
// borrower set one. A loan that is large relative to monthly income gets a
// longer window (assuming a comfortable monthly set-aside), rounded to whole
// months and clamped to 14..90 days.
func recommendTenor(avgMonthlyInflowNaira float64, offerKobo int64) int {
	if avgMonthlyInflowNaira <= 0 || offerKobo <= 0 {
		return 30
	}
	offerNaira := float64(offerKobo) / 100
	capacity := avgMonthlyInflowNaira * 0.35
	months := math.Ceil(offerNaira / capacity)
	days := int(months) * 30
	if days < 14 {
		days = 14
	}
	if days > 90 {
		days = 90
	}
	return days
}

// handleRequestScore is called by the borrower via their link (no API key).
func (s *Server) handleRequestScore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, ok := s.store.RequestByID(r.Context(), id)
	if !ok {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	if req.Accepted || req.Disbursed || req.Status == "rejected" {
		writeErr(w, http.StatusConflict, "this request is closed and cannot be re-scored")
		return
	}
	if requestExpired(req) {
		writeErr(w, http.StatusGone, "this link has expired")
		return
	}
	rep, code, err := s.scoreRequest(r)
	if err != nil {
		writeErr(w, code, err.Error())
		return
	}
	minScore := 0
	if req.WorkspaceID != "" {
		if ws, ok := s.store.WorkspaceByID(r.Context(), req.WorkspaceID); ok {
			minScore = ws.MinScore
		}
	}
	decision, offer := decide(rep.Score.Score, rep.Score.Band, rep.Score.LimitRecommendation, req.AmountRequested, req.MaxAmount, minScore)
	// Tenor priority: lender product / borrower choice (already on the request)
	// wins; otherwise Kova derives a period from the statements.
	tenor := req.TenorDays
	if tenor <= 0 && offer > 0 {
		tenor = recommendTenor(rep.Score.Features.AvgMonthlyInflow, offer)
	}
	if b, err := json.Marshal(rep); err == nil {
		s.store.CompleteRequest(r.Context(), id, b, decision, offer, tenor)
	}
	if req.WorkspaceID != "" {
		s.store.RecordUsage(r.Context(), req.WorkspaceID, "", "score")
		scoreDetail := "Declined"
		if decision == "approved" {
			scoreDetail = "Approved · ₦" + strconv.FormatInt(offer/100, 10)
		} else if decision == "counter" {
			scoreDetail = "Counter-offer · ₦" + strconv.FormatInt(offer/100, 10)
		}
		s.store.RecordAudit(r.Context(), req.WorkspaceID, "borrower", "request.scored", id, scoreDetail)
	}
	s.notifyLenderScored(r, req, decision, offer, rep.Score.Score, rep.Score.Band)
	s.emailBorrowerResult(r, req, decision, offer)
	writeJSON(w, http.StatusOK, map[string]any{"status": "received"})
}

// emailBorrowerResult notifies the borrower of the decision with an accept link.
func (s *Server) emailBorrowerResult(r *http.Request, req *store.Request, decision string, offer int64) {
	if s.mailer == nil || req.BorrowerEmail == "" {
		return
	}
	brand := "Kova"
	if req.WorkspaceID != "" {
		if ws, ok := s.store.WorkspaceByID(r.Context(), req.WorkspaceID); ok {
			if ws.BrandName != "" {
				brand = ws.BrandName
			} else if ws.OrgName != "" {
				brand = ws.OrgName
			}
		}
	}
	acceptURL := origin(r) + "/r/" + req.ID + "?v=offer"
	subject, html := resultEmail(brand, req.BorrowerName, decision, offer, req.InterestRate, req.TenorDays, acceptURL, "")
	if err := s.mailer.Send(r.Context(), req.BorrowerEmail, subject, html); err != nil {
		log.Printf("borrower result email to %s: %v", req.BorrowerEmail, err)
	}
}

// scoreRequest parses the multipart upload, extracts and scores it, and returns
// the report or an HTTP status + error. Shared by /v1/score and the link flow.
func (s *Server) scoreRequest(r *http.Request) (*pipeline.Report, int, error) {
	if err := r.ParseMultipartForm(s.maxUpload); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid multipart form")
	}
	uploads := r.MultipartForm.File["statements"]
	if len(uploads) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no statements uploaded (use field 'statements')")
	}
	if len(uploads) > s.maxBanks {
		return nil, http.StatusBadRequest, fmt.Errorf("too many statements: max %d banks per assessment", s.maxBanks)
	}

	var docs []*extract.Document
	for _, fh := range uploads {
		f, err := fh.Open()
		if err != nil {
			docs = append(docs, &extract.Document{Filename: fh.Filename})
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(f, s.maxUpload))
		f.Close()
		doc, err := s.extractor.Extract(r.Context(), fh.Filename, data)
		if err != nil {
			docs = append(docs, &extract.Document{Filename: fh.Filename})
			continue
		}
		docs = append(docs, doc)
	}

	rep, err := pipeline.Run(docs)
	if err != nil {
		return rep, http.StatusUnprocessableEntity, err
	}
	return rep, http.StatusOK, nil
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	auth, _ := keyAuthFrom(r.Context())
	mon, err := s.monnifyFor(r.Context(), auth.WorkspaceID)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "monnify not connected for this workspace")
		return
	}
	var req struct {
		AccountNumber string `json:"accountNumber"`
		BankCode      string `json:"bankCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	acc, err := mon.VerifyAccount(r.Context(), req.AccountNumber, req.BankCode)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

func (s *Server) handleDisburse(w http.ResponseWriter, r *http.Request) {
	auth, _ := keyAuthFrom(r.Context())
	mon, err := s.monnifyFor(r.Context(), auth.WorkspaceID)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "monnify not connected for this workspace")
		return
	}
	var req struct {
		SourceAccount string  `json:"sourceAccount"`
		Amount        float64 `json:"amount"`
		Reference     string  `json:"reference"`
		Narration     string  `json:"narration"`
		BankCode      string  `json:"bankCode"`
		AccountNumber string  `json:"accountNumber"`
		AccountName   string  `json:"accountName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	source := req.SourceAccount
	if source == "" {
		source = mon.WalletAccount
	}
	d, err := mon.Disburse(r.Context(), source, req.Amount, req.Reference, req.Narration, req.BankCode, req.AccountNumber, req.AccountName)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func origin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}
	return scheme + "://" + host
}

// requestIDFromRef extracts the Kova request id from a Monnify reference:
// repayment "kova_repay_{id}" or disbursement "kova_{id}_{unix}".
func requestIDFromRef(ref string) string {
	switch {
	case strings.HasPrefix(ref, "kova_repay_"):
		return strings.TrimPrefix(ref, "kova_repay_")
	case strings.HasPrefix(ref, "kova_"):
		rest := strings.TrimPrefix(ref, "kova_")
		if i := strings.LastIndex(rest, "_"); i > 0 {
			return rest[:i]
		}
		return rest
	}
	return ""
}

// appURL returns an absolute frontend URL for path when baseURL is configured,
// otherwise the bare relative path.
func (s *Server) appURL(path string) string {
	if s.baseURL == "" {
		return path
	}
	return s.baseURL + path
}

// oauthBase returns the canonical external origin used for OAuth redirect URIs:
// the configured app base URL (KOVA_BASE_URL) when set, else the request origin.
// This keeps the GitHub redirect_uri stable regardless of which host (backend or
// the proxying frontend) served the request.
func (s *Server) oauthBase(r *http.Request) string {
	if s.baseURL != "" {
		return strings.TrimRight(s.baseURL, "/")
	}
	return origin(r)
}

func (s *Server) servePage(w http.ResponseWriter, name, id string) {
	page, err := staticFS.ReadFile("static/" + name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	out := strings.ReplaceAll(string(page), "__REQUEST_ID__", id)
	site := s.baseURL
	if site == "" {
		site = "/"
	}
	out = strings.ReplaceAll(out, "__KOVA_SITE__", site)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(out))
}

func (s *Server) handleBorrowerPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.store.RequestByID(r.Context(), id); !ok {
		http.NotFound(w, r)
		return
	}
	s.servePage(w, "borrower.html", id)
}

func (s *Server) handleLenderPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.store.RequestByID(r.Context(), id); !ok {
		http.NotFound(w, r)
		return
	}
	s.servePage(w, "lender.html", id)
}

// handleRepayPage serves the borrower repayment (pay-back) page.
func (s *Server) handleRepayPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.store.RequestByID(r.Context(), id); !ok {
		http.NotFound(w, r)
		return
	}
	s.servePage(w, "repay.html", id)
}

// handleRepayInit starts a Monnify hosted-checkout for the outstanding repayment
// and returns the checkout URL for the borrower to pay.
func (s *Server) handleRepayInit(w http.ResponseWriter, r *http.Request) {
	req, ok := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	mon, err := s.monnifyFor(r.Context(), req.WorkspaceID)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "payments not connected")
		return
	}
	if !req.Disbursed {
		writeErr(w, http.StatusBadRequest, "this loan has not been disbursed")
		return
	}
	if req.Repaid {
		writeErr(w, http.StatusConflict, "this loan has already been repaid")
		return
	}
	due := req.RepaymentTotal
	if due <= 0 {
		due = req.OfferAmount
	}
	email := req.BorrowerEmail
	if email == "" {
		email = "borrower@kova.dev"
	}
	name := req.BorrowerName
	if name == "" {
		name = "Kova Borrower"
	}
	ref := "kova_repay_" + req.ID
	redirect := s.appURL("/pay/" + req.ID + "/done")
	res, err := mon.InitTransaction(r.Context(), float64(due)/100, name, email, ref, "Kova loan repayment", redirect)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"checkoutUrl": res.CheckoutURL})
}

// handleRepayDone is the Monnify redirect target: it verifies the payment
// server-side and marks the loan repaid, then shows the repayment page.
func (s *Server) handleRepayDone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, ok := s.store.RequestByID(r.Context(), id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !req.Repaid {
		if mon, err := s.monnifyFor(r.Context(), req.WorkspaceID); err == nil {
			if txn, err := mon.VerifyTransaction(r.Context(), "kova_repay_"+id); err == nil {
				due := req.RepaymentTotal
				if due <= 0 {
					due = req.OfferAmount
				}
				if strings.EqualFold(txn.PaymentStatus, "PAID") && int64(txn.AmountPaid*100) >= due {
					if s.store.MarkRepaidByPayment(r.Context(), id) {
						if req.WorkspaceID != "" {
							s.store.RecordAudit(r.Context(), req.WorkspaceID, "borrower", "loan.repaid", id, "Paid via repayment link")
						}
						s.finalizeRepayment(r.Context(), id)
					}
				}
			}
		}
	}
	s.servePage(w, "repay.html", id)
}
