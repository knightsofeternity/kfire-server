// Package gametitle reduces a store's game name to the name the catalog knows,
// so the same game bought on PlayStation, Switch and PC lands on one page:
// console hours then add up with PC hours. The result is only ever slugged to
// find a catalog game, never shown.
package gametitle

import (
	"regexp"
	"strings"
)

var (
	trademarks = strings.NewReplacer("®", "", "™", "", "©", "")
	spaces     = regexp.MustCompile(`\s+`)
	// Platform tails the PlayStation Store appends.
	psnTail = regexp.MustCompile(`(?i)\s*(?:[-:]\s*)?(?:PS4\s*(?:&|and|/)\s*PS5|PS5\s*(?:&|and|/)\s*PS4|PlayStation\s*4\s*Edition|PlayStation4\s*Edition|PS[45](?:\s*Version)?)\s*$`)
	// Platform and edition tails of the Nintendo eShop. Only at the very end:
	// "Jamboree - Nintendo Switch 2 Edition + Jamboree TV" is a different
	// product and keeps its name.
	nintendoTail = regexp.MustCompile(`(?i)\s*(?:[-:\x{2013}\x{2014}]\s*)?(?:(?:for|pour)\s+(?:the\s+)?Nintendo\s+Switch(?:\s*2)?|Nintendo\s+Switch\s*2\s+Edition|[ÉE]dition\s+Essentielle|Essential\s+Edition)\s*$`)
	// Language tags of regional releases: "(Français)", "(Deutsch)",
	// "(English/Chinese/Korean/Japanese Ver.)". A year in brackets is part of
	// the name ("Hitman (2016)") and is kept.
	languageTail = regexp.MustCompile(`(?i)\s*\((?:[^()]*\bVer\.?|Fran[cç]ais|French|English|Deutsch|German|Espa[nñ]ol|Spanish|Italiano|Italian|Portugu[eê]s|Portuguese|Nederlands|Polski|Русский|日本語|中文|한국어)\)\s*$`)
)

// Normalize strips trademark signs and the platform, edition and language
// tails a store adds to a game's name.
func Normalize(name string) string {
	s := trademarks.Replace(name)
	s = spaces.ReplaceAllString(strings.TrimSpace(s), " ")
	for {
		t := strings.TrimSpace(psnTail.ReplaceAllString(s, ""))
		t = strings.TrimSpace(nintendoTail.ReplaceAllString(t, ""))
		t = strings.TrimSpace(languageTail.ReplaceAllString(t, ""))
		t = strings.TrimRight(t, " :-–—")
		if t == s || t == "" {
			break
		}
		s = t
	}
	return s
}
