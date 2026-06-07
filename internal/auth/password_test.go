package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	const password = "correct horse battery staple"

	encoded, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("unexpected encoding: %s", encoded)
	}

	ok, err := VerifyPassword(password, encoded)
	if err != nil || !ok {
		t.Fatalf("correct password rejected: ok=%v err=%v", ok, err)
	}

	ok, err = VerifyPassword("wrong password", encoded)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Fatal("wrong password accepted")
	}
}

func TestHashPasswordUsesUniqueSalts(t *testing.T) {
	a, _ := HashPassword("same password")
	b, _ := HashPassword("same password")
	if a == b {
		t.Fatal("two hashes of the same password are identical (salt reuse)")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	for _, encoded := range []string{
		"",
		"$argon2id$v=19$m=65536,t=3,p=2$notbase64!!$notbase64!!",
		"$bcrypt$whatever",
		"$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA", // wrong version
	} {
		if _, err := VerifyPassword("x", encoded); err == nil {
			t.Errorf("malformed hash accepted: %q", encoded)
		}
	}
}
