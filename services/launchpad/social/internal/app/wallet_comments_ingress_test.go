package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestWalletCommentsReadPassesTheIngressWithoutASession(t *testing.T) {
	h := newPlatformHarness(t)
	actor, target := addr(0xa11ce), addr(0xb0b)
	identity := h.wire.session("Bearer wallet-comments-reader", actor)
	peer, err := x509.ParseCertificate(h.pki.gatewayClient.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{
		binding:      config.SocialRuntimeBinding{GatewayDNS: "gateway.railway.internal", GatewayCertSHA256: h.pki.gatewayClient.SHA256},
		identity:     h.app.platform.authority,
		chatIdentity: h.app.platform.authority,
	}
	runtime.request = runtime.authenticateRequest
	issuer := sharedauth.ServiceAssertionIssuer{
		Issuer: "gateway", Audience: "social", Key: testpki.Ed25519(t).Private,
		Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.ActorAssertionHeader, SignatureHeader: sharedauth.ActorSignatureHeader},
	}
	path := "/v2/social/users/" + target + "/comments"
	for _, tc := range []struct {
		name, method, path string
		trusted, session   bool
		status             int
	}{
		{"anonymous comments", http.MethodGet, path, true, false, http.StatusNoContent},
		{"followers require session", http.MethodGet, "/v2/social/users/" + target + "/followers", true, false, http.StatusUnauthorized},
		{"following requires session", http.MethodGet, "/v2/social/users/" + target + "/following", true, false, http.StatusUnauthorized},
		{"comment creation requires session", http.MethodPost, "/v2/social/pool/" + h.pool + "/comments", true, false, http.StatusUnauthorized},
		{"comments require gateway peer", http.MethodGet, path, false, false, http.StatusForbidden},
		{"session comments", http.MethodGet, path, true, true, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Actor-Wallet", target)
			if tc.trusted {
				req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}, VerifiedChains: [][]*x509.Certificate{{peer}}}
			}
			if tc.session {
				req.Header.Set("Authorization", "Bearer wallet-comments-reader")
				req.Header.Set("X-Request-ID", uuid.NewString())
				if err := issuer.SignRequest(req, identity, nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				called = true
				principal, present := fanout.PrincipalFromContext(req.Context())
				interactionActor, interactionErr := interactions.Identity(req.Context())
				if tc.session {
					if !present || principal.Wallet != actor || principal.SessionID != identity.SessionID || req.Header.Get("X-Actor-Wallet") != actor || interactionErr != nil || interactionActor.Wallet != actor {
						t.Errorf("session actor = %+v, present %t, header %q, interaction actor %+v: %v", principal, present, req.Header.Get("X-Actor-Wallet"), interactionActor, interactionErr)
					}
				} else if present || req.Header.Get("X-Actor-Wallet") != "" || interactionErr == nil {
					t.Errorf("anonymous actor = %+v, present %t, header %q, interaction actor %+v: %v", principal, present, req.Header.Get("X-Actor-Wallet"), interactionActor, interactionErr)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			rw := httptest.NewRecorder()
			runtime.ingress(next).ServeHTTP(rw, req)
			if rw.Code != tc.status {
				t.Fatalf("ingress status = %d, want %d: %s", rw.Code, tc.status, rw.Body.String())
			}
			if want := tc.status == http.StatusNoContent; called != want {
				t.Fatalf("next handler reached = %t, want %t", called, want)
			}
		})
	}
	t.Run("rows", TestWalletCommentsReadAnswersRowsThroughTheIngress)
}

func TestWalletCommentsReadAnswersRowsThroughTheIngress(t *testing.T) {
	h := newDevHarness(t)
	ctx := context.Background()
	actor, other, secondPool := addr(0xa11ce), addr(0xb0b), addr(0x9002)
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,nft_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,2,$6,101,$7)`, secondPool, addr(0x7002), h.creator, addr(0xb7), "0x"+strings.Repeat("ef", 32), time.Now().Unix(), "0x"+strings.Repeat("12", 32)); err != nil {
		t.Fatal(err)
	}
	store := interactions.Store{DB: h.app.pool, Authority: &devsocial.Policy{DB: h.app.pool, Pools: devsocial.LegacyPools}}
	olderID, newerID, otherID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, input := range []interactions.Request{
		{Actor: actor, Pool: h.pool, ID: olderID, Key: "comment:" + olderID, Body: "first pool comment"},
		{Actor: actor, Pool: secondPool, ID: newerID, Key: "comment:" + newerID, Body: "second pool comment"},
		{Actor: other, Pool: h.pool, ID: otherID, Key: "comment:" + otherID, Body: "another wallet comment"},
	} {
		if err := store.CreateComment(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.dbConn.Exec(ctx, `UPDATE chat.interaction_comments SET created_at=$2 WHERE id=$1`, olderID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `UPDATE chat.interaction_comments SET created_at=$2 WHERE id=$1`, newerID, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	pki := newPlatformPKI(t)
	runtime := &socialConnectedRuntime{
		binding:      config.SocialRuntimeBinding{GatewayDNS: "gateway.railway.internal", GatewayCertSHA256: pki.gatewayClient.SHA256},
		interactions: &interactions.HTTP{Store: store, Policy: &interactions.PGPolicy{DB: h.app.pool}},
	}
	router := chi.NewRouter()
	router.Mount("/v2/social", http.StripPrefix("/v2/social", runtime.interactions.Routes()))
	server := httptest.NewUnstartedServer(runtime.ingress(router))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pki.socialServer.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.ca.Roots()}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := pki.mtlsClient(pki.gatewayClient)
	t.Cleanup(client.CloseIdleConnections)
	resp, err := client.Get(server.URL + "/v2/social/users/" + actor + "/comments")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("wallet comments status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var result struct {
		Data struct {
			Items []interactions.CommentView `json:"items"`
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data.Items) != 2 || result.NextCursor != nil {
		t.Fatalf("wallet comments = %+v, next cursor = %v", result.Data.Items, result.NextCursor)
	}
	for i, want := range []interactions.CommentView{
		{ID: newerID, Pool: secondPool, Author: actor, Body: "second pool comment"},
		{ID: olderID, Pool: h.pool, Author: actor, Body: "first pool comment"},
	} {
		got := result.Data.Items[i]
		if got.ID != want.ID || got.Pool != want.Pool || got.Author != want.Author || got.Body != want.Body {
			t.Errorf("comment %d = %+v, want %+v", i, got, want)
		}
	}
	if resp.Header.Get("ETag") == "" {
		t.Fatal("wallet comments response has no entity tag")
	}
}
