package api

import "testing"

func TestWeakPassword(t *testing.T) {
	weak := []struct{ pw, user, email string }{
		{"password1234", "robin", "robin@org.test"},       // common
		{"robin-the-best", "robin", "robin@org.test"},     // contains username
		{"sunfish99-mail", "robin", "sunfish99@org.test"}, // contains email local part
		{"aaaaaaaaaaaa", "robin", "robin@org.test"},       // single repeated rune
	}
	for _, c := range weak {
		if !weakPassword(c.pw, c.user, c.email) {
			t.Errorf("expected weak: %q", c.pw)
		}
	}

	strong := []struct{ pw, user, email string }{
		{"correct horse battery", "robin", "robin@org.test"},
		{"7Gk!tangerine-cloud", "robin", "robin@org.test"},
	}
	for _, c := range strong {
		if weakPassword(c.pw, c.user, c.email) {
			t.Errorf("expected strong: %q", c.pw)
		}
	}
}
