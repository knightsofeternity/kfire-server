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
