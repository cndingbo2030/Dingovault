package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultDevSecret is used only when DINGO_JWT_SECRET is unset (development).
const DefaultDevSecret = "dingovault-dev-only-change-me"

// Claims is the JWT payload (subject = user id).
type Claims struct {
	jwt.RegisteredClaims
}

// JWT issues and verifies access tokens.
type JWT struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewJWTFromEnv builds a JWT helper using DINGO_JWT_SECRET (or dev default if allowDevDefault).
// In production (DINGO_ENV=production), DINGO_JWT_SECRET must be set to a non-default value.
func NewJWTFromEnv(issuer string, ttl time.Duration, allowDevDefault bool) (*JWT, error) {
	raw := strings.TrimSpace(os.Getenv("DINGO_JWT_SECRET"))
	var sec string
	if raw != "" {
		sec = raw
	} else {
		if IsProduction() {
			return nil, errors.New("DINGO_ENV=production requires DINGO_JWT_SECRET to be set")
		}
		if !allowDevDefault {
			return nil, errors.New("DINGO_JWT_SECRET is required for SaaS API")
		}
		sec = DefaultDevSecret
	}
	if IsProduction() && sec == DefaultDevSecret {
		return nil, errors.New("production: DINGO_JWT_SECRET must not equal the built-in development default; set a unique secret")
	}
	if len(sec) < 16 {
		return nil, fmt.Errorf("DINGO_JWT_SECRET must be at least 16 bytes")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if issuer == "" {
		issuer = "dingovault"
	}
	return &JWT{secret: []byte(sec), issuer: issuer, ttl: ttl}, nil
}

// MintAccessToken returns a signed JWT for userID (JWT "sub" claim).
func (j *JWT) MintAccessToken(userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("empty user id")
	}
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	return t.SignedString(j.secret)
}

// ParseAccessToken validates a bearer token and returns claims.
func (j *JWT) ParseAccessToken(tokenString string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token claims")
	}
	if strings.TrimSpace(c.Subject) == "" {
		return nil, errors.New("token missing subject")
	}
	return c, nil
}
