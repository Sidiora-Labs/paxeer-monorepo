package search

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestCacheAndPreferenceInputBounds(t *testing.T) {
	ctx := context.Background()
	var absent *Service
	if _, err := absent.Public(ctx, IntegratedRequest{}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := absent.Personalize(ctx, IntegratedPage{}, auth.IdentityPrincipal{}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := absent.CacheMeasurement(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if err := absent.Admit("127.0.0.1"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, _, _, err := absent.state(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	pool := integratedDatabase(t)
	for _, maximum := range []int{0, 129} {
		if _, err := NewService(Store{Pool: pool}, nil, maximum); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	client := redis.NewClient(&redis.Options{ContextTimeoutEnabled: true, MaxRetries: -1, DialTimeout: time.Second})
	defer client.Close()
	if _, err := NewService(Store{Pool: pool}, client, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	service, err := NewService(Store{Pool: pool}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Public(ctx, IntegratedRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := service.Personalize(ctx, IntegratedPage{}, auth.IdentityPrincipal{}); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(err)
	}
	for _, p := range []PreferenceSnapshot{
		{}, {AccountID: uuid.NewString(), Wallet: fixtureAddress(1), Revision: 1, MutedCreators: make([]string, 1001)},
		{AccountID: uuid.NewString(), Wallet: fixtureAddress(1), Revision: 1, SavedPools: []string{fixtureAddress(2), fixtureAddress(2)}},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid preferences: %+v %v", p, err)
		}
	}
	for _, wire := range [][]byte{make([]byte, 131073), []byte(`{"payload":{},"mac":"invalid"}`)} {
		if _, ok := service.decodeCached("key", wire); ok {
			t.Fatal("unbound cache accepted")
		}
	}
	raw := []byte(`[]`)
	mac := hmac.New(sha256.New, service.signingKey[:])
	mac.Write([]byte("key"))
	mac.Write([]byte{0})
	mac.Write(raw)
	wire, err := json.Marshal(authenticatedPage{Payload: raw, MAC: hex.EncodeToString(mac.Sum(nil))})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := service.decodeCached("key", wire); ok {
		t.Fatal("signed non-page accepted")
	}
	for i := 0; i < 4096; i++ {
		service.admissions[fmt.Sprint(i)] = admissionWindow{start: time.Now()}
	}
	if err := service.Admit("127.0.0.2"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	for k := range service.admissions {
		service.admissions[k] = admissionWindow{start: time.Now().Add(-time.Hour)}
	}
	if err := service.Admit("127.0.0.2"); err != nil || len(service.admissions) != 1 {
		t.Fatalf("expired admission retained: %d %v", len(service.admissions), err)
	}
}

func TestPublicSearchCoalescesSuccessfulDatabaseRead(t *testing.T) {
	pool := integratedDatabase(t)
	d := seedIndexed(t, pool, 180)
	service, err := NewService(Store{Pool: pool}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackSearch(blocker)
	if _, err := blocker.Exec(ctx, "LOCK TABLE stats_search.documents_v2 IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	request := IntegratedRequest{Request: Request{Query: d.PoolAddress, Kind: "pool", Limit: 25}, Sort: "address"}
	type result struct {
		page IntegratedPage
		err  error
	}
	results := make(chan result, 2)
	query := func() { page, err := service.Public(ctx, request); results <- result{page, err} }
	go query()
	wait := func(wantWaiters int) {
		t.Helper()
		deadline := time.Now().Add(100 * time.Millisecond)
		for {
			measurement, err := service.CacheMeasurement()
			if err != nil {
				t.Fatal(err)
			}
			if measurement.ActiveFills == 1 && measurement.Waiters == wantWaiters {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("fill state: %+v", measurement)
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(0)
	go query()
	wait(1)
	other := request
	other.Query = fixtureAddress(181)
	if _, err := service.Public(ctx, other); !errors.Is(err, ErrBusy) {
		t.Fatalf("parallel fill admitted: %v", err)
	}
	if err := blocker.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || len(a.page.Items) != 1 || len(b.page.Items) != 1 || *a.page.Items[0].PoolAddress != d.PoolAddress || *b.page.Items[0].PoolAddress != d.PoolAddress {
		t.Fatalf("coalesced results: %+v %+v", a, b)
	}
	a.page.Items[0].Ticker = "changed"
	if b.page.Items[0].Ticker == "changed" {
		t.Fatal("coalesced pages share mutable items")
	}
	measured, err := service.CacheMeasurement()
	if err != nil || measured.Coalesced != 1 || measured.DatabaseReads != 1 || measured.ActiveFills != 0 {
		t.Fatalf("coalesced counters: %+v %v", measured, err)
	}
}
