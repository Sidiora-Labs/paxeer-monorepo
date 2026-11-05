package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestV2WatchlistRoutesRegistered(t *testing.T) {
	mux, ok := (HTTP{}).Handler().(interface {
		Handler(*http.Request) (http.Handler, string)
	})
	if !ok {
		t.Fatal("HTTP.Handler does not expose its ServeMux route lookup")
	}
	for _, test := range []struct {
		method  string
		path    string
		pattern string
	}{
		{http.MethodGet, "/v2/me/watchlist", "GET /v2/me/watchlist"},
		{http.MethodPut, "/v2/me/watchlist/0x0000000000000000000000000000000000000001", "PUT /v2/me/watchlist/{poolAddress}"},
		{http.MethodDelete, "/v2/me/watchlist/0x0000000000000000000000000000000000000001", "DELETE /v2/me/watchlist/{poolAddress}"},
		{http.MethodPatch, "/v2/profiles/0x0000000000000000000000000000000000000001", "PATCH /v2/profiles/{wallet}"},
	} {
		t.Run(test.pattern, func(t *testing.T) {
			handler, pattern := mux.Handler(httptest.NewRequest(test.method, test.path, nil))
			if handler == nil || pattern != test.pattern {
				t.Fatalf("route = (%v, %q), want a handler for %q", handler, pattern, test.pattern)
			}
		})
	}
}

