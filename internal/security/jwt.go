package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenIssuer    = "koom-api"
	accessTokenAudience  = "koom-mobile"
	webSocketTokenIssuer = "koom-api"
	webSocketAudience    = "koom-websocket"
)

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func SignAccessToken(userID, secret string, ttl time.Duration) (string, error) {
	return signToken(userID, secret, ttl, accessTokenIssuer, accessTokenAudience)
}

func ParseAccessToken(tokenString, secret string) (Claims, error) {
	return parseToken(tokenString, secret, accessTokenIssuer, accessTokenAudience)
}

func SignWebSocketToken(userID, secret string, ttl time.Duration) (string, error) {
	return signToken(userID, secret, ttl, webSocketTokenIssuer, webSocketAudience)
}

func ParseWebSocketToken(tokenString, secret string) (Claims, error) {
	return parseToken(tokenString, secret, webSocketTokenIssuer, webSocketAudience)
}

func signToken(userID, secret string, ttl time.Duration, issuer string, audience string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func parseToken(tokenString, secret string, issuer string, audience string) (Claims, error) {
	claims := Claims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
	)
	token, err := parser.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
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
	if claims.UserID == "" || claims.Subject == "" || claims.UserID != claims.Subject {
		return Claims{}, fmt.Errorf("invalid token subject")
	}
	return claims, nil
}
