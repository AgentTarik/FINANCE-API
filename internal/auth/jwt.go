package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTIssuer struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

func NewJWTIssuerFromEnv() (*JWTIssuer, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	iss := os.Getenv("JWT_ISS")
	if iss == "" {
		iss = "finance-api"
	}
	aud := os.Getenv("JWT_AUD")
	ttl := 15 * time.Minute
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		}
	}
	return &JWTIssuer{
		secret:   []byte(secret),
		issuer:   iss,
		audience: aud,
		ttl:      ttl,
	}, nil
}

func (j *JWTIssuer) Issue(userID string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(j.ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    j.issuer,
		Subject:   userID,
		Audience:  jwt.ClaimStrings{j.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)), // small skew
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	return signed, exp, err
}
