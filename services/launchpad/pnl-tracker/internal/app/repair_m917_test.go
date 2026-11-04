package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepairM_917_FreshInstallMigrationContract(t *testing.T) {
	repairM917ValidateMigrationPlan(t, nil)
}
func TestRepairM_917_ExistingSchemaUpgradeMigrationContract(t *testing.T) {
	repairM917ValidateMigrationPlan(t, []int64{80003, 160000, 180000})
}

func repairM917Read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repair29Root(), path))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func repairM917ValidateMigrationPlan(t *testing.T, existing []int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := repair29Root()
	registry, err := proofLaunchpadRewardRegistry(root)
	if err != nil {
		t.Fatalf("reward-proof-schema: production registry admission: %v", err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, id := range []int64{80003, 160000, 180000, 180001} {
		m, ok := catalog[id]
		if !ok || m.Service != "core/pnl-tracker" || m.Schema != "pnl" {
			t.Fatalf("reward prerequisite %d is unregistered", id)
		}
		sql := repairM917Read(t, filepath.Join("launchpad", m.Path))
		if production.Checksum(sql) != m.SHA256 {
			t.Fatalf("migration %d source differs from registration", id)
		}
		sources[string(sql)] = fmt.Sprintf("%d", id)
	}
	chain := repairM917RPC(t)
	file := filepath.Join(t.TempDir(), "database.json")
	database := repairM917Database(t, file, sources)
	database.State.Legacy = map[string]string{"creator": chain.alice, "pool": chain.pool, "legacy_position": "holdings=7;remaining_cost=13;revision=9"}
	coordinator := database.pool(t, "migration_coordinator")
	// The production bootstrap provisions predecessor tables and history from their
	// registered bytes. No reward request/audit DDL is supplied by this test.
	proofLaunchpadBootstrapRewardCoordinator(t, ctx, coordinator, root)
	beforeUpgrade := database.snapshot()
	if len(beforeUpgrade.Applied) != 3 || beforeUpgrade.Tables["l18_requests"] != nil || beforeUpgrade.Tables["l18_audit"] != nil {
		t.Fatal("bootstrap already supplied reward request storage")
	}
	for _, id := range []int64{80003, 160000, 180000} {
		if beforeUpgrade.Applied[id] != catalog[id].SHA256 {
			t.Fatalf("bootstrap omitted predecessor %d", id)
		}
	}
	if len(existing) > 0 {
		for _, id := range existing {
			if beforeUpgrade.Applied[id] != catalog[id].SHA256 {
				t.Fatalf("existing schema predecessor %d checksum differs", id)
			}
		}
		coordinator.Close()
		database.stop()
		database = repairM917Database(t, file, sources)
		coordinator = database.pool(t, "migration_coordinator")
		if !reflect.DeepEqual(beforeUpgrade, database.snapshot()) {
			t.Fatal("existing schema/history changed on reopen before upgrade")
		}
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	runtimePool := database.pool(t, "pnl_runtime")
	if ledger, err := newRewardLedger(ctx, runtimePool, "pnl", "pnld", nil, guard.Assert); err == nil || ledger != nil || !strings.Contains(err.Error(), "migration 180001 required") {
		t.Fatalf("unmigrated producer admission: ledger=%v err=%v", ledger, err)
	}
	if !reflect.DeepEqual(beforeUpgrade, database.snapshot()) {
		t.Fatal("unmigrated readiness changed the schema or durable rows")
	}

	// The actual coordinator, not just Manifest.Validate, must refuse changed
	// bytes, incompatible readers, absent approvals and missing prerequisites.
	target := catalog[180001]
	sql := repairM917Read(t, filepath.Join("launchpad", target.Path))
	conn, err := coordinator.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for _, bad := range []struct {
		name     string
		sql      []byte
		approval string
		reader   int64
	}{
		{"source drift", append(bytes.Clone(sql), []byte("\n-- drift")...), registry.ApprovalDigest, target.ReaderMinVersion},
		{"unapproved release", sql, strings.Repeat("0", 64), target.ReaderMinVersion},
		{"incompatible reader", sql, registry.ApprovalDigest, 0},
	} {
		if err := production.ApplyCoordinatedMigration(ctx, conn.Conn(), target, bad.sql, catalog, target.InterfaceDigest, bad.approval, bad.reader); err == nil {
			t.Fatalf("coordinator admitted %s", bad.name)
		}
	}
	missing := map[int64]production.MigrationManifest{}
	for id, m := range catalog {
		missing[id] = m
	}
	delete(missing, 180000)
	if err := production.ApplyCoordinatedMigration(ctx, conn.Conn(), target, sql, missing, target.InterfaceDigest, registry.ApprovalDigest, 1); err == nil {
		t.Fatal("coordinator admitted an unregistered predecessor")
	}
	conn.Release()
	if !reflect.DeepEqual(beforeUpgrade, database.snapshot()) {
		t.Fatal("refused migration changed database")
	}
	proofLaunchpadApplyRegisteredRewardMigration(t, ctx, coordinator, root)
	installed := database.snapshot()
	if installed.Applied[180001] != target.SHA256 || len(installed.Applied) != 4 || len(installed.Attempts) != 1 || installed.Attempts["1"] != "applied" || len(installed.History) != 4 || installed.History[3] != 180001 || len(installed.DDL) != 4 || installed.DDL[3] != "180001" {
		t.Fatalf("coordinator did not durably apply registered reward storage: %+v", installed)
	}
	if !reflect.DeepEqual(beforeUpgrade.Legacy, installed.Legacy) {
		t.Fatal("reward upgrade rewrote existing business rows")
	}
	var roles struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err := json.Unmarshal(repairM917Read(t, "launchpad/spec/production-system/integration/migrations/grants.json"), &roles); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, role := range roles.Roles {
		if role.RuntimeRole == "pnl_runtime" {
			repairM917Grant(t, database, role)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("runtime manifest absent")
	}
	open := func(pool *pgxpool.Pool) (*httptest.Server, *rpc.Client) {
		t.Helper()
		ledger, err := newRewardLedger(ctx, pool, "pnl", "pnld", nil, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		client, err := rpc.DialHTTP(chain.server.URL)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(client.Close)
		hashes := map[string]string{}
		for _, address := range []string{l18.NFT, l18.Accumulator, l18.FeesRouter, l18.USDL} {
			hashes[address] = crypto.Keccak256Hash([]byte{0x60, 0, 0x60, 0}).Hex()
		}
		creator, err := l18.NewCommittedCreator(ctx, client, time.Now, func(context.Context) (l18.Checkpoint, error) {
			return l18.Checkpoint{Number: 15, Hash: fmt.Sprintf("0x%064x", 15)}, nil
		}, l18.ManifestDigest, hashes)
		if err != nil {
			t.Fatal(err)
		}
		journey, err := l18.NewJourneyService(ledger, creator, bytes.Repeat([]byte{1}, 32), time.Now, registry.ApprovalDigest, target.InterfaceDigest)
		if err != nil {
			t.Fatal(err)
		}
		router := chi.NewRouter()
		registerRewardRoutes(router, l18.HTTP{Service: journey, Authenticate: func(req *http.Request) (l18.Actor, error) {
			proof, err := auth.VerifyEOA(req.Header.Get("X-Proof-Wallet"), req.Method+" "+req.URL.RequestURI(), req.Header.Get("X-Proof-Signature"), nil)
			return l18.Actor{Wallet: proof.Wallet}, err
		}})
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		return server, client
	}
	server, client := open(runtimePool)
	key := uuid.NewString()
	body := fmt.Sprintf(`{"rewardIds":["creator:1"],"recipient":%q}`, chain.alice)
	send := func(server *httptest.Server, body string, want int) []byte {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/v2/me/rewards:prepare-claim", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		signingKey, err := crypto.HexToECDSA(strings.Repeat("0", 63) + "1")
		if err != nil {
			t.Fatal(err)
		}
		repair29Sign(t, req, signingKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != want || res.Header.Get("Cache-Control") != "private, no-store" || res.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("claim transport status=%d want=%d headers=%v err=%v body=%s", res.StatusCode, want, res.Header, err, raw)
		}
		return raw
	}
	first := send(server, body, http.StatusOK)
	decoded, err := iface.DecodeRewardClaimResponse(first)
	if err != nil {
		t.Fatalf("real consumer rejected prepared claim: %v: %s", err, first)
	}
	tx := decoded.Data.TransactionRequest
	calldata, err := chain.abis[l18.FeesRouter].Pack("claimFees", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	if tx.ChainId != "125" || tx.From != chain.alice || tx.To != l18.FeesRouter || tx.ValueRaw != "0" || tx.Data != hexutil.Encode(calldata) || tx.GasLimitRaw == nil || *tx.GasLimitRaw != "100000" || decoded.Data.EstimatedGasPaxRaw != "100000000000000" || decoded.Quality != "complete" || decoded.Watermark == nil || decoded.Watermark.BlockHash != fmt.Sprintf("0x%064x", 15) {
		t.Fatalf("decoded claim differs from producer interfaces: %+v", decoded)
	}
	if _, err := iface.DecodeRewardClaimResponse(first[:len(first)/2]); err == nil {
		t.Fatal("consumer accepted truncated producer response")
	}
	var damaged map[string]any
	if err := json.Unmarshal(first, &damaged); err != nil {
		t.Fatal(err)
	}
	delete(damaged["data"].(map[string]any)["transactionRequest"].(map[string]any), "to")
	raw, err := json.Marshal(damaged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := iface.DecodeRewardClaimResponse(raw); err == nil {
		t.Fatal("consumer accepted missing transaction target")
	}
	beforeReplay := database.snapshot()
	request := beforeReplay.Tables["l18_requests"]
	audit := beforeReplay.Tables["l18_audit"]
	outbox := beforeReplay.Tables["l18_outbox"]
	if len(request.Rows) != 1 || len(audit.Rows) != 1 || len(outbox.Rows) != 1 {
		t.Fatalf("preparation must commit exactly one request/audit/outbox row: %d/%d/%d", len(request.Rows), len(audit.Rows), len(outbox.Rows))
	}
	identity := chain.alice + ":" + key
	r, a, o := request.Rows[0], audit.Rows[0], outbox.Rows[0]
	stored, err := hex.DecodeString(strings.TrimPrefix(r[5], `\x`))
	if err != nil {
		t.Fatal(err)
	}
	if r[0] != chain.alice || r[1] != key || r[3] != "POST" || r[4] != "/v2/me/rewards:prepare-claim" || !bytes.Equal(stored, first) || r[7] != registry.ApprovalDigest || r[8] != target.InterfaceDigest || a[0] != identity || a[1] != chain.alice || a[2] != "prepare_claim" || !repairM917EqualJSON([]byte(a[4]), first) || o[0] != "prepare:"+identity || o[1] != "reward.claim_prepared" {
		t.Fatalf("durable request/audit/outbox binding differs: request=%v audit=%v outbox=%v", r, a, o)
	}
	var event struct{ Actor, Key, Digest, Release, Interface string }
	if err := json.Unmarshal([]byte(o[2]), &event); err != nil {
		t.Fatal(err)
	}
	if event.Actor != chain.alice || event.Key != key || event.Digest != r[2] || event.Release != r[7] || event.Interface != r[8] {
		t.Fatalf("outbox event lost request/deployment identity: %+v", event)
	}
	server.Close()
	client.Close()
	runtimePool.Close()
	coordinator.Close()
	database.stop()
	database = repairM917Database(t, file, sources)
	if !reflect.DeepEqual(beforeReplay, database.snapshot()) {
		t.Fatal("durable rows did not recover from the closed store")
	}
	runtimePool = database.pool(t, "pnl_runtime")
	server, client = open(runtimePool)
	replay := send(server, body, http.StatusOK)
	if !bytes.Equal(first, replay) {
		t.Fatalf("reopened claim bytes changed: first=%s replay=%s", first, replay)
	}
	if _, err := iface.DecodeRewardClaimResponse(replay); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeReplay, database.snapshot()) {
		t.Fatal("replay rewrote or duplicated durable identities")
	}
	send(server, strings.Replace(body, "creator:1", "creator:2", 1), http.StatusConflict)
	if !reflect.DeepEqual(beforeReplay, database.snapshot()) {
		t.Fatal("conflicting request changed durable state")
	}
	chain.mu.Lock()
	chain.owner = chain.bob
	chain.mu.Unlock()
	send(server, body, http.StatusForbidden)
	if !reflect.DeepEqual(beforeReplay, database.snapshot()) {
		t.Fatal("revoked ownership replay changed durable state")
	}
}

func TestRepairM_917_ApprovalBindings(t *testing.T) {
	source := repairM917Read(t, "launchpad/spec/production-system/integration/migrations/registry.json")
	approval := repairM917Read(t, "launchpad/spec/production-system/integration/migrations/pnl-reward-approval.json")
	reader := repairM917Read(t, "sdk/launchpad/shared/interfaces/v2/validators.gen.go")
	for _, bad := range []struct {
		name                     string
		source, approval, reader []byte
	}{
		{"changed approval", source, append(bytes.Clone(approval), byte(' ')), reader},
		{"changed reader", source, approval, append(bytes.Clone(reader), byte(' '))},
		{"unapproved registry", bytes.Replace(source, []byte(`"approval_digest": "`), []byte(`"approval_digest": "0`), 1), approval, reader},
	} {
		if _, err := migrate.LoadRewardRegistry(bad.source, bad.approval, bad.reader); err == nil {
			t.Fatalf("loader accepted %s", bad.name)
		}
	}
}

type repairM917Chain struct {
	mu                      sync.Mutex
	alice, bob, pool, owner string
	abis                    map[string]abi.ABI
	server                  *httptest.Server
}

func repairM917RPC(t *testing.T) *repairM917Chain {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.Repeat("0", 63) + "1")
	if err != nil {
		t.Fatal(err)
	}
	c := &repairM917Chain{alice: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), bob: "0x000000000000000000000000000000000000000c", pool: "0x0000000000000000000000000000000000000065", abis: map[string]abi.ABI{}}
	c.owner = c.alice
	for address, name := range map[string]string{l18.NFT: "SidioraNFT", l18.Accumulator: "FeeAccumulator", l18.FeesRouter: "FeesRouter"} {
		a, err := abi.JSON(bytes.NewReader(repairM917Read(t, "services/launchpad/pnl-tracker/internal/production/l18/"+name+".json")))
		if err != nil {
			t.Fatal(err)
		}
		c.abis[address] = a
	}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		result, err := c.result(request.Method, request.Params)
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if err != nil {
			response["error"] = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			response["result"] = result
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(c.server.Close)
	return c
}
func (c *repairM917Chain) result(method string, args []json.RawMessage) (any, error) {
	switch method {
	case "eth_chainId":
		return "0x7d", nil
	case "eth_getBlockByNumber":
		var number string
		if len(args) != 2 || json.Unmarshal(args[0], &number) != nil || number != "0xf" {
			return nil, fmt.Errorf("unrecorded checkpoint")
		}
		return map[string]string{"number": "0xf", "hash": fmt.Sprintf("0x%064x", 15)}, nil
	case "eth_gasPrice":
		return "0x3b9aca00", nil
	}
	selectorIndex := 1
	if method == "eth_getStorageAt" {
		selectorIndex = 2
	}
	var selector struct {
		Hash      string `json:"blockHash"`
		Canonical bool   `json:"requireCanonical"`
	}
	if len(args) <= selectorIndex || json.Unmarshal(args[selectorIndex], &selector) != nil || selector.Hash != fmt.Sprintf("0x%064x", 15) || !selector.Canonical {
		return nil, fmt.Errorf("unpinned RPC read %s", method)
	}
	switch method {
	case "eth_getCode":
		return "0x60006000", nil
	case "eth_getStorageAt":
		return "0x" + strings.Repeat("0", 64), nil
	case "eth_estimateGas":
		var call struct{ From, To, Data string }
		if json.Unmarshal(args[0], &call) != nil {
			return nil, fmt.Errorf("invalid estimate call")
		}
		data, err := c.abis[l18.FeesRouter].Pack("claimFees", big.NewInt(1))
		if err != nil {
			return nil, err
		}
		if call.From != c.alice || call.To != l18.FeesRouter || call.Data != hexutil.Encode(data) {
			return nil, fmt.Errorf("estimate differs from committed claim interface")
		}
		return "0x186a0", nil
	case "eth_call":
		var call struct{ To, Data string }
		if err := json.Unmarshal(args[0], &call); err != nil {
			return nil, err
		}
		a, ok := c.abis[strings.ToLower(call.To)]
		if !ok {
			return nil, fmt.Errorf("unrecorded call target")
		}
		data, err := hexutil.Decode(call.Data)
		if err != nil || len(data) < 4 {
			return nil, fmt.Errorf("malformed ABI call")
		}
		m, err := a.MethodById(data[:4])
		if err != nil {
			return nil, err
		}
		input, err := m.Inputs.Unpack(data[4:])
		if err != nil || len(input) != 1 {
			return nil, fmt.Errorf("malformed ABI input")
		}
		var value any
		switch m.Name {
		case "ownerOf", "getPoolAddress", "getFeeStrategy":
			id, ok := input[0].(*big.Int)
			if !ok || id.Cmp(big.NewInt(1)) != 0 {
				return nil, fmt.Errorf("unrecorded NFT")
			}
			switch m.Name {
			case "ownerOf":
				value = common.HexToAddress(c.owner)
			case "getPoolAddress":
				value = common.HexToAddress(c.pool)
			case "getFeeStrategy":
				value = uint8(0)
			}
		case "supportsAirdrop", "getAccumulatedFees":
			pool, ok := input[0].(common.Address)
			if !ok || pool != common.HexToAddress(c.pool) {
				return nil, fmt.Errorf("unrecorded pool")
			}
			if m.Name == "supportsAirdrop" {
				value = false
			} else {
				value = big.NewInt(10000000)
			}
		default:
			return nil, fmt.Errorf("unrecorded ABI method %s", m.Name)
		}
		raw, err := m.Outputs.Pack(value)
		return hexutil.Encode(raw), err
	}
	return nil, fmt.Errorf("unrecorded RPC method %s", method)
}
