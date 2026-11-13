package source

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignedSelectionHTTPAdmissionBounds(t *testing.T) {
	requestKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{31}, ed25519.SeedSize))
	approvalKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{32}, ed25519.SeedSize))
	keys := map[string][]byte{"requester": requestKey.Public().(ed25519.PublicKey), "approver": approvalKey.Public().(ed25519.PublicKey)}
	authority, err := NewSelectionAuthority("canonical", keys)
	if err != nil {
		t.Fatal(err)
	}
	node := repair997Serve(t)
	options := fopts(node.server.URL)
	s := repair997Open(t, options)
	server := httptest.NewServer(authority.Handler(s))
	defer server.Close()
	auth := SourceSelectionAuthorization{ID: "selection", Kind: "incident_recovery", Scope: "canonical", Reason: "canonical history", RequestedBy: "requester", ApprovedBy: "approver", State: "approved", Revision: 1, Expires: time.Now().Add(time.Hour), SourceID: "primary", Height: 128, Hash: committedFixtureHash(128)}
	auth.PayloadHash = SourceSelectionPayloadHash(auth.Scope, auth.SourceID, auth.Height, auth.Hash)
	signed := repair997Signed(t, auth, requestKey, approvalKey)
	body, err := json.Marshal(signed)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, body string
		status             int
	}{
		{"method", http.MethodGet, "", 405},
		{"empty", http.MethodPost, "", 400},
		{"unknown field", http.MethodPost, `{"unknown":true}`, 400},
		{"oversize", http.MethodPost, `{"Authorization":{"Reason":"` + strings.Repeat("x", 17<<10) + `"}}`, 400},
		{"trailing value", http.MethodPost, string(body) + " {}", 400},
		{"invalid signature", http.MethodPost, `{}`, 403},
		{"authority binding", http.MethodPost, string(body), 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			message, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.status || len(message) == 0 {
				t.Fatalf("selection response %d %q", resp.StatusCode, message)
			}
			if tc.status == 405 && resp.Header.Get("Allow") != http.MethodPost {
				t.Fatal("allowed method absent")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := authority.AuthorizeSourceSelection(ctx, auth.ID, auth.SourceID, auth.Height, auth.Hash); err != context.Canceled {
		t.Fatalf("canceled selection %v", err)
	}
	for _, tc := range []struct {
		scope string
		keys  map[string][]byte
	}{
		{"", keys}, {strings.Repeat("x", 129), keys}, {"canonical", nil}, {"canonical", map[string][]byte{"bad id": keys["requester"], "approver": keys["approver"]}}, {"canonical", map[string][]byte{"requester": {1}, "approver": keys["approver"]}},
	} {
		if _, err := NewSelectionAuthority(tc.scope, tc.keys); err == nil {
			t.Fatal("invalid signing authority admitted")
		}
	}
	// The source applies selections only after the same verifier is installed.
	s.opts.SelectionAuthority = authority
	if status := repair997Post(t, server.URL, signed); status != http.StatusConflict {
		t.Fatalf("unknown source selection %d", status)
	}
}
