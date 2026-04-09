package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encPrefix = "dv1:"

// MasterCipher provides AES-256-GCM encryption for block content at rest (SQLite text column).
type MasterCipher struct {
	gcm cipher.AEAD
}

// NewMasterCipher derives a 256-bit key from passphrase using SHA-256 and builds GCM.
func NewMasterCipher(passphrase string) (*MasterCipher, error) {
	if strings.TrimSpace(passphrase) == "" {
		return nil, fmt.Errorf("empty master key")
	}
	sum := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &MasterCipher{gcm: gcm}, nil
}

// EncryptString returns a prefixed base64 payload (nonce || ciphertext).
func (m *MasterCipher) EncryptString(plain string) (string, error) {
	if m == nil {
		return plain, nil
	}
	nonce := make([]byte, m.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := m.gcm.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, ct...)
	return encPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

// DecryptString reverses EncryptString. Returns an error if the value is not decryptable.
func (m *MasterCipher) DecryptString(s string) (string, error) {
	if m == nil {
		return s, nil
	}
	if !strings.HasPrefix(s, encPrefix) {
		return "", errors.New("not encrypted blob")
	}
	raw, err := base64.RawStdEncoding.DecodeString(s[len(encPrefix):])
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	ns := m.gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("truncated ciphertext")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := m.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Store) revealContent(stored string) string {
	if s == nil || s.masterCipher == nil {
		return stored
	}
	plain, err := s.masterCipher.DecryptString(stored)
	if err != nil {
		return stored
	}
	return plain
}

func (s *Store) sealContent(plain string) (string, error) {
	if s == nil || s.masterCipher == nil {
		return plain, nil
	}
	return s.masterCipher.EncryptString(plain)
}
