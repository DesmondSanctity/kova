package api

import (
	"context"
	"encoding/json"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	emailpkg "kova/internal/email"
	"kova/internal/store"
)

const sessionCookie = "kova_session"

var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

type githubOAuth struct {
	clientID     string
	clientSecret string
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, userID string) {
	tok, err := s.store.CreateSession(r.Context(), userID)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(30 * 24 * time.Hour),
	})
}

func (s *Server) currentUser(r *http.Request) (*store.User, bool) {
	if s.store == nil {
		return nil, false
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, false
	}
	return s.store.UserBySession(r.Context(), c.Value)
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, ok := s.currentUser(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not signed in")
	}
	return u, ok
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Password, Name string }
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := s.store.Signup(r.Context(), body.Email, body.Password, body.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.setSession(w, r, u.ID)
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Password string }
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := s.store.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	s.setSession(w, r, u.ID)
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email string }
	_ = decodeJSON(r, &body)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	code, ok := s.store.CreateReset(r.Context(), email)
	// Always respond ok so we don't leak which emails have accounts.
	resp := map[string]any{"status": "ok"}
	if ok {
		if err := s.mailer.Send(r.Context(), email, "Your Kova password reset code", resetEmailHTML(code)); err != nil {
			log.Printf("forgot: send email to %s: %v", email, err)
		}
		// When no provider is configured, surface the code for local/pilot use.
		if _, noop := s.mailer.(emailpkg.Noop); noop {
			resp["devCode"] = code
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Code, Password string }
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.store.ResetPassword(r.Context(), body.Email, body.Code, body.Password); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid or expired code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func resetEmailHTML(code string) string {
	body := `<h1 style="margin:0 0 10px;font-size:19px;font-weight:600;color:#0f172a">Reset your password</h1>
	<p style="margin:0 0 20px;font-size:15px;line-height:1.55;color:#475569">Enter this code to set a new password. It expires in 15 minutes.</p>
	<div style="font-size:30px;font-weight:700;letter-spacing:8px;text-align:center;padding:18px;background:#f4f4f5;border-radius:12px;color:#0f172a">` + code + `</div>
	<p style="margin:20px 0 0;font-size:13px;line-height:1.5;color:#94a3b8">If you didn't request this, you can safely ignore this email.</p>`
	return emailLayout("Kova", body)
}

// emailLayout wraps email body content in a clean, branded shell used across
// all Kova transactional emails.
func emailLayout(brand, bodyHTML string) string {
	if brand == "" {
		brand = "Kova"
	}
	b := html.EscapeString(brand)
	return `<div style="background:#f4f4f5;padding:32px 16px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif">
  <div style="max-width:480px;margin:0 auto;background:#ffffff;border:1px solid #e6e6e9;border-radius:16px;overflow:hidden">
    <div style="padding:20px 28px;border-bottom:1px solid #f0f0f2;font-size:15px;font-weight:600;color:#0f172a;letter-spacing:-0.01em">` + b + `</div>
    <div style="padding:28px">` + bodyHTML + `</div>
    <div style="padding:16px 28px;border-top:1px solid #f0f0f2;font-size:12px;color:#94a3b8">Sent by ` + b + ` · Powered by Kova</div>
  </div>
</div>`
}

// emailButton renders a primary call-to-action button for emails.
func emailButton(url, label string) string {
	return `<a href="` + url + `" style="display:inline-block;background:#0f172a;color:#ffffff;text-decoration:none;padding:12px 22px;border-radius:10px;font-size:15px;font-weight:600">` + label + `</a>`
}

// --- GitHub OAuth ---

func (s *Server) handleGitHubStart(w http.ResponseWriter, r *http.Request) {
	if s.github.clientID == "" {
		http.Redirect(w, r, s.appURL("/login?error=github_not_configured"), http.StatusFound)
		return
	}
	q := url.Values{
		"client_id":    {s.github.clientID},
		"redirect_uri": {s.oauthBase(r) + "/auth/github/callback"},
		"scope":        {"read:user user:email"},
	}
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" || s.github.clientID == "" {
		http.Redirect(w, r, s.appURL("/login?error=github"), http.StatusFound)
		return
	}
	tok, err := s.exchangeGitHubCode(code, s.oauthBase(r))
	if err != nil {
		http.Redirect(w, r, s.appURL("/login?error=github"), http.StatusFound)
		return
	}
	gh, err := fetchGitHubUser(tok)
	if err != nil {
		http.Redirect(w, r, s.appURL("/login?error=github"), http.StatusFound)
		return
	}
	u, err := s.store.UpsertGitHub(r.Context(), gh.id, gh.email, gh.name, gh.avatar)
	if err != nil {
		http.Redirect(w, r, s.appURL("/login?error=github"), http.StatusFound)
		return
	}
	s.setSession(w, r, u.ID)
	http.Redirect(w, r, s.appURL("/dashboard"), http.StatusFound)
}

