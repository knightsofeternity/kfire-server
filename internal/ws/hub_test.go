package ws

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestMatchResultPayloadValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"complet", `{"game_slug":"hearthstone","mode":"battlegrounds","result":"loss","turns":22,"placement":5,"played_at":"2026-08-02T17:44:59Z"}`, true},
		{"sans placement ni tours", `{"game_slug":"hearthstone","mode":"constructed","result":"win","played_at":"2026-08-02T17:44:59Z"}`, true},
		{"slug manquant", `{"mode":"battlegrounds","result":"loss","played_at":"2026-08-02T17:44:59Z"}`, false},
		{"mode inconnu", `{"game_slug":"hearthstone","mode":"arena","result":"loss","played_at":"2026-08-02T17:44:59Z"}`, false},
		{"resultat inconnu", `{"game_slug":"hearthstone","mode":"battlegrounds","result":"maybe","played_at":"2026-08-02T17:44:59Z"}`, false},
		{"placement trop grand", `{"game_slug":"hearthstone","mode":"battlegrounds","result":"loss","placement":9,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"placement zero", `{"game_slug":"hearthstone","mode":"battlegrounds","result":"loss","placement":0,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"date manquante", `{"game_slug":"hearthstone","mode":"battlegrounds","result":"loss"}`, false},
		{"tours negatifs", `{"game_slug":"hearthstone","mode":"battlegrounds","result":"loss","turns":-1,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"tours a zero", `{"game_slug":"hearthstone","mode":"constructed","result":"loss","turns":0,"played_at":"2026-08-02T17:44:59Z"}`, true},
		{"date future", matchBody(time.Now().Add(time.Hour)), false},
		{"date a peine future, tolerance d horloge", matchBody(time.Now().Add(time.Minute)), true},
		{"date passee", matchBody(time.Now().Add(-time.Hour)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p matchResultPayload
			err := json.Unmarshal([]byte(tc.body), &p)
			got := err == nil && p.valid()
			if got != tc.ok {
				t.Errorf("valid() = %v, want %v", got, tc.ok)
			}
		})
	}
}

// matchBody builds a minimal valid payload ending at the given instant, so the
// clock-skew cases stay true whenever the suite runs.
func matchBody(at time.Time) string {
	return fmt.Sprintf(`{"game_slug":"hearthstone","mode":"battlegrounds","result":"loss","placement":4,"played_at":%q}`,
		at.UTC().Format(time.RFC3339))
}
