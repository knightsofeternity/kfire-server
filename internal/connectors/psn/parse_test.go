package psn

import "testing"

func TestParseDuration(t *testing.T) {
	cases := map[string]int64{
		"PT626H29M12S": 626*3600 + 29*60 + 12,
		"PT50H22M12S":  50*3600 + 22*60 + 12,
		"PT45M":        45 * 60,
		"PT30S":        30,
		"PT0S":         0,
		"P1DT2H":       26 * 3600,
		"":             0,
		"garbage":      0,
	}
	for in, want := range cases {
		if got := ParseDuration(in); got != want {
			t.Errorf("ParseDuration(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestNormalizeTitle(t *testing.T) {
	cases := map[string]string{
		"Diablo® IV":                        "Diablo IV",
		"Gran Turismo™ 7":                   "Gran Turismo 7",
		"Kena: Bridge of Spirits PS4 & PS5": "Kena: Bridge of Spirits",
		"Marvel’s Wolverine":                "Marvel’s Wolverine",
		"Minecraft : PlayStation®4 Edition": "Minecraft",
		"FINAL FANTASY XVI":                 "FINAL FANTASY XVI",
		"Hogwarts Legacy PS5 Version":       "Hogwarts Legacy",
		"Astro Bot - PS5":                   "Astro Bot",
		"  The Outer Worlds 2  ":            "The Outer Worlds 2",
	}
	for in, want := range cases {
		if got := NormalizeTitle(in); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
