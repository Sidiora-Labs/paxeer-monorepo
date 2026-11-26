package interactions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

type walletCommentsPage struct {
	Data struct {
		Items []CommentView `json:"items"`
	} `json:"data"`
	Next *string `json:"nextCursor"`
}

type walletCommentsFixture struct {
	*connectedFixture
	otherPool            string
	first, second, reply string
}

func walletCommentsData(t *testing.T) walletCommentsFixture {
	t.Helper()
	f := connected(t)
	a, b := f.wallets[0], f.wallets[1]
	pool := "0x" + strings.Repeat("e", 40)
	mustConnected(t, func() error { _, err := f.db.Exec(f.ctx[a], `INSERT INTO pools VALUES($1,$2)`, pool, b); return err }())
	create := func(actor, pool, body, parent string) string {
		t.Helper()
		id, err := uuid.NewV7()
		mustConnected(t, err)
		mustConnected(t, f.store.CreateComment(f.ctx[actor], Request{Actor: actor, Pool: pool, ID: id.String(), Body: body, Parent: parent, Key: id.String()}))
		return id.String()
	}
	first := create(a, f.pool, "First pool comment", "")
	parent := create(b, f.pool, "Another wallet's comment", "")
	create(b, pool, "Another wallet's second pool comment", "")
	second := create(a, pool, "Second pool comment", "")
	hidden := create(a, f.pool, "Hidden comment", "")
	reply := create(a, f.pool, "Reply to another wallet", parent)
	mustConnected(t, f.store.HideComment(f.ctx[b], Request{Actor: b, Pool: f.pool, ID: hidden, Body: "Community policy", Revision: 1, Key: "hide-wallet-comment"}))
	mustConnected(t, f.store.React(f.ctx[b], Request{Actor: b, ID: second, Kind: likeAction, Key: "like-wallet-comment"}))
	return walletCommentsFixture{f, pool, first, second, reply}
}

func readWalletCommentsPage(t *testing.T, routes http.Handler, path, tag string, status int) (walletCommentsPage, *httptest.ResponseRecorder) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if tag != "" {
		request.Header.Set("If-None-Match", tag)
	}
	response := httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("GET %s = %d, want %d: %s", path, response.Code, status, response.Body.String())
	}
	var page walletCommentsPage
	if status == http.StatusOK {
		mustConnected(t, json.Unmarshal(response.Body.Bytes(), &page))
	}
	return page, response
}

