package psn

import (
	"regexp"
	"strconv"
	"strings"
)

var isoDuration = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

// ParseDuration turns Sony's ISO 8601 durations ("PT626H29M12S") into
// seconds. Anything unreadable counts as zero: a bad value must not stop an
// import, it only loses that one game's hours.
func ParseDuration(s string) int64 {
	m := isoDuration.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || s == "" {
		return 0
	}
	n := func(i int) int64 {
		if m[i] == "" {
			return 0
		}
		f, err := strconv.ParseFloat(m[i], 64)
		if err != nil {
			return 0
		}
		return int64(f)
	}
	return n(1)*86400 + n(2)*3600 + n(3)*60 + n(4)
}

var (
	trademarks = strings.NewReplacer("®", "", "™", "", "©", "")
	// Platform tails Sony appends to a store name, never part of the game's.
	platformTail = regexp.MustCompile(`(?i)\s*(?:[-:]\s*)?(?:PS4\s*(?:&|and|/)\s*PS5|PS5\s*(?:&|and|/)\s*PS4|PlayStation\s*4\s*Edition|PlayStation4\s*Edition|PS[45](?:\s*Version)?)\s*$`)
	spaces       = regexp.MustCompile(`\s+`)
)

// NormalizeTitle strips what makes the same game read differently on the
// PlayStation Store and elsewhere: trademark signs and platform tails. Its
// result is only ever slugged to find a catalog game, never shown.
func NormalizeTitle(name string) string {
	s := trademarks.Replace(name)
	s = spaces.ReplaceAllString(strings.TrimSpace(s), " ")
	for {
		t := strings.TrimSpace(platformTail.ReplaceAllString(s, ""))
		t = strings.TrimRight(t, " :-")
		if t == s || t == "" {
			break
		}
		s = t
	}
	return s
}
