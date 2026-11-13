package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	vmruntime "github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func repair29Root() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
}

// PGlite executes the real deployment SQL and PostgreSQL wire protocol in WASM;
// no PostgreSQL installation, shared database or service credentials are used.
func repair29Store(t *testing.T, ctx context.Context, directory string) (string, func()) {
	t.Helper()
	script := filepath.Join(repair29Root(), "services/launchpad/pnl-tracker/internal/app/testdata/repair29/store.mjs")
	cmd := exec.CommandContext(ctx, "node", script, directory)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("embedded SQL shutdown: %v: %s", err, stderr.String())
		}
	}
	t.Cleanup(stop)
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		err := cmd.Wait()
		stopped = true
		t.Fatalf("embedded SQL startup: process=%v scanner=%v: %s", err, scanner.Err(), stderr.String())
	}
	var address string
	if err := json.Unmarshal(scanner.Bytes(), &address); err != nil {
		t.Fatalf("embedded SQL address %q: %v", scanner.Text(), err)
	}
	return "postgres://postgres:postgres@" + address + "/postgres?sslmode=disable", stop
}

func repair29Pool(t *testing.T, ctx context.Context, dsn string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}

func repair29SQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, query, args...); err != nil {
		t.Fatalf("SQL %s: %v", query, err)
	}
}

func applyRewardDeploymentMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, root, name string) {
	t.Helper()
	applyRepair29Migration(t, ctx, pool, root, name, "pnl")
}

