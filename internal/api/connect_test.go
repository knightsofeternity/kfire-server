package api

import "testing"

func TestSignVerifyState(t *testing.T) {
	secret := []byte("test-secret")
	token := signState(secret, "user-123")

	got, ok := verifyState(secret, token)
	if !ok || got != "user-123" {
		t.Fatalf("round trip failed: got %q ok=%v", got, ok)
	}
}

func TestVerifyStateRejectsTampering(t *testing.T) {
	secret := []byte("test-secret")
	token := signState(secret, "user-123")

	// Swap the user id but keep the original signature.
	parts := token
	tampered := "attacker" + parts[len("user-123"):]
	if _, ok := verifyState(secret, tampered); ok {
		t.Fatal("tampered state accepted")
	}

	if _, ok := verifyState([]byte("other-secret"), token); ok {
		t.Fatal("state validated under the wrong secret")
	}

	for _, bad := range []string{"", "a.b", "a.b.c.d", "noseparators"} {
		if _, ok := verifyState(secret, bad); ok {
			t.Errorf("malformed state accepted: %q", bad)
		}
	}
}
