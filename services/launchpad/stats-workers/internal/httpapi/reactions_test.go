package httpapi_test

import (
	"context"
	"errors"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	redis "github.com/redis/go-redis/v9"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
)

type reactionsResp struct {
	PoolAddress string `json:"poolAddress"`
	Reactions   struct {
		Bullish int `json:"bullish"`
		Hot     int `json:"hot"`
		Diamond int `json:"diamond"`
		Bearish int `json:"bearish"`
		Trash   int `json:"trash"`
		Warning int `json:"warning"`
	} `json:"reactions"`
	Total    int     `json:"total"`
	UserVote *string `json:"userVote"`
}

func TestReactionsRoutes(t *testing.T) {
	repair1039LegacyRetirement(t)
	repair1039DurableReplacement(t)
}

func TestPhase3DurableRoutesRejectHostileBodiesAndMissingAuthority(t *testing.T) {
	router := chi.NewRouter()
	policies := auth.PolicyRegistry{Version: "phase3", Actions: map[string]auth.Policy{"reaction.replace": {Scope: "wallet", OwnerRequired: true, RouteTemplate: "/v2/stats/{poolAddress}/reactions"}}}
	if err := httpapi.RegisterDurableReactions(router, httpapi.DurableReactionDeps{Policies: policies}); err != nil {
		t.Fatal(err)
	}
	httpapi.RegisterDurableAnalytics(router, analytics.ReactionAuthority{})
	pool := "0x" + strings.Repeat("1", 40)
	valid := `{"chainId":"125","pool":"` + pool + `","wallet":"0x` + strings.Repeat("2", 40) + `","reaction":"hot","nonce":"attack","expiry":1,"expectedRevision":0}`
	for _, c := range []struct {
		body   string
		status int
	}{
		{strings.Repeat("x", 65537), 413},
		{`{"reaction":"hot","reaction":"diamond"}`, 400},
		{valid + ` {}`, 400},
		{strings.Replace(valid, `"nonce":"attack"`, `"nonce":"attack","unknown":true`, 1), 400},
		{strings.Replace(valid, `"expectedRevision":0`, `"expectedRevision":-1`, 1), 422},
		{strings.Replace(valid, `"reaction":"hot"`, `"reaction":"unknown"`, 1), 422},
		{strings.Replace(valid, `"chainId":"125"`, `"chainId":"1"`, 1), 422},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/v2/stats/"+pool+"/reactions", strings.NewReader(c.body)))
		if response.Code != c.status {
			t.Fatal(c.status, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"pressure", "risk", "reactions", "analytics/revisions"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/v2/stats/"+pool+"/"+path, nil))
		if response.Code != 503 {
			t.Fatal("missing durable dependency fabricated success", path, response.Code, response.Body.String())
		}
	}
}
func TestPhase3RealRedisConcurrentAdmissionAndDependencyFailure(t *testing.T) {
	binary, err := exec.LookPath("redis-server")
	if err != nil {
		t.Fatal("real redis-server required", err)
	}
	dir := t.TempDir()
	socket := filepath.Join(dir, "redis.sock")
	child := exec.Command(binary, "--port", "0", "--unixsocket", socket, "--unixsocketperm", "700", "--dir", dir, "--save", "", "--appendonly", "no")
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			child.Process.Kill()
			child.Wait()
		}
	}()
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket, MaxRetries: -1, DialTimeout: 100 * time.Millisecond, ReadTimeout: 100 * time.Millisecond, WriteTimeout: 100 * time.Millisecond})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err = client.Ping(ctx).Err(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	admission := &httpapi.ReactionAdmission{Redis: client, Namespace: "phase3", WalletPerMinute: 12, IPPerMinute: 12}
	results := make(chan error, 32)
	wallet := "0x" + strings.Repeat("3", 40)
	for n := 0; n < 32; n++ {
		go func() { results <- admission.Admit(ctx, wallet, "127.0.0.1") }()
	}
	accepted, rejected := 0, 0
	for n := 0; n < 32; n++ {
		err := <-results
		if err == nil {
			accepted++
		} else if errors.Is(err, analytics.ErrCooldown) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 12 || rejected != 20 {
		t.Fatal("admission was not atomic", accepted, rejected)
	}
	for _, key := range []string{"phase3:{reaction-admission}:wallet:" + wallet, "phase3:{reaction-admission}:ip:127.0.0.1"} {
		count, err := client.Get(ctx, key).Int()
		if err != nil || count != 12 {
			t.Fatal(key, count, err)
		}
		ttl, err := client.PTTL(ctx, key).Result()
		if err != nil || ttl <= 0 || ttl > time.Minute {
			t.Fatal(key, ttl, err)
		}
	}
	if err = admission.Admit(ctx, "bad-wallet", "127.0.0.1"); !errors.Is(err, analytics.ErrInvalidInput) {
		t.Fatal(err)
	}
	if err = child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	child.Wait()
	stopped = true
	if err = admission.Admit(ctx, wallet, "127.0.0.1"); !errors.Is(err, analytics.ErrDependencyUnavailable) {
		t.Fatal("Redis outage did not fail closed", err)
	}
	var absent *httpapi.ReactionAdmission
	if err = absent.Admit(ctx, wallet, "127.0.0.1"); !errors.Is(err, analytics.ErrDependencyUnavailable) {
		t.Fatal(err)
	}
}