func applyRepair29Migration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, root, name, schema string) {
	t.Helper()
	sql, err := os.ReadFile(filepath.Join(root, "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	quoted := pgx.Identifier{schema}.Sanitize()
	rendered := strings.ReplaceAll(strings.ReplaceAll(string(sql), `"pnl"`, quoted), "pnl.", quoted+".")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, rendered); err != nil {
		t.Fatalf("deployment migration %s: %v", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestProductionRewardSchemaReadiness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dsn, stop := repair29Store(t, ctx, filepath.Join(t.TempDir(), "readiness"))
	defer stop()
	pool := repair29Pool(t, ctx, dsn)
	defer pool.Close()
	schema := "repair29_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	q := pgx.Identifier{schema}.Sanitize()
	sql := func(query string) { t.Helper(); repair29SQL(t, ctx, pool, query) }
	sql("CREATE SCHEMA " + q)
	applyRepair29Migration(t, ctx, pool, repair29Root(), "180000.sql", schema)
	guard, err := queue.NewEpochGuard("recovery", 1, "local")
	if err != nil {
		t.Fatal(err)
	}
	if ledger, err := newRewardLedger(ctx, pool, schema, "pnld", nil, guard.Assert); err == nil || ledger != nil {
		t.Fatalf("unmigrated reward ledger admitted: ledger=%v err=%v", ledger, err)
	} else if !strings.Contains(err.Error(), "reward migration 180001 required (l18_requests)") {
		t.Fatalf("unmigrated reward ledger failed for an unexpected reason: %v", err)
	}
	var requestTable, auditTable *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass($1)::text,to_regclass($2)::text", q+".l18_requests", q+".l18_audit").Scan(&requestTable, &auditTable); err != nil {
		t.Fatal(err)
	}
	if requestTable != nil || auditTable != nil {
		t.Fatal("runtime admission created reward tables instead of requiring deployed migrations")
	}
	applyRepair29Migration(t, ctx, pool, repair29Root(), "180001.sql", schema)
	if ledger, err := newRewardLedger(ctx, pool, schema, "pnld", nil, guard.Assert); err != nil || ledger == nil {
		t.Fatalf("migrated reward ledger rejected: ledger=%v err=%v", ledger, err)
	}
	t.Run("incompatible_response", func(t *testing.T) {
		sql("ALTER TABLE " + q + ".l18_requests ALTER COLUMN response TYPE text USING response::text")
		defer sql("ALTER TABLE " + q + ".l18_requests ALTER COLUMN response TYPE bytea USING response::bytea")
		if err := checkRewardRequestSchema(ctx, pool, schema); err == nil {
			t.Fatal("readiness admitted incompatible response storage")
		}
	})
	for _, table := range []string{"l18_requests", "l18_audit"} {
		t.Run("missing_"+table, func(t *testing.T) {
			sql("ALTER TABLE " + q + "." + table + " RENAME TO repair_hidden")
			defer sql("ALTER TABLE " + q + ".repair_hidden RENAME TO " + table)
			if err := checkRewardRequestSchema(ctx, pool, schema); err == nil {
				t.Fatalf("readiness admitted missing %s", table)
			}
		})
		t.Run("identity_"+table, func(t *testing.T) {
			sql("ALTER TABLE " + q + "." + table + " DROP CONSTRAINT " + table + "_pkey")
			defer func() {
				key := "identity"
				if table == "l18_requests" {
					key = "actor,request_key"
				}
				sql("ALTER TABLE " + q + "." + table + " ADD PRIMARY KEY (" + key + ")")
			}()
			if err := checkRewardRequestSchema(ctx, pool, schema); err == nil {
				t.Fatalf("readiness admitted missing %s identity constraint", table)
			}
		})
	}
	role := pgx.Identifier{schema + "_role"}.Sanitize()
	sql("CREATE ROLE " + role + " NOLOGIN")
	sql("GRANT USAGE ON SCHEMA " + q + " TO " + role)
	sql("GRANT SELECT ON " + q + ".l18_requests," + q + ".l18_audit TO " + role)
	sql("SET ROLE " + role)
	if err := checkRewardRequestSchema(ctx, pool, schema); err == nil {
		t.Fatal("readiness admitted SELECT-only reward storage")
	}
	sql("RESET ROLE")
	sql("GRANT INSERT ON " + q + ".l18_requests TO " + role)
	sql("SET ROLE " + role)
	if err := checkRewardRequestSchema(ctx, pool, schema); err == nil {
		t.Fatal("readiness admitted unwritable reward audit")
	}
	sql("RESET ROLE")
	sql("GRANT INSERT ON " + q + ".l18_audit TO " + role)
	sql("SET ROLE " + role)
	if err := checkRewardRequestSchema(ctx, pool, schema); err != nil {
		t.Fatalf("deployed runtime privileges rejected: %v", err)
	}
	sql("RESET ROLE")
}

type repair29Artifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode string          `json:"bytecode"`
}

// Execute source-compiled constructors at their genesis addresses, including
// proxy initialization. Storage comes from EVM execution, never fabricated RPCs.
func repair29Genesis(t *testing.T, artifacts map[string]repair29Artifact, owner common.Address) (types.GenesisAlloc, map[string]common.Address, map[string]abi.ABI) {
	t.Helper()
	chain := *params.AllDevChainProtocolChanges
	chain.ChainID = big.NewInt(125)
	db, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatal(err)
	}
	slots := map[common.Address]map[common.Hash]bool{}
	cfg := &vmruntime.Config{State: db, Origin: owner, ChainConfig: &chain, GasLimit: 30_000_000, EVMConfig: vm.Config{Tracer: &tracing.Hooks{
		OnOpcode: func(_ uint64, op byte, _, _ uint64, scope tracing.OpContext, _ []byte, _ int, _ error) {
			if vm.OpCode(op) == vm.SSTORE {
				addr := scope.Address()
				if slots[addr] == nil {
					slots[addr] = map[common.Hash]bool{}
				}
				stack := scope.StackData()
				slots[addr][common.Hash(stack[len(stack)-1].Bytes32())] = true
			}
		},
	}}}
	abis := map[string]abi.ABI{}
	for name, artifact := range artifacts {
		parsed, err := abi.JSON(bytes.NewReader(artifact.ABI))
		if err != nil {
			t.Fatal(err)
		}
		abis[name] = parsed
	}
	deployed := map[common.Address]bool{}
	construct := func(name string, address common.Address, args ...any) {
		t.Helper()
		argsRaw, err := abis[name].Pack("", args...)
		if err != nil {
			t.Fatal(err)
		}
		code := append(common.FromHex(artifacts[name].Bytecode), argsRaw...)
		db.CreateAccount(address)
		db.SetCode(address, code, tracing.CodeChangeUnspecified)
		runtimeCode, _, err := vmruntime.Call(address, nil, cfg)
		if err != nil || len(runtimeCode) == 0 {
			t.Fatalf("%s constructor: %v result=%x", name, err, runtimeCode)
		}
		db.SetCode(address, runtimeCode, tracing.CodeChangeUnspecified)
		deployed[address] = true
	}
	addresses := map[string]common.Address{
		"USDL": common.HexToAddress(l18.USDL), "SidioraNFT": common.HexToAddress(l18.NFT),
		"FeeAccumulator": common.HexToAddress(l18.Accumulator), "FeesRouter": common.HexToAddress(l18.FeesRouter),
		"ProtocolConfig": common.HexToAddress("0x2001"), "PoolRegistry": common.HexToAddress("0x2002"),
		"SidioraPool": common.HexToAddress("0x2003"), "Token": common.HexToAddress("0x2004"),
	}
	construct("SidioraERC20", addresses["USDL"], "USDL", "USDL", big.NewInt(1_000_000_000_000), owner)
	construct("SidioraERC20", addresses["Token"], "Reward proof token", "RPT", big.NewInt(1_000_000_000_000_000_000), owner)
	zero := common.Address{}
	proxy := func(name string, number int64, args ...any) {
		t.Helper()
		implementation := common.BigToAddress(big.NewInt(number))
		construct(name, implementation)
		initializer, err := abis[name].Pack("initialize", args...)
		if err != nil {
			t.Fatal(err)
		}
		construct("UUPSProxy", addresses[name], implementation, initializer)
	}
	proxy("ProtocolConfig", 0x3001, addresses["USDL"], zero, owner)
	proxy("PoolRegistry", 0x3002, zero, owner)
	proxy("FeeAccumulator", 0x3003, addresses["ProtocolConfig"], owner, addresses["PoolRegistry"], zero, addresses["USDL"], owner)
	proxy("SidioraNFT", 0x3004, "Fee rights", "FEE", zero, owner)
	proxy("FeesRouter", 0x3005, addresses["SidioraNFT"], addresses["FeeAccumulator"], addresses["PoolRegistry"], owner)
	proxy("SidioraPool", 0x3006, addresses["Token"], addresses["USDL"], zero, addresses["FeeAccumulator"], zero, addresses["ProtocolConfig"], owner, big.NewInt(1_000_000_000_000), big.NewInt(999_999_999_999_999_997))
	alloc := types.GenesisAlloc{owner: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)}}
	for address := range deployed {
		storage := map[common.Hash]common.Hash{}
		for slot := range slots[address] {
			storage[slot] = db.GetState(address, slot)
		}
		alloc[address] = types.Account{Code: db.GetCode(address), Storage: storage, Balance: new(big.Int)}
	}
	return alloc, addresses, abis
}

