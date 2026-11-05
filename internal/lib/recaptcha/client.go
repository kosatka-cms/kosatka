package recaptcha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const siteVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

// Verifier checks a user response token with Google's siteverify API.
type Verifier interface {
	Verify(ctx context.Context, responseToken, remoteIP string) (bool, error)
}

// Client implements Verifier for Google reCAPTCHA v2/v3 using the secret key.
type Client struct {
	secret     string
	httpClient *http.Client
	verifyURL  string
}

// NewClient returns a verifier backed by Google's API. secret must be
// non-empty; timeout bounds a verification request.
func NewClient(secret string, timeout time.Duration) *Client {
	return &Client{
		secret: strings.TrimSpace(secret),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		verifyURL: siteVerifyURL,
	}
}

type siteVerifyResponse struct {
	Success bool `json:"success"`
}

// Verify POSTs to siteverify and returns whether Google accepted the token.
func (c *Client) Verify(ctx context.Context, responseToken, remoteIP string) (bool, error) {
	if c == nil || c.secret == "" {
		return false, fmt.Errorf("recaptcha: missing secret")
	}
	token := strings.TrimSpace(responseToken)
	if token == "" {
		return false, nil
	}

	form := url.Values{}
	form.Set("secret", c.secret)
	form.Set("response", token)
	if rip := strings.TrimSpace(remoteIP); rip != "" {
		form.Set("remoteip", rip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("recaptcha: siteverify status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out siteVerifyResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("recaptcha: decode response: %w", err)
	}
	return out.Success, nil
}
