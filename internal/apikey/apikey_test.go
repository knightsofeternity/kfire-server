package apikey

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	full, prefix, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(full, "kfire_") {
		t.Errorf("full key %q must start with kfire_", full)
	}
	if len(prefix) != 12 || !strings.HasPrefix(full, prefix) {
		t.Errorf("prefix %q must be the first 12 chars of %q", prefix, full)
	}
	if len(hash) != 32 {
		t.Errorf("hash must be 32 bytes (sha256), got %d", len(hash))
	}
	if got := Hash(full); string(got) != string(hash) {
		t.Errorf("Hash(full) != returned hash")
	}
}

func TestGenerateUnique(t *testing.T) {
	a, _, _, _ := Generate()
	b, _, _, _ := Generate()
	if a == b {
		t.Errorf("two generated keys must differ")
	}
}

func TestHashDeterministic(t *testing.T) {
	if string(Hash("kfire_abc")) != string(Hash("kfire_abc")) {
		t.Errorf("Hash must be deterministic")
	}
	if string(Hash("kfire_abc")) == string(Hash("kfire_abd")) {
		t.Errorf("different inputs must hash differently")
	}
}
