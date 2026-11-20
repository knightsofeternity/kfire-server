package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// WowGuildReport is one member's view of a guild, as the KFireGuilde addon
// wrote it and the wowplayed package validated it. Names are "Name-Realm".
type WowGuildReport struct {
	GuildKey    string // lower(guild_name)
	GuildName   string
	RealmNorm   string
	SnapshotAt  time.Time
	Roster      []WowGuildRosterRow
	Members     []WowGuildMember
	Professions []WowGuildProfession
	ItemNames   map[string]string
	Presence    []WowGuildPresenceDay
}

// WowGuildRosterRow is one line of the in-game guild roster.
type WowGuildRosterRow struct {
	Name       string     `json:"name"`
	Level      *int       `json:"level,omitempty"`
	Class      *string    `json:"class,omitempty"`
	Rank       *string    `json:"rank,omitempty"`
	RankIndex  *int       `json:"rank_index,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

// WowGuildMember is a character's addon profile.
type WowGuildMember struct {
	Name         string
	Pseudo       *string
	Main         *string
	Role         *string
	AddonVersion *string
	UpdatedAt    time.Time
}

// WowGuildProfession is one profession of one character.
type WowGuildProfession struct {
	Name       string
	Profession string
	Level      *int
	Max        *int
	Recipes    []string
	UpdatedAt  time.Time
}

// WowGuildPresenceDay is one day of hourly presence; a nil hour is unknown.
type WowGuildPresenceDay struct {
	Day   time.Time // midnight UTC of the server day
	Hours [24]*int
}

// RecordWowGuild writes one report in a single transaction. The guild is
// created on first sight; the roster is replaced only by a newer snapshot;
// profiles and professions keep their newest updated_at; presence keeps the
// larger count of each hour; item names take the last reported name.
func (s *Store) RecordWowGuild(ctx context.Context, userID, gameID string, r WowGuildReport) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var guildID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO wow_guilds (game_id, guild_key, realm_norm, name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (game_id, guild_key, realm_norm) DO UPDATE SET updated_at = now()
		RETURNING id`, gameID, r.GuildKey, r.RealmNorm, r.GuildName).Scan(&guildID); err != nil {
		return err
	}

	// The roster: only a strictly newer snapshot replaces it. The row lock
	// taken by the UPDATE serialises two members reporting at once.
	tag, err := tx.Exec(ctx, `
		UPDATE wow_guilds SET snapshot_at = $2, name = $3
		WHERE id = $1 AND (snapshot_at IS NULL OR snapshot_at < $2)`,
		guildID, r.SnapshotAt, r.GuildName)
	if err != nil {
		return err
	}
	b := &pgx.Batch{}
	if tag.RowsAffected() == 1 {
		b.Queue(`DELETE FROM wow_guild_roster WHERE guild_id = $1`, guildID)
		for _, m := range r.Roster {
			b.Queue(`
				INSERT INTO wow_guild_roster (guild_id, name, level, class, rank, rank_index, last_seen_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (guild_id, name) DO NOTHING`,
				guildID, m.Name, m.Level, m.Class, m.Rank, m.RankIndex, m.LastSeenAt)
		}
	}
	for _, m := range r.Members {
		b.Queue(`
			INSERT INTO wow_guild_members (guild_id, name, pseudo, main, role, addon_version, updated_at, reported_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (guild_id, name) DO UPDATE SET
			    pseudo = EXCLUDED.pseudo, main = EXCLUDED.main, role = EXCLUDED.role,
			    addon_version = EXCLUDED.addon_version, updated_at = EXCLUDED.updated_at,
			    reported_by = EXCLUDED.reported_by
			WHERE EXCLUDED.updated_at > wow_guild_members.updated_at`,
			guildID, m.Name, m.Pseudo, m.Main, m.Role, m.AddonVersion, m.UpdatedAt, userID)
	}
	for _, p := range r.Professions {
		recipes := p.Recipes
		if recipes == nil {
			recipes = []string{}
		}
		b.Queue(`
			INSERT INTO wow_guild_professions (guild_id, name, profession, level, max_level, recipes, updated_at, reported_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (guild_id, name, profession) DO UPDATE SET
			    level = EXCLUDED.level, max_level = EXCLUDED.max_level, recipes = EXCLUDED.recipes,
			    updated_at = EXCLUDED.updated_at, reported_by = EXCLUDED.reported_by
			WHERE EXCLUDED.updated_at > wow_guild_professions.updated_at`,
			guildID, p.Name, p.Profession, p.Level, p.Max, recipes, p.UpdatedAt, userID)
	}
	for _, d := range r.Presence {
		b.Queue(`
			INSERT INTO wow_guild_presence (guild_id, day, hours) VALUES ($1, $2, $3)
			ON CONFLICT (guild_id, day) DO UPDATE SET
			    hours = ARRAY(
			        SELECT greatest(o, n)
			        FROM unnest(wow_guild_presence.hours, EXCLUDED.hours) WITH ORDINALITY AS u(o, n, i)
			        ORDER BY i),
			    updated_at = now()`,
			guildID, d.Day.Format("2006-01-02"), d.Hours[:])
	}
	if len(r.ItemNames) > 0 {
		ids := make([]string, 0, len(r.ItemNames))
		names := make([]string, 0, len(r.ItemNames))
		for id, n := range r.ItemNames {
			ids = append(ids, id)
			names = append(names, n)
		}
		b.Queue(`
			INSERT INTO wow_item_names (game_id, item_id, name)
			SELECT $1, i, n FROM unnest($2::text[], $3::text[]) AS u(i, n)
			ON CONFLICT (game_id, item_id) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
			WHERE wow_item_names.name <> EXCLUDED.name`,
			gameID, ids, names)
	}
	if b.Len() > 0 {
		if err := tx.SendBatch(ctx, b).Close(); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// WowGuild is a known guild, for the list and the page header.
type WowGuild struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Realm       string     `json:"realm"`
	SnapshotAt  *time.Time `json:"snapshot_at,omitempty"`
	RosterCount int        `json:"roster_count"`
	AddonCount  int        `json:"addon_count"`
}

