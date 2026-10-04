package epic

import "testing"

func TestParseCode(t *testing.T) {
	const code = "0123456789abcdef0123456789abcdef"
	page := `{"warning":"Do not share","redirectUrl":"https://localhost/launcher/authorized?code=` + code +
		`","authorizationCode":"` + code + `","exchangeCode":null,"sid":null,"clientId":"34a02cf8f4414e29b15921876da36f9a"}`
	cases := map[string]string{
		page:               code,
		"  " + code + "\n": code,
		"":                 "",
		"not a code":       "",
		// The client id is 32 hex too: it must never be taken for the code.
		`{"clientId":"34a02cf8f4414e29b15921876da36f9a"}`: "",
	}
	for in, want := range cases {
		got, ok := ParseCode(in)
		if got != want || ok != (want != "") {
			t.Errorf("ParseCode(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
}
