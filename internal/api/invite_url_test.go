package api

import "testing"

func TestInviteURL(t *testing.T) {
	cases := []struct {
		publicURL string
		code      string
		want      string
	}{
		{"https://kfire.io", "abc123", "https://kfire.io/?invite=abc123"},
		{"https://kfire.io/", "abc123", "https://kfire.io//?invite=abc123"}, // documents current behaviour: no trailing-slash normalisation
		{"", "x", "/?invite=x"},
	}
	for _, tc := range cases {
		if got := inviteURL(tc.publicURL, tc.code); got != tc.want {
			t.Errorf("inviteURL(%q,%q) = %q, want %q", tc.publicURL, tc.code, got, tc.want)
		}
	}
}
