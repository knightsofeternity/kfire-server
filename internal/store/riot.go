package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// RiotAccount is a member's League routing: which platform host serves their
// League data and which cluster serves their match history.
type RiotAccount struct {
	UserID       string
	PUUID        string
	RiotID       string
	Platform     string
	MatchCluster string
	RegionSource string
}

// RiotProfileRow is one member's cached League blob. ActivityVisible carries
// the member's "show what I am playing" toggle, so callers can honour it when
// deciding whether to reveal a match in progress.
type RiotProfileRow struct {
	UserID          string
	Username        string
	AvatarURL       *string
	ActivityVisible bool
	Data            []byte
	LastSyncedAt    time.Time
}

// RiotPlayer is a member currently in a League session, with what the live
// poller needs to query Riot.
type RiotPlayer struct {
	UserID   string
	PUUID    string
	Platform string
}

// UpsertRiotRouting writes a member's platform and cluster. source is "auto"
// when resolved from Riot and "manual" when the member corrected it. An
// automatic resolution never overwrites a manual correction.
func (s *Store) UpsertRiotRouting(ctx context.Context, userID, platform, cluster, source string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO riot_accounts (user_id, platform, match_cluster, region_source, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id) DO UPDATE SET
			platform      = EXCLUDED.platform,
			match_cluster = EXCLUDED.match_cluster,
			region_source = EXCLUDED.region_source,
			updated_at    = now()
		WHERE riot_accounts.region_source = 'auto' OR EXCLUDED.region_source = 'manual'`,
		userID, platform, cluster, source)
	return err
}

// RiotAccountFor returns a member's linked Riot identity and routing, or
// ErrNotFound when they have not linked an account.
func (s *Store) RiotAccountFor(ctx context.Context, userID string) (RiotAccount, error) {
	var a RiotAccount
	err := s.pool.QueryRow(ctx, `
		SELECT la.user_id, la.provider_user_id, COALESCE(la.display_name, ''),
		       ra.platform, ra.match_cluster, ra.region_source
		FROM linked_accounts la
		JOIN riot_accounts ra ON ra.user_id = la.user_id
		WHERE la.user_id = $1 AND la.provider = 'riot'`, userID).
		Scan(&a.UserID, &a.PUUID, &a.RiotID, &a.Platform, &a.MatchCluster, &a.RegionSource)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// DeleteRiotData removes a member's routing and every cached Riot profile. The
// linked_accounts row is deleted separately by DeleteLinkedAccount.
func (s *Store) DeleteRiotData(ctx context.Context, userID string) error {
	batch := &pgx.Batch{}
	batch.Queue(`DELETE FROM riot_accounts WHERE user_id = $1`, userID)
	batch.Queue(`DELETE FROM riot_game_profile WHERE user_id = $1`, userID)
	return s.pool.SendBatch(ctx, batch).Close()
}

// UpsertRiotProfile stores a member's blob for one game.
func (s *Store) UpsertRiotProfile(ctx context.Context, userID, gameID string, data []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO riot_game_profile (user_id, game_id, data, last_synced_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, game_id) DO UPDATE SET
			data = EXCLUDED.data, last_synced_at = now()`,
		userID, gameID, data)
	return err
}

// RiotProfileSyncedAt returns when a member's blob was last written, or the
// zero time when it never was. It is the refresh throttle.
func (s *Store) RiotProfileSyncedAt(ctx context.Context, userID, gameID string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT last_synced_at FROM riot_game_profile WHERE user_id = $1 AND game_id = $2`,
		userID, gameID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

// RiotProfilesByGame returns every member's blob for a game, joined to their
// username, plus the newest sync time. Banned members are excluded, matching
// the Battle.net aggregate.
func (s *Store) RiotProfilesByGame(ctx context.Context, gameID string) ([]RiotProfileRow, time.Time, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.user_id, u.username, u.avatar_url, u.activity_visible,
		       p.data, p.last_synced_at
		FROM riot_game_profile p
		JOIN users u ON u.id = p.user_id AND u.banned_at IS NULL
		WHERE p.game_id = $1
		ORDER BY u.username ASC`, gameID)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var out []RiotProfileRow
	var newest time.Time
	for rows.Next() {
		var r RiotProfileRow
		if err := rows.Scan(&r.UserID, &r.Username, &r.AvatarURL, &r.ActivityVisible,
			&r.Data, &r.LastSyncedAt); err != nil {
			return nil, time.Time{}, err
		}
		out = append(out, r)
		if r.LastSyncedAt.After(newest) {
			newest = r.LastSyncedAt
		}
	}
	return out, newest, rows.Err()
}

// RiotProfileForUserGame returns one member's blob, or nil when absent.
func (s *Store) RiotProfileForUserGame(ctx context.Context, userID, gameID string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM riot_game_profile WHERE user_id = $1 AND game_id = $2`,
		userID, gameID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return data, err
}

// RiotPlayersInGame returns the members who have an open session on gameID and
// a linked Riot account. It is what makes the live poller cost nothing when
// nobody is playing.
func (s *Store) RiotPlayersInGame(ctx context.Context, gameID string) ([]RiotPlayer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT la.user_id, la.provider_user_id, ra.platform
		FROM game_sessions s
		JOIN linked_accounts la ON la.user_id = s.user_id AND la.provider = 'riot'
		JOIN riot_accounts ra ON ra.user_id = s.user_id
		JOIN users u ON u.id = s.user_id AND u.banned_at IS NULL
		WHERE s.game_id = $1 AND s.ended_at IS NULL`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RiotPlayer
	for rows.Next() {
		var p RiotPlayer
		if err := rows.Scan(&p.UserID, &p.PUUID, &p.Platform); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
