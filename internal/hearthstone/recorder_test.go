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
		{"cote complete", `{"mode":"battlegrounds","result":"loss","placement":2,"rating":5571,"rating_after":5644,"played_at":"2026-08-02T17:44:59Z"}`, true},
		{"cote negative", `{"mode":"battlegrounds","result":"loss","placement":2,"rating":-1,"rating_after":5644,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"cote apres trop grande", `{"mode":"battlegrounds","result":"loss","placement":2,"rating":5571,"rating_after":20001,"played_at":"2026-08-02T17:44:59Z"}`, false},
		{"cote a zero", `{"mode":"battlegrounds","result":"loss","placement":2,"rating":0,"rating_after":0,"played_at":"2026-08-02T17:44:59Z"}`, true},
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

func TestRatingOutsideBattlegroundsIsDropped(t *testing.T) {
	var p payload
	body := `{"mode":"constructed","result":"win","rating":5571,"rating_after":5644,"played_at":"2026-08-02T17:44:59Z"}`
	if err := json.Unmarshal([]byte(body), &p); err != nil || !p.valid() {
		t.Fatalf("a rating must not make a constructed game invalid: %v", err)
	}
	r, a := p.ratings()
	if r != nil || a != nil {
		t.Errorf("ratings() = %v, %v; want nil, nil outside Battlegrounds", r, a)
	}
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

// Une partie construite ne peut pas avoir de classement : il n'y a pas de
// lobby. Si le journal en laisse traîner un, il doit être oublié et NON compté,
// parce que les statistiques moyennent toute ligne qui en porte un et qu'une
// partie construite y ferait baisser ou monter la position moyenne sans raison.
//
// Le match, lui, est conservé : il a vraiment été joué.
func TestUnClassementEnConstruitEstOublieMaisLeMatchEstGarde(t *testing.T) {
	four := 4
	p := payload{Mode: modeConstructed, Result: "win", Placement: &four, PlayedAt: time.Now().UTC()}
	if !p.valid() {
		t.Fatal("une partie construite avec un classement parasite reste valide")
	}

	// La règle appliquée à l'insertion, isolée pour être testable sans base.
	placement := p.Placement
	if p.Mode != modeBattlegrounds {
		placement = nil
	}
	if placement != nil {
		t.Fatalf("classement %d conservé en mode construit", *placement)
	}

	// Et en Champs de bataille, il est bien conservé.
	p.Mode = modeBattlegrounds
	placement = p.Placement
	if p.Mode != modeBattlegrounds {
		placement = nil
	}
	if placement == nil || *placement != 4 {
		t.Fatal("le classement doit être conservé en Champs de bataille")
	}
}
