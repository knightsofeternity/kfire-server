// Package games seeds the games catalog from Discord's public "detectable
// applications" list — the same database Discord uses for its own game
// detection. ~10k games ship at least one detectable executable.
package games

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// DetectableURL is Discord's public (unauthenticated) detectable games list.
const DetectableURL = "https://discord.com/api/v10/applications/detectable"

type detectableApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IconHash    string `json:"icon_hash"`
	Executables []struct {
		Name       string `json:"name"`
		OS         string `json:"os"`
		IsLauncher bool   `json:"is_launcher"`
	} `json:"executables"`
}

// FetchSeed downloads and normalizes the Discord list into game seeds.
func FetchSeed(ctx context.Context) ([]store.GameSeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DetectableURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kfire-server (https://github.com/knightsofeternity/kfire-server)")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch detectable list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch detectable list: HTTP %d", resp.StatusCode)
	}

	var apps []detectableApp
	if err := json.NewDecoder(resp.Body).Decode(&apps); err != nil {
		return nil, fmt.Errorf("decode detectable list: %w", err)
	}
	return normalize(apps), nil
}

// normalize keeps games with at least one non-launcher executable and cleans
// up executable names for process matching.
func normalize(apps []detectableApp) []store.GameSeed {
	seeds := make([]store.GameSeed, 0, len(apps))
	for _, app := range apps {
		exes := make([]string, 0, len(app.Executables))
		seen := map[string]struct{}{}
		for _, exe := range app.Executables {
			if exe.IsLauncher {
				// "Playing Battle.net" is noise, not presence.
				continue
			}
			// Discord ships paths like "_retail_/wow.exe"; the client matches
			// on the process basename, lowercased.
			name := strings.ToLower(path.Base(strings.ReplaceAll(exe.Name, `\`, `/`)))
			if name == "" || name == "." {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			exes = append(exes, name)
		}
		if len(exes) == 0 {
			continue
		}

		var iconURL string
		if app.IconHash != "" {
			iconURL = fmt.Sprintf("https://cdn.discordapp.com/app-icons/%s/%s.png", app.ID, app.IconHash)
		}

		seeds = append(seeds, store.GameSeed{
			DiscordAppID:    app.ID,
			Name:            app.Name,
			Slug:            Slugify(app.Name),
			ExecutableNames: exes,
			IconURL:         iconURL,
		})
	}
	return seeds
}

var (
	slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)
	slugDashes  = regexp.MustCompile(`-{2,}`)
)

// Slugify converts a game name to a URL-safe slug ("Counter-Strike 2" →
// "counter-strike-2"). Uniqueness across the catalog is handled at insert
// time by store.UpsertGames.
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = slugInvalid.ReplaceAllString(s, "-")
	s = slugDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "game"
	}
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}
