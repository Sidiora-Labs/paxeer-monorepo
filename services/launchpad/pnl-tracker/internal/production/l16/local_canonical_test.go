package l16

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

type localCanonicalChain struct {
	RPC, Factory, Wallet, Token, Stream string
	Height                              uint64
}

type localCanonicalFixture struct {
	ctx       context.Context
	chain     localCanonicalChain
	db        coordinatortest.Database
	authority *BaselineAuthority
	env       []string
	root      string
}

func newLocalCanonicalFixture(t *testing.T, mode string) localCanonicalFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
	t.Cleanup(cancel)
	root := coordinatortest.RepositoryRoot(t)
	db := coordinatortest.Bootstrap(t, realMarketEnv(t, "PNL_TEST_POSTGRES_DSN"), "indexer_runtime", "pnl_runtime")
	control := t.TempDir()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GOWORK=off", "L16_FIXTURE_MODE="+mode, "L16_FIXTURE_CONTROL="+control, "L16_FIXTURE_ROOT="+root, "L16_FIXTURE_ADMIN_DSN="+db.AdminDSN, "L16_FIXTURE_WALLET_KEY="+hex.EncodeToString(crypto.FromECDSA(key)))
	binary := localCanonicalProducer.load(t, "canonical producer", func() (string, error) {
		return buildLocalCanonicalProducer(ctx, root, env)
	})
	indexer := filepath.Join(root, "services/launchpad/indexer")
	cmd := exec.CommandContext(ctx, binary, "-test.count=1", "-test.timeout=13m", "-test.run=^TestPnlCanonicalFixture$", "-test.v")
	cmd.Dir, cmd.Env = filepath.Join(indexer, "internal/processor"), env
	logPath := filepath.Join(control, "indexer.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = log, log
	started := time.Now()
	if err = cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() {
		if err := os.WriteFile(filepath.Join(control, "release"), nil, 0600); err != nil {
			t.Error(err)
		}
		select {
		case err := <-exited:
			if err != nil {
				raw, _ := os.ReadFile(logPath)
				t.Errorf("canonical producer: %v\n%s", err, raw)
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
			t.Error("canonical producer did not release its chain")
		}
		_ = log.Close()
	})
	var chain localCanonicalChain
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, readErr := os.ReadFile(filepath.Join(control, "chain.json"))
		if readErr == nil {
			if err = json.Unmarshal(raw, &chain); err != nil {
				t.Fatal(err)
			}
			break
		}
		if !os.IsNotExist(readErr) {
			t.Fatal(readErr)
		}
		select {
		case err := <-exited:
			exited <- err
			raw, _ := os.ReadFile(logPath)
			t.Fatalf("canonical producer: %v\n%s", err, raw)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	t.Logf("canonical producer ready: %s", time.Since(started))
	pool, err := pgxpool.New(ctx, db.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rpc, err := ethclient.DialContext(ctx, chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpc.Close)
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewBaselineAuthorityWithEpochGuard(pool, rpc, "pnl", chain.Stream, "pnl", "g1", "pnl-local", "inventory-v1", guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	authority.factory = chain.Factory
	return localCanonicalFixture{ctx: ctx, chain: chain, db: db, authority: authority, env: env, root: root}
}

func localCanonicalBaseline(t *testing.T) (context.Context, *BaselineAuthority, string, string) {
	t.Helper()
	fixture := newLocalCanonicalFixture(t, "fresh")
	return fixture.ctx, fixture.authority, fixture.chain.Wallet, fixture.chain.Token
}

func localCanonicalWorker(t *testing.T, prefix string) (context.Context, *LiveTransferWorker, auth.IdentityPrincipal, liveFixtureValues) {
	t.Helper()
	mode := map[string]string{"L16_GIFT_": "gift", "L16_DENSE_": "dense", "L16_MIXED_": "mixed"}[prefix]
	if mode == "" {
		t.Fatal("unknown canonical portfolio scenario")
	}
	fixture := newLocalCanonicalFixture(t, mode)
	ctx, a := fixture.ctx, fixture.authority
	second := localCanonicalSecondNode(t, fixture)
	identity, access := localCanonicalIdentity(t, fixture)
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}}
	outbox, err := delivery.NewStoreWithEpochGuard(a.pool, "pnl", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(a, second, [2]string{"local-primary", "local-fork"}, strings.Repeat("c", 64), outbox)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: a.pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/v3/me/portfolio", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+access)
	principal, err := identity.AuthenticateRequest(ctx, request)
	if err != nil || principal.Wallet != fixture.chain.Wallet {
		t.Fatal(principal.Wallet, err)
	}
	values := liveFixtureValues{prefix + "WALLET": fixture.chain.Wallet, prefix + "TOKEN": fixture.chain.Token}
	if mode == "dense" {
		values[prefix+"EXPECTED_HISTORY_COUNT"] = "10001"
	}
	return ctx, worker, principal, values
}

func localCanonicalSecondNode(t *testing.T, fixture localCanonicalFixture) *ethclient.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(fixture.ctx, "anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--chain-id", "125", "--fork-url", fixture.chain.RPC, "--fork-block-number", strconv.FormatUint(fixture.chain.Height, 10), "--silent")
	log, err := os.Create(filepath.Join(t.TempDir(), "second-node.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = log.Close() })
	client, err := ethclient.DialContext(fixture.ctx, fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		height, err := client.BlockNumber(fixture.ctx)
		if err == nil && height == fixture.chain.Height {
			return client
		}
		select {
		case <-fixture.ctx.Done():
			t.Fatal(fixture.ctx.Err())
		case <-ticker.C:
		}
	}
}

// localCanonicalIdentity runs the gateway's actual wallet login and session
// authority in a temporary module that may import the gateway's internal API.
func localCanonicalIdentity(t *testing.T, fixture localCanonicalFixture) (auth.HTTPIdentityAuthority, string) {
	t.Helper()
	dir := t.TempDir()
	env := append(fixture.env, "L16_FIXTURE_RPC="+fixture.chain.RPC)
	binary := localCanonicalIdentityProgram.load(t, "canonical identity", func() (string, error) {
		return buildLocalCanonicalIdentity(fixture.ctx, fixture.root, env)
	})
	cmd := exec.CommandContext(fixture.ctx, binary)
	cmd.Dir, cmd.Env = filepath.Dir(binary), env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "identity.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = log
	started := time.Now()
	if err = cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			raw, _ := os.ReadFile(logPath)
			t.Errorf("identity authority: %v\n%s", err, raw)
		}
		_ = log.Close()
	})
	var identity struct{ URL, ServiceToken, AccessToken string }
	if err = json.NewDecoder(io.LimitReader(stdout, 16384)).Decode(&identity); err != nil {
		raw, _ := os.ReadFile(logPath)
		t.Fatalf("identity session: %v\n%s", err, raw)
	}
	t.Logf("canonical identity ready: %s", time.Since(started))
	return auth.HTTPIdentityAuthority{Endpoint: identity.URL, ServiceToken: identity.ServiceToken, Client: &http.Client{Timeout: 2 * time.Second}}, identity.AccessToken
}
