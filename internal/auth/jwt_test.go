package auth

import (
	"testing"
	"time"
)

func TestJWTMintAndParse(t *testing.T) {
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
