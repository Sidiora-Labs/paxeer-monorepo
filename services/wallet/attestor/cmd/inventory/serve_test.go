package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
)

type loopbackGateway struct {
	mu       sync.Mutex
	requests []server.InventoryKey
	posts    []server.WalletInventory
	confirm  bool
	server   *httptest.Server
}

func newLoopbackGateway(t *testing.T, requests []server.InventoryKey) *loopbackGateway {
	t.Helper()
	gateway := &loopbackGateway{requests: requests, confirm: true}
	gateway.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-admin" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		gateway.mu.Lock()
		defer gateway.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == requestsPath:
			_ = json.NewEncoder(w).Encode(requestEnvelope{InventoryRequests: gateway.requests})
		case r.Method == http.MethodPost && r.URL.Path == admitPath:
			var body struct {
				Token string `json:"token"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			verifier := gatewayVerifier(t)
			var inventory server.WalletInventory
			if err := verifier.Decode(body.Token, agent.InventoryAudience, maxLifetime, &inventory); err != nil {
				t.Fatalf("gateway received invalid inventory: %v", err)
			}
			gateway.posts = append(gateway.posts, inventory)
			members := make([]postMember, 0, len(inventory.Members))
			for _, member := range inventory.Members {
				sequence := inventory.Sequence
				if !gateway.confirm {
					sequence = "0"
				}
				members = append(members, postMember{NodeID: member.ID, Sequence: sequence})
			}
			_ = json.NewEncoder(w).Encode(postResponse{Sequence: inventory.Sequence, Members: members, Retained: &gateway.confirm})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(gateway.server.Close)
	return gateway
}

var testGatewayVerifier *agent.SignedVerifier

func gatewayVerifier(t *testing.T) *agent.SignedVerifier {
	t.Helper()
	if testGatewayVerifier == nil {
		t.Fatal("gateway verifier not initialized")
	}
	return testGatewayVerifier
}

func newServeFixture(t *testing.T, requests []server.InventoryKey) (*inventoryService, *loopbackGateway, *inventoryFixture) {
	t.Helper()
	f := newFixture(t)
	f.init(t)
	verifier, err := agent.NewSignedVerifier(f.public, "wallet-inventory-owner", "wallet-tenant")
	if err != nil {
		t.Fatal(err)
	}
	testGatewayVerifier = verifier
	t.Cleanup(func() { testGatewayVerifier = nil })
	gateway := newLoopbackGateway(t, requests)
	service := &inventoryService{
		settings: serveSettings{
			gatewayURL:         gateway.server.URL,
			adminToken:         "test-admin",
			stateDir:           f.dir,
			interval:           time.Second,
			renewalMargin:      time.Second,
			maxGenerationsHour: 100,
			maxKeysOwner:       100,
		},
		private:  *f.key,
		verifier: verifier,
		client:   gateway.server.Client(),
	}
	return service, gateway, f
}

func generationRequest(keyID, curve, owner string) server.InventoryKey {
	return server.InventoryKey{KeyID: keyID, Curve: curve, Owner: owner, Operations: []string{"generate"}}
}

func signingRequest(keyID, curve, owner string) server.InventoryKey {
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	return server.InventoryKey{KeyID: keyID, Epoch: 1, Curve: curve, PublicKey: hex.EncodeToString(public), Owner: owner, Operations: []string{"sign", "refresh"}}
}

func decodeState(t *testing.T, f *inventoryFixture) server.WalletInventory {
	t.Helper()
	return f.decode(t)
}

func TestServeApprovesAndPostsGeneration(t *testing.T) {
	request := generationRequest("account:user-a:standard:ed25519:0", "ed25519", "user-a")
	service, gateway, f := newServeFixture(t, []server.InventoryKey{request})
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(gateway.posts) != 1 || len(gateway.posts[0].Keys) != 1 || !reflect.DeepEqual(gateway.posts[0].Keys[0], request) {
		t.Fatalf("generation was not posted: %+v", gateway.posts)
	}
	if got := decodeState(t, f); got.Sequence != "2" || got.Keys[0].KeyID != request.KeyID {
		t.Fatalf("state did not retain generation: %+v", got)
	}
	journal, err := os.ReadFile(filepath.Join(f.dir, stateJournalName))
	if err != nil || !strings.Contains(string(journal), `"key_id":"account:user-a:standard:ed25519:0"`) {
		t.Fatalf("approval journal missing key: %v %s", err, journal)
	}
}

func TestServeFollowsGenerationWithSigningRequest(t *testing.T) {
	keyID := "account:user-b:standard:ed25519:0"
	service, gateway, f := newServeFixture(t, []server.InventoryKey{generationRequest(keyID, "ed25519", "user-b")})
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	gateway.requests = []server.InventoryKey{signingRequest(keyID, "ed25519", "user-b")}
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := decodeState(t, f)
	if got.Sequence != "3" || len(got.Keys) != 1 || !hasOperation(got.Keys[0], "sign") || got.Keys[0].PublicKey == "" {
		t.Fatalf("signing admission did not follow generation: %+v", got)
	}
}

func TestServeLeavesSigningWithoutGenerationUnapproved(t *testing.T) {
	keyID := "account:user-c:standard:ed25519:0"
	service, gateway, f := newServeFixture(t, []server.InventoryKey{signingRequest(keyID, "ed25519", "user-c")})
	before := readFile(t, f.inventory)
	requests := append([]server.InventoryKey(nil), gateway.requests...)
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(gateway.posts) != 0 || decodeState(t, f).Sequence != "1" {
		t.Fatalf("unapproved signing request changed state: %+v", gateway.posts)
	}
	if string(readFile(t, f.inventory)) != string(before) || !reflect.DeepEqual(gateway.requests, requests) {
		t.Fatal("unapproved signing request changed the inventory or request list")
	}
	journal, err := os.ReadFile(filepath.Join(f.dir, stateJournalName))
	if err != nil || !strings.Contains(string(journal), `"unapproved":1`) {
		t.Fatalf("unapproved count missing: %v %s", err, journal)
	}
}

func TestServeAppliesHourlyGenerationLimit(t *testing.T) {
	first := generationRequest("wallet:user-d:standard:secp256k1:0", "secp256k1", "user-d")
	second := generationRequest("wallet:user-d:funded:secp256k1:0", "secp256k1", "user-d")
	service, gateway, f := newServeFixture(t, []server.InventoryKey{first, second})
	service.settings.maxGenerationsHour = 1
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := decodeState(t, f)
	if len(got.Keys) != 1 || got.Keys[0].KeyID != first.KeyID || len(gateway.posts) != 1 {
		t.Fatalf("hourly limit not applied: %+v", got)
	}
	journal, err := os.ReadFile(filepath.Join(f.dir, stateJournalName))
	if err != nil || !strings.Contains(string(journal), `"unapproved":1`) {
		t.Fatalf("limited request count missing: %v %s", err, journal)
	}
	gateway.requests = []server.InventoryKey{second}
	restarted := &inventoryService{settings: service.settings, private: service.private, verifier: service.verifier, client: service.client}
	if err := restarted.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := decodeState(t, f); got.Sequence != "2" || len(got.Keys) != 1 || got.Keys[0].KeyID != first.KeyID || len(gateway.posts) != 1 {
		t.Fatalf("hourly limit did not hold across rounds and restart: %+v", got)
	}
}

func TestServeRenewsInsideMargin(t *testing.T) {
	service, gateway, f := newServeFixture(t, nil)
	service.settings.renewalMargin = 10 * 24 * time.Hour
	current := decodeState(t, f)
	current.ExpiresAt = time.Now().Add(2 * time.Hour).Unix()
	token, err := encodeInventory(f.key, current)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeInventory(f.inventory, []byte(token+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := decodeState(t, f)
	if len(gateway.posts) != 1 || got.Sequence != "2" || got.ExpiresAt <= current.ExpiresAt {
		t.Fatalf("inventory was not renewed: before=%+v after=%+v", current, got)
	}
	if !reflect.DeepEqual(got.Members, current.Members) || !reflect.DeepEqual(got.Keys, current.Keys) {
		t.Fatalf("renewal changed members or keys: %+v", got)
	}
}

func TestServeUnconfirmedPostLeavesSequenceForRetry(t *testing.T) {
	request := generationRequest("account:user-e:standard:ed25519:0", "ed25519", "user-e")
	service, gateway, f := newServeFixture(t, []server.InventoryKey{request})
	before, err := os.ReadFile(f.inventory)
	if err != nil {
		t.Fatal(err)
	}
	gateway.confirm = false
	if err := service.round(context.Background()); err == nil {
		t.Fatal("unconfirmed post unexpectedly succeeded")
	}
	after, err := os.ReadFile(f.inventory)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || decodeState(t, f).Sequence != "1" {
		t.Fatal("unconfirmed post changed inventory state")
	}
	if _, err := os.Stat(filepath.Join(f.dir, stateJournalName)); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed post wrote an approval journal: %v", err)
	}
	gateway.confirm = true
	if err := service.round(context.Background()); err != nil {
		t.Fatal(err)
	}
	if decodeState(t, f).Sequence != "2" {
		t.Fatal("retry did not carry the same candidate sequence")
	}
	if len(gateway.posts) != 2 || gateway.posts[0].Sequence != "2" || gateway.posts[1].Sequence != gateway.posts[0].Sequence {
		t.Fatalf("post retry sequences differ: %+v", gateway.posts)
	}
}

func TestServeHealthRequiresRecentRound(t *testing.T) {
	service, _, _ := newServeFixture(t, nil)
	service.settings.interval = 20 * time.Millisecond
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	service.healthHandler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("initial health status = %d", response.Code)
	}
	service.markRound(time.Now())
	response = httptest.NewRecorder()
	service.healthHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("healthy status = %d", response.Code)
	}
	service.markRound(time.Now().Add(-time.Minute))
	response = httptest.NewRecorder()
	service.healthHandler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("stale health status = %d", response.Code)
	}
}
