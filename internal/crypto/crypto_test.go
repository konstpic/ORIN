package crypto

import (
	"bytes"
	"strings"
	"testing"
)

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRoundTrip(t *testing.T) {
	c, err := New(testKey)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for _, plaintext := range []string{
		"hello",
		"",
		strings.Repeat("x", 4096),
		`{"user":"admin","password":"hunter2"}`,
	} {
		ct, err := c.Encrypt([]byte(plaintext))
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		got, err := c.Decrypt(ct)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if plaintext == "" && got != nil {
			t.Fatalf("empty plaintext should round-trip to nil; got %v", got)
		}
		if plaintext != "" && !bytes.Equal([]byte(plaintext), got) {
			t.Fatalf("mismatch: want %q got %q", plaintext, got)
		}
	}
}

func TestNonceRandomness(t *testing.T) {
	c, _ := New(testKey)
	a, _ := c.Encrypt([]byte("same"))
	b, _ := c.Encrypt([]byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same plaintext should differ (random nonce)")
	}
}

func TestBadKey(t *testing.T) {
	if _, err := New("tooshort"); err == nil {
		t.Fatal("expected error for short key")
	}
	if _, err := New("zzzzzz"); err == nil {
		t.Fatal("expected error for non-hex key")
	}
}
