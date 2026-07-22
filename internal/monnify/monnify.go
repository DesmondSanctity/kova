// Package monnify is a small client for the Monnify APIs: auth, account name
// verification, disbursement (with OTP), and collections. Credentials are per-lender.
package monnify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// flexFloat unmarshals a JSON number or a quoted numeric string (Monnify returns
// some amounts as strings, e.g. "amountPaid": "10000.00").
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

const (
	defaultBaseURL      = "https://sandbox.monnify.com"
	defaultAPIKey       = "MK_TEST_GC3B8XG2XX"
	defaultSecretKey    = "A663NRZA544DDPEM7KDN7Z8HRV6YXD8S"
	defaultContractCode = "5867418298"
)

type Client struct {
	BaseURL       string
	APIKey        string
	SecretKey     string
	ContractCode  string
	WalletAccount string
	HTTP          *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// New builds a client from env (MONNIFY_BASE_URL, MONNIFY_API_KEY,
// MONNIFY_SECRET_KEY, MONNIFY_CONTRACT_CODE), falling back to sandbox defaults.
func New() *Client {
	return &Client{
		BaseURL:       envOr("MONNIFY_BASE_URL", defaultBaseURL),
		APIKey:        envOr("MONNIFY_API_KEY", defaultAPIKey),
		SecretKey:     envOr("MONNIFY_SECRET_KEY", defaultSecretKey),
		ContractCode:  envOr("MONNIFY_CONTRACT_CODE", defaultContractCode),
		WalletAccount: os.Getenv("MONNIFY_WALLET_ACCOUNT"),
		HTTP:          &http.Client{Timeout: 20 * time.Second},
	}
}

// NewWithCreds builds a client from explicit per-lender credentials.
func NewWithCreds(baseURL, apiKey, secretKey, contractCode, walletAccount string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		SecretKey:     secretKey,
		ContractCode:  contractCode,
		WalletAccount: walletAccount,
		HTTP:          &http.Client{Timeout: 20 * time.Second},
	}
}

type response[T any] struct {
	RequestSuccessful bool   `json:"requestSuccessful"`
	ResponseMessage   string `json:"responseMessage"`
	ResponseBody      T      `json:"responseBody"`
}

type Account struct {
	AccountNumber string `json:"accountNumber"`
	AccountName   string `json:"accountName"`
	BankCode      string `json:"bankCode"`
}

