package gog

import (
	"net/url"
	"regexp"
	"strings"
)

var bareCode = regexp.MustCompile(`^[A-Za-z0-9_-]{20,}$`)

// ParseCode reads the code out of what a member pasted: the whole address of
// the page GOG ends on (…/on_login_success?origin=client&code=…), or the code
// alone.
func ParseCode(pasted string) (string, bool) {
	s := strings.TrimSpace(pasted)
	if u, err := url.Parse(s); err == nil && u.RawQuery != "" {
		if code := u.Query().Get("code"); bareCode.MatchString(code) {
			return code, true
		}
		return "", false
	}
	if bareCode.MatchString(s) {
		return s, true
	}
	return "", false
}
