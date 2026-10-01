package psn

import (
	"github.com/knightsofeternity/kfire-server/internal/gametitle"
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

// NormalizeTitle reduces a PlayStation Store name to the catalog's name. The
// rules are shared with the other stores, see internal/gametitle.
func NormalizeTitle(name string) string { return gametitle.Normalize(name) }