func TestWalletCommentsAcrossPools(t *testing.T) {
	f := walletCommentsData(t)
	a, b, c := f.wallets[0], f.wallets[1], f.wallets[2]
	routes := f.http.AuthenticatedRoutes(sharedauth.HTTPIdentityAuthority{})
	path := "/users/" + a + "/comments"
	page, response := readWalletCommentsPage(t, routes, path, "", http.StatusOK)
	if page.Data.Items == nil || len(page.Data.Items) != 3 || page.Next != nil {
		t.Fatalf("wallet comments = %+v", page)
	}
	for i, id := range []string{f.reply, f.second, f.first} {
		item := page.Data.Items[i]
		if item.ID != id || item.Author != a || item.Revision != "1" || item.Edited {
			t.Fatalf("comment %d = %+v", i, item)
		}
		if _, err := time.Parse(time.RFC3339Nano, item.CountAsOf); err != nil {
			t.Fatal(err)
		}
	}
	if page.Data.Items[0].Parent == nil || page.Data.Items[0].Pool != f.pool || page.Data.Items[1].Pool != f.otherPool || page.Data.Items[1].Likes != "1" || page.Data.Items[2].Likes != "0" {
		t.Fatalf("comment views = %+v", page.Data.Items)
	}
	t.Logf("wallet route response: %s", response.Body.String())
	tag := response.Header().Get("ETag")
	if tag == "" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("comment headers = %v", response.Header())
	}
	_, cached := readWalletCommentsPage(t, routes, path, tag, http.StatusNotModified)
	if cached.Body.Len() != 0 {
		t.Fatalf("cached body = %s", cached.Body.String())
	}
	first, _ := readWalletCommentsPage(t, routes, path+"?limit=1", "", http.StatusOK)
	if len(first.Data.Items) != 1 || first.Data.Items[0].ID != f.reply || first.Next == nil {
		t.Fatalf("first wallet page = %+v", first)
	}
	second, _ := readWalletCommentsPage(t, routes, path+"?limit=1&cursor="+url.QueryEscape(*first.Next), "", http.StatusOK)
	if len(second.Data.Items) != 1 || second.Data.Items[0].ID != f.second || second.Next == nil {
		t.Fatalf("second wallet page = %+v", second)
	}
	third, _ := readWalletCommentsPage(t, routes, path+"?limit=1&cursor="+url.QueryEscape(*second.Next), "", http.StatusOK)
	if len(third.Data.Items) != 1 || third.Data.Items[0].ID != f.first || third.Next != nil {
		t.Fatalf("last wallet page = %+v", third)
	}
	pool, _ := readWalletCommentsPage(t, routes, "/pool/"+f.pool+"/comments?limit=1", "", http.StatusOK)
	if pool.Next == nil {
		t.Fatal("pool cursor missing")
	}
	for _, invalid := range []string{path + "?limit=1&cursor=" + url.QueryEscape(*pool.Next), "/users/" + b + "/comments?cursor=" + url.QueryEscape(*first.Next), path + "?limit=0", path + "?limit=invalid", path + "?cursor=invalid", "/users/invalid/comments"} {
		readWalletCommentsPage(t, routes, invalid, "", http.StatusBadRequest)
	}
	readWalletCommentsPage(t, routes, "/pool/"+f.pool+"/comments?cursor="+url.QueryEscape(*first.Next), "", http.StatusBadRequest)
	empty, _ := readWalletCommentsPage(t, routes, "/users/0x"+strings.Repeat("1", 40)+"/comments", "", http.StatusOK)
	if empty.Data.Items == nil || len(empty.Data.Items) != 0 || empty.Next != nil {
		t.Fatalf("unknown wallet = %+v", empty)
	}
	upper, _ := readWalletCommentsPage(t, routes, "/users/0x"+strings.ToUpper(a[2:])+"/comments", "", http.StatusOK)
	if len(upper.Data.Items) != 3 {
		t.Fatalf("normalized wallet = %+v", upper)
	}
	mustConnected(t, f.store.React(f.ctx[c], Request{Actor: c, ID: f.second, Kind: likeAction, Key: "another-wallet-like"}))
	liked, changed := readWalletCommentsPage(t, routes, path, tag, http.StatusOK)
	if liked.Data.Items[1].Likes != "2" || changed.Header().Get("ETag") == tag {
		t.Fatalf("liked wallet page = %+v", liked)
	}
	tx, err := f.db.Begin(f.ctx[c])
	mustConnected(t, err)
	mustConnected(t, f.store.BeginErasureTx(f.ctx[c], tx, ErasureCommand{CaseID: uuid.NewString(), Wallet: c, PolicyVersion: "retention-v2"}))
	mustConnected(t, tx.Commit(f.ctx[c]))
	erased, changedAgain := readWalletCommentsPage(t, routes, path, changed.Header().Get("ETag"), http.StatusOK)
	if erased.Data.Items[1].Likes != "1" || changedAgain.Header().Get("ETag") == changed.Header().Get("ETag") {
		t.Fatalf("erased reaction = %+v", erased)
	}
	mustConnected(t, f.store.EditComment(f.ctx[a], Request{Actor: a, ID: f.first, Body: "Edited first pool comment", Revision: 1, Key: "edit-wallet-comment"}))
	edited, _ := readWalletCommentsPage(t, routes, path, changedAgain.Header().Get("ETag"), http.StatusOK)
	if !edited.Data.Items[2].Edited || edited.Data.Items[2].Revision != "2" || edited.Data.Items[2].Body != "Edited first pool comment" {
		t.Fatalf("edited wallet page = %+v", edited)
	}
	mustConnected(t, f.store.HideComment(f.ctx[b], Request{Actor: b, Pool: f.otherPool, ID: f.second, Body: "Community policy", Revision: 1, Key: "hide-liked-wallet-comment"}))
	hidden, hiddenResponse := readWalletCommentsPage(t, routes, path, "", http.StatusOK)
	if len(hidden.Data.Items) != 2 {
		t.Fatalf("hidden wallet page = %+v", hidden)
	}
	if hiddenResponse.Header().Get("ETag") == changedAgain.Header().Get("ETag") {
		t.Fatal("hidden comment retained ETag")
	}
	for i := 0; i < 256; i++ {
		id, createErr := uuid.NewV7()
		mustConnected(t, createErr)
		mustConnected(t, f.store.CreateComment(f.ctx[a], Request{Actor: a, Pool: f.otherPool, ID: id.String(), Body: fmt.Sprintf("Wallet comment %d", i), Key: id.String()}))
	}
	limited, _ := readWalletCommentsPage(t, routes, path+"?limit=1000", "", http.StatusOK)
	if len(limited.Data.Items) != 100 || limited.Next == nil {
		t.Fatalf("clamped limit = %d, cursor %v", len(limited.Data.Items), limited.Next)
	}
	defaultPage, _ := readWalletCommentsPage(t, routes, path, "", http.StatusOK)
	if len(defaultPage.Data.Items) != 50 || defaultPage.Next == nil {
		t.Fatalf("default limit = %d", len(defaultPage.Data.Items))
	}
	mustConnected(t, func() error { _, err := f.db.Exec(f.ctx[a], `ANALYZE interaction_comments`); return err }())
	rows, err := f.db.Query(f.ctx[a], "EXPLAIN "+walletCommentsQuery, a, nil, "", 51)
	mustConnected(t, err)
	defer rows.Close()
	for rows.Next() {
		var line string
		mustConnected(t, rows.Scan(&line))
		t.Log(line)
	}
	mustConnected(t, rows.Err())
}

