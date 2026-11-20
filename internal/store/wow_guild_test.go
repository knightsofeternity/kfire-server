package store

import (
	"fmt"
	"testing"
	"time"
)

// Several members report the same guild: the newest snapshot owns the roster,
// the newest profile wins, presence keeps the larger count of each hour, and
// a character in a member's /played links to that member.
func TestWowGuildMergesReports(t *testing.T) {
	st, ctx := openRatingDB(t)
	tag := fmt.Sprint(time.Now().UnixNano())
	var orgID, userA, userB, gameID string
	if err := st.pool.QueryRow(ctx, `SELECT id FROM orgs LIMIT 1`).Scan(&orgID); err != nil {
		st.pool.QueryRow(ctx, `INSERT INTO orgs (name, slug) VALUES ('t', 't') RETURNING id`).Scan(&orgID)
	}
	for _, p := range []struct {
		id   *string
		name string
	}{{&userA, "ga" + tag}, {&userB, "gb" + tag}} {
		if err := st.pool.QueryRow(ctx, `INSERT INTO users (org_id, username, email, password_hash) VALUES ($1, $2, $3, 'x') RETURNING id`,
			orgID, p.name, p.name+"@t.test").Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	st.pool.QueryRow(ctx, `INSERT INTO games (name, slug, executable_names, platform) VALUES ($1, $2, '{}', 'pc') RETURNING id`,
		"WoW F "+tag, "wowf-"+tag).Scan(&gameID)
	t.Cleanup(func() {
		st.pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, []string{userA, userB})
		st.pool.Exec(ctx, `DELETE FROM games WHERE id = $1`, gameID)
	})
	at := time.Date(2026, 11, 20, 21, 0, 0, 0, time.UTC)
	// Member A plays Ouranos on Cho'gall: the roster line must point to A.
	if err := st.UpsertWowPlayed(ctx, userA, gameID, WowPlayed{Region: "eu", RealmNorm: "Chogall", Realm: "Cho'gall",
		Name: "Ouranos", PlayedSeconds: 10, RecordedAt: at}); err != nil {
		t.Fatal(err)
	}
	n := func(v int) *int { return &v }
	s := func(v string) *string { return &v }
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	var pa, pb WowGuildPresenceDay
	pa.Day, pb.Day = day, day
	pa.Hours[20], pa.Hours[21] = n(3), n(8)
	pb.Hours[21], pb.Hours[22] = n(5), n(2)

	repA := WowGuildReport{
		GuildKey: "knights of eternity", GuildName: "Knights of Eternity", RealmNorm: "Chogall", SnapshotAt: at,
		Roster: []WowGuildRosterRow{
			{Name: "Ouranos-Chogall", Level: n(60), Class: s("MAGE"), Rank: s("Officier"), RankIndex: n(1)},
			{Name: "Malkai-Chogall", Level: n(40)},
		},
		Members: []WowGuildMember{{Name: "Ouranos-Chogall", Pseudo: s("Oura"), Role: s("D"), UpdatedAt: at}},
		Professions: []WowGuildProfession{{Name: "Malkai-Chogall", Profession: "Couture", Level: n(75), Max: n(150),
			Recipes: []string{"2996", "s7418"}, UpdatedAt: at}},
		ItemNames: map[string]string{"2996": "Rouleau"},
		Presence:  []WowGuildPresenceDay{pa},
	}
	if err := st.RecordWowGuild(ctx, userA, gameID, repA); err != nil {
		t.Fatal(err)
	}
	// B's addon has an older snapshot and an older profile, but newer presence
	// for some hours, and the guild name in another case.
	repB := WowGuildReport{
		GuildKey: "knights of eternity", GuildName: "KNIGHTS OF ETERNITY", RealmNorm: "Chogall", SnapshotAt: at.Add(-time.Hour),
		Roster:   []WowGuildRosterRow{{Name: "Stale-Chogall"}},
		Members:  []WowGuildMember{{Name: "Ouranos-Chogall", Pseudo: s("Old"), UpdatedAt: at.Add(-time.Hour)}},
		Presence: []WowGuildPresenceDay{pb},
	}
	if err := st.RecordWowGuild(ctx, userB, gameID, repB); err != nil {
		t.Fatal(err)
	}
	guilds, err := st.WowGuilds(ctx, gameID)
	if err != nil || len(guilds) != 1 || guilds[0].Name != "Knights of Eternity" || guilds[0].RosterCount != 2 {
		t.Fatalf("guilds = %+v (err %v), want one guild, A's name and roster", guilds, err)
	}
	roster, err := st.WowGuildRoster(ctx, guilds[0].ID)
	if err != nil || len(roster) != 2 {
		t.Fatalf("roster = %+v (err %v)", roster, err)
	}
	o := roster[0]
	if o.Name != "Ouranos-Chogall" || o.Pseudo == nil || *o.Pseudo != "Oura" || o.UserID == nil || *o.UserID != userA {
		t.Errorf("Ouranos = %+v, want A's newest profile and A linked", o)
	}
	if roster[1].UserID != nil {
		t.Errorf("Malkai is in nobody's /played, got %v", *roster[1].UserID)
	}
	// A newer snapshot replaces the roster whole.
	repB.SnapshotAt = at.Add(time.Hour)
	repB.Members, repB.Presence = nil, nil
	if err := st.RecordWowGuild(ctx, userB, gameID, repB); err != nil {
		t.Fatal(err)
	}
	roster, _ = st.WowGuildRoster(ctx, guilds[0].ID)
	if len(roster) != 1 || roster[0].Name != "Stale-Chogall" {
		t.Errorf("roster after a newer snapshot = %+v", roster)
	}
	days, err := st.WowGuildPresenceSince(ctx, guilds[0].ID, day.AddDate(0, 0, -1))
	if err != nil || len(days) != 1 {
		t.Fatalf("presence = %+v (err %v)", days, err)
	}
	h := days[0].Hours
	if h[19] != nil || *h[20] != 3 || *h[21] != 8 || *h[22] != 2 {
		t.Errorf("presence hours 19-22 = %v %v %v %v, want nil 3 8 2", h[19], h[20], h[21], h[22])
	}
	profs, err := st.WowGuildProfessionRows(ctx, guilds[0].ID)
	if err != nil || len(profs["Couture"]) != 1 || len(profs["Couture"][0].Recipes) != 2 {
		t.Fatalf("professions = %+v (err %v)", profs, err)
	}
	names, err := st.WowItemNames(ctx, gameID, []string{"2996", "s7418"})
	if err != nil || len(names) != 1 || names["2996"] != "Rouleau" {
		t.Errorf("item names = %v (err %v)", names, err)
	}
	if g, err := st.WowGuildByID(ctx, gameID, "not-a-uuid"); err != ErrNotFound || g != nil {
		t.Errorf("unknown id: %v %v, want ErrNotFound", g, err)
	}
}
