package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultAPIBaseURL = "https://api.razorpay.com/v1"
	maxProviderBody   = 1 << 20
)

var (
	ErrUnavailable       = errors.New("Razorpay is not configured")
	ErrInvalidRequest    = errors.New("Razorpay request is invalid")
	ErrProviderResponse  = errors.New("Razorpay returned an invalid response")
	providerIDPattern    = regexp.MustCompile(`^[A-Za-z0-9_]{8,80}$`)
	checkoutSignatureHex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
)

type Credentials struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
}

type OrderRequest struct {
	Amount   int64             `json:"amount"`
	Currency string            `json:"currency"`
	Receipt  string            `json:"receipt"`
	Notes    map[string]string `json:"notes,omitempty"`
}

type Order struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Receipt  string `json:"receipt"`
	Status   string `json:"status"`
}

type Payment struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	OrderID  string `json:"order_id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Captured bool   `json:"captured"`
}

type RazorpayClient struct {
	keyID         string
	keySecret     string
	webhookSecret string
	baseURL       string
	httpClient    *http.Client
}

func NewRazorpayClient(credentials Credentials, client *http.Client) (*RazorpayClient, error) {
	return NewRazorpayClientAt(credentials, client, DefaultAPIBaseURL)
}

// NewRazorpayClientAt exposes endpoint injection for isolated HTTP contract tests.
// Application configuration must use NewRazorpayClient and cannot select a live URL.
func NewRazorpayClientAt(credentials Credentials, client *http.Client, apiBaseURL string) (*RazorpayClient, error) {
	if !strings.HasPrefix(credentials.KeyID, "rzp_test_") || len(credentials.KeyID) < len("rzp_test_x") ||
		len(credentials.KeySecret) < 8 || len(credentials.WebhookSecret) < 8 {
		return nil, ErrUnavailable
	}
	parsed, err := url.Parse(apiBaseURL)
	localTestHost := parsed != nil && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost")
	if err != nil || parsed.Scheme != "https" && !localTestHost {
		return nil, ErrInvalidRequest
	}
	if apiBaseURL != DefaultAPIBaseURL && !localTestHost {
		return nil, ErrInvalidRequest
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &RazorpayClient{keyID: credentials.KeyID, keySecret: credentials.KeySecret, webhookSecret: credentials.WebhookSecret,
		baseURL: strings.TrimRight(apiBaseURL, "/"), httpClient: client}, nil
}

func (c *RazorpayClient) KeyID() string { return c.keyID }

func (c *RazorpayClient) CreateOrder(ctx context.Context, input OrderRequest) (Order, error) {
	if input.Amount <= 0 || input.Currency != "INR" || len(input.Receipt) < 1 || len(input.Receipt) > 40 || strings.ContainsAny(input.Receipt, "\r\n\x00") || len(input.Notes) > 15 {
		return Order{}, ErrInvalidRequest
	}
	for key, value := range input.Notes {
		if len(key) > 256 || len(value) > 256 || strings.ContainsAny(key+value, "\r\n\x00") {
			return Order{}, ErrInvalidRequest
		}
	}
	var result Order
	if err := c.doJSON(ctx, http.MethodPost, "/orders", input, &result); err != nil {
		return Order{}, err
	}
	if !providerIDPattern.MatchString(result.ID) || result.Amount != input.Amount || result.Currency != input.Currency || result.Receipt != input.Receipt || result.Status != "created" {
		return Order{}, ErrProviderResponse
	}
	return result, nil
}

func (c *RazorpayClient) FetchPayment(ctx context.Context, paymentID string) (Payment, error) {
	if !providerIDPattern.MatchString(paymentID) || !strings.HasPrefix(paymentID, "pay_") {
		return Payment{}, ErrInvalidRequest
	}
	var result Payment
	if err := c.doJSON(ctx, http.MethodGet, "/payments/"+url.PathEscape(paymentID), nil, &result); err != nil {
		return Payment{}, err
	}
	if result.ID != paymentID || !strings.HasPrefix(result.OrderID, "order_") || result.Amount <= 0 || result.Currency != "INR" ||
		(result.Status != "created" && result.Status != "authorized" && result.Status != "captured" && result.Status != "refunded" && result.Status != "failed") {
		return Payment{}, ErrProviderResponse
	}
	return result, nil
}

// FetchOrderPayments uses Razorpay's order-scoped payment collection for
// reconciliation. The request asks for the largest supported bounded page.
func (c *RazorpayClient) FetchOrderPayments(ctx context.Context, orderID string) ([]Payment, error) {
	if !providerIDPattern.MatchString(orderID) || !strings.HasPrefix(orderID, "order_") {
		return nil, ErrInvalidRequest
	}
	var result struct {
		Entity string    `json:"entity"`
		Count  int       `json:"count"`
		Items  []Payment `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/orders/"+url.PathEscape(orderID)+"/payments?count=100", nil, &result); err != nil {
		return nil, err
	}
	if result.Entity != "collection" || result.Count < 0 || result.Count != len(result.Items) || result.Count > 100 {
		return nil, ErrProviderResponse
	}
	for _, payment := range result.Items {
		if !providerIDPattern.MatchString(payment.ID) || !strings.HasPrefix(payment.ID, "pay_") || payment.OrderID != orderID || payment.Amount <= 0 || payment.Currency != "INR" ||
			(payment.Status != "created" && payment.Status != "authorized" && payment.Status != "captured" && payment.Status != "failed" && payment.Status != "refunded") {
			return nil, ErrProviderResponse
		}
		if payment.Status == "captured" && !payment.Captured {
			return nil, ErrProviderResponse
		}
	}
	return result.Items, nil
}

func (c *RazorpayClient) VerifyCheckoutSignature(orderID, paymentID, signature string) bool {
	if !strings.HasPrefix(orderID, "order_") || !providerIDPattern.MatchString(orderID) ||
		!strings.HasPrefix(paymentID, "pay_") || !providerIDPattern.MatchString(paymentID) || !checkoutSignatureHex.MatchString(signature) {
		return false
	}
	return verifyHMAC([]byte(orderID+"|"+paymentID), signature, c.keySecret)
}

func (c *RazorpayClient) VerifyWebhookSignature(rawBody []byte, signature string) bool {
	if len(rawBody) == 0 || len(rawBody) > maxProviderBody || !checkoutSignatureHex.MatchString(signature) {
		return false
	}
	return verifyHMAC(rawBody, signature, c.webhookSecret)
}

func CreateCheckoutSignature(orderID, paymentID, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(orderID + "|" + paymentID))
	return hex.EncodeToString(mac.Sum(nil))
}

func SignWebhookBody(rawBody []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyHMAC(message []byte, signature, secret string) bool {
	provided, err := hex.DecodeString(signature)
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(message)
	return hmac.Equal(provided, mac.Sum(nil))
}

func (c *RazorpayClient) doJSON(ctx context.Context, method, path string, body any, target any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return ErrInvalidRequest
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return ErrInvalidRequest
	}
	request.SetBasicAuth(c.keyID, c.keySecret)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("Razorpay request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		// Provider error bodies may contain request and account details; never propagate or log them.
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderBody))
		return fmt.Errorf("Razorpay request failed with HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxProviderBody+1))
	if err := decoder.Decode(target); err != nil {
		return ErrProviderResponse
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrProviderResponse
	}
	return nil
}
