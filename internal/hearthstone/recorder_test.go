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

// matchBody construit une charge utile minimale valide se terminant à l'instant
// donné, pour que les cas de dérive d'horloge restent vrais quand la suite tourne.
func matchBody(at time.Time) string {
	return fmt.Sprintf(`{"mode":"battlegrounds","result":"loss","placement":4,"played_at":%q}`,
		at.UTC().Format(time.RFC3339))
}

func TestRecorderRevendiqueSonSlug(t *testing.T) {
	if got := NewRecorder(nil).Slug(); got != "hearthstone" {
		t.Errorf("Slug() = %q, want \"hearthstone\"", got)
	}
}

func TestRecordRendLaSentinelleDansLesDeuxCas(t *testing.T) {
	r := NewRecorder(nil)

	jsonCasse := r.Record(context.Background(), "u1", "g1", json.RawMessage(`{`))
	if !errors.Is(jsonCasse, matchrecord.ErrInvalidPayload) {
		t.Errorf("json illisible = %v, want ErrInvalidPayload", jsonCasse)
	}
	if !strings.Contains(jsonCasse.Error(), "json illisible") {
		t.Errorf("le message doit nommer la classe d'échec, got %q", jsonCasse)
	}

	champsRefuses := r.Record(context.Background(), "u1", "g1",
		json.RawMessage(`{"mode":"arena","result":"loss","played_at":"2026-08-02T17:44:59Z"}`))
	if !errors.Is(champsRefuses, matchrecord.ErrInvalidPayload) {
		t.Errorf("champs refusés = %v, want ErrInvalidPayload", champsRefuses)
	}
	if !strings.Contains(champsRefuses.Error(), `mode="arena"`) {
		t.Errorf("le message doit porter le mode fautif, got %q", champsRefuses)
	}
}