// WowGuilds lists the guilds of one edition, the largest roster first.
func (s *Store) WowGuilds(ctx context.Context, gameID string) ([]WowGuild, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.name, g.realm_norm, g.snapshot_at,
		       (SELECT count(*) FROM wow_guild_roster r WHERE r.guild_id = g.id),
		       (SELECT count(*) FROM wow_guild_members m WHERE m.guild_id = g.id)
		FROM wow_guilds g WHERE g.game_id = $1
		ORDER BY 5 DESC, g.name`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WowGuild{}
	for rows.Next() {
		var g WowGuild
		if err := rows.Scan(&g.ID, &g.Name, &g.Realm, &g.SnapshotAt, &g.RosterCount, &g.AddonCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// WowGuildByID returns one guild of an edition, or ErrNotFound.
func (s *Store) WowGuildByID(ctx context.Context, gameID, id string) (*WowGuild, error) {
	var g WowGuild
	err := s.pool.QueryRow(ctx, `
		SELECT g.id, g.name, g.realm_norm, g.snapshot_at,
		       (SELECT count(*) FROM wow_guild_roster r WHERE r.guild_id = g.id),
		       (SELECT count(*) FROM wow_guild_members m WHERE m.guild_id = g.id)
		FROM wow_guilds g WHERE g.game_id = $1 AND g.id::text = $2`, gameID, id).
		Scan(&g.ID, &g.Name, &g.Realm, &g.SnapshotAt, &g.RosterCount, &g.AddonCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// WowGuildRosterEntry is a roster line merged with the character's addon
// profile and, when one of its members has it in their /played, the KFIRE
// member.
type WowGuildRosterEntry struct {
	WowGuildRosterRow
	Pseudo           *string    `json:"pseudo,omitempty"`
	Main             *string    `json:"main,omitempty"`
	Role             *string    `json:"role,omitempty"`
	AddonVersion     *string    `json:"addon_version,omitempty"`
	ProfileUpdatedAt *time.Time `json:"profile_updated_at,omitempty"`
	UserID           *string    `json:"user_id,omitempty"`
	Username         *string    `json:"username,omitempty"`
}

// linkedUser picks, for a "Name-Realm" column, the KFIRE member whose
// /played has the character on the guild's edition (the most recent record
// wins if two members claim it).
const linkedUser = `
	LEFT JOIN LATERAL (
		SELECT wp.user_id, u.username FROM wow_played wp JOIN users u ON u.id = wp.user_id
		WHERE wp.game_id = g.game_id
		  AND lower(wp.name) = lower(split_part(%[1]s, '-', 1))
		  AND regexp_replace(lower(wp.realm_norm), '[[:space:]''’-]', '', 'g')
		    = regexp_replace(lower(substr(%[1]s, strpos(%[1]s, '-') + 1)), '[[:space:]''’-]', '', 'g')
		ORDER BY wp.recorded_at DESC LIMIT 1
	) lu ON true`

// sprintfLinked fills linkedUser for one "Name-Realm" column. Names never
// hold a hyphen, so the realm is everything after the first one; the realm is
// compared without case, spaces, quotes or hyphens.
func sprintfLinked(col string) string { return fmt.Sprintf(linkedUser, col) }

// WowGuildRoster returns the roster merged with profiles and KFIRE members.
func (s *Store) WowGuildRoster(ctx context.Context, guildID string) ([]WowGuildRosterEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.name, r.level, r.class, r.rank, r.rank_index, r.last_seen_at,
		       m.pseudo, m.main, m.role, m.addon_version, m.updated_at,
		       lu.user_id::text, lu.username
		FROM wow_guild_roster r
		JOIN wow_guilds g ON g.id = r.guild_id
		LEFT JOIN wow_guild_members m ON m.guild_id = r.guild_id AND m.name = r.name
		`+sprintfLinked("r.name")+`
		WHERE r.guild_id = $1
		ORDER BY r.rank_index NULLS LAST, r.level DESC NULLS LAST, r.name`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WowGuildRosterEntry{}
	for rows.Next() {
		var e WowGuildRosterEntry
		if err := rows.Scan(&e.Name, &e.Level, &e.Class, &e.Rank, &e.RankIndex, &e.LastSeenAt,
			&e.Pseudo, &e.Main, &e.Role, &e.AddonVersion, &e.ProfileUpdatedAt,
			&e.UserID, &e.Username); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// WowGuildCrafter is one character's level in a profession.
type WowGuildCrafter struct {
	Name      string    `json:"name"`
	Level     *int      `json:"level,omitempty"`
	Max       *int      `json:"max,omitempty"`
	Recipes   []string  `json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    *string   `json:"user_id,omitempty"`
	Username  *string   `json:"username,omitempty"`
}

// WowGuildProfessionRows returns every profession line of a guild, keyed by
// profession, with the characters' KFIRE members.
func (s *Store) WowGuildProfessionRows(ctx context.Context, guildID string) (map[string][]WowGuildCrafter, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.profession, p.name, p.level, p.max_level, p.recipes, p.updated_at,
		       lu.user_id::text, lu.username
		FROM wow_guild_professions p
		JOIN wow_guilds g ON g.id = p.guild_id
		`+sprintfLinked("p.name")+`
		WHERE p.guild_id = $1
		ORDER BY p.profession, p.level DESC NULLS LAST, p.name`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]WowGuildCrafter{}
	for rows.Next() {
		var prof string
		var c WowGuildCrafter
		if err := rows.Scan(&prof, &c.Name, &c.Level, &c.Max, &c.Recipes, &c.UpdatedAt,
			&c.UserID, &c.Username); err != nil {
			return nil, err
		}
		out[prof] = append(out[prof], c)
	}
	return out, rows.Err()
}

// WowItemNames resolves item and spell ids of an edition; unknown ids are
// simply absent.
func (s *Store) WowItemNames(ctx context.Context, gameID string, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT item_id, name FROM wow_item_names WHERE game_id = $1 AND item_id = ANY($2)`, gameID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, n string
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// WowGuildPresenceSince returns the presence days on or after since, oldest
// first.
func (s *Store) WowGuildPresenceSince(ctx context.Context, guildID string, since time.Time) ([]WowGuildPresenceDay, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT day, hours FROM wow_guild_presence
		WHERE guild_id = $1 AND day >= $2::date ORDER BY day`, guildID, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WowGuildPresenceDay
	for rows.Next() {
		var d WowGuildPresenceDay
		var hours []*int32
		if err := rows.Scan(&d.Day, &hours); err != nil {
			return nil, err
		}
		for i := 0; i < 24 && i < len(hours); i++ {
			if hours[i] != nil {
				v := int(*hours[i])
				d.Hours[i] = &v
			}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
