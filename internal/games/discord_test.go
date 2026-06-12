package games

import (
	"reflect"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Counter-Strike 2":        "counter-strike-2",
		"World of Warcraft":       "world-of-warcraft",
		"Baldur's Gate 3":         "baldur-s-gate-3",
		"  -- Weird   name! --  ": "weird-name",
		"日本語のみ":                   "game",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	apps := []detectableApp{
		{
			ID:       "123",
			Name:     "World of Warcraft",
			IconHash: "abc",
			Executables: []struct {
				Name       string `json:"name"`
				OS         string `json:"os"`
				IsLauncher bool   `json:"is_launcher"`
			}{
				{Name: "_retail_/Wow.exe", OS: "win32"},
				{Name: `_ptr_\wow.exe`, OS: "win32"}, // backslash path, dup after basename
				{Name: "world of warcraft.app", OS: "darwin"},
				{Name: "battle.net.exe", OS: "win32", IsLauncher: true}, // excluded
			},
		},
		{ID: "456", Name: "Launcher Only", Executables: []struct {
			Name       string `json:"name"`
			OS         string `json:"os"`
			IsLauncher bool   `json:"is_launcher"`
		}{{Name: "launcher.exe", OS: "win32", IsLauncher: true}}},
		{ID: "789", Name: "No Executables"},
	}

	seeds := normalize(apps)
	if len(seeds) != 1 {
		t.Fatalf("expected 1 seed (launcher-only and exe-less excluded), got %d", len(seeds))
	}

	s := seeds[0]
	if s.DiscordAppID != "123" || s.Slug != "world-of-warcraft" {
		t.Errorf("unexpected seed identity: %+v", s)
	}
	wantExes := []string{"wow.exe", "world of warcraft.app"}
	if !reflect.DeepEqual(s.ExecutableNames, wantExes) {
		t.Errorf("executables = %v, want %v", s.ExecutableNames, wantExes)
	}
	if s.IconURL != "https://cdn.discordapp.com/app-icons/123/abc.png" {
		t.Errorf("unexpected icon URL: %s", s.IconURL)
	}
}

// exeList builds the anonymous Executables slice from plain names (non-launcher).
func exeList(names ...string) []struct {
	Name       string `json:"name"`
	OS         string `json:"os"`
	IsLauncher bool   `json:"is_launcher"`
} {
	out := make([]struct {
		Name       string `json:"name"`
		OS         string `json:"os"`
		IsLauncher bool   `json:"is_launcher"`
	}, len(names))
	for i, n := range names {
		out[i].Name = n
	}
	return out
}

func TestNormalizeFiltersGenericExecutables(t *testing.T) {
	apps := []detectableApp{
		// Blocklisted helper (tap.exe) dropped, specific exe kept.
		{ID: "spell", Name: "Spellcraft", Executables: exeList("tap.exe", "unitycrashhandler64.exe", "Spellcraft.exe")},
		// "game.exe" appears in 5 apps below ⇒ frequency filter drops it everywhere.
		{ID: "g1", Name: "Game One", Executables: exeList("game.exe", "gameone.exe")},
		{ID: "g2", Name: "Game Two", Executables: exeList("game.exe", "gametwo.exe")},
		{ID: "g3", Name: "Game Three", Executables: exeList("game.exe", "gamethree.exe")},
		{ID: "g4", Name: "Game Four", Executables: exeList("game.exe", "gamefour.exe")},
		// Only generic names ⇒ no specific exe left ⇒ excluded entirely.
		{ID: "allgen", Name: "All Generic", Executables: exeList("game.exe")},
	}

	byID := map[string][]string{}
	for _, s := range normalize(apps) {
		byID[s.DiscordAppID] = s.ExecutableNames
	}

	if got, want := byID["spell"], []string{"spellcraft.exe"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Spellcraft executables = %v, want %v (tap.exe + crash handler dropped)", got, want)
	}
	if got, want := byID["g1"], []string{"gameone.exe"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Game One executables = %v, want %v (game.exe dropped by frequency)", got, want)
	}
	if _, ok := byID["allgen"]; ok {
		t.Error("All Generic should be excluded: its only executable is generic")
	}
}

func TestNormalizeAddsCuratedExeForGameDiscordMisses(t *testing.T) {
	// Discord lists Vampire Crawlers (steam 3265700) with an empty executables
	// array, so without a curated entry normalize() drops it and the client can
	// never detect it. The curated exe must bring it back into the catalog.
	apps := []detectableApp{
		{ID: "vc", Name: "Vampire Crawlers: The Turbo Wildcard from Vampire Survivors"},
	}

	seeds := normalize(apps)
	if len(seeds) != 1 {
		t.Fatalf("expected Vampire Crawlers seeded via curated exe, got %d seeds", len(seeds))
	}
	if got, want := seeds[0].ExecutableNames, []string{"vampire crawlers.exe"}; !reflect.DeepEqual(got, want) {
		t.Errorf("executables = %v, want %v", got, want)
	}
}

func TestNormalizeExcludesWrongGameExecutable(t *testing.T) {
	// Discord lists Enlisted's binary (enlisted.exe) under CRSED: F.O.A.D.;
	// it must be dropped from CRSED while a real exe is kept.
	apps := []detectableApp{
		{ID: "crsed", Name: "CRSED: F.O.A.D.", Executables: exeList("win64/enlisted.exe", "win64/cuisine_royale.exe")},
	}
	seeds := normalize(apps)
	if len(seeds) != 1 {
		t.Fatalf("expected CRSED seeded (keeps cuisine_royale.exe), got %d", len(seeds))
	}
	for _, e := range seeds[0].ExecutableNames {
		if e == "enlisted.exe" {
			t.Errorf("CRSED must not carry enlisted.exe: %v", seeds[0].ExecutableNames)
		}
	}
	if want := []string{"cuisine_royale.exe"}; !reflect.DeepEqual(seeds[0].ExecutableNames, want) {
		t.Errorf("executables = %v, want %v", seeds[0].ExecutableNames, want)
	}
}

func TestNormalizeSkipsTestVariantsAndInstallers(t *testing.T) {
	apps := []detectableApp{
		{ID: "pubg", Name: "PUBG: BATTLEGROUNDS", Executables: exeList("TslGame.exe")},
		{ID: "pubgtest", Name: "PUBG: Test Server", Executables: exeList("execpubg.exe")},
		{ID: "hd2", Name: "Hidden & Dangerous 2: Courage Under Fire", Executables: exeList("setup.exe")},
		{ID: "steel", Name: "Steel Circus", Executables: exeList("sc.exe")},
	}

	byID := map[string][]string{}
	for _, s := range normalize(apps) {
		byID[s.DiscordAppID] = s.ExecutableNames
	}

	if got, want := byID["pubg"], []string{"tslgame.exe"}; !reflect.DeepEqual(got, want) {
		t.Errorf("PUBG executables = %v, want %v", got, want)
	}
	if _, ok := byID["pubgtest"]; ok {
		t.Error("PUBG: Test Server should be excluded (non-retail test variant)")
	}
	if _, ok := byID["hd2"]; ok {
		t.Error("Hidden & Dangerous 2 should be excluded: only generic setup.exe")
	}
	if _, ok := byID["steel"]; ok {
		t.Error("Steel Circus should be excluded: sc.exe is a system tool / too short")
	}
}
