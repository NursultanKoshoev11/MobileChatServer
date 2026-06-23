package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const SignedInvitePrefix = "INV1"

type SignedInviteClaims struct {
	GroupID   string `json:"group_id"`
	Code      string `json:"invite_code"`
	ExpiresAt int64  `json:"exp"`
}

func SignSignedInvite(groupID string, code string, secret string, ttl time.Duration) (string, error) {
	claims := SignedInviteClaims{GroupID: strings.TrimSpace(groupID), Code: strings.TrimSpace(code), ExpiresAt: time.Now().UTC().Add(ttl).Unix()}
	if claims.GroupID == "" || claims.Code == "" {
		return "", fmt.Errorf("signed invite requires group and code")
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return SignedInvitePrefix + "." + payload + "." + signSignedInvitePayload(payload, secret), nil
}

func ParseSignedInvite(value string, secret string) (SignedInviteClaims, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 3 || parts[0] != SignedInvitePrefix {
		return SignedInviteClaims{}, fmt.Errorf("invalid signed invite format")
	}
	expected := signSignedInvitePayload(parts[1], secret)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return SignedInviteClaims{}, fmt.Errorf("invalid signed invite signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return SignedInviteClaims{}, err
	}
	var claims SignedInviteClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return SignedInviteClaims{}, err
	}
	if claims.GroupID == "" || claims.Code == "" {
		return SignedInviteClaims{}, fmt.Errorf("invalid signed invite claims")
	}
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return SignedInviteClaims{}, fmt.Errorf("signed invite expired")
	}
	return claims, nil
}

func signSignedInvitePayload(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func MakeQR(groupID string, code string, secret string, ttl time.Duration) (string, error) {
	return SignSignedInvite(groupID, code, secret, ttl)
}

func ReadQR(value string, secret string) (SignedInviteClaims, error) {
	return ParseSignedInvite(value, secret)
}

func MakeQREnv(groupID string, code string, ttl time.Duration) (string, error) {
	return MakeQR(groupID, code, os.Getenv("JWT_SECRET"), ttl)
}