func TestPostgresHTTPV2Watchlist(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	p := owner(t, s, 8230)
	other := owner(t, s, 8231)
	wallet := fmt.Sprintf("0x%040x", 8230)
	pools := []string{fmt.Sprintf("0x%040x", 8232), fmt.Sprintf("0x%040x", 8233)}
	_, err := s.DB.Exec(ctx, `CREATE TABLE indexer_canonical_creations_v1(pool text PRIMARY KEY)`)
	must(t, err)
	for _, pool := range pools {
		_, err = s.DB.Exec(ctx, `INSERT INTO indexer_canonical_creations_v1(pool) VALUES($1)`, pool)
		must(t, err)
	}
	lookup := func(ctx context.Context, chain int, pool string) (bool, error) {
		if chain != 125 {
			return false, ErrInvalid
		}
		var exists bool
		err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer_canonical_creations_v1 WHERE pool=$1)`, pool).Scan(&exists)
		return exists, err
	}
	h := HTTP{Store: s, CursorKey: bytes.Repeat([]byte{7}, 32), PoolLookup: lookup}
	handler := h.Handler()
	check := func(response *httptest.ResponseRecorder, status int, schema string) {
		t.Helper()
		if response.Code != status {
			t.Fatalf("status = %d, want %d: %s", response.Code, status, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal(response.Header())
		}
		if schema != "" {
			if response.Header().Get("Content-Type") != "application/json" {
				t.Fatal(response.Header())
			}
			must(t, v2.ValidateJSON(schema, response.Body.Bytes()))
		}
	}
	empty := httpRequest(handler, p, "GET", "/v2/me/watchlist", "")
	check(empty, http.StatusOK, "WatchlistGetResponse")
	emptyPage, err := v2.DecodeWatchlistGetResponse(empty.Body.Bytes())
	must(t, err)
	if len(emptyPage.Data.Items) != 0 || emptyPage.Data.Items == nil || emptyPage.Revision != nil || emptyPage.NextCursor != nil {
		t.Fatal(empty.Body.String())
	}
	for i, pool := range pools {
		response := httpRequest(handler, p, "PUT", "/v2/me/watchlist/0x"+strings.ToUpper(pool[2:]), `{"alertEnabled":true}`)
		check(response, http.StatusOK, "WatchlistPutResponse")
		put, err := v2.DecodeWatchlistPutResponse(response.Body.Bytes())
		must(t, err)
		entry := put.Data.Entry
		if entry.Wallet != wallet || entry.PoolAddress != pool || entry.Order != v2.SafeInteger(i) || !entry.AlertEnabled || entry.AddedAt != "2026-10-10T00:00:00.000Z" || put.Revision == nil || string(*put.Revision) != fmt.Sprint(i+1) || put.NextCursor != nil || put.Quality != "complete" {
			t.Fatal(response.Body.String())
		}
	}
	response := httpRequest(handler, p, "GET", "/v2/me/watchlist?limit=1", "")
	check(response, http.StatusOK, "WatchlistGetResponse")
	first, err := v2.DecodeWatchlistGetResponse(response.Body.Bytes())
	must(t, err)
	if len(first.Data.Items) != 1 || first.Data.Items[0].PoolAddress != pools[0] || first.Data.Items[0].Order != 0 || first.NextCursor == nil || len(*first.NextCursor) > 256 || first.Revision == nil || *first.Revision != "2" {
		t.Fatal(response.Body.String())
	}
	cursorPath := "/v2/me/watchlist?limit=1&cursor=" + url.QueryEscape(*first.NextCursor)
	response = httpRequest(handler, p, "GET", cursorPath, "")
	check(response, http.StatusOK, "WatchlistGetResponse")
	second, err := v2.DecodeWatchlistGetResponse(response.Body.Bytes())
	must(t, err)
	if len(second.Data.Items) != 1 || second.Data.Items[0].PoolAddress != pools[1] || second.Data.Items[0].Wallet != wallet || second.Data.Items[0].Order != 1 || second.NextCursor != nil {
		t.Fatal(response.Body.String())
	}
	check(httpRequest(handler, other, "GET", cursorPath, ""), http.StatusBadRequest, "Error")
	check(httpRequest(handler, p, "GET", "/v2/me/watchlist?cursor=invalid", ""), http.StatusBadRequest, "Error")
	for _, query := range []string{"limit=0", "limit=101", "limit=1&limit=2", "accountId=" + other.AccountID.String()} {
		check(httpRequest(handler, p, "GET", "/v2/me/watchlist?"+query, ""), http.StatusUnprocessableEntity, "Error")
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		path, body := "/v2/me/watchlist", ""
		if method != "GET" {
			path += "/" + pools[0]
		}
		if method == "PUT" {
			body = `{"alertEnabled":true}`
		}
		check(httpRequest(handler, Principal{}, method, path, body), http.StatusUnauthorized, "Error")
	}
	for _, body := range []string{`{}`, `{"alertEnabled":null}`, `{"alertEnabled":true,"unknown":1}`} {
		check(httpRequest(handler, p, "PUT", "/v2/me/watchlist/"+pools[0], body), http.StatusUnprocessableEntity, "Error")
	}
	check(httpRequest(handler, p, "PUT", "/v2/me/watchlist/"+fmt.Sprintf("0x%040x", 8234), `{"alertEnabled":true}`), http.StatusNotFound, "Error")
	missingLookup := h
	missingLookup.PoolLookup = nil
	check(httpRequest(missingLookup.Handler(), p, "PUT", "/v2/me/watchlist/"+pools[0], `{"alertEnabled":true}`), http.StatusServiceUnavailable, "Error")
	for range 2 {
		response = httpRequest(handler, p, "PUT", "/v2/me/watchlist/"+pools[0], `{"alertEnabled":false}`)
		check(response, http.StatusOK, "WatchlistPutResponse")
		put, err := v2.DecodeWatchlistPutResponse(response.Body.Bytes())
		must(t, err)
		if put.Data.Entry.AlertEnabled || put.Data.Entry.Order != 0 || put.Revision == nil || *put.Revision != "3" {
			t.Fatal(response.Body.String())
		}
	}
	stale := httpRequest(handler, p, "GET", cursorPath, "")
	check(stale, http.StatusConflict, "Error")
	var wire WireError
	must(t, json.Unmarshal(stale.Body.Bytes(), &wire))
	if wire.Code != "RESNAPSHOT_REQUIRED" {
		t.Fatal(wire)
	}
	check(httpRequest(handler, p, "DELETE", "/v2/me/watchlist/"+pools[0], `{}`), http.StatusUnprocessableEntity, "Error")
	for _, principal := range []Principal{other, p, p} {
		response = httpRequest(handler, principal, "DELETE", "/v2/me/watchlist/"+pools[0], "")
		check(response, http.StatusNoContent, "")
		if response.Body.Len() != 0 {
			t.Fatal("DELETE returned a body", response.Body.String())
		}
		if principal.AccountID == other.AccountID {
			var count int
			must(t, s.DB.QueryRow(ctx, `SELECT count(*) FROM account_watch WHERE account_id=$1 AND pool=$2`, p.AccountID, pools[0]).Scan(&count))
			if count != 1 {
				t.Fatal("other owner deleted entry")
			}
		}
	}
	response = httpRequest(handler, p, "GET", "/v2/me/watchlist", "")
	check(response, http.StatusOK, "WatchlistGetResponse")
	remaining, err := v2.DecodeWatchlistGetResponse(response.Body.Bytes())
	must(t, err)
	if len(remaining.Data.Items) != 1 || remaining.Data.Items[0].PoolAddress != pools[1] || remaining.Data.Items[0].Order != 0 || remaining.Revision == nil || *remaining.Revision != "4" {
		t.Fatal(response.Body.String())
	}
}

func TestPostgresHTTPV2ProfilePatch(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 8240)
	other := owner(t, s, 8241)
	wallet := fmt.Sprintf("0x%040x", 8240)
	path := "/v2/profiles/" + wallet
	handler := (HTTP{Store: s}).Handler()
	body := `{"expectedRevision":"0","displayName":"<script>payload</script>","bio":"plain","avatarAssetId":null}`
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- httpRequest(handler, p, "PATCH", path, body)
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for result := range results {
		if result.Header().Get("Content-Type") != "application/json" || result.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(result.Header())
		}
		switch result.Code {
		case http.StatusOK:
			success++
			must(t, v2.ValidateJSON("ProfilePatchResponse", result.Body.Bytes()))
			response, err := v2.DecodeProfilePatchResponse(result.Body.Bytes())
			must(t, err)
			profile := response.Data.Profile
			if profile.Wallet != wallet || profile.DisplayName != "<script>payload</script>" || profile.Bio != "plain" || profile.Revision != "1" || profile.AvatarAssetId != nil || profile.BannerAssetId != nil || profile.Website != nil || response.Revision == nil || *response.Revision != "1" || response.Quality != "complete" || response.NextCursor != nil || bytes.Contains(result.Body.Bytes(), []byte("<script>")) {
				t.Fatal(result.Body.String())
			}
		case http.StatusConflict:
			conflict++
			must(t, v2.ValidateJSON("Error", result.Body.Bytes()))
			var wire WireError
			must(t, json.Unmarshal(result.Body.Bytes(), &wire))
			if wire.Code != "REVISION_CONFLICT" || wire.Retryable {
				t.Fatal(wire)
			}
		default:
			t.Fatal(result.Code, result.Body.String())
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	for _, test := range []struct {
		principal Principal
		path      string
		body      string
		status    int
	}{
		{p, path, `{"expectedRevision":"0","bio":"stale"}`, http.StatusConflict},
		{other, path, `{"expectedRevision":"1","bio":"stolen"}`, http.StatusForbidden},
		{Principal{}, path, `{"expectedRevision":"1","bio":"unauthenticated"}`, http.StatusUnauthorized},
		{p, "/v2/profiles/invalid", `{"expectedRevision":"1","bio":"invalid"}`, http.StatusUnprocessableEntity},
		{p, path, `{"expectedRevision":"1","bio":null}`, http.StatusUnprocessableEntity},
		{p, path, `{"expectedRevision":"1","unknown":true}`, http.StatusUnprocessableEntity},
		{p, "/v1/profiles/" + wallet, `{"expectedRevision":"0","bio":"stale v1"}`, http.StatusPreconditionFailed},
	} {
		response := httpRequest(handler, test.principal, "PATCH", test.path, test.body)
		if response.Code != test.status {
			t.Fatalf("%s = %d, want %d: %s", test.path, response.Code, test.status, response.Body.String())
		}
		must(t, v2.ValidateJSON("Error", response.Body.Bytes()))
	}
	request := httptest.NewRequest("PATCH", path, strings.NewReader(`{"expectedRevision":"1","bio":"bad etag"}`))
	request = request.WithContext(WithPrincipal(request.Context(), p))
	request.Header.Set("If-Match", `"0"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatal(response.Code, response.Body.String())
	}
	must(t, v2.ValidateJSON("Error", response.Body.Bytes()))
	response = httpRequest(handler, p, "PATCH", path, `{"expectedRevision":"1","displayName":"Updated","bio":"second"}`)
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	must(t, v2.ValidateJSON("ProfilePatchResponse", response.Body.Bytes()))
	updated, err := v2.DecodeProfilePatchResponse(response.Body.Bytes())
	must(t, err)
	if updated.Data.Profile.DisplayName != "Updated" || updated.Data.Profile.Bio != "second" || updated.Data.Profile.Revision != "2" || updated.Revision == nil || *updated.Revision != "2" {
		t.Fatal(response.Body.String())
	}
	stored, err := s.Public(context.Background(), wallet)
	must(t, err)
	if stored.Revision != "2" || stored.DisplayName != "Updated" || stored.Bio != "second" {
		t.Fatal(stored)
	}
}
