package psnsync

import (
	"reflect"
	"sort"
	"testing"
)

func TestToAccept(t *testing.T) {
	linked := map[string]string{"111": "u1", "222": "u2"}
	got := toAccept([]string{"999", "111", "222"}, linked)
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"111", "222"}) {
		t.Fatalf("got %v: a stranger's request must be left alone", got)
	}
	if len(toAccept(nil, linked)) != 0 {
		t.Fatal("no request, nothing to accept")
	}
}

func TestBatches(t *testing.T) {
	ids := make([]string, 250)
	for i := range ids {
		ids[i] = "x"
	}
	b := batches(ids, 100)
	if len(b) != 3 || len(b[0]) != 100 || len(b[2]) != 50 {
		t.Fatalf("batches = %d/%d/%d", len(b), len(b[0]), len(b[2]))
	}
}
