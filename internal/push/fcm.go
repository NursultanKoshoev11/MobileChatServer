package push

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
)

const firebaseMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"

type FCMNotifier struct {
	ProjectID   string
	ClientEmail string
	PrivateKey  string
	HTTPClient  *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func (n *FCMNotifier) Enabled() bool {
	return strings.TrimSpace(n.ProjectID) != "" && strings.TrimSpace(n.ClientEmail) != "" && strings.TrimSpace(n.PrivateKey) != ""
}

func (n *FCMNotifier) SendToTokens(ctx context.Context, tokens []string, message service.PushMessage) error {
	if !n.Enabled() || len(tokens) == 0 {
		return nil
	}
	accessToken, err := n.getAccessToken(ctx)
	if err != nil {
		return err
	}
	client := n.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	seen := map[string]bool{}
	failed := 0
	var lastErr error
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		if err := n.sendOne(ctx, client, accessToken, token, message); err != nil {
			failed++
			lastErr = err
			continue
		}
	}
	if failed > 0 {
		return fmt.Errorf("fcm send failed for %d token(s): %w", failed, lastErr)
	}
	return nil
}

func (n *FCMNotifier) sendOne(ctx context.Context, client *http.Client, accessToken, token string, message service.PushMessage) error {
	payload := map[string]any{
		"message": map[string]any{
			"token": token,
			"notification": map[string]string{
				"title": message.Title,
				"body":  message.Body,
			},
			"data": message.Data,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", url.PathEscape(n.ProjectID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	text, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	return fmt.Errorf("fcm send failed: status=%d body=%s", res.StatusCode, string(text))
}

func (n *FCMNotifier) getAccessToken(ctx context.Context) (string, error) {
	n.mu.Lock()
	if n.accessToken != "" && time.Now().Before(n.expiresAt.Add(-5*time.Minute)) {
		token := n.accessToken
		n.mu.Unlock()
		return token, nil
	}
	n.mu.Unlock()

	assertion, err := n.signedJWT()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := n.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var response struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || response.AccessToken == "" {
		return "", fmt.Errorf("fcm token failed: status=%d error=%s description=%s", res.StatusCode, response.Error, response.ErrorDesc)
	}
	expiresIn := response.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	n.mu.Lock()
	n.accessToken = response.AccessToken
	n.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	n.mu.Unlock()
	return response.AccessToken, nil
}

func (n *FCMNotifier) signedJWT() (string, error) {
	privateKey, err := parsePrivateKey(n.PrivateKey)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iss":   n.ClientEmail,
		"scope": firebaseMessagingScope,
		"aud":   "https://oauth2.googleapis.com/token",
		"iat":   now,
		"exp":   now + 3600,
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\n", "\n")
	raw = strings.ReplaceAll(raw, "\\r", "")
	raw = strings.ReplaceAll(raw, "\\u003d", "=")
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("invalid private key pem")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return rsaKey, nil
}
