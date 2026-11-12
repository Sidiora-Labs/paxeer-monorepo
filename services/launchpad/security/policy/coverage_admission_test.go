package policy

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestPublicReadAdmissionUsesRedisAndSignedEdgeLease(t *testing.T) {
	socket := localSecurityRedis(t)
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler, err := Admission(AdmissionOptions{Redis: client, Namespace: "coverage-public", Class: classPublicRead, Cost: 1}, next)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/public", nil)
	record := httptest.NewRecorder()
	handler.ServeHTTP(record, request)
	if record.Code != http.StatusNoContent {
		t.Fatalf("admitted public read: %d", record.Code)
	}
	request.RemoteAddr = "invalid-principal"
	record = httptest.NewRecorder()
	handler.ServeHTTP(record, request)
	if record.Code != http.StatusBadRequest {
		t.Fatalf("invalid request address: %d", record.Code)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	lease := EdgeLease{Origin: "public-cache", PolicyVersion: "v1", ID: "cache-lease", Issued: now.Add(-time.Second), Expires: now.Add(30 * time.Second), PerSecond: 1}
	wire, err := json.Marshal(lease)
	if err != nil {
		t.Fatal(err)
	}
	edge, err := NewEdgeAdmission(lease, ed25519.Sign(private, wire), public, lease.Origin, now)
	if err != nil {
		t.Fatal(err)
	}
	missing := redis.NewClient(&redis.Options{Network: "unix", Addr: socket + ".absent", MaxRetries: -1, DialTimeout: time.Millisecond})
	t.Cleanup(func() {
		if err := missing.Close(); err != nil {
			t.Error(err)
		}
	})
	handler, err = Admission(AdmissionOptions{Redis: missing, Namespace: "coverage-cache", Class: classPublicRead, Cost: 1, CachedPublicRead: true, Edge: edge}, next)
	if err != nil {
		t.Fatal(err)
	}
	request.RemoteAddr = "192.0.2.1:1234"
	record = httptest.NewRecorder()
	handler.ServeHTTP(record, request)
	if record.Code != http.StatusNoContent {
		t.Fatalf("signed cached read: %d", record.Code)
	}
	exhausted := false
	for range 10 {
		record = httptest.NewRecorder()
		handler.ServeHTTP(record, request)
		if record.Code == http.StatusServiceUnavailable && strings.Contains(record.Body.String(), "unavailable") {
			exhausted = true
			break
		}
	}
	if !exhausted {
		t.Fatal("signed lease did not enforce its per-second allowance")
	}
}

func TestBoundaryAndEgressConfigurationAcceptOnlyDeclaredPaths(t *testing.T) {
	crossing := Crossing{ID: "wallet-gateway", Caller: "wallet", Recipient: "gateway", Authority: "signed-session", Threat: "replay", Claims: []string{"wallet", "session"}, PublicIdentity: true}
	if err := ValidateBoundaries([]Crossing{crossing}); err != nil {
		t.Fatalf("declared gateway boundary: %v", err)
	}
	if err := ValidateBoundaries([]Crossing{crossing, crossing}); err == nil {
		t.Fatal("duplicate boundary accepted")
	}
	crossing.Claims = []string{"wallet", "wallet"}
	if err := ValidateBoundaries([]Crossing{crossing}); err == nil {
		t.Fatal("duplicate claim accepted")
	}
	crossing.Claims = []string{"unknown"}
	if err := ValidateBoundaries([]Crossing{crossing}); err == nil {
		t.Fatal("unknown claim accepted")
	}
	client, err := EgressClient([]EgressDestination{{Name: "asset", Hostname: "assets.example", Authority: "public-content"}})
	if err != nil || client == nil {
		t.Fatalf("declared public egress: %v", err)
	}
	if _, err := EgressClient([]EgressDestination{{Name: "asset", Hostname: "assets.example", Authority: "public-content"}, {Name: "asset", Hostname: "other.example", Authority: "public-content"}}); err == nil {
		t.Fatal("duplicate destination accepted")
	}
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	record := httptest.NewRecorder()
	PrivateBoundary(WorkloadVerifier{}, nil).ServeHTTP(record, request)
	if record.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing private handler: %d", record.Code)
	}
}