type Disbursement struct {
	Reference string  `json:"reference"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
}

// Token returns a valid access token, refreshing when expired.
func (c *Client) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/auth/login", nil)
	if err != nil {
		return "", err
	}
	basic := base64.StdEncoding.EncodeToString([]byte(c.APIKey + ":" + c.SecretKey))
	req.Header.Set("Authorization", "Basic "+basic)

	var out response[struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int64  `json:"expiresIn"`
	}]
	if err := c.do(req, &out); err != nil {
		return "", err
	}
	if !out.RequestSuccessful || out.ResponseBody.AccessToken == "" {
		return "", fmt.Errorf("monnify auth failed: %s", out.ResponseMessage)
	}
	c.token = out.ResponseBody.AccessToken
	ttl := out.ResponseBody.ExpiresIn
	if ttl <= 0 {
		ttl = 3000
	}
	c.tokenExp = time.Now().Add(time.Duration(ttl-60) * time.Second)
	return c.token, nil
}

// VerifyAccount resolves an account number + bank code to its registered name.
func (c *Client) VerifyAccount(ctx context.Context, accountNumber, bankCode string) (*Account, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/api/v1/disbursements/account/validate?accountNumber=%s&bankCode=%s",
		c.BaseURL, url.QueryEscape(accountNumber), url.QueryEscape(bankCode))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var out response[Account]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful {
		return nil, fmt.Errorf("account verification failed: %s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// BVNMatch is the result of matching a BVN against a bank account.
type BVNMatch struct {
	BVN           string `json:"bvn"`
	AccountNumber string `json:"accountNumber"`
	AccountName   string `json:"accountName"`
	MatchStatus   string `json:"matchStatus"` // FULL_MATCH | PARTIAL_MATCH | NO_MATCH
}

// MatchBVN verifies a BVN belongs to the holder of an account (Monnify VAS).
func (c *Client) MatchBVN(ctx context.Context, bvn, accountNumber, bankCode string) (*BVNMatch, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{
		"bvn":           bvn,
		"accountNumber": accountNumber,
		"bankCode":      bankCode,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/vas/bvn-account-match", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	var out response[BVNMatch]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful {
		return nil, fmt.Errorf("bvn verification failed: %s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// Disburse initiates a single transfer from the merchant wallet to an account.
func (c *Client) Disburse(ctx context.Context, sourceAccount string, amount float64, ref, narration, bankCode, accountNumber, accountName string) (*Disbursement, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"amount":                   amount,
		"reference":                ref,
		"narration":                narration,
		"destinationBankCode":      bankCode,
		"destinationAccountNumber": accountNumber,
		"destinationAccountName":   accountName,
		"currency":                 "NGN",
		"sourceAccountNumber":      sourceAccount,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v2/disbursements/single", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	var out response[Disbursement]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful {
		return nil, fmt.Errorf("disbursement failed: %s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// AuthorizeDisbursement completes a PENDING_AUTHORIZATION transfer by submitting
// the OTP Monnify emailed to the merchant (MFA is on by default on all accounts).
func (c *Client) AuthorizeDisbursement(ctx context.Context, ref, otp string) (*Disbursement, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]string{"reference": ref, "authorizationCode": otp})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v2/disbursements/single/validate-otp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	var out response[Disbursement]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful {
		return nil, fmt.Errorf("%s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// DisbursementStatus fetches the authoritative status of a single transfer by
// its reference (SUCCESS, PENDING_AUTHORIZATION, EXPIRED, FAILED, REVERSED…).
func (c *Client) DisbursementStatus(ctx context.Context, ref string) (*Disbursement, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/api/v2/disbursements/single/summary?reference=%s", c.BaseURL, url.QueryEscape(ref))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var out response[Disbursement]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful {
		return nil, fmt.Errorf("%s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// ResendDisbursementOTP requests a fresh OTP for a pending transfer.
func (c *Client) ResendDisbursementOTP(ctx context.Context, ref string) error {
	tok, err := c.Token(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"reference": ref})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v2/disbursements/single/resend-otp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	var out response[json.RawMessage]
	if err := c.do(req, &out); err != nil {
		return err
	}
	if !out.RequestSuccessful {
		return fmt.Errorf("%s", out.ResponseMessage)
	}
	return nil
}

// InitResult is the response from initializing a hosted-checkout transaction.
type InitResult struct {
	CheckoutURL          string `json:"checkoutUrl"`
	TransactionReference string `json:"transactionReference"`
	PaymentReference     string `json:"paymentReference"`
}

// InitTransaction starts a hosted-checkout collection and returns a checkout URL
// to redirect the payer to. Amount is in naira.
func (c *Client) InitTransaction(ctx context.Context, amountNaira float64, name, email, paymentRef, description, redirectURL string) (*InitResult, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"amount":             amountNaira,
		"customerName":       name,
		"customerEmail":      email,
		"paymentReference":   paymentRef,
		"paymentDescription": description,
		"currencyCode":       "NGN",
		"contractCode":       c.ContractCode,
		"redirectUrl":        redirectURL,
		"paymentMethods":     []string{"CARD", "ACCOUNT_TRANSFER"},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/merchant/transactions/init-transaction", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	var out response[InitResult]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful || out.ResponseBody.CheckoutURL == "" {
		return nil, fmt.Errorf("could not start payment: %s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// Transaction is the subset of a verified collection we care about.
type Transaction struct {
	PaymentStatus string    `json:"paymentStatus"`
	AmountPaid    flexFloat `json:"amountPaid"`
}

// VerifyTransaction fetches the authoritative status of a collection by the
// merchant-supplied payment reference. Amount fields are naira.
func (c *Client) VerifyTransaction(ctx context.Context, paymentRef string) (*Transaction, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/api/v2/merchant/transactions/query?paymentReference=%s", c.BaseURL, url.QueryEscape(paymentRef))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var out response[Transaction]
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if !out.RequestSuccessful {
		return nil, fmt.Errorf("could not verify payment: %s", out.ResponseMessage)
	}
	return &out.ResponseBody, nil
}

// request body keyed with the merchant secret, hex-encoded (monnify-signature header).
func (c *Client) VerifyWebhook(body []byte, signature string) bool {
	if signature == "" {
		return false
	}
	mac := hmac.New(sha512.New, []byte(c.SecretKey))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", req.URL.Path, err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
