package security

import (
	"testing"
	"time"
)

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("strong-password", 10)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "strong-password" {
		t.Fatal("password hash must not equal raw password")
	}
	if !CheckPassword(hash, "strong-password") {
		t.Fatal("CheckPassword rejected correct password")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("CheckPassword accepted wrong password")
	}
}

func TestAccessTokenSignAndParse(t *testing.T) {
	secret := "test-secret-with-at-least-32-characters"
	token, err := SignAccessToken("U-TEST", secret, time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken returned error: %v", err)
	}
	claims, err := ParseAccessToken(token, secret)
	if err != nil {
		t.Fatalf("ParseAccessToken returned error: %v", err)
	}
	if claims.UserID != "U-TEST" {
		t.Fatalf("expected user id U-TEST, got %s", claims.UserID)
	}
}

func TestOpaqueTokenAndHash(t *testing.T) {
	token, err := NewOpaqueToken(32)
	if err != nil {
		t.Fatalf("NewOpaqueToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("token must not be empty")
	}
	hashA := HashToken(token)
	hashB := HashToken(token)
	if hashA == "" || hashB == "" {
		t.Fatal("hash must not be empty")
	}
	if hashA != hashB {
		t.Fatal("HashToken must be deterministic")
	}
	if hashA == token {
		t.Fatal("token hash must not equal token")
	}
}
