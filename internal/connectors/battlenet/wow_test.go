package battlenet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthURLRequestsProfileScopes(t *testing.T) {
	c := New("id", "secret")
	u := c.AuthURL("state123", "https://kfire/cb")
	for _, want := range []string{"openid", "wow.profile", "sc2.profile", "d3.profile"} {
		if !strings.Contains(u, want) {
			t.Errorf("AuthURL missing scope %q: %s", want, u)
		}
	}
}

func TestExchangeCodeCapturesScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"tok","token_type":"bearer","expires_in":86399,"scope":"openid wow.profile"}`))
	}))
	defer srv.Close()

	c := New("id", "secret")
	c.OAuthBase = srv.URL
	tok, err := c.ExchangeCode(context.Background(), "code", "https://kfire/cb")
	if err != nil {
		t.Fatal(err)
	}
	if tok.Scope != "openid wow.profile" {
		t.Errorf("scope = %q, want %q", tok.Scope, "openid wow.profile")
	}
}

func fakeWowAPI(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/profile/user/wow":
			w.Write([]byte(`{"wow_accounts":[{"characters":[
				{"name":"Tankette","id":1,"realm":{"slug":"hyjal","name":"Hyjal"},
				 "playable_class":{"name":"Warrior"},"playable_race":{"name":"Tauren"},
				 "faction":{"type":"HORDE","name":"Horde"},"level":80}]}]}`))
		case strings.HasSuffix(r.URL.Path, "/mythic-keystone-profile"):
			w.Write([]byte(`{"current_mythic_rating":{"rating":2451.7}}`))
		case strings.HasSuffix(r.URL.Path, "/encounters/raids"):
			w.Write([]byte(`{"expansions":[{"expansion":{"name":"The War Within"},
				"instances":[{"instance":{"name":"Nerub-ar Palace"},
				"modes":[{"difficulty":{"type":"MYTHIC"},
				"progress":{"completed_count":3,"total_count":8}}]}]}]}`))
		case strings.Contains(r.URL.Path, "/profile/wow/character/"):
			w.Write([]byte(`{"name":"Tankette","equipped_item_level":639,"level":80}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

func TestGetWowAccountCharacters(t *testing.T) {
	srv := fakeWowAPI(t)
	defer srv.Close()
	c := New("id", "secret")
	c.APIBase = srv.URL

	chars, err := c.WowAccountCharacters(context.Background(), "tok", "profile-eu")
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 1 || chars[0].Name != "Tankette" || chars[0].RealmSlug != "hyjal" ||
		chars[0].Class != "Warrior" || chars[0].Race != "Tauren" || chars[0].Faction != "Horde" || chars[0].Level != 80 {
		t.Fatalf("unexpected character: %+v", chars)
	}
}

func TestEnrichWowCharacter(t *testing.T) {
	srv := fakeWowAPI(t)
	defer srv.Close()
	c := New("id", "secret")
	c.APIBase = srv.URL

	ch := WowCharacter{Name: "Tankette", RealmSlug: "hyjal"}
	if err := c.EnrichWowCharacter(context.Background(), "tok", "profile-eu", &ch); err != nil {
		t.Fatal(err)
	}
	if ch.ItemLevel != 639 {
		t.Errorf("item level = %d, want 639", ch.ItemLevel)
	}
	if ch.MythicRating == nil || *ch.MythicRating < 2451 || *ch.MythicRating > 2452 {
		t.Errorf("mythic rating = %v, want ~2451.7", ch.MythicRating)
	}
	if ch.RaidSummary == nil {
		t.Error("raid summary not populated")
	}
}
