package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// Needs a real database: the rules under test ARE SQL (the latest known rating,
// matches without one skipped). Skipped without TEST_DATABASE_URL, which must
// point at a throwaway database: this test migrates it and writes into it.
func openRatingDB(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL absent")
	}
	ctx := context.Background()
	st, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st, ctx
}

// seedMemberAndGame creates one org, one member and the Hearthstone game,
// returning the member and game ids. users.org_id has no default, so the org
// is created first.
func seedMemberAndGame(t *testing.T, st *Store, ctx context.Context, name string) (string, string) {
	t.Helper()
	var orgID, userID, gameID string
	if err := st.pool.QueryRow(ctx,
		`INSERT INTO orgs (name, slug) VALUES ($1, $1) RETURNING id`,
		name).Scan(&orgID); err != nil {
		t.Fatalf("org: %v", err)
	}
	if err := st.pool.QueryRow(ctx,
		`INSERT INTO users (org_id, username, email, password_hash) VALUES ($1, $2, $2 || '@test.local', 'x') RETURNING id`,
		orgID, name).Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := st.pool.QueryRow(ctx,
		`INSERT INTO games (name, slug) VALUES ('Hearthstone', 'hearthstone-' || $1)
		 RETURNING id`, name).Scan(&gameID); err != nil {
		t.Fatalf("game: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = st.pool.Exec(ctx, `DELETE FROM games WHERE id = $1`, gameID)
		_, _ = st.pool.Exec(ctx, `DELETE FROM orgs WHERE id = $1`, orgID)
	})
	return userID, gameID
}

func ptr(n int) *int { return &n }

func TestHearthstoneRatingIsStoredAndTheLatestKnownWins(t *testing.T) {
	st, ctx := openRatingDB(t)
	userID, gameID := seedMemberAndGame(t, st, ctx, "cote1")
	t0 := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	bg := func(at time.Time, placement int, rating, after *int) HearthstoneMatch {
		return HearthstoneMatch{UserID: userID, GameID: gameID, Mode: "battlegrounds",
			Result: "loss", Placement: ptr(placement), PlayedAt: at, Rating: rating, RatingAfter: after}
	}
	for _, m := range []HearthstoneMatch{
		bg(t0, 4, ptr(5500), ptr(5571)),
		bg(t0.Add(time.Hour), 2, ptr(5571), ptr(5644)),
		// Played without HDT: no rating. It must not hide the last known one.
		bg(t0.Add(2*time.Hour), 6, nil, nil),
	} {
		if err := st.InsertHearthstoneMatch(ctx, m); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	r, err := st.HearthstoneLatestRatingFor(ctx, userID, gameID)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if r == nil || r.RatingAfter != 5644 || !r.At.Equal(t0.Add(time.Hour)) {
		t.Fatalf("latest = %+v, want 5644 at t0+1h", r)
	}

	byGame, err := st.HearthstoneLatestRatingsByGame(ctx, gameID)
	if err != nil {
		t.Fatalf("by game: %v", err)
	}
	if got := byGame[userID]; got.RatingAfter != 5644 {
		t.Fatalf("by game = %+v, want 5644", got)
	}

	recent, err := st.HearthstoneRecentFor(ctx, userID, gameID, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 3 || recent[0].Rating != nil || recent[1].RatingAfter == nil || *recent[1].RatingAfter != 5644 {
		t.Fatalf("recent = %+v", recent)
	}
}

func TestAMemberWithoutRatingHasNone(t *testing.T) {
	st, ctx := openRatingDB(t)
	userID, gameID := seedMemberAndGame(t, st, ctx, "cote2")
	if err := st.InsertHearthstoneMatch(ctx, HearthstoneMatch{UserID: userID, GameID: gameID,
		Mode: "battlegrounds", Result: "loss", Placement: ptr(3), PlayedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	r, err := st.HearthstoneLatestRatingFor(ctx, userID, gameID)
	if err != nil || r != nil {
		t.Fatalf("latest = %+v, %v; want nil, nil", r, err)
	}
	byGame, err := st.HearthstoneLatestRatingsByGame(ctx, gameID)
	if err != nil {
		t.Fatalf("by game: %v", err)
	}
	if _, ok := byGame[userID]; ok {
		t.Fatalf("a member without rating must be absent from the map")
	}
}
