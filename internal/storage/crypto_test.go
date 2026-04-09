package storage

import "testing"

func TestMasterCipherRoundTrip(t *testing.T) {
	t.Parallel()
	c, err := NewMasterCipher("test-passphrase-for-unit-test")
	if err != nil {
		t.Fatal(err)
	}
	plain := "hello 世界 [[link]]"
	enc, err := c.EncryptString(plain)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("expected ciphertext")
	}
	out, err := c.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if out != plain {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestRevealContentPlaintext(t *testing.T) {
	t.Parallel()
	s := &Store{}
	if got := s.revealContent("plain"); got != "plain" {
		t.Fatalf("got %q", got)
	}
}
