package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenIssuer   = "koom-api"
	accessTokenAudience = "koom-mobile"
)

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func SignAccessToken(userID, secret string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    accessTokenIssuer,
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseAccessToken(tokenString, secret string) (Claims, error) {
	claims := Claims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return Claims{}, err
	}
	if !token.Valid {
		return Claims{}, fmt.Errorf("invalid token")
	}
	if claims.UserID == "" {
		return Claims{}, fmt.Errorf("missing user id")
	}
	if claims.Issuer != accessTokenIssuer {
		return Claims{}, fmt.Errorf("invalid token issuer")
	}
	if !hasAudience(claims.Audience, accessTokenAudience) {
		return Claims{}, fmt.Errorf("invalid token audience")
	}
	return claims, nil
}

func hasAudience(audiences jwt.ClaimStrings, expected string) bool {
	for _, audience := range audiences {
		if audience == expected {
			return true
		}
	}
	return false
}