func TestRepair_29_RewardRequestSchemaRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := repair29Root()
	compile := exec.CommandContext(ctx, "node", filepath.Join(root, "services/launchpad/pnl-tracker/internal/app/testdata/repair29/contracts.mjs"), root)
	var compileErr bytes.Buffer
	compile.Stderr = &compileErr
	raw, err := compile.Output()
	if err != nil {
		t.Fatalf("production contract compilation: %v: %s", err, compileErr.String())
	}
	artifacts := map[string]repair29Artifact{}
	if err := json.Unmarshal(raw, &artifacts); err != nil {
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	owner := crypto.PubkeyToAddress(key.PublicKey)
	wallet := strings.ToLower(owner.Hex())
	alloc, addresses, abis := repair29Genesis(t, artifacts, owner)
	ipc := filepath.Join(t.TempDir(), "chain.ipc")
	sim := simulated.NewBackend(alloc, func(nc *node.Config, ec *ethconfig.Config) {
		chain := *ec.Genesis.Config
		chain.ChainID = big.NewInt(125)
		ec.Genesis.Config = &chain
		nc.IPCPath = ipc
	})
	defer sim.Close()
	// Attach to the backend's real JSON-RPC server through its owned IPC socket.
	// The node configuration exposes that socket without a live RPC dependency.
	r, err := rpc.DialIPC(ctx, ipc)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	signer, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	signer.Context = ctx
	transact := func(name, method string, args ...any) *types.Receipt {
		t.Helper()
		address := addresses[name]
		contractABI := abis[name]
		if name == "USDL" || name == "Token" {
			contractABI = abis["SidioraERC20"]
		}
		data, err := contractABI.Pack(method, args...)
		if err != nil {
			t.Fatal(err)
		}
		// ERC20 checkpoints append a new storage slot in the next block. Estimate
		// against that pending block so latest-block checkpoint updates cannot
		// underprice the actual transaction's storage writes.
		var gas hexutil.Uint64
		if err := r.CallContext(ctx, &gas, "eth_estimateGas", map[string]any{"from": owner, "to": address, "data": hexutil.Encode(data)}, "pending"); err != nil {
			t.Fatalf("%s.%s pending gas estimate: %v", name, method, err)
		}
		signer.GasLimit = uint64(gas)
		contract := bind.NewBoundContract(address, contractABI, sim.Client(), sim.Client(), sim.Client())
		tx, err := contract.Transact(signer, method, args...)
		if err != nil {
			t.Fatalf("%s.%s: %v", name, method, err)
		}
		sim.Commit()
		receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
		if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("%s.%s gasLimit=%d receipt=%+v err=%v", name, method, tx.Gas(), receipt, err)
		}
		return receipt
	}
	zero := common.Address{}
	transact("ProtocolConfig", "setProtocolFeeBps", big.NewInt(0))
	transact("ProtocolConfig", "setBaseFeeBps", big.NewInt(100))
	transact("ProtocolConfig", "setFeeWeights", big.NewInt(0), big.NewInt(0), big.NewInt(0))
	transact("PoolRegistry", "grantRole", crypto.Keccak256Hash([]byte("FACTORY_ROLE")), owner)
	transact("FeeAccumulator", "grantRole", crypto.Keccak256Hash([]byte("POOL_ROLE")), addresses["SidioraPool"])
	transact("FeeAccumulator", "grantRole", crypto.Keccak256Hash([]byte("FEES_ROUTER_ROLE")), addresses["FeesRouter"])
	transact("SidioraNFT", "grantRole", crypto.Keccak256Hash([]byte("MINTER_ROLE")), owner)
	transact("PoolRegistry", "register", addresses["SidioraPool"], addresses["Token"], owner, zero, big.NewInt(1))
	minted := transact("SidioraNFT", "mint", owner, addresses["SidioraPool"], uint8(0))
	transact("Token", "transfer", addresses["SidioraPool"], big.NewInt(999_999_999_999_999_997))
	transact("USDL", "transfer", addresses["SidioraPool"], big.NewInt(1_000_000_000))
	head, err := sim.Client().HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	transact("SidioraPool", "swap", big.NewInt(1_000_000_000), big.NewInt(0), true, common.HexToAddress("0xdead"), new(big.Int).SetUint64(head.Time+300))

	directory := filepath.Join(t.TempDir(), "reward-recovery")
	dsn, stop := repair29Store(t, ctx, directory)
	defer func() { stop() }()
	pool := repair29Pool(t, ctx, dsn)
	defer func() { pool.Close() }()
	schema := "repair29_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	q := pgx.Identifier{schema}.Sanitize()
	repair29SQL(t, ctx, pool, "CREATE SCHEMA "+q+"; CREATE SCHEMA recovery; CREATE SCHEMA indexer")
	applyRepair29Migration(t, ctx, pool, root, "180000.sql", schema)
	guard, err := queue.NewEpochGuard("recovery", 1, "local")
	if err != nil {
		t.Fatal(err)
	}
	if ledger, err := newRewardLedger(ctx, pool, schema, "pnld", nil, guard.Assert); err == nil || ledger != nil {
		t.Fatal("reward routes could become ready before request/audit migration")
	}
	applyRepair29Migration(t, ctx, pool, root, "180001.sql", schema)
	epochTemplate, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/production/recovery/epoch.sql"))
	if err != nil {
		t.Fatal(err)
	}
	epochSQL, err := queue.RenderEpochSQL("recovery", string(epochTemplate))
	if err != nil {
		t.Fatal(err)
	}
	repair29SQL(t, ctx, pool, epochSQL)
	repair29SQL(t, ctx, pool, `INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,1,'local',1,$1)`, strings.Repeat("a", 64))
	repair29SQL(t, ctx, pool, `CREATE TABLE indexer.indexer_canonical_creations (
  nft_id numeric PRIMARY KEY,pool text NOT NULL,creator text NOT NULL,
  height bigint NOT NULL,block_hash text NOT NULL,transaction_hash text NOT NULL,
  transaction_index numeric NOT NULL,log_index numeric NOT NULL)`)
	var creation *types.Log
	for _, entry := range minted.Logs {
		if entry.Address == addresses["SidioraNFT"] && len(entry.Topics) == 4 && entry.Topics[0] == crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")) && entry.Topics[1] == (common.Hash{}) {
			creation = entry
		}
	}
	if creation == nil {
		t.Fatal("real NFT mint produced no creation event")
	}
	repair29SQL(t, ctx, pool, `INSERT INTO indexer.indexer_canonical_creations VALUES(1,$1,$2,$3,$4,$5,$6,$7)`, strings.ToLower(addresses["SidioraPool"].Hex()), wallet, creation.BlockNumber, creation.BlockHash.Hex(), creation.TxHash.Hex(), creation.TxIndex, creation.Index)
	hashes := map[string]string{}
	for _, target := range []string{l18.NFT, l18.Accumulator, l18.FeesRouter, l18.USDL} {
		var code, slot hexutil.Bytes
		if err := r.CallContext(ctx, &code, "eth_getCode", target, "latest"); err != nil {
			t.Fatal(err)
		}
		hashes[target] = crypto.Keccak256Hash(code).Hex()
		if err := r.CallContext(ctx, &slot, "eth_getStorageAt", target, "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", "latest"); err != nil || len(slot) != 32 {
			t.Fatalf("proxy implementation: %v", err)
		}
		implementation := common.BytesToAddress(slot[12:])
		if implementation != zero {
			if err := r.CallContext(ctx, &code, "eth_getCode", implementation.Hex(), "latest"); err != nil {
				t.Fatal(err)
			}
			hashes[strings.ToLower(implementation.Hex())] = crypto.Keccak256Hash(code).Hex()
		}
	}
	committed := func(c context.Context) (l18.Checkpoint, error) {
		h, err := sim.Client().HeaderByNumber(c, nil)
		if err != nil {
			return l18.Checkpoint{}, err
		}
		return l18.Checkpoint{Number: h.Number.Uint64(), Hash: h.Hash().Hex()}, nil
	}
	release, contractDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	open := func() *httptest.Server {
		t.Helper()
		ledger, err := newRewardLedger(ctx, pool, schema, "pnld", nil, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		creator, err := l18.NewCommittedCreator(ctx, r, time.Now, committed, l18.ManifestDigest, hashes)
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := creator.PrepareClaim(ctx, "1", wallet, wallet)
		if err != nil || prepared.Preview.ClaimableRaw != "10000000" {
			t.Fatalf("real creator preparation=%+v err=%v", prepared, err)
		}
		service, err := l18.NewJourneyService(ledger, creator, bytes.Repeat([]byte{1}, 32), time.Now, release, contractDigest)
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
		return server
	}
	idempotencyKey := uuid.NewString()
	body := fmt.Sprintf(`{"rewardIds":["creator:1"],"recipient":%q}`, wallet)
	send := func(server *httptest.Server, body string, status int) []byte {
		t.Helper()
		path := "/v2/me/rewards:prepare-claim"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		repair29Sign(t, req, key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("claim transport headers=%v", response.Header)
		}
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != status {
			t.Fatalf("claim HTTP status=%d want=%d err=%v body=%s", response.StatusCode, status, err, raw)
		}
		return raw
	}
	snapshot := func(response []byte) string {
		t.Helper()
		var stored string
		var identities, valid bool
		query := `SELECT jsonb_build_array(to_jsonb(r),to_jsonb(a),to_jsonb(o))::text,
   r.actor=$1 AND r.request_key=$2 AND a.identity=r.actor||':'||r.request_key::text
    AND o.identity='prepare:'||a.identity,
   r.response=$3 AND r.method='POST' AND r.path='/v2/me/rewards:prepare-claim'
    AND r.release_digest=$4 AND r.interface_digest=$5 AND r.expires_at>now()
    AND a.action='prepare_claim' AND a.payload=convert_from(r.response,'UTF8')::jsonb
    AND o.topic='reward.claim_prepared'
   FROM ` + q + `.l18_requests r JOIN ` + q + `.l18_audit a ON a.identity=r.actor||':'||r.request_key::text
   JOIN ` + q + `.l18_outbox o ON o.identity='prepare:'||a.identity WHERE r.actor=$1 AND r.request_key=$2`
		if err := pool.QueryRow(ctx, query, wallet, idempotencyKey, response, release, contractDigest).Scan(&stored, &identities, &valid); err != nil {
			t.Fatal(err)
		}
		if !identities || !valid {
			t.Fatalf("durable preparation identity/response mismatch: %s", stored)
		}
		for _, table := range []string{"l18_requests", "l18_audit", "l18_outbox"} {
			var count int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+q+"."+table).Scan(&count); err != nil || count != 1 {
				t.Fatalf("durable %s count=%d want=1 err=%v", table, count, err)
			}
		}
		return stored
	}
	server := open()
	first := send(server, body, http.StatusOK)
	decoded, err := iface.DecodeRewardClaimResponse(first)
	if err != nil {
		t.Fatalf("real consumer rejected preparation: %v body=%s", err, first)
	}
	calldata, err := abis["FeesRouter"].Pack("claimFees", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	transaction := decoded.Data.TransactionRequest
	if transaction.ChainId != "125" || transaction.From != wallet || transaction.To != l18.FeesRouter || transaction.Data != hexutil.Encode(calldata) || transaction.ValueRaw != "0" || transaction.GasLimitRaw == nil || decoded.Data.EstimatedGasPaxRaw == "0" {
		t.Fatalf("consumer transaction=%+v", decoded.Data)
	}
	gas, ok := new(big.Int).SetString(string(*transaction.GasLimitRaw), 10)
	if !ok || gas.Sign() <= 0 {
		t.Fatalf("consumer gas limit=%v", transaction.GasLimitRaw)
	}
	cost, ok := new(big.Int).SetString(string(decoded.Data.EstimatedGasPaxRaw), 10)
	if !ok || cost.Sign() <= 0 {
		t.Fatalf("consumer gas cost=%s", decoded.Data.EstimatedGasPaxRaw)
	}
	if decoded.Quality != "complete" || decoded.Watermark == nil || decoded.Watermark.ChainId != "125" {
		t.Fatalf("consumer metadata=%+v", decoded)
	}
	if _, err := iface.DecodeRewardClaimResponse(first[:len(first)/2]); err == nil {
		t.Fatal("consumer accepted truncated producer response")
	}
	before := snapshot(first)
	server.Close()
	pool.Close()
	stop()
	dsn, stop = repair29Store(t, ctx, directory)
	pool = repair29Pool(t, ctx, dsn)
	server = open()
	replay := send(server, body, http.StatusOK)
	if !bytes.Equal(first, replay) {
		t.Fatalf("persisted preparation changed after database/service reopen: first=%s replay=%s", first, replay)
	}
	if _, err := iface.DecodeRewardClaimResponse(replay); err != nil {
		t.Fatalf("consumer rejected recovered preparation: %v", err)
	}
	if after := snapshot(replay); before != after {
		t.Fatalf("request/audit/outbox identities changed on replay: before=%s after=%s", before, after)
	}
	send(server, strings.Replace(body, "creator:1", "creator:2", 1), http.StatusConflict)
	if after := snapshot(first); before != after {
		t.Fatalf("conflicting reuse changed durable preparation: before=%s after=%s", before, after)
	}
}

func repair29Sign(t *testing.T, req *http.Request, key *ecdsa.PrivateKey) {
	t.Helper()
	signature, err := crypto.Sign(accounts.TextHash([]byte(req.Method+" "+req.URL.RequestURI())), key)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	req.Header.Set("X-Proof-Wallet", strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()))
	req.Header.Set("X-Proof-Signature", hexutil.Encode(signature))
}
