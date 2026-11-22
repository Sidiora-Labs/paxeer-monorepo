package cache

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/google/uuid"
)

func localAdversarialCacheFixture(t *testing.T, authenticated bool) adversarialCacheFixture {
	t.Helper()
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	redisURL := os.Getenv("L09_REDIS_URL")
	if redisURL == "" {
		t.Fatal("L09_REDIS_URL must name the reserved cache database")
	}
	t.Setenv(internaltest.ExternalRedisEnv, redisURL)
	redisURL = internaltest.NewRedisURL(t)
	options, err := url.Parse(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("L09_EXCLUSIVE_REDIS_DB", strings.TrimPrefix(options.Path, "/"))
	t.Setenv("L09_DATABASE_URL", dsn)
	t.Setenv("L09_REDIS_URL", redisURL)
	t.Setenv("L09_NAMESPACE", "cache_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
	ctx := context.Background()
	root := internaltest.RepositoryRoot(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, schema := range []string{"pnl", "users"} {
		if _, err = tx.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"170000", "130000", "60001", "340000"} {
		raw, err := os.ReadFile(filepath.Join(root, "migrations", name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES('market-1','fill-1',$1,'market-v1',$2,1700000000,'null','{"usdlAmountRaw":"1000000"}',1,1)`, strings.Repeat("a", 64), "0x"+strings.Repeat("1", 40)); err != nil {
		t.Fatal(err)
	}
	if !authenticated {
		return newCacheFixture(t, dsn, redisURL, os.Getenv("L09_NAMESPACE"))
	}
	identity := startCacheIdentity(t, dsn, root)
	t.Setenv("L09_IDENTITY_ENDPOINT", identity.URL)
	t.Setenv("L09_IDENTITY_SERVICE_TOKEN", identity.ServiceToken)
	t.Setenv("L09_WALLET_A_ACCESS", identity.Sessions[0].Token)
	t.Setenv("L09_WALLET_B_ACCESS", identity.Sessions[1].Token)
	for i, session := range identity.Sessions {
		if _, err = pool.Exec(ctx, `INSERT INTO pnl.portfolio_snapshots(id,wallet,source_event,digest,revision,expires_at,summary,observed_at) VALUES($1,$2,$3,$4,1,now()+interval '1 hour',jsonb_build_object('balance',$5::text),now())`, uuid.NewString(), session.Wallet, "portfolio-"+strconv.Itoa(i), strings.Repeat("b", 64), strconv.Itoa(100*(i+1))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_replay_checkpoints VALUES('chain125',1,$1)`, "0x"+strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO users.operation_ledger(actor,action,nonce,envelope_digest,body_digest,request_id,response,response_digest) VALUES($1,'portfolio.read','1',$2,$2,$3,$4,$2)`, identity.Sessions[0].Wallet, strings.Repeat("d", 64), uuid.NewString(), []byte(`{"status":"complete"}`)); err != nil {
		t.Fatal(err)
	}
	return newCacheFixture(t, dsn, redisURL, os.Getenv("L09_NAMESPACE"))
}

type cacheIdentity struct {
	URL, ServiceToken string
	Sessions          []struct{ Wallet, Token string }
}

func startCacheIdentity(t *testing.T, dsn, root string) cacheIdentity {
	t.Helper()
	chain := internaltest.NewAnvil(t)
	chain.Commit()
	source := filepath.Join(root, "services", "launchpad", "gateway")
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	const gateway = "github.com/Sidiora-Technologies/KindleLaunch/media/gateway"
	manifest := strings.Replace(string(raw), "module "+gateway, "module "+gateway+"/cachefixture", 1)
	relative := regexp.MustCompile(`(?m)(=>\s+)(\.[^\s]+)`)
	manifest = relative.ReplaceAllStringFunc(manifest, func(line string) string {
		parts := relative.FindStringSubmatch(line)
		return parts[1] + strconv.Quote(filepath.Join(source, parts[2]))
	})
	manifest += "\nrequire " + gateway + " v0.0.0\nreplace " + gateway + " => " + strconv.Quote(source) + "\n"
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(source, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "go.sum"), sums, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/identity/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "main.go"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "go", "run", "-mod=mod", ".")
	cmd.Dir = dir
	database, err := url.Parse(dsn)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	query := database.Query()
	query.Set("search_path", "users,pg_catalog")
	database.RawQuery = query.Encode()
	cmd.Env = append(os.Environ(), "GOWORK=off", "CACHE_IDENTITY_DSN="+database.String(), "CACHE_IDENTITY_RPC="+chain.URL, "CACHE_IDENTITY_MIGRATION="+filepath.Join(source, "internal", "devidentity", "migrations", "00001_identity.sql"))
	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	logs, err := os.Create(filepath.Join(t.TempDir(), "identity.log"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cmd.Stderr = logs
	if err = cmd.Start(); err != nil {
		cancel()
		_ = logs.Close()
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	t.Cleanup(func() { _ = input.Close(); cancel(); <-exited; _ = logs.Close() })
	var identity cacheIdentity
	decoded := make(chan error, 1)
	go func() { decoded <- json.NewDecoder(output).Decode(&identity) }()
	select {
	case err = <-decoded:
		if err != nil {
			log, _ := os.ReadFile(logs.Name())
			t.Fatalf("identity startup: %v\n%s", err, log)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("identity startup deadline exceeded")
	}
	if identity.URL == "" || len(identity.ServiceToken) < 32 || len(identity.Sessions) != 2 {
		t.Fatal("identity requires two admitted sessions")
	}
	return identity
}
