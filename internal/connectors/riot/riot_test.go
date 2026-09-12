package riot

import "testing"

func TestMatchCluster(t *testing.T) {
	cases := map[string]string{
		"euw1": "europe", "eun1": "europe", "tr1": "europe", "ru": "europe", "me1": "europe",
		"na1": "americas", "br1": "americas", "la1": "americas", "la2": "americas",
		"kr": "asia", "jp1": "asia",
		"oc1": "sea", "ph2": "sea", "sg2": "sea", "th2": "sea", "tw2": "sea", "vn2": "sea",
	}
	for platform, want := range cases {
		if got := MatchCluster(platform); got != want {
			t.Errorf("MatchCluster(%q) = %q, want %q", platform, got, want)
		}
	}
}

func TestMatchClusterUnknownFallsBackToEurope(t *testing.T) {
	if got := MatchCluster("zz9"); got != "europe" {
		t.Errorf("MatchCluster(unknown) = %q, want europe", got)
	}
}

func TestKnownPlatformsCoversEveryMappedPlatform(t *testing.T) {
	if len(KnownPlatforms()) != 17 {
		t.Errorf("KnownPlatforms() has %d entries, want 17", len(KnownPlatforms()))
	}
}