func TestErasedWalletCommentsAreEmpty(t *testing.T) {
	f := walletCommentsData(t)
	a := f.wallets[0]
	routes := f.http.AuthenticatedRoutes(sharedauth.HTTPIdentityAuthority{})
	path := "/users/" + a + "/comments"
	_, before := readWalletCommentsPage(t, routes, path, "", http.StatusOK)
	tx, err := f.db.Begin(f.ctx[a])
	mustConnected(t, err)
	command := ErasureCommand{CaseID: uuid.NewString(), Wallet: a, PolicyVersion: "retention-v2"}
	mustConnected(t, f.store.BeginErasureTx(f.ctx[a], tx, command))
	mustConnected(t, tx.Commit(f.ctx[a]))
	check := func() {
		t.Helper()
		page, _ := readWalletCommentsPage(t, routes, path, before.Header().Get("ETag"), http.StatusOK)
		if page.Data.Items == nil || len(page.Data.Items) != 0 || page.Next != nil {
			t.Fatalf("erased wallet page = %+v", page)
		}
	}
	check()
	completed := false
	for i := 0; i < 16; i++ {
		receipt, err := f.store.EraseBatch(f.ctx[a], command.CaseID)
		mustConnected(t, err)
		if receipt.State == "completed" {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatal("wallet erasure did not complete")
	}
	check()
}

func TestWalletCommentsProfileReads(t *testing.T) {
	f := walletCommentsData(t)
	a := f.wallets[0]
	for i := 0; i < 48; i++ {
		id, err := uuid.NewV7()
		mustConnected(t, err)
		mustConnected(t, f.store.CreateComment(f.ctx[a], Request{Actor: a, Pool: f.pool, ID: id.String(), Body: fmt.Sprintf("Profile comment %d", i), Key: id.String()}))
	}
	routes := f.http.AuthenticatedRoutes(sharedauth.HTTPIdentityAuthority{})
	page, _ := readWalletCommentsPage(t, routes, "/users/"+a+"/comments", "", http.StatusOK)
	if len(page.Data.Items) != 50 || page.Next == nil {
		t.Fatalf("profile page = %+v", page)
	}
	fixturePath := os.Getenv("WALLET_COMMENTS_PROFILE_FIXTURE")
	if fixturePath != "" {
		server := httptest.NewServer(http.StripPrefix("/v2/social", routes))
		defer server.Close()
		missing := httptest.NewServer(http.NotFoundHandler())
		defer missing.Close()
		unavailable := httptest.NewServer(http.StripPrefix("/v2/social", (&HTTP{}).Routes()))
		defer unavailable.Close()
		raw, err := json.Marshal(map[string]any{"origin": server.URL, "missingOrigin": missing.URL, "unavailableOrigin": unavailable.URL, "wallet": a, "emptyWallet": f.wallets[2], "pool": f.pool, "otherPool": f.otherPool, "items": page.Data.Items, "nextCursor": page.Next})
		mustConnected(t, err)
		mustConnected(t, os.WriteFile(fixturePath+".tmp", raw, 0600))
		mustConnected(t, os.Rename(fixturePath+".tmp", fixturePath))
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.Fatal("profile comment reads timed out")
			case <-ticker.C:
				if _, err := os.Stat(filepath.Join(filepath.Dir(fixturePath), "closed")); err == nil {
					return
				} else if !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
		}
	}
}
