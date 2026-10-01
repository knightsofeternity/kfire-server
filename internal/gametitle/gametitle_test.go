package gametitle

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		// PlayStation
		"Diablo® IV":                                "Diablo IV",
		"Gran Turismo™ 7":                           "Gran Turismo 7",
		"Kena: Bridge of Spirits PS4 & PS5":         "Kena: Bridge of Spirits",
		"Minecraft : PlayStation®4 Edition":         "Minecraft",
		"Diablo III: Eternal Collection (Français)": "Diablo III: Eternal Collection",
		"Hitman (2016)":                             "Hitman (2016)",
		// Nintendo, real names from a play log
		"Minecraft for Nintendo Switch":                "Minecraft",
		"Fortnite pour Nintendo Switch":                "Fortnite",
		"FIFA 23 Édition Essentielle":                  "FIFA 23",
		"Rocket League®":                               "Rocket League",
		"Mario Kart 8 Deluxe":                          "Mario Kart 8 Deluxe",
		"Kirby Air Riders – Nintendo Switch 2 Edition": "Kirby Air Riders",
		"Super Mario Party Jamboree – Nintendo Switch 2 Edition + Jamboree TV": "Super Mario Party Jamboree – Nintendo Switch 2 Edition + Jamboree TV",
		"Hades II Essential Edition": "Hades II",
		"Nintendo Switch Sports":     "Nintendo Switch Sports",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
