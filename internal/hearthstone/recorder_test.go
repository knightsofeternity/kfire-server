package hearthstone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
)

func TestPayloadValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"complet", `{"mode":"battlegrounds","result":"loss","turns":22,"placement":5,"played_at":"2026-08-02T17:44:59Z"}`, true},
		{"sans placement ni tours", `{"mode":"constructed","result":"win","played_at":"2026-08-02T17:44:59Z"}`, true},
		{"mode inconnu", `{"mode":"arena","result":"loss","played_at":"2026-08-02T17:44:59Z"}`, false},
		{"resultat inconnu", `{"mode":"battlegrounds","result":"maybe","played_at":"2026-08-02T17:44:59Z"}`, false},
		{"placement trop grand", `{"mode":"battlegrounds","result":"loss","placement":9,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"placement zero", `{"mode":"battlegrounds","result":"loss","placement":0,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"date manquante", `{"mode":"battlegrounds","result":"loss"}`, false},
		{"tours negatifs", `{"mode":"battlegrounds","result":"loss","turns":-1,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"tours a zero", `{"mode":"constructed","result":"loss","turns":0,"played_at":"2026-08-02T17:44:59Z"}`, true},
		{"date future", matchBody(time.Now().Add(time.Hour)), false},
		{"date a peine future, tolerance d horloge", matchBody(time.Now().Add(time.Minute)), true},
		{"date passee", matchBody(time.Now().Add(-time.Hour)), true},
		{"heros", `{"mode":"battlegrounds","result":"loss","placement":5,"hero_card_id":"BG28_HERO_400","played_at":"2026-08-02T17:44:59Z"}`, true},
		{"heros avec skin", `{"mode":"battlegrounds","result":"loss","placement":5,"hero_card_id":"TB_BaconShop_HERO_45_SKIN_F","played_at":"2026-08-02T17:44:59Z"}`, true},
		{"heros nomme au lieu d etre identifie", `{"mode":"battlegrounds","result":"loss","placement":5,"hero_card_id":"Rafaam l'evade","played_at":"2026-08-02T17:44:59Z"}`, false},
		{"heros vide", `{"mode":"battlegrounds","result":"loss","placement":5,"hero_card_id":"","played_at":"2026-08-02T17:44:59Z"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p payload
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
	return fmt.Sprintf(`{"mode":"battlegrounds","result":"loss","placement":4,"played_at":%q}`,
		at.UTC().Format(time.RFC3339))
}

func TestRecorderClaimsItsSlug(t *testing.T) {
	if got := NewRecorder(nil).Slug(); got != "hearthstone" {
		t.Errorf("Slug() = %q, want \"hearthstone\"", got)
	}
}

func TestRecordReturnsTheSentinelInBothCases(t *testing.T) {
	r := NewRecorder(nil)

	malformedJSON := r.Record(context.Background(), "u1", "g1", json.RawMessage(`{`))
	if !errors.Is(malformedJSON, matchrecord.ErrInvalidPayload) {
		t.Errorf("unreadable json = %v, want ErrInvalidPayload", malformedJSON)
	}
	if !strings.Contains(malformedJSON.Error(), "unreadable json") {
		t.Errorf("message should name the failure class, got %q", malformedJSON)
	}

	rejectedFields := r.Record(context.Background(), "u1", "g1",
		json.RawMessage(`{"mode":"arena","result":"loss","played_at":"2026-08-02T17:44:59Z"}`))
	if !errors.Is(rejectedFields, matchrecord.ErrInvalidPayload) {
		t.Errorf("rejected fields = %v, want ErrInvalidPayload", rejectedFields)
	}
	if !strings.Contains(rejectedFields.Error(), `mode="arena"`) {
		t.Errorf("message should carry the offending mode, got %q", rejectedFields)
	}
}
