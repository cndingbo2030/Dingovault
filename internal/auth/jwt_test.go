package auth

import (
	"testing"
	"time"
)

func TestJWTMintAndParse(t *testing.T) {
	t.Setenv("DINGO_ENV", "")
	t.Setenv("DINGO_JWT_SECRET", "")
	j, err := NewJWTFromEnv("test-iss", time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := j.MintAccessToken("user-42")
	if err != nil {
		t.Fatal(err)
	}
	c, err := j.ParseAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "user-42" {
		t.Fatalf("subject %q", c.Subject)
	}
}

func TestJWTProductionRequiresSecret(t *testing.T) {
	t.Setenv("DINGO_ENV", "production")
	t.Setenv("DINGO_JWT_SECRET", "")
	_, err := NewJWTFromEnv("test-iss", time.Hour, true)
	if err == nil {
		t.Fatal("expected error when DINGO_JWT_SECRET unset in production")
	}
}

func TestJWTProductionRejectsDefaultSecret(t *testing.T) {
	t.Setenv("DINGO_ENV", "prod")
	t.Setenv("DINGO_JWT_SECRET", DefaultDevSecret)
	_, err := NewJWTFromEnv("test-iss", time.Hour, true)
	if err == nil {
		t.Fatal("expected error when using default dev secret in production")
	}
}
