package epic

import (
	"regexp"
	"strings"
)

var (
	codeField = regexp.MustCompile(`"authorizationCode"\s*:\s*"([0-9a-f]{32})"`)
	bareCode  = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// ParseCode reads the authorizationCode out of what a member pasted: the
// whole JSON page Epic shows, or the code alone. The page also carries the
// 32-hex clientId, so only the named field or a bare code is accepted.
func ParseCode(pasted string) (string, bool) {
	if m := codeField.FindStringSubmatch(pasted); m != nil {
		return m[1], true
	}
	s := strings.TrimSpace(pasted)
	if bareCode.MatchString(s) {
		return s, true
	}
	return "", false
}
