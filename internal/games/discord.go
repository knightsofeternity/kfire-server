// Package games seeds the games catalog from Discord's public "detectable
// applications" list - the same database Discord uses for its own game
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
	ID             string `json:"id"`
	Name           string `json:"name"`
	IconHash       string `json:"icon_hash"`
	CoverImageHash string `json:"cover_image_hash"`
	Executables    []struct {
		Name       string `json:"name"`
		OS         string `json:"os"`
		IsLauncher bool   `json:"is_launcher"`
	} `json:"executables"`
	ThirdPartySKUs []struct {
		Distributor string `json:"distributor"`
		ID          string `json:"id"`
	} `json:"third_party_skus"`
}

// steamAppID returns the app's Steam AppID from its third-party SKUs, if any.
func (a detectableApp) steamAppID() string {
	for _, sku := range a.ThirdPartySKUs {
		if sku.Distributor == "steam" && sku.ID != "" {
			return sku.ID
		}
	}
	return ""
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

// genericExecutables are helper/shared binaries that ship with many unrelated
// games - engine crash handlers, redistributables, runtimes, VPN/driver helpers.
// Matching presence on them produces false positives: a Unity crash handler or a
// VPN TAP adapter idling in the background would make a player look "in game".
var genericExecutables = map[string]struct{}{
	"unitycrashhandler64.exe": {}, "unitycrashhandler32.exe": {}, "unitycrashhandler.exe": {},
	"crashhandler.exe": {}, "crashreporter.exe": {}, "crashpad_handler.exe": {},
	"ueprereqsetup_x64.exe": {}, "ue4prereqsetup_x64.exe": {}, "prerequisites.exe": {},
	"tap.exe":     {},
	"game.exe":    {}, "launcher.exe": {}, "start.exe": {}, "play.exe": {}, "autorun.exe": {},
	"vc_redist.x64.exe": {}, "vc_redist.x86.exe": {}, "dxsetup.exe": {}, "dotnet.exe": {},
	"python.exe": {}, "pythonw.exe": {}, "java.exe": {}, "javaw.exe": {}, "mono.exe": {},
	"node.exe": {}, "nw.exe": {}, "cmd.exe": {},
	// Installers / updaters / config tools ship with countless games and match
	// whenever someone installs or configures anything (e.g. "Hidden & Dangerous
	// 2" was detected via setup.exe).
	"setup.exe": {}, "setup_x64.exe": {}, "setup_x86.exe": {},
	"install.exe": {}, "installer.exe": {}, "uninstall.exe": {}, "unins000.exe": {},
	"update.exe": {}, "updater.exe": {}, "launcher_updater.exe": {}, "patcher.exe": {},
	"config.exe": {}, "settings.exe": {}, "run.exe": {},
	// Windows system binaries: a catalog exe colliding with one of these matches
	// routine OS activity (e.g. "Steel Circus" was detected via sc.exe, the
	// Service Control tool). Short names are handled by the length guard below.
	"sc.exe": {}, "net.exe": {}, "net1.exe": {}, "reg.exe": {}, "ping.exe": {},
	"ftp.exe": {}, "find.exe": {}, "findstr.exe": {}, "sort.exe": {}, "more.exe": {},
	"tree.exe": {}, "mode.exe": {}, "where.exe": {}, "whoami.exe": {},
	"tasklist.exe": {}, "taskkill.exe": {}, "control.exe": {}, "regedit.exe": {},
	"rundll32.exe": {}, "svchost.exe": {}, "conhost.exe": {}, "dllhost.exe": {},
	"werfault.exe": {}, "explorer.exe": {}, "notepad.exe": {}, "mmc.exe": {},
	"powershell.exe": {}, "pwsh.exe": {}, "wscript.exe": {}, "cscript.exe": {}, "mshta.exe": {},
	// Anti-cheat bootstrappers ship with many unrelated games and run briefly at
	// launch under a shared name, so they cross-attribute presence (e.g. launching
	// any Easy Anti-Cheat game flashed "VRChat" via start_protected_game.exe).
	// The frequency filter misses them because only a handful of games list them.
	"start_protected_game.exe": {}, // Easy Anti-Cheat launcher
	"easyanticheat.exe": {}, "easyanticheat_setup.exe": {}, "easyanticheat_eos_setup.exe": {},
	"beservice.exe": {}, "bedaisy.exe": {}, "battleye.exe": {}, "be_setup.exe": {}, // BattlEye
	// Ambiguous short names that collide across unrelated processes/games and
	// can't identify any one of them. "lms.exe" is the only exe of the delisted
	// "Last Man Standing" but also matches other processes (it flickered "Last
	// Man Standing" for a member playing a different, uninstalled-LMS game).
	"lms.exe": {},
}

// minExeStem is the shortest executable stem (the name without its extension) we
// trust for matching. Stems of 1-2 characters (e.g. "sc.exe") collide with
// system tools and produce false positives.
const minExeStem = 3

// testVariant matches catalog names for non-retail editions (test servers, PTR,
// betas, playtests). Their executables overlap the main game and steal its
// presence (e.g. "PUBG: Test Server" via execpubg.exe), so we don't detect on
// them.
var testVariant = regexp.MustCompile(
	`(?i)(test server|public test|play ?test|closed test|open test|closed beta|open beta|public beta|beta test|experimental|\bPTR\b|\bPTS\b)`)

// extraExecutables adds curated process names the Discord list misses or names
// differently, keyed by the game's slug. Example: Palia's Steam build runs
// PaliaClientSteam-Win64-Shipping.exe while the list only has the Epic name.
// These bypass the generic/short/frequency filters since they are vetted.
var extraExecutables = map[string][]string{
	"palia": {"paliaclientsteam-win64-shipping.exe"},
	// Discord lists Vampire Crawlers (steam 3265700) with no executables at all,
	// so the game was dropped from the catalog and never detected. Its binary is
	// "Vampire Crawlers.exe".
	"vampire-crawlers-the-turbo-wildcard-from-vampire-survivors": {"vampire crawlers.exe"},
}

// wrongExecutables drops process names Discord lists under the WRONG game,
// keyed by the game's slug. Example: Discord lists Enlisted's binary
// (enlisted.exe) under CRSED: F.O.A.D. (same studio), so playing Enlisted was
// falsely detected as CRSED. Removed only from that game; the correct game
// (Enlisted) keeps it.
var wrongExecutables = map[string][]string{
	"crsed-f-o-a-d": {"enlisted.exe"},
	// Discord lists ascension.exe under the card game, but that binary is the
	// WoW Ascension client (curated entry, migration 0028). Keeping both made
	// every session count twice. The card game keeps ascensiongame.exe.
	"ascension-deckbuilding-game": {"ascension.exe"},
}

// maxGamesPerExecutable bounds how many distinct games may share an executable
// basename before it's considered non-discriminating and dropped from matching.
// e.g. "hl2.exe" ships with ~34 Source games - it can't identify any one of them.
const maxGamesPerExecutable = 3

// basename lowercases an executable path and keeps only its file name. Discord
// ships paths like "_retail_/wow.exe"; the client matches the process basename.
func basename(raw string) string {
	name := strings.ToLower(path.Base(strings.ReplaceAll(raw, `\`, `/`)))
	if name == "." {
		return ""
	}
	return name
}

// tooShort reports whether an executable's stem (name without its extension) is
// short enough to collide with system tools (e.g. "sc.exe" -> "sc").
func tooShort(name string) bool {
	stem := name
	if i := strings.LastIndex(name, "."); i > 0 {
		stem = name[:i]
	}
	return len(stem) < minExeStem
}

// normalize keeps games with at least one specific, non-launcher executable and
// cleans up executable names for process matching. It drops generic helper
// binaries and any name shared by too many games, so the client never reports a
// player as in a game they aren't running.
func normalize(apps []detectableApp) []store.GameSeed {
	// First pass: count how many distinct apps each executable basename appears
	// in, so we can drop non-discriminating names ("game.exe" ships with ~190).
	freq := map[string]int{}
	for _, app := range apps {
		seen := map[string]struct{}{}
		for _, exe := range app.Executables {
			if exe.IsLauncher {
				continue
			}
			name := basename(exe.Name)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			freq[name]++
		}
	}

	seeds := make([]store.GameSeed, 0, len(apps))
	for _, app := range apps {
		if testVariant.MatchString(app.Name) {
			continue // non-retail edition; its exes would shadow the main game
		}
		exes := make([]string, 0, len(app.Executables))
		seen := map[string]struct{}{}
		// Pre-mark executables Discord lists under the wrong game so the loop
		// skips them (also keeps them out of the curated additions below).
		for _, w := range wrongExecutables[Slugify(app.Name)] {
			seen[w] = struct{}{}
		}
		for _, exe := range app.Executables {
			if exe.IsLauncher {
				// "Playing Battle.net" is noise, not presence.
				continue
			}
			name := basename(exe.Name)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			if _, generic := genericExecutables[name]; generic {
				continue
			}
			if tooShort(name) {
				continue // 1-2 char stems collide with system tools
			}
			if freq[name] > maxGamesPerExecutable {
				continue // shared by too many games to identify this one
			}
			exes = append(exes, name)
		}
		// Curated additions (vetted store-specific variants the list misses).
		for _, extra := range extraExecutables[Slugify(app.Name)] {
			if _, dup := seen[extra]; !dup {
				seen[extra] = struct{}{}
				exes = append(exes, extra)
			}
		}
		if len(exes) == 0 {
			continue
		}

		var iconURL, coverURL string
		if app.IconHash != "" {
			iconURL = fmt.Sprintf("https://cdn.discordapp.com/app-icons/%s/%s.png", app.ID, app.IconHash)
		}
		if app.CoverImageHash != "" {
			coverURL = fmt.Sprintf("https://cdn.discordapp.com/app-icons/%s/%s.png", app.ID, app.CoverImageHash)
		}

		seeds = append(seeds, store.GameSeed{
			DiscordAppID:    app.ID,
			Name:            app.Name,
			Slug:            Slugify(app.Name),
			ExecutableNames: exes,
			IconURL:         iconURL,
			CoverURL:        coverURL,
			SteamAppID:      app.steamAppID(),
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
