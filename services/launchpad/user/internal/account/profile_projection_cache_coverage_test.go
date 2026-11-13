package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileProjectionETagTracksGenerationAndDeletion(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9691)
	const wallet = "0x00000000000000000000000000000000000025db"
	metrics := &Metrics{}
	h := HTTP{Store: s, Runtime: &Runtime{UseReadSwitch: true, Metrics: metrics}}
	read := func(match string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/v1/profiles/"+wallet, nil)
		r.SetPathValue("wallet", wallet)
		if match != "" {
			r.Header.Set("If-None-Match", match)
		}
		w := httptest.NewRecorder()
		h.profileGet(w, r)
		return w
	}
	initial := read("")
	if initial.Code != http.StatusOK || !strings.Contains(initial.Header().Get("ETag"), ":1\"") || initial.Header().Get("X-Profile-Projection") != "stable" {
		t.Fatalf("initial projection = %d %s %s", initial.Code, initial.Header().Get("ETag"), initial.Body.String())
	}
	if fresh := read(initial.Header().Get("ETag")); fresh.Code != http.StatusNotModified || fresh.Body.Len() != 0 {
		t.Fatalf("unchanged projection = %d %s", fresh.Code, fresh.Body.String())
	}
	if stale := read(`"older"`); stale.Code != http.StatusOK || stale.Header().Get("ETag") != initial.Header().Get("ETag") {
		t.Fatalf("stale projection = %d %s", stale.Code, stale.Body.String())
	}
	if _, err := s.DB.Exec(context.Background(), `UPDATE account_state SET deleted=true WHERE account_id=$1`, p.AccountID); err != nil {
		t.Fatal(err)
	}
	deleted := read("")
	if deleted.Code != http.StatusOK || strings.Contains(deleted.Body.String(), "private-name") {
		t.Fatalf("deleted public profile = %d %s", deleted.Code, deleted.Body.String())
	}
	snapshot := metrics.Snapshot()
	if snapshot["projection.success"].Count < 2 || snapshot["projection.stale"].Count != 1 {
		t.Fatalf("projection metrics = %+v", snapshot)
	}
}
