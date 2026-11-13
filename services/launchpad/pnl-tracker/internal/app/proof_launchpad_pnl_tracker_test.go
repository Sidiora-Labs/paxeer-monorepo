// Proves: l18.Creator.PrepareClaim and l18.JourneyService.Rewards -> registerRewardRoutes / PrepareClaim / net/http -> v2.DecodeRewardsResponse and v2.DecodeRewardClaimResponse.
//
// Actual contracts deployed by l18/fixture.cjs on an owned Anvil process and an
// isolated PostgreSQL database produce the results. No RPC, repository, handler,
// transport, or decoder is replaced. Authentication verifies real EIP-191
// signatures with shared/auth.VerifyEOA. The first Go consumer checks both
// operations, rejects damaged producer bytes, reopens the service and storage
// connection, and replays a persisted preparation. A real NFT transfer must
// revoke that replay immediately, proving the producer rechecks ownership.
// The proof waits for the real transfer receipt before checking the new owner;
// transaction submission alone does not guarantee that Anvil has mined it.
// Reward storage is provisioned exclusively by the checked-in deployment SQL,
// including migration 180001, before the production ledger admission check.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProofLaunchpadPnlTracker(t *testing.T) {
	proveLaunchpadPnLTracker(t)
}

func proveLaunchpadPnLTracker(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
	r, fixture := proofLaunchpadChain(t, ctx, root)
	dsn, pool := proofLaunchpadPostgres(t, ctx)
	execSQL := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	execSQL("CREATE SCHEMA recovery; CREATE SCHEMA indexer")
	epochTemplate, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/production/recovery/epoch.sql"))
	if err != nil {
		t.Fatal(err)
	}
	epochSQL, err := queue.RenderEpochSQL("recovery", string(epochTemplate))
	if err != nil {
		t.Fatal(err)
	}
	execSQL(epochSQL)
	execSQL(`INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,1,'local',1,$1)`, strings.Repeat("a", 64))
	proofLaunchpadBootstrapRewardCoordinator(t, ctx, pool, root)
	proofLaunchpadApplyRegisteredRewardMigration(t, ctx, pool, root)
	execSQL(`CREATE TABLE indexer.indexer_canonical_creations (
		nft_id numeric PRIMARY KEY,pool text NOT NULL,creator text NOT NULL,
		height bigint NOT NULL,block_hash text NOT NULL,transaction_hash text NOT NULL,
		transaction_index numeric NOT NULL,log_index numeric NOT NULL);
		CREATE TABLE indexer.indexer_ingest_watermarks (
		chain_id bigint,stream_id text,raw_complete_height bigint,typed_complete_height bigint);
		CREATE TABLE indexer.indexer_block_completeness (
		chain_id bigint,stream_id text,height bigint,block_hash text,
		fetch_complete boolean,required_decode_complete boolean)`)
	var head struct {
		Number hexutil.Uint64 `json:"number"`
		Hash   string         `json:"hash"`
	}
	if err := r.CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false); err != nil {
		t.Fatal(err)
	}
	var minted []struct {
		Topics           []string       `json:"topics"`
		BlockNumber      hexutil.Uint64 `json:"blockNumber"`
		BlockHash        string         `json:"blockHash"`
		TransactionHash  string         `json:"transactionHash"`
		TransactionIndex hexutil.Uint64 `json:"transactionIndex"`
		LogIndex         hexutil.Uint64 `json:"logIndex"`
	}
	if err := r.CallContext(ctx, &minted, "eth_getLogs", map[string]any{
		"address": l18.NFT, "fromBlock": "0x0", "toBlock": "latest",
		"topics": []any{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")).Hex(), common.Hash{}.Hex(), nil, common.BigToHash(big.NewInt(1)).Hex()},
	}); err != nil || len(minted) != 1 {
		t.Fatalf("actual NFT creation: logs=%+v err=%v", minted, err)
	}
	m := minted[0]
	execSQL(`INSERT INTO indexer.indexer_canonical_creations VALUES(1,$1,$2,$3,$4,$5,$6,$7)`, fixture.Pool, fixture.Alice, uint64(m.BlockNumber), m.BlockHash, m.TransactionHash, uint64(m.TransactionIndex), uint64(m.LogIndex))
	execSQL(`INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'proof', $1,$1)`, uint64(head.Number))
	execSQL(`INSERT INTO indexer.indexer_block_completeness VALUES(125,'proof',$1,$2,true,true)`, uint64(head.Number), head.Hash)
	guard, err := queue.NewEpochGuard("recovery", 1, "local")
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for _, target := range []string{l18.NFT, l18.Accumulator, l18.FeesRouter, l18.USDL} {
		var code, slot hexutil.Bytes
		if err := r.CallContext(ctx, &code, "eth_getCode", target, "latest"); err != nil {
			t.Fatal(err)
		}
		hashes[target] = crypto.Keccak256Hash(code).Hex()
		if err := r.CallContext(ctx, &slot, "eth_getStorageAt", target, "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", "latest"); err != nil || len(slot) != 32 {
			t.Fatalf("actual implementation slot: %v", err)
		}
		implementation := strings.ToLower(common.BytesToAddress(slot[12:]).Hex())
		if implementation != (common.Address{}).Hex() {
			if err := r.CallContext(ctx, &code, "eth_getCode", implementation, "latest"); err != nil {
				t.Fatal(err)
			}
			hashes[implementation] = crypto.Keccak256Hash(code).Hex()
		}
	}
	committed := func(ctx context.Context) (l18.Checkpoint, error) {
		var h struct {
			Number hexutil.Uint64
			Hash   string
		}
		err := r.CallContext(ctx, &h, "eth_getBlockByNumber", "latest", false)
		return l18.Checkpoint{Number: uint64(h.Number), Hash: h.Hash}, err
	}
	creator, err := l18.NewCommittedCreator(ctx, r, time.Now, committed, l18.ManifestDigest, hashes)
	if err != nil {
		t.Fatal(err)
	}
	open := func(p *pgxpool.Pool) (*l18.JourneyService, *httptest.Server) {
		t.Helper()
		ledger, err := newRewardLedger(ctx, p, "pnl", "proof", nil, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		service, err := l18.NewJourneyService(ledger, creator, bytes.Repeat([]byte{1}, 32), time.Now, strings.Repeat("a", 64), strings.Repeat("b", 64))
		if err != nil {
			t.Fatal(err)
		}
		service, err = service.WithSourceStream("proof")
		if err != nil {
			t.Fatal(err)
		}
		router := chi.NewRouter()
		registerRewardRoutes(router, l18.HTTP{Service: service, Authenticate: func(req *http.Request) (l18.Actor, error) {
			proof, err := auth.VerifyEOA(req.Header.Get("X-Proof-Wallet"), req.Method+" "+req.URL.RequestURI(), req.Header.Get("X-Proof-Signature"), nil)
			return l18.Actor{Wallet: proof.Wallet}, err
		}})
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		return service, server
	}
	service, server := open(pool)
	var lastRewardsRequest string
	var firstClaim []byte
	var persistedPreparation string
	claimKey := uuid.NewString()
	claim := []byte(fmt.Sprintf(`{"rewardIds":["creator:1"],"recipient":%q}`, fixture.Alice))
	for attempt := 0; attempt < 2; attempt++ {
		prepared, err := creator.PrepareClaim(ctx, "1", fixture.Alice, fixture.Alice)
		if err != nil || prepared.ChainID != 125 || prepared.To != l18.FeesRouter || prepared.Caller != fixture.Alice || prepared.Value != "0" || prepared.Preview.ClaimableRaw != "10000000" {
			t.Fatalf("real prepared claim=%+v err=%v", prepared, err)
		}
		routerABI := proofLaunchpadABI(t, root, "periphery", "FeesRouter")
		calldata, err := routerABI.Pack("claimFees", big.NewInt(1))
		if err != nil || prepared.Data != hexutil.Encode(calldata) {
			t.Fatalf("claim calldata=%s err=%v", prepared.Data, err)
		}
		page, err := service.Rewards(ctx, l18.Actor{Wallet: fixture.Alice}, "creator_fees", "", 50)
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("real reward page=%+v err=%v", page, err)
		}
		reward := page.Items[0]
		if reward.RewardId != "creator:1" || reward.Wallet != fixture.Alice || reward.ClaimableRaw != "10000000" || reward.EarnedRaw != "10000000" || reward.ClaimedRaw != "0" || reward.SourcePosition == nil || reward.SourcePosition.TransactionHash != m.TransactionHash {
			t.Fatalf("consumer reward=%+v", reward)
		}

		status, headers, raw := proofLaunchpadRequest(t, ctx, r, server, fixture.Alice, http.MethodGet, "/v2/me/rewards?programId=creator_fees", uuid.NewString(), nil)
		if status != http.StatusOK || headers.Get("Cache-Control") != "private, no-store" || headers.Get("Content-Type") != "application/json" {
			t.Fatalf("rewards transport status=%d headers=%v body=%s", status, headers, raw)
		}
		observed, err := iface.DecodeRewardsResponse(raw)
		if err != nil || len(observed.Data.Items) != 1 || observed.RequestId == lastRewardsRequest || observed.Quality != "complete" || observed.Watermark == nil || observed.Watermark.BlockHash != page.Block.Hash {
			t.Fatalf("consumer rewards=%+v err=%v body=%s", observed, err, raw)
		}
		if got := observed.Data.Items[0]; got.RewardId != reward.RewardId || got.ClaimableRaw != reward.ClaimableRaw || got.ClaimedRaw != reward.ClaimedRaw || got.EarnedRaw != reward.EarnedRaw || got.Wallet != reward.Wallet || got.AssetAddress != l18.USDL || got.SourcePosition == nil || got.SourcePosition.TransactionHash != m.TransactionHash {
			t.Fatalf("HTTP consumer reward=%+v producer=%+v", got, reward)
		}
		lastRewardsRequest = observed.RequestId
		if _, err := iface.DecodeRewardsResponse(raw[:len(raw)/2]); err == nil {
			t.Fatal("consumer decoded truncated producer rewards")
		}
		status, headers, raw = proofLaunchpadRequest(t, ctx, r, server, fixture.Alice, http.MethodPost, "/v2/me/rewards:prepare-claim", claimKey, claim)
		if status != http.StatusOK || headers.Get("Cache-Control") != "private, no-store" || headers.Get("Content-Type") != "application/json" {
			t.Fatalf("claim transport status=%d headers=%v body=%s", status, headers, raw)
		}
		observedClaim, err := iface.DecodeRewardClaimResponse(raw)
		if err != nil {
			t.Fatalf("claim consumer decode: %v body=%s", err, raw)
		}
		transaction := observedClaim.Data.TransactionRequest
		if observedClaim.Quality != "complete" || observedClaim.Watermark == nil || observedClaim.Watermark.BlockHash != prepared.Preview.Block.Hash || transaction.From != prepared.Caller || transaction.To != prepared.To || transaction.Data != prepared.Data || transaction.ChainId != "125" || transaction.ValueRaw != "0" || transaction.GasLimitRaw == nil {
			t.Fatalf("HTTP consumer claim=%+v producer=%+v", observedClaim, prepared)
		}
		gas, ok := new(big.Int).SetString(string(*transaction.GasLimitRaw), 10)
		if !ok || gas.Sign() <= 0 {
			t.Fatalf("gas limit=%v", transaction.GasLimitRaw)
		}
		cost, ok := new(big.Int).SetString(string(observedClaim.Data.EstimatedGasPaxRaw), 10)
		if !ok || cost.Sign() <= 0 {
			t.Fatalf("gas cost=%s", observedClaim.Data.EstimatedGasPaxRaw)
		}
		if attempt == 0 {
			firstClaim = append([]byte(nil), raw...)
		} else if !bytes.Equal(firstClaim, raw) {
			t.Fatalf("persisted preparation changed after reopen: first=%s reopened=%s", firstClaim, raw)
		}
		var persisted string
		if err := pool.QueryRow(ctx, `SELECT jsonb_build_array(to_jsonb(r),to_jsonb(a),to_jsonb(o))::text
			FROM pnl.l18_requests r
			JOIN pnl.l18_audit a ON a.identity=r.actor||':'||r.request_key::text
			JOIN pnl.l18_outbox o ON o.identity='prepare:'||a.identity
			WHERE r.actor=$1 AND r.request_key=$2`, fixture.Alice, claimKey).Scan(&persisted); err != nil {
			t.Fatalf("durable request, audit and outbox identity: %v", err)
		}
		if attempt == 0 {
			persistedPreparation = persisted
		} else if persisted != persistedPreparation {
			t.Fatalf("stored preparation changed after reconnect and replay: before=%s after=%s", persistedPreparation, persisted)
		}
		if _, err := iface.DecodeRewardClaimResponse(raw[:len(raw)/2]); err == nil {
			t.Fatal("consumer decoded truncated producer preparation")
		}
		// Preserve valid JSON but damage a chain discriminator in producer bytes.
		var damaged map[string]any
		if err := json.Unmarshal(raw, &damaged); err != nil {
			t.Fatal(err)
		}
		damaged["data"].(map[string]any)["transactionRequest"].(map[string]any)["chainId"] = "1"
		malformed, err := json.Marshal(damaged)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := iface.DecodeRewardClaimResponse(malformed); err == nil {
			t.Fatal("consumer accepted a preparation for the wrong chain")
		}

		if attempt == 0 {
			server.Close()
			pool.Close()
			pool, err = pgxpool.New(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			service, server = open(pool)
		}
	}
	// A real NFT transfer revokes the old owner's preparation on the very next
	// HTTP request, proving the route cannot serve an earlier result from a cache.
	nftABI := proofLaunchpadABI(t, root, "core", "SidioraNFT")
	transfer, err := nftABI.Pack("transferFrom", common.HexToAddress(fixture.Alice), common.HexToAddress(fixture.Bob), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	var txHash string
	if err := r.CallContext(ctx, &txHash, "eth_sendTransaction", map[string]any{"from": fixture.Alice, "to": l18.NFT, "data": hexutil.Encode(transfer), "gas": "0x1e8480"}); err != nil {
		t.Fatal(err)
	}
	var receipt *struct{ Status hexutil.Uint64 }
	for deadline := time.Now().Add(5 * time.Second); ; {
		if err := r.CallContext(ctx, &receipt, "eth_getTransactionReceipt", txHash); err != nil {
			t.Fatalf("actual transfer receipt=%+v err=%v", receipt, err)
		}
		if receipt != nil {
			if receipt.Status != 1 {
				t.Fatalf("actual transfer receipt=%+v", receipt)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("actual transfer %s was not mined within five seconds", txHash)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := creator.PrepareClaim(ctx, "1", fixture.Alice, fixture.Alice); !errors.Is(err, l18.ErrUnauthorized) {
		t.Fatalf("old owner preparation err=%v", err)
	}
	for _, tc := range []struct {
		body   []byte
		status int
		code   string
	}{
		{claim, http.StatusForbidden, "FORBIDDEN"},
		{[]byte(`{"rewardIds":`), http.StatusBadRequest, "MALFORMED_JSON"},
		{[]byte(fmt.Sprintf(`{"rewardIds":["creator:999999"],"recipient":%q}`, fixture.Alice)), http.StatusNotFound, "NOT_FOUND"},
	} {
		key := uuid.NewString()
		if tc.code == "FORBIDDEN" {
			key = claimKey
		}
		status, _, raw := proofLaunchpadRequest(t, ctx, r, server, fixture.Alice, http.MethodPost, "/v2/me/rewards:prepare-claim", key, tc.body)
		envelope, err := iface.DecodeError(raw)
		if status != tc.status || err != nil || envelope.Code != tc.code {
			t.Fatalf("HTTP refusal status=%d body=%s err=%v", status, raw, err)
		}
	}
	status, _, raw := proofLaunchpadRequest(t, ctx, r, server, fixture.Alice, http.MethodPost, "/v2/me/rewards/claim", uuid.NewString(), claim)
	if status != http.StatusNotFound {
		t.Fatalf("retired route status=%d body=%s", status, raw)
	}
	bobClaim := []byte(fmt.Sprintf(`{"rewardIds":["creator:1"],"recipient":%q}`, fixture.Bob))
	status, _, raw = proofLaunchpadRequest(t, ctx, r, server, fixture.Bob, http.MethodPost, "/v2/me/rewards:prepare-claim", uuid.NewString(), bobClaim)
	bobPrepared, err := iface.DecodeRewardClaimResponse(raw)
	if status != http.StatusOK || err != nil || bobPrepared.Data.TransactionRequest.From != fixture.Bob || bobPrepared.Watermark == nil || bobPrepared.Watermark.BlockHash == head.Hash {
		t.Fatalf("new owner consumer status=%d claim=%+v err=%v body=%s", status, bobPrepared, err, raw)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pnl.l18_requests").Scan(&count); err != nil || count != 2 {
		t.Fatalf("preparation persistence: count=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pnl.l18_audit WHERE action='prepare_claim'").Scan(&count); err != nil || count != 2 {
		t.Fatalf("preparation audit persistence: count=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pnl.l18_outbox WHERE topic='reward.claim_prepared'").Scan(&count); err != nil || count != 2 {
		t.Fatalf("preparation outbox persistence: count=%d err=%v", count, err)
	}
}

type proofLaunchpadFixture struct{ Pool, Alice, Bob string }

func proofLaunchpadRewardRegistry(root string) (production.MigrationRegistry, error) {
	raw, err := os.ReadFile(filepath.Join(root, "migrations/registry.json"))
	if err != nil {
		return production.MigrationRegistry{}, err
	}
	approval, err := os.ReadFile(filepath.Join(root, "migrations/pnl-reward-approval.json"))
	if err != nil {
		return production.MigrationRegistry{}, err
	}
	reader, err := os.ReadFile(filepath.Join(root, "sdk/launchpad/shared/interfaces/v2/validators.gen.go"))
	if err != nil {
		return production.MigrationRegistry{}, err
	}
	return migrate.LoadRewardRegistry(raw, approval, reader)
}

// Bootstrap the legacy deployment and predecessor history before the reward
// coordinator runs, as the shared coordinator fixture does for legacy services.
func proofLaunchpadBootstrapRewardCoordinator(t *testing.T, ctx context.Context, pool *pgxpool.Pool, root string) {
	t.Helper()
	registry, err := proofLaunchpadRewardRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", production.CoordinatorKey).Scan(&locked); err != nil || !locked {
		t.Fatalf("bootstrap coordinator lock: %v", err)
	}
	defer func() {
		if err := conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", production.CoordinatorKey).Scan(&locked); err != nil || !locked {
			t.Errorf("bootstrap coordinator unlock: %v", err)
		}
	}()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(production.MigrationControlDDL())
	history, err := production.HistoryDDL("core/pnl-tracker", "pnl")
	if err != nil {
		t.Fatal(err)
	}
	// HistoryDDL uses SET LOCAL ROLE and therefore belongs to one transaction.
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, production.OnlineIndexControlDDL()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, history); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	exec("CREATE ROLE pnl_owner NOLOGIN; GRANT USAGE, CREATE ON SCHEMA pnl TO pnl_owner")
	for _, id := range []int64{80003, 160000, 180000} {
		m := catalog[id]
		sql, err := os.ReadFile(filepath.Join(root, "launchpad", m.Path))
		if err != nil {
			t.Fatal(err)
		}
		if production.Checksum(sql) != m.SHA256 {
			t.Fatalf("bootstrap predecessor %d source drift", id)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='2s'"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{"pnl_owner"}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "RESET ROLE"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO migration_control.pnl_tracker_versions(version_id,is_applied) VALUES($1,true)", id); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO migration_control.applied_migrations(migration_id,service,schema_name,checksum,registry_digest) VALUES($1,$2,$3,$4,$5)", id, m.Service, m.Schema, m.SHA256, registry.ApprovalDigest); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func proofLaunchpadApplyRegisteredRewardMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, root string) {
	t.Helper()
	registry, err := proofLaunchpadRewardRegistry(root)
	if err != nil {
		t.Fatalf("production reward migration registry: %v", err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	migration, ok := catalog[180001]
	if !ok || migration.Service != "core/pnl-tracker" || migration.Schema != "pnl" {
		t.Fatal("production registry does not register reward migration 180001 for core/pnl-tracker")
	}
	sql, err := os.ReadFile(filepath.Join(root, "launchpad", migration.Path))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if err := production.ApplyCoordinatedMigration(ctx, conn.Conn(), migration, sql, catalog, migration.InterfaceDigest, registry.ApprovalDigest, migration.ReaderMinVersion); err != nil {
		t.Fatalf("production reward migration 180001: %v", err)
	}
}

func proofLaunchpadPostgres(t *testing.T, ctx context.Context) (string, *pgxpool.Pool) {
	t.Helper()
	// Discover installed executables, never an operator-supplied service URL.
	bindir, err := exec.CommandContext(ctx, "pg_config", "--bindir").Output()
	if err != nil {
		t.Fatalf("services/launchpad/pnl-tracker proof requires local PostgreSQL binaries: %v", err)
	}
	dir, err := os.MkdirTemp("", "proof-launchpad-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("owned PostgreSQL requires an unprivileged postgres account: %v", err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	command := func(name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(strings.TrimSpace(string(bindir)), name), args...)
		cmd.Dir = dir
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		return cmd
	}
	data := filepath.Join(dir, "data")
	if output, err := command("initdb", "-D", data, "--auth=trust", "--username=proof", "--no-locale", "--encoding=UTF8").CombinedOutput(); err != nil {
		t.Fatalf("owned PostgreSQL initdb: %v: %s", err, output)
	}
	log, err := os.Create(filepath.Join(dir, "postgres.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	// A private Unix socket directory avoids both an external server and port races.
	postgres := command("postgres", "-D", data, "-h", "", "-k", dir)
	postgres.Stdout, postgres.Stderr = log, log
	if err := postgres.Start(); err != nil {
		t.Fatalf("owned PostgreSQL start: %v", err)
	}
	t.Cleanup(func() { _ = postgres.Process.Kill(); _ = postgres.Wait() })
	endpoint := url.URL{Scheme: "postgres", User: url.User("proof"), Path: "/postgres"}
	query := endpoint.Query()
	query.Set("host", dir)
	query.Set("sslmode", "disable")
	endpoint.RawQuery = query.Encode()
	dsn := endpoint.String()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ready, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		if err := pool.Ping(ready); err == nil {
			break
		}
		select {
		case <-ready.Done():
			output, _ := os.ReadFile(log.Name())
			t.Fatalf("owned PostgreSQL did not become ready: %v: %s", ready.Err(), output)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := migrate.Up(ctx, dsn); err != nil {
		t.Fatalf("owned PostgreSQL real pnl migrations: %v", err)
	}
	return dsn, pool
}

func proofLaunchpadChain(t *testing.T, ctx context.Context, root string) (*rpc.Client, proofLaunchpadFixture) {
	t.Helper()
	r, fixture := internaltest.NewAnvilFixture(t, ctx, root)
	return r, proofLaunchpadFixture{Pool: fixture.Pool, Alice: fixture.Alice, Bob: fixture.Bob}
}

func proofLaunchpadABI(t *testing.T, root, group, name string) abi.ABI {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "contracts/launchpad/artifacts/contracts", group, name+".sol", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	a, err := abi.JSON(bytes.NewReader(artifact.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func proofLaunchpadRequest(t *testing.T, ctx context.Context, r *rpc.Client, server *httptest.Server, wallet, method, path, key string, body []byte) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var signature string
	if err := r.CallContext(ctx, &signature, "personal_sign", hexutil.Encode([]byte(method+" "+path)), wallet); err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Proof-Wallet", wallet)
	req.Header.Set("X-Proof-Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.ContentLength >= 0 && response.ContentLength != int64(len(raw)) {
		t.Fatal("incomplete HTTP response: " + strconv.Itoa(len(raw)))
	}
	return response.StatusCode, response.Header, raw
}
