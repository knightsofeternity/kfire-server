package riot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChampionNamesResolvesIdsAndCachesTheVersion(t *testing.T) {
	var versionCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/versions.json":
			atomic.AddInt32(&versionCalls, 1)
			_, _ = w.Write([]byte(`["15.18.1","15.17.1"]`))
		case "/cdn/15.18.1/data/en_US/champion.json":
			_, _ = w.Write([]byte(`{"data":{"Jhin":{"key":"202","id":"Jhin","name":"Jhin"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dd := NewDataDragon()
	dd.Base = srv.URL

	for i := 0; i < 3; i++ {
		name, icon, err := dd.Champion(context.Background(), 202)
		if err != nil {
			t.Fatalf("Champion: %v", err)
		}
		if name != "Jhin" {
			t.Errorf("name = %q, want Jhin", name)
		}
		want := srv.URL + "/cdn/15.18.1/img/champion/Jhin.png"
		if icon != want {
			t.Errorf("icon = %q, want %q", icon, want)
		}
	}
	if got := atomic.LoadInt32(&versionCalls); got != 1 {
		t.Errorf("versions.json fetched %d times, want 1 (must be cached)", got)
	}
}

func TestChampionUnknownIDReturnsEmptyNameWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/versions.json":
			_, _ = w.Write([]byte(`["15.18.1"]`))
		default:
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	}))
	defer srv.Close()

	dd := NewDataDragon()
	dd.Base = srv.URL
	name, icon, err := dd.Champion(context.Background(), 999)
	if err != nil {
		t.Fatalf("an unknown champion must not be an error: %v", err)
	}
	if name != "" || icon != "" {
		t.Errorf("want empty name and icon, got %q %q", name, icon)
	}
}

func TestChampionFallsBackWhenDataDragonIsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dd := NewDataDragon()
	dd.Base = srv.URL
	name, icon, err := dd.Champion(context.Background(), 202)
	if err == nil {
		t.Fatal("want an error when Data Dragon is unreachable")
	}
	if name != "" || icon != "" {
		t.Errorf("want empty name and icon on failure, got %q %q", name, icon)
	}
}

func TestChampionIsSafeUnderConcurrentUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/versions.json":
			_, _ = w.Write([]byte(`["15.18.1"]`))
		default:
			_, _ = w.Write([]byte(`{"data":{"Jhin":{"key":"202","id":"Jhin","name":"Jhin"}}}`))
		}
	}))
	defer srv.Close()

	dd := NewDataDragon()
	dd.Base = srv.URL

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _, _ = dd.Champion(context.Background(), 202)
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}