func (s *Server) exchangeGitHubCode(code, base string) (string, error) {
	form := url.Values{
		"client_id":     {s.github.clientID},
		"client_secret": {s.github.clientSecret},
		"code":          {code},
		"redirect_uri":  {base + "/auth/github/callback"},
	}
	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.AccessToken, nil
}

type ghUser struct{ id, email, name, avatar string }

func fetchGitHubUser(token string) (ghUser, error) {
	get := func(u string, v any) error {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return json.NewDecoder(resp.Body).Decode(v)
	}
	var u struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := get("https://api.github.com/user", &u); err != nil {
		return ghUser{}, err
	}
	email := u.Email
	if email == "" {
		var emails []struct {
			Email   string `json:"email"`
			Primary bool   `json:"primary"`
		}
		if get("https://api.github.com/user/emails", &emails) == nil {
			for _, e := range emails {
				if e.Primary {
					email = e.Email
				}
			}
		}
	}
	name := u.Name
	if name == "" {
		name = u.Login
	}
	return ghUser{id: strconv.Itoa(u.ID), email: email, name: name, avatar: u.AvatarURL}, nil
}

// --- dashboard API ---

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	ws, hasWs := s.store.WorkspaceForUser(r.Context(), u.ID)
	resp := map[string]any{"user": u, "needsOnboarding": !hasWs}
	if hasWs {
		keys, _ := s.store.ListKeys(r.Context(), ws.ID)
		if keys == nil {
			keys = []*store.Key{}
		}
		resp["workspace"] = ws
		resp["keys"] = keys
		resp["usage"] = s.store.UsageSummary(r.Context(), ws.ID)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if _, has := s.store.WorkspaceForUser(r.Context(), u.ID); has {
		writeErr(w, http.StatusConflict, "workspace already exists")
		return
	}
	var body struct{ Name, OrgName, UseCase string }
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = body.OrgName
	}
	ws, err := s.store.CreateWorkspace(r.Context(), u.ID, body.Name, body.OrgName, body.UseCase)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (s *Server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	var body struct {
		OrgName        string              `json:"orgName"`
		BrandName      string              `json:"brandName"`
		BrandColor     string              `json:"brandColor"`
		BrandTextColor string              `json:"brandTextColor"`
		SupportEmail   string              `json:"supportEmail"`
		LoanProducts   []store.LoanProduct `json:"loanProducts"`
		MinScore       int                 `json:"minScore"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	orgName := strings.TrimSpace(body.OrgName)
	if orgName == "" {
		orgName = ws.OrgName
	}
	color := strings.TrimSpace(body.BrandColor)
	if color != "" && !hexColor.MatchString(color) {
		writeErr(w, http.StatusBadRequest, "brand color must be a hex value like #8b7cff")
		return
	}
	textColor := strings.TrimSpace(body.BrandTextColor)
	if textColor != "" && !hexColor.MatchString(textColor) {
		writeErr(w, http.StatusBadRequest, "button text color must be a hex value like #ffffff")
		return
	}
	products := []store.LoanProduct{}
	for _, p := range body.LoanProducts {
		if p.MaxAmount <= 0 {
			continue
		}
		products = append(products, p)
	}
	updated, ok := s.store.UpdateWorkspaceSettings(r.Context(), ws.ID, orgName,
		strings.TrimSpace(body.BrandName), color, textColor, strings.TrimSpace(body.SupportEmail), products, body.MinScore)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "could not update workspace")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// resolveWorkspaceID authenticates a lender action via EITHER a dashboard session
// OR an API key, and returns the acting workspace id. It never lets one tenant act
// on another: sessions resolve to the user's own workspace, and API keys resolve to
// the key's own workspace. needSecret requires a secret-scoped key (sessions always
// qualify, since they are the workspace owner); publishable keys are rejected.
func (s *Server) resolveWorkspaceID(w http.ResponseWriter, r *http.Request, needSecret bool) (string, bool) {
	// 1. Dashboard session — owner of their workspace.
	if u, ok := s.currentUser(r); ok {
		if ws, has := s.store.WorkspaceForUser(r.Context(), u.ID); has {
			return ws.ID, true
		}
		writeErr(w, http.StatusForbidden, "no workspace for this account")
		return "", false
	}
	// 2. API key — must be a workspace-scoped key; secret required for privileged ops.
	sc := scopeAny
	if needSecret {
		sc = scopeSecret
	}
	if auth, ok := s.authorizeKey(r, sc); ok {
		if auth.WorkspaceID == "" {
			writeErr(w, http.StatusForbidden, "this API key is not scoped to a workspace")
			return "", false
		}
		return auth.WorkspaceID, true
	}
	writeErr(w, http.StatusUnauthorized, "sign in or provide a valid API key")
	return "", false
}

func (s *Server) workspaceFor(w http.ResponseWriter, r *http.Request) (*store.Workspace, bool) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return nil, false
	}
	ws, has := s.store.WorkspaceForUser(r.Context(), u.ID)
	if !has {
		writeErr(w, http.StatusBadRequest, "no workspace — complete onboarding first")
		return nil, false
	}
	return ws, true
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	var body struct{ Name string }
	_ = decodeJSON(r, &body)
	k, err := s.store.CreateKey(r.Context(), ws.ID, body.Name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.store.RecordAudit(r.Context(), ws.ID, "lender", "key.created", k.ID, k.Name)
	writeJSON(w, http.StatusOK, k)
}

func (s *Server) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	if !s.store.RevokeKey(r.Context(), ws.ID, r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	s.store.RecordAudit(r.Context(), ws.ID, "lender", "key.revoked", r.PathValue("id"), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) handleKeyAllowlist(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	var body struct {
		Domains []string `json:"domains"`
		IPs     []string `json:"ips"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !s.store.UpdateKeyAllowlist(r.Context(), ws.ID, r.PathValue("id"), clean(body.Domains), clean(body.IPs)) {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, false)
	if !ok {
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	_ = decodeJSON(r, &body)
	req, err := s.store.CreateRequest(r.Context(), wsID, body.Note, 0, 0, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.store.RecordAudit(r.Context(), wsID, "lender", "link.created", req.ID, body.Note)
	base := origin(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": req.ID, "note": req.Note, "status": req.Status,
		"borrowerUrl": base + "/r/" + req.ID, "viewUrl": base + "/v/" + req.ID,
	})
}

// handleRejectLink lets the lender decline a request before payout.
func (s *Server) handleRejectLink(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	if req.Disbursed {
		writeErr(w, http.StatusBadRequest, "already disbursed — cannot reject")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &body)
	reason := strings.TrimSpace(body.Reason)
	if !s.store.RejectRequest(r.Context(), req.ID) {
		writeErr(w, http.StatusConflict, "could not reject this request")
		return
	}
	auditDetail := reason
	if auditDetail == "" {
		auditDetail = req.Note
	}
	s.store.RecordAudit(r.Context(), wsID, "lender", "request.rejected", req.ID, auditDetail)
	// Notify the borrower they were declined, with the lender's reason if given.
	if s.mailer != nil && req.BorrowerEmail != "" {
		brand := "Kova"
		if ws, ok := s.store.WorkspaceByID(r.Context(), wsID); ok {
			if ws.BrandName != "" {
				brand = ws.BrandName
			} else if ws.OrgName != "" {
				brand = ws.OrgName
			}
		}
		subject, html := resultEmail(brand, req.BorrowerName, "declined", 0, 0, 0, "", reason)
		if err := s.mailer.Send(r.Context(), req.BorrowerEmail, subject, html); err != nil {
			log.Printf("reject email to %s: %v", req.BorrowerEmail, err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// handleDisburseLink pays out an accepted offer from the Monnify wallet.
// Dashboard-only: disbursement is not exposed to API keys / the SDK.
func (s *Server) handleDisburseLink(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	wsID := ws.ID
	if s.monnify == nil {
		writeErr(w, http.StatusServiceUnavailable, "monnify not configured")
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	if !req.Accepted {
		writeErr(w, http.StatusBadRequest, "borrower has not accepted an offer yet")
		return
	}
	if req.Disbursed {
		writeErr(w, http.StatusConflict, "already disbursed")
		return
	}
	// If a payout is already awaiting OTP authorization and hasn't expired, reuse
	// it — surface the OTP step again rather than creating a duplicate transfer.
	// The still-valid emailed code (or "Resend code") authorizes this same one.
	if req.DisbursementStatus == "PENDING_AUTHORIZATION" && req.DisbursementRef != "" && !disbursementRefExpired(req.DisbursementRef) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "otp_required", "reference": req.DisbursementRef, "pending": true})
		return
	}
	source := s.monnify.WalletAccount
	if len(source) != 10 {
		writeErr(w, http.StatusServiceUnavailable, "disbursement wallet not configured — set MONNIFY_WALLET_ACCOUNT to your 10-digit Monnify wallet account number (Dashboard → Disbursements → Wallet)")
		return
	}
	// Unique reference per attempt so an expired/failed payout can be retried
	// (Monnify rejects a reused reference).
	ref := "kova_" + req.ID + "_" + strconv.FormatInt(time.Now().Unix(), 10)
	narration := "Kova loan disbursement"
	name := req.AccountName
	if name == "" {
		name = req.BorrowerName
	}
	if name == "" {
		name = "Kova Borrower"
	}
	d, err := s.monnify.Disburse(r.Context(), source, float64(req.OfferAmount)/100, ref, narration, req.BankCode, req.AccountNumber, name)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "disbursement failed: "+err.Error())
		return
	}
	// MFA is on by default on Monnify accounts: the transfer sits in
	// PENDING_AUTHORIZATION until an emailed OTP is submitted. Only finalize once
	// the money has actually moved (SUCCESS/COMPLETED).
	switch d.Status {
	case "SUCCESS", "COMPLETED":
		s.finalizeDisbursement(r.Context(), req, d.Reference, wsID)
		writeJSON(w, http.StatusOK, map[string]any{"status": "disbursed", "reference": d.Reference})
	case "PENDING_AUTHORIZATION":
		s.store.SetDisbursementPending(r.Context(), req.ID, ref)
		s.store.RecordAudit(r.Context(), wsID, "lender", "loan.otp_sent", req.ID, "₦"+strconv.FormatInt(req.OfferAmount/100, 10)+" payout to "+name)
		writeJSON(w, http.StatusOK, map[string]any{"status": "otp_required", "reference": ref})
	default:
		writeErr(w, http.StatusBadGateway, "disbursement not completed (status: "+d.Status+")")
	}
}

// disbursementRefExpired reports whether a pending payout reference
// ("kova_{id}_{unix}") is older than the OTP validity window.
func disbursementRefExpired(ref string) bool {
	i := strings.LastIndex(ref, "_")
	if i < 0 {
		return true
	}
	sec, err := strconv.ParseInt(ref[i+1:], 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(sec, 0)) > 15*time.Minute
}

// finalizeDisbursement marks the loan paid out, builds the repayment schedule,
// and records the audit + usage. Shared by direct and OTP-authorized payouts.
func (s *Server) finalizeDisbursement(ctx context.Context, req *store.Request, ref, wsID string) {
	s.store.MarkDisbursed(ctx, req.ID, ref)
	// Build the repayment schedule: principal + interest due at tenor end.
	tenor := req.TenorDays
	if tenor <= 0 {
		tenor = 30
	}
	interest := int64(float64(req.OfferAmount) * req.InterestRate / 100)
	s.store.SetRepayment(ctx, req.ID, req.OfferAmount+interest, time.Now().AddDate(0, 0, tenor))
	payee := req.AccountName
	if payee == "" {
		payee = req.BorrowerName
	}
	if payee == "" {
		payee = "borrower"
	}
	s.store.RecordAudit(ctx, wsID, "lender", "loan.disbursed", req.ID, "₦"+strconv.FormatInt(req.OfferAmount/100, 10)+" to "+payee)
	if req.WorkspaceID != "" {
		s.store.RecordUsage(ctx, req.WorkspaceID, "", "disburse")
	}
}

// handleAuthorizeLink completes a pending payout by validating the Monnify OTP.
// Dashboard-only.
func (s *Server) handleAuthorizeLink(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	wsID := ws.ID
	if s.monnify == nil {
		writeErr(w, http.StatusServiceUnavailable, "monnify not configured")
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	if req.Disbursed {
		writeErr(w, http.StatusConflict, "already disbursed")
		return
	}
	var body struct {
		OTP string `json:"otp"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.OTP) == "" {
		writeErr(w, http.StatusBadRequest, "otp is required")
		return
	}
	ref := req.DisbursementRef
	if ref == "" {
		ref = "kova_" + req.ID
	}
	d, err := s.monnify.AuthorizeDisbursement(r.Context(), ref, strings.TrimSpace(body.OTP))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if d.Status != "SUCCESS" && d.Status != "COMPLETED" {
		writeErr(w, http.StatusBadGateway, "authorization not completed (status: "+d.Status+")")
		return
	}
	s.finalizeDisbursement(r.Context(), req, d.Reference, wsID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "disbursed", "reference": d.Reference})
}

// handleResendOTP asks Monnify to re-send the disbursement OTP. Dashboard-only.
func (s *Server) handleResendOTP(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFor(w, r)
	if !ok {
		return
	}
	wsID := ws.ID
	if s.monnify == nil {
		writeErr(w, http.StatusServiceUnavailable, "monnify not configured")
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	ref := req.DisbursementRef
	if ref == "" {
		ref = "kova_" + req.ID
	}
	if err := s.monnify.ResendDisbursementOTP(r.Context(), ref); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "otp_resent"})
}

// handleRepayLink marks a disbursed loan as repaid (lender-confirmed collection).
func (s *Server) handleRepayLink(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	if !s.store.MarkRepaid(r.Context(), wsID, r.PathValue("id")) {
		writeErr(w, http.StatusConflict, "could not mark repaid (loan not disbursed or already repaid)")
		return
	}
	s.store.RecordAudit(r.Context(), wsID, "lender", "loan.repaid", r.PathValue("id"), "Marked repaid by lender")
	s.finalizeRepayment(r.Context(), r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]string{"status": "repaid"})
}

// handleVerifyRepayment confirms a borrower's repayment with Monnify (server-side
// Verify Transaction API) and marks the loan repaid if paid. This is the reliable
// path — it works whether or not the borrower's browser redirected back.
func (s *Server) handleVerifyRepayment(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	if s.monnify == nil {
		writeErr(w, http.StatusServiceUnavailable, "monnify not configured")
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	if req.Repaid {
		writeJSON(w, http.StatusOK, map[string]string{"status": "repaid"})
		return
	}
	if !req.Disbursed {
		writeErr(w, http.StatusBadRequest, "this loan has not been disbursed")
		return
	}
	txn, err := s.monnify.VerifyTransaction(r.Context(), "kova_repay_"+req.ID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	due := req.RepaymentTotal
	if due <= 0 {
		due = req.OfferAmount
	}
	if strings.EqualFold(txn.PaymentStatus, "PAID") && int64(txn.AmountPaid*100) >= due {
		if s.store.MarkRepaidByPayment(r.Context(), req.ID) {
			s.finalizeRepayment(r.Context(), req.ID)
		}
		s.store.RecordAudit(r.Context(), wsID, "lender", "loan.repaid", req.ID, "Confirmed paid with Monnify")
		writeJSON(w, http.StatusOK, map[string]string{"status": "repaid"})
		return
	}
	status := txn.PaymentStatus
	if status == "" {
		status = "no payment found yet"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "not_paid", "paymentStatus": status})
}
func (s *Server) handleRequestRepayment(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	if !req.Disbursed {
		writeErr(w, http.StatusBadRequest, "this loan has not been disbursed")
		return
	}
	if req.Repaid {
		writeErr(w, http.StatusConflict, "this loan is already repaid")
		return
	}
	if req.BorrowerEmail == "" {
		writeErr(w, http.StatusBadRequest, "no borrower email on file")
		return
	}
	if !s.sendRepaymentEmail(r.Context(), req) {
		writeErr(w, http.StatusBadGateway, "could not send the repayment email")
		return
	}
	s.store.SetRepaymentReminded(r.Context(), req.ID)
	s.store.RecordAudit(r.Context(), wsID, "lender", "repayment.requested", req.ID, "Repayment link emailed to "+req.BorrowerEmail)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// handleResendOffer re-sends the borrower's decision/accept email while the
// offer is still open (not accepted, declined, or disbursed).
func (s *Server) handleResendOffer(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	req, found := s.store.RequestByID(r.Context(), r.PathValue("id"))
	if !found || req.WorkspaceID != wsID {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	if req.Decision != "approved" && req.Decision != "counter" {
		writeErr(w, http.StatusBadRequest, "no offer to send yet")
		return
	}
	if req.Accepted || req.Disbursed || req.Status == "rejected" {
		writeErr(w, http.StatusConflict, "this offer has already been actioned")
		return
	}
	if req.BorrowerEmail == "" {
		writeErr(w, http.StatusBadRequest, "no borrower email on file")
		return
	}
	s.emailBorrowerResult(r, req, req.Decision, req.OfferAmount)
	s.store.RecordAudit(r.Context(), wsID, "lender", "offer.resent", req.ID, "Estimate email resent to "+req.BorrowerEmail)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// sendRepaymentEmail emails the borrower a Monnify repayment link. Safe to call
// from both request handlers and the background reminder scheduler.
func (s *Server) sendRepaymentEmail(ctx context.Context, req *store.Request) bool {
	if s.mailer == nil || req.BorrowerEmail == "" {
		return false
	}
	brand := "Kova"
	if req.WorkspaceID != "" {
		if ws, ok := s.store.WorkspaceByID(ctx, req.WorkspaceID); ok {
			if ws.BrandName != "" {
				brand = ws.BrandName
			} else if ws.OrgName != "" {
				brand = ws.OrgName
			}
		}
	}
	due := req.RepaymentTotal
	if due <= 0 {
		due = req.OfferAmount
	}
	payURL := s.appURL("/pay/" + req.ID)
	subject, body := repaymentEmail(brand, req.BorrowerName, due, req.RepaymentDueAt, payURL)
	if err := s.mailer.Send(ctx, req.BorrowerEmail, subject, body); err != nil {
		log.Printf("repayment email to %s: %v", req.BorrowerEmail, err)
		return false
	}
	return true
}

// repaymentEmail builds the borrower repayment request email.
func repaymentEmail(brand, name string, dueKobo int64, dueAt *time.Time, payURL string) (string, string) {
	naira := "₦" + strconv.FormatInt(dueKobo/100, 10)
	greeting := "Hi"
	if name != "" {
		greeting = "Hi " + html.EscapeString(name)
	}
	bn := html.EscapeString(brand)
	dueLine := ""
	if dueAt != nil && !dueAt.IsZero() {
		dueLine = ` due on <b style="color:#0f172a">` + dueAt.Format("2 January 2006") + `</b>`
	}
	body := `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">Repay your loan</h1>
		<p style="margin:0 0 4px;font-size:15px;line-height:1.55;color:#475569">` + greeting + `, your repayment to ` + bn + ` is now` + dueLine + `:</p>
		<div style="margin:2px 0 18px;font-size:34px;font-weight:300;letter-spacing:-0.02em;color:#0f172a">` + naira + `</div>` +
		emailButton(payURL, "Pay "+naira+" securely") +
		`<p style="margin:18px 0 0;font-size:13px;line-height:1.5;color:#94a3b8">You'll pay securely via Monnify. This link stays valid until the loan is repaid.</p>`
	return brand + ": repayment of " + naira + " is due", emailLayout(brand, body)
}

// receiptEmail builds the borrower's repayment receipt (loan fully repaid).
func receiptEmail(brand, name string, amountKobo int64, repaidAt *time.Time) (string, string) {
	naira := "₦" + strconv.FormatInt(amountKobo/100, 10)
	greeting := "Hi"
	if name != "" {
		greeting = "Hi " + html.EscapeString(name)
	}
	bn := html.EscapeString(brand)
	dateLine := ""
	if repaidAt != nil && !repaidAt.IsZero() {
		dateLine = `<div style="margin:0 0 18px;padding:12px 16px;background:#f4f4f5;border-radius:12px;font-size:13.5px;color:#334155">Paid on <b style="color:#0f172a">` + repaidAt.Format("2 January 2006") + `</b></div>`
	}
	body := `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">Payment received</h1>
		<p style="margin:0 0 4px;font-size:15px;line-height:1.55;color:#475569">` + greeting + `, thank you — we've received your repayment to ` + bn + `:</p>
		<div style="margin:2px 0 16px;font-size:34px;font-weight:300;letter-spacing:-0.02em;color:#0f172a">` + naira + `</div>` +
		dateLine +
		`<p style="margin:0;font-size:15px;line-height:1.55;color:#475569">Your loan is now <b style="color:#0f172a">fully repaid</b>. Keep this email as your receipt.</p>`
	return bn + ": payment received — receipt for " + naira, emailLayout(brand, body)
}

// resultEmail builds the borrower notification for a decision. For approvals it
// frames the amount as a score-based estimate (the lender has final say); for
// declines it optionally includes the lender's reason.
func resultEmail(brand, name, decision string, offer int64, rate float64, tenorDays int, acceptURL, reason string) (string, string) {
	naira := func(k int64) string { return "₦" + strconv.FormatInt(k/100, 10) }
	greeting := "Hi"
	if name != "" {
		greeting = "Hi " + html.EscapeString(name)
	}
	bn := html.EscapeString(brand)
	var chips []string
	if rate > 0 {
		chips = append(chips, "Interest "+strconv.FormatFloat(rate, 'f', -1, 64)+"%")
	}
	if tenorDays > 0 {
		chips = append(chips, "Repay in "+strconv.Itoa(tenorDays)+" days")
	}
	terms := ""
	if len(chips) > 0 {
		terms = `<div style="margin:0 0 18px;font-size:14px;color:#475569">` + strings.Join(chips, " · ") + `</div>`
	}
	amount := `<div style="margin:2px 0 6px;font-size:34px;font-weight:300;letter-spacing:-0.02em;color:#0f172a">` + naira(offer) + `</div>`
	disclaimer := `<div style="margin:8px 0 20px;padding:12px 14px;background:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;font-size:12.5px;line-height:1.5;color:#64748b">This is an estimate based on your bank statements. ` + bn + ` has the final say and may reduce or adjust the amount before payout.</div>`
	var subject, body string
	switch decision {
	case "approved", "counter":
		subject = brand + ": your score is in"
		body = `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">Your score is in</h1>
		<p style="margin:0 0 4px;font-size:15px;line-height:1.55;color:#475569">` + greeting + `, based on your statements, here's an estimate of what ` + bn + ` could offer you:</p>` +
			amount + terms + disclaimer + emailButton(acceptURL, "Apply now")
	default:
		subject = brand + ": update on your application"
		reasonBlock := ""
		if r := strings.TrimSpace(reason); r != "" {
			reasonBlock = `<div style="margin:16px 0 0;padding:12px 14px;background:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;font-size:13.5px;line-height:1.5;color:#475569"><b style="color:#0f172a">Reason:</b> ` + html.EscapeString(r) + `</div>`
		}
		body = `<h1 style="margin:0 0 12px;font-size:20px;font-weight:600;color:#0f172a">Update on your application</h1>
		<p style="margin:0;font-size:15px;line-height:1.55;color:#475569">` + greeting + `, thank you for applying. ` + bn + ` is unable to extend an offer at this time.</p>` + reasonBlock
	}
	return subject, emailLayout(brand, body)
}

// handleListAudit returns the workspace audit trail.
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	events, _ := s.store.ListAudit(r.Context(), wsID, 100)
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	wsID, ok := s.resolveWorkspaceID(w, r, true)
	if !ok {
		return
	}
	links, _ := s.store.ListRequests(r.Context(), wsID)
	base := origin(r)
	out := make([]map[string]any, 0, len(links))
	for _, l := range links {
		m := map[string]any{
			"id": l.ID, "note": l.Note, "status": l.Status, "createdAt": l.CreatedAt,
			"borrowerUrl": base + "/r/" + l.ID, "viewUrl": base + "/v/" + l.ID,
			"borrowerName": l.BorrowerName, "borrowerEmail": l.BorrowerEmail,
			"amountRequested": l.AmountRequested, "maxAmount": l.MaxAmount,
			"interestRate": l.InterestRate, "tenorDays": l.TenorDays,
			"decision": l.Decision, "offerAmount": l.OfferAmount,
			"accepted": l.Accepted, "accountName": l.AccountName,
			"disbursed": l.Disbursed, "disbursedAt": l.DisbursedAt,
			"disbursementStatus": l.DisbursementStatus,
			"repaymentTotal":     l.RepaymentTotal, "repaymentDueAt": l.RepaymentDueAt,
			"repaid": l.Repaid, "repaidAt": l.RepaidAt,
		}
		if len(l.Report) > 0 {
			var rep struct {
				Score struct {
					Score int    `json:"score"`
					Band  string `json:"band"`
				} `json:"score"`
			}
			if json.Unmarshal(l.Report, &rep) == nil {
				m["score"] = rep.Score.Score
				m["band"] = rep.Score.Band
			}
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": out})
}

func clean(list []string) []string {
	out := []string{}
	for _, v := range list {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
