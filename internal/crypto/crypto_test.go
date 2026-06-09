package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func newKey(t *testing.T) string {
	t.Helper()
	k := make([]byte, 32)
	rand.Read(k)
	return base64.StdEncoding.EncodeToString(k)
}

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := New(newKey(t))
	if err != nil {
		t.Fatal(err)
	}
	const secret = "oauth-access-token-12345"
	sealed, err := c.SealString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(secret)) {
		t.Fatal("plaintext leaked into ciphertext")
	}
	got, err := c.OpenString(sealed)
	if err != nil || got != secret {
		t.Fatalf("round trip failed: got %q err %v", got, err)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	c, _ := New(newKey(t))
	a, _ := c.SealString("same")
	b, _ := c.SealString("same")
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same value are identical (nonce reuse)")
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	c, _ := New(newKey(t))
	sealed, _ := c.SealString("secret")
	sealed[len(sealed)-1] ^= 0xff // flip a tag bit
	if _, err := c.Open(sealed); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestNewRejectsBadKey(t *testing.T) {
	for _, k := range []string{"", "not-base64!!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := New(k); err == nil {
			t.Errorf("accepted bad key: %q", k)
		}
	}
}

func TestWrongKeyCannotOpen(t *testing.T) {
	c1, _ := New(newKey(t))
	c2, _ := New(newKey(t))
	sealed, _ := c1.SealString("secret")
	if _, err := c2.Open(sealed); err == nil {
		t.Fatal("a different key opened the ciphertext")
	}
}
