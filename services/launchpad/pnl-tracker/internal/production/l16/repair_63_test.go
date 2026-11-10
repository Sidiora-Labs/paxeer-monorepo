package l16

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Registry paths are relative to the launchpad tree, not the monorepo root
// or the package working directory. Bind every SQL input before using it.
func repair63MigrationInputs(t *testing.T) (map[int64]string, production.RoleManifest) {
	t.Helper()
	root := filepath.Join(coordinatortest.RepositoryRoot(t), "launchpad")
	directory := filepath.Join(root, "spec", "production-system", "integration", "migrations")
	raw, err := os.ReadFile(filepath.Join(directory, "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry production.MigrationRegistry
	if err = json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	inputs := make(map[int64]string, len(registry.Registrations))
	for _, migration := range registry.Registrations {
		sql, err := os.ReadFile(filepath.Join(root, migration.Path))
		if err != nil {
			t.Fatal(err)
		}
		if production.Checksum(sql) != migration.SHA256 {
			t.Fatalf("migration %d differs from its registered checksum", migration.MigrationID)
		}
		if _, exists := inputs[migration.MigrationID]; exists {
			t.Fatalf("migration %d registered twice", migration.MigrationID)
		}
		inputs[migration.MigrationID] = string(sql)
	}
	raw, err = os.ReadFile(filepath.Join(directory, "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var grants struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err = json.Unmarshal(raw, &grants); err != nil {
		t.Fatal(err)
	}
	for _, role := range grants.Roles {
		if role.RuntimeRole == "pnl_runtime" {
			if err = role.Validate(); err != nil {
				t.Fatal(err)
			}
			return inputs, role
		}
	}
	t.Fatal("coordinator grants omit pnl_runtime")
	return nil, production.RoleManifest{}
}

func TestRepair_63_MigrationFixturePaths(t *testing.T) {
	inputs, role := repair63MigrationInputs(t)
	for _, id := range []int64{60000, 160002} {
		if inputs[id] == "" {
			t.Fatalf("coordinator fixture omitted registered migration %d", id)
		}
	}
	if _, err := role.GrantSQL(); err != nil {
		t.Fatalf("real coordinator cannot render pnl_runtime grants: %v", err)
	}
}

func TestRepair_63_CoordinatorWalletContract(t *testing.T) {
	inputs, role := repair63MigrationInputs(t)
	migration := inputs[160004]
	walletDDL := strings.Split(RebuildDDL, "CREATE TABLE inventory_l16_archive_v2")[0]
	if !strings.Contains(migration, walletDDL) {
		t.Fatal("coordinator wallet tables and index differ from the real producer schema")
	}
	for _, statement := range strings.Split(RebuildDDL, "\n") {
		if strings.HasPrefix(statement, "CREATE TRIGGER inventory_l16_range_immutable_v3") || strings.HasPrefix(statement, "CREATE TRIGGER inventory_l16_interval_immutable_v3") {
			if !strings.Contains(migration, statement) {
				t.Fatalf("coordinator wallet evidence lacks the real immutable trigger: %s", statement)
			}
		}
	}
	for _, grant := range role.Grants {
		switch grant.Object {
		case "inventory_l16_active_v2":
			if reflect.DeepEqual(grant.Privileges, []string{"UPDATE"}) {
				if !reflect.DeepEqual(grant.Columns, []string{"generation", "snapshot_digest", "height"}) || grant.GrantOption {
					t.Fatal("active pointer UPDATE exceeds the reviewed columns", grant)
				}
			}
		case "inventory_l16_wallet_ranges_v3", "inventory_l16_wallet_intervals_v3":
			if !reflect.DeepEqual(grant.Privileges, []string{"SELECT", "INSERT"}) || len(grant.Columns) != 0 || grant.GrantOption {
				t.Fatal("wallet evidence grants exceed append-only producer privileges", grant)
			}
		}
	}
	manifestPath := filepath.Join(coordinatortest.RepositoryRoot(t), "launchpad", "spec", "production-system", "integration", "migrations", "grants.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	readers := map[string]bool{"api_runtime": false, "api_rest_runtime": false}
	for _, reader := range manifest.Roles {
		if _, ownsDiscovery := readers[reader.RuntimeRole]; !ownsDiscovery {
			continue
		}
		statements, err := reader.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, view := range []struct {
			name    string
			columns string
		}{
			{"inventory_l16_wallet_ranges_read_v3", `"snapshot_id", "wallet", "generation", "start_at", "end_at", "release_digest", "input_digest", "watermark"`},
			{"inventory_l16_wallet_intervals_read_v3", `"snapshot_id", "interval", "bucket", "payload"`},
		} {
			definition := `CREATE VIEW "pnl"."` + view.name + `" WITH (security_barrier=true) AS SELECT ` + view.columns
			if !strings.Contains(migration, definition) || !strings.Contains(migration, `REVOKE ALL ON "pnl"."`+view.name+`" FROM PUBLIC;`) {
				t.Fatalf("consumer interface is missing or public: %s", view.name)
			}
			want := `GRANT SELECT (` + view.columns + `) ON TABLE "pnl"."` + view.name + `" TO "` + reader.RuntimeRole + `"`
			found := false
			for _, statement := range statements {
				found = found || statement == want
			}
			if !found {
				t.Fatalf("real coordinator cannot grant the discovery reader interface: %s", want)
			}
		}
		readers[reader.RuntimeRole] = true
	}
	for reader, found := range readers {
		if !found {
			t.Fatalf("coordinator manifest omitted owning discovery reader %s", reader)
		}
	}
}

func repair63RPCKey(method string, params json.RawMessage) (string, error) {
	var args []any
	if err := json.Unmarshal(params, &args); err != nil {
		return "", fmt.Errorf("%s params: %w", method, err)
	}
	if method == "eth_call" {
		if len(args) != 2 {
			return "", fmt.Errorf("eth_call params: expected 2 elements, got %d", len(args))
		}
		raw, err := json.Marshal(args[1])
		if err != nil {
			return "", fmt.Errorf("eth_call params[1] block identifier: %w", err)
		}
		var block rpc.BlockNumberOrHash
		if err = json.Unmarshal(raw, &block); err != nil || block.BlockNumber == nil && block.BlockHash == nil {
			return "", fmt.Errorf("eth_call params[1] block identifier %s: invalid block tag or identifier object (%v)", raw, err)
		}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("%s params: %w", method, err)
	}
	return method + " " + string(raw), nil
}

func TestRepair_63_TranscriptBlockParameter(t *testing.T) {
	for _, block := range []string{`"latest"`, `"0x2063ec5"`, `{"blockHash":"0x1cd6b450aa87745be16a8760a928a2e7ddd1097e872bdcd99b31ff66f77c2945"}`} {
		if _, err := repair63RPCKey("eth_call", json.RawMessage(`[{},`+block+`]`)); err != nil {
			t.Fatalf("producer block parameter %s rejected: %v", block, err)
		}
	}
	if _, err := repair63RPCKey("eth_call", json.RawMessage(`[{}, {"blockHash":42}]`)); err == nil || !strings.Contains(err.Error(), "eth_call params[1]") {
		t.Fatalf("invalid block parameter must name its offending element: %v", err)
	}
}

func TestRepair_63_RecordedReplayHeaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, repair63RecordedRPC(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, recorded := range []struct {
		height int64
		hash   string
	}{
		{0x2063ed5, "0x50f4e42d7247aa1665d1aaebc33039c8fc8e493727a864085ba77910dda56f6b"},
		{0x2063ec4, "0x81a18d822e327fbcbfe2be91b7f666bfa043016cb0e3f094c5bbabe91f9d192b"},
	} {
		number := big.NewInt(recorded.height)
		header, hash, err := reportedHeaderByNumber(ctx, client, number)
		if err != nil || hash != recorded.hash {
			t.Fatalf("recorded replay header %x: hash=%s err=%v", recorded.height, hash, err)
		}
		next, _, err := reportedHeaderByNumber(ctx, client, big.NewInt(recorded.height+1))
		if err != nil || next.ParentHash != common.HexToHash(hash) {
			t.Fatalf("recorded replay header %x does not match the producer transcript's next parent: %v", recorded.height, err)
		}
		byHash, err := client.HeaderByHash(ctx, common.HexToHash(hash))
		if err != nil || !reflect.DeepEqual(byHash, header) {
			t.Fatalf("recorded replay header %x differs by hash: %v", recorded.height, err)
		}
		block, err := reportedBlockByNumber(ctx, client, number)
		if err != nil || block.Hash != common.HexToHash(hash) || len(block.Transactions) != 1 {
			t.Fatalf("recorded replay block %x lost its header or transaction: %+v %v", recorded.height, block, err)
		}
	}
	code, err := client.CodeAtHash(ctx,
		common.HexToAddress("0x85fcd13735f4309833a503ee804ea32395851479"),
		common.HexToHash("0xd955d693c649ee8e9a69be1e9d6d683ac05a6b846ecbb6f0d165ff1a698ceb68"))
	if err != nil || len(code) != 4230 || production.Checksum(code) != "32ae0f6e3bebe0f39b69a2790c61f7899a77c6b6d5584f53c8783053cf98e9a4" {
		t.Fatalf("recorded historical emitter code differs from the producer transcript: bytes=%d sha256=%s err=%v", len(code), production.Checksum(code), err)
	}
}

// Replay the indexer's committed node traffic. State reads absent from the
// receipt qualification transcript are evaluated from its canonical ERC20
// transfer history, using the same production log decoder as the participant.
func repair63RecordedRPC(t *testing.T) string {
	t.Helper()
	path := filepath.Join(coordinatortest.RepositoryRoot(t), "services", "launchpad", "indexer", "internal", "internaltest", "chainfixture", "testdata", "paxeer125-market.jsonl.gz")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if production.Checksum(raw) != "8e30a49734f6381b6234e64d2ee01367a9c47939d1cd1c13b5755914e43b61a1" {
		t.Fatal("producer RPC transcript differs from its committed checksum")
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	headers, err := os.ReadFile(filepath.Join("testdata", "repair_63_headers.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if production.Checksum(headers) != "fb6c89748c9e21bc2824fcc4df39cd8dc448332f1fcf1141e2e1cee2826db418" {
		t.Fatal("supplemental producer RPC headers differ from their committed checksum")
	}
	responses := map[string]json.RawMessage{}
	rpcErrors := map[string]json.RawMessage{}
	blocks := map[string]json.RawMessage{}
	heights := map[string]uint64{}
	logs := map[string]types.Log{}
	scanner := bufio.NewScanner(io.MultiReader(z, bytes.NewReader(headers)))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var record struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		method, params, ok := strings.Cut(record.Key, " ")
		if !ok || json.Valid(record.Result) == json.Valid(record.Error) {
			t.Fatalf("producer RPC transcript record %d %s: expected exactly one result or error", line, record.Key)
		}
		key, err := repair63RPCKey(method, json.RawMessage(params))
		if err != nil {
			t.Fatalf("producer RPC transcript record %d %s: %v", line, record.Key, err)
		}
		if len(record.Error) != 0 {
			rpcErrors[key] = record.Error
			continue
		}
		responses[key] = record.Result
		if method == "eth_getBlockByNumber" || method == "eth_getBlockByHash" {
			var block struct {
				Hash   string         `json:"hash"`
				Number hexutil.Uint64 `json:"number"`
			}
			if err := json.Unmarshal(record.Result, &block); err != nil {
				t.Fatal(err)
			}
			// JSON-RPC's false form retains the recorded header and replaces
			// full transaction objects with their producer-recorded hashes.
			var header map[string]json.RawMessage
			if err := json.Unmarshal(record.Result, &header); err != nil {
				t.Fatalf("producer RPC transcript record %d %s block: %v", line, record.Key, err)
			}
			var transactions []json.RawMessage
			if err := json.Unmarshal(header["transactions"], &transactions); err != nil {
				t.Fatalf("producer RPC transcript record %d %s transactions: %v", line, record.Key, err)
			}
			hashes := make([]common.Hash, len(transactions))
			for i, raw := range transactions {
				if err := json.Unmarshal(raw, &hashes[i]); err != nil {
					var transaction struct {
						Hash *common.Hash `json:"hash"`
					}
					if err = json.Unmarshal(raw, &transaction); err != nil || transaction.Hash == nil {
						t.Fatalf("producer RPC transcript record %d %s transactions[%d].hash: %v", line, record.Key, i, err)
					}
					hashes[i] = *transaction.Hash
				}
			}
			header["transactions"], err = json.Marshal(hashes)
			if err != nil {
				t.Fatal(err)
			}
			withoutTransactions, err := json.Marshal(header)
			if err != nil {
				t.Fatal(err)
			}
			blocks[block.Hash] = withoutTransactions
			headerKey, err := repair63RPCKey("eth_getBlockByNumber", json.RawMessage(fmt.Sprintf(`[%q,false]`, hexutil.EncodeUint64(uint64(block.Number)))))
			if err != nil {
				t.Fatal(err)
			}
			responses[headerKey] = withoutTransactions
			heights[block.Hash] = uint64(block.Number)
		}
		if method == "eth_getTransactionReceipt" {
			var receipt types.Receipt
			if err := json.Unmarshal(record.Result, &receipt); err != nil {
				t.Fatal(err)
			}
			for _, log := range receipt.Logs {
				logs[fmt.Sprintf("%s:%d", log.TxHash.Hex(), log.Index)] = *log
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	serve := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(request.Params) == 0 || string(request.Params) == "null" {
			request.Params = json.RawMessage(`[]`)
		}
		var args []json.RawMessage
		if err := json.Unmarshal(request.Params, &args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		key, err := repair63RPCKey(request.Method, request.Params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result := responses[key]
		rpcError := rpcErrors[key]
		if request.Method == "eth_getBlockByHash" && len(args) == 2 {
			var hash string
			if json.Unmarshal(args[0], &hash) == nil {
				result = blocks[hash]
			}
		}
		if request.Method == "eth_getCode" && len(args) == 2 {
			var token, number string
			if json.Unmarshal(args[0], &token) == nil && json.Unmarshal(args[1], &number) == nil && token == realMarketToken && number == hexutil.EncodeUint64(33963715) {
				// The recorded factory creation deploys this token in the next block.
				result = json.RawMessage(`"0x"`)
			}
		}
		if request.Method == "eth_call" && len(args) == 2 && len(result) == 0 && len(rpcError) == 0 {
			var call struct {
				To    string `json:"to"`
				Input string `json:"input"`
			}
			var anchor rpc.BlockNumberOrHash
			if json.Unmarshal(args[0], &call) == nil && json.Unmarshal(args[1], &anchor) == nil && strings.EqualFold(call.To, realMarketToken) {
				height, exists := uint64(0), false
				if hash, ok := anchor.Hash(); ok {
					height, exists = heights[hash.Hex()]
				} else if number, ok := anchor.Number(); ok && number >= 0 {
					for _, recordedHeight := range heights {
						if recordedHeight == uint64(number) {
							height, exists = recordedHeight, true
							break
						}
					}
				}
				if exists {
					value := new(big.Int)
					known := false
					if call.Input == "0x313ce567" {
						// The producer records the immutable token decimal setting.
						for key, raw := range responses {
							if strings.HasPrefix(key, "eth_call ") && strings.Contains(key, `"input":"0x313ce567"`) && strings.Contains(key, `"to":"`+realMarketToken+`"`) {
								result = raw
							}
						}
					} else if call.Input == hexutil.Encode(totalSupplySelector) || strings.HasPrefix(call.Input, hexutil.Encode(balanceSelector)) && len(call.Input) == 74 {
						known = true
						wallet := ""
						if len(call.Input) == 74 {
							wallet = strings.ToLower(common.HexToAddress("0x" + call.Input[len(call.Input)-40:]).Hex())
						}
						for _, log := range logs {
							if log.BlockNumber > height || !strings.EqualFold(log.Address.Hex(), realMarketToken) || len(log.Topics) == 0 || log.Topics[0] != transferTopic {
								continue
							}
							move, err := decodeTransfer(log)
							if err != nil {
								known = false
								break
							}
							amount, _ := new(big.Int).SetString(move.Amount, 10)
							if wallet != "" && move.To == wallet || wallet == "" && move.From == zeroAddress {
								value.Add(value, amount)
							}
							if wallet != "" && move.From == wallet || wallet == "" && move.To == zeroAddress {
								value.Sub(value, amount)
							}
						}
					}
					if known && value.Sign() >= 0 && value.BitLen() <= 256 {
						result, _ = json.Marshal(hexutil.Encode(value.FillBytes(make([]byte, 32))))
					}
				}
			}
		}
		if request.Method == "eth_getLogs" && len(args) == 1 {
			var filter struct {
				From      hexutil.Uint64   `json:"fromBlock"`
				To        hexutil.Uint64   `json:"toBlock"`
				Addresses []common.Address `json:"address"`
				Topics    [][]common.Hash  `json:"topics"`
			}
			if json.Unmarshal(args[0], &filter) == nil && len(filter.Addresses) == 1 && filter.Addresses[0] == common.HexToAddress(realMarketToken) && len(filter.Topics) == 1 && len(filter.Topics[0]) == 1 && filter.Topics[0][0] == transferTopic {
				selected := []types.Log{}
				for _, log := range logs {
					if log.BlockNumber >= uint64(filter.From) && log.BlockNumber <= uint64(filter.To) && log.Address == filter.Addresses[0] && len(log.Topics) > 0 && log.Topics[0] == transferTopic {
						selected = append(selected, log)
					}
				}
				sort.Slice(selected, func(i, j int) bool {
					if selected[i].BlockNumber != selected[j].BlockNumber {
						return selected[i].BlockNumber < selected[j].BlockNumber
					}
					return selected[i].Index < selected[j].Index
				})
				result, _ = json.Marshal(selected)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if len(rpcError) != 0 {
			response["error"] = rpcError
		} else if len(result) == 0 {
			response["error"] = map[string]any{"code": -32601, "message": "unrecorded repair RPC: " + request.Method + " " + string(request.Params)}
		} else {
			response["result"] = result
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(serve.Close)
	return serve.URL
}

func repair63CurrentFillRows(t *testing.T, ctx context.Context, raw json.RawMessage) json.RawMessage {
	t.Helper()
	client, err := ethclient.DialContext(ctx, repair63RecordedRPC(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("recorded buy/sell fixture has %d fills", len(rows))
	}
	for i, row := range rows {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(row["payload"], &payload); err != nil {
			t.Fatal(err)
		}
		var execution map[string]json.RawMessage
		if err := json.Unmarshal(payload["execution"], &execution); err != nil {
			t.Fatal(err)
		}
		// The recorded producer used feeBps. Re-encode that fact with the
		// current producer codec, preserving and checking the old digest first.
		fee, legacy := execution["feeBps"]
		if _, current := execution["feeBPS"]; !legacy || current {
			t.Fatalf("recorded fill %d is not the expected legacy feeBps wire", i)
		}
		delete(execution, "feeBps")
		execution["feeBPS"] = fee
		var err error
		payload["execution"], err = json.Marshal(execution)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		fill, err := accounting.DecodeFillV1(wire)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		var recordedDigest string
		if err = json.Unmarshal(row["source_fact_digest"], &recordedDigest); err != nil {
			t.Fatal(err)
		}
		legacyCanonical := bytes.Replace(canonical, []byte(`"feeBPS":`), []byte(`"feeBps":`), 1)
		if delivery.Hash(legacyCanonical) != recordedDigest {
			t.Fatalf("recorded fill %d changed beyond the producer's feeBPS spelling", i)
		}
		height, ok := new(big.Int).SetString(fill.Position.Height, 10)
		if !ok {
			t.Fatal("recorded fill height is invalid")
		}
		header, hash, err := reportedHeaderByNumber(ctx, client, height)
		if err != nil || hash != fill.Position.BlockHash || fmt.Sprint(header.Time) != fill.BlockTimestampSeconds {
			t.Fatalf("recorded fill %d does not bind the replayed header: %s %v", i, hash, err)
		}
		headerWire, err := json.Marshal(header)
		if err != nil {
			t.Fatal(err)
		}
		// The supplemental capture reflects the node's corrected receipt root.
		// Bind the current producer encoding to that recorded header artifact.
		fill.Provenance.HeaderSHA256 = delivery.Hash(headerWire)
		canonical, err = json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := accounting.FillDigest(fill)
		if err != nil {
			t.Fatal(err)
		}
		row["payload"] = canonical
		row["source_fact_digest"], err = json.Marshal(digest)
		if err != nil {
			t.Fatal(err)
		}
	}
	wire, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

// Bootstrap is deliberately unmodified: neither RebuildDDL nor supplemental
// GRANTs may conceal a missing coordinator migration or runtime permission.
func repair63Database(t *testing.T, ctx context.Context) (coordinatortest.Database, *pgxpool.Pool) {
	t.Helper()
	db := coordinatortest.Bootstrap(t, realMarketEnv(t, "PNL_TEST_POSTGRES_DSN"), "pnl_runtime")
	admin, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	raw, err := os.ReadFile(filepath.Join("testdata", "indexer_real_market", "traded.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tables map[string]json.RawMessage
	if err = json.Unmarshal(raw, &tables); err != nil {
		t.Fatal(err)
	}
	tables["indexer.indexer_accounting_fills"] = repair63CurrentFillRows(t, ctx, tables["indexer.indexer_accounting_fills"])
	// These are the real indexer's recorded outputs. PnL evidence is withheld
	// so both tests must produce it through the service under its runtime role.
	pending := []string{}
	for name := range tables {
		if strings.HasPrefix(name, "indexer.") {
			pending = append(pending, name)
		}
	}
	sort.Strings(pending)
	for len(pending) > 0 {
		remaining := []string{}
		var last error
		for _, name := range pending {
			if err = insertRecordedTable(ctx, admin, name, tables[name]); err != nil {
				remaining = append(remaining, name)
				last = err
			}
		}
		if len(remaining) == len(pending) {
			t.Fatalf("load producer-recorded tables %v: %v", remaining, last)
		}
		pending = remaining
	}
	// Renew only the source lease, as in loadRealMarketStage; no economic fact,
	// completeness watermark, replay payload, schema or runtime ACL is altered.
	if _, err = admin.Exec(ctx, `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()+interval '10 minutes',last_heartbeat_at=clock_timestamp() WHERE chain_id=125 AND stream_id=$1`, realMarketStream); err != nil {
		t.Fatal(err)
	}
	return db, admin
}

func repair63Service(t *testing.T, ctx context.Context, db coordinatortest.Database) *Service {
	t.Helper()
	runtime, err := pgxpool.New(ctx, db.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	var role string
	var elevated bool
	if err = runtime.QueryRow(ctx, `SELECT current_user,rolsuper OR rolcreatedb OR rolcreaterole OR rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&role, &elevated); err != nil {
		t.Fatal(err)
	}
	if role != db.Runtimes["pnl_runtime"].Role || elevated {
		t.Fatalf("replay is not running as the coordinator's least-privilege pnl_runtime: %s elevated=%v", role, elevated)
	}
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := ethclient.DialContext(ctx, repair63RecordedRPC(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(primary.Close)
	archive, err := ethclient.DialContext(ctx, repair63RecordedRPC(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(archive.Close)
	authority, err := NewBaselineAuthorityWithEpochGuard(runtime, primary, "pnl", realMarketStream, "pnl", "g1", "pnl-1", "inventory-v1", guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}}
	outbox, err := delivery.NewStoreWithEpochGuard(runtime, "pnl", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(authority, archive, [2]string{"repair-primary", "repair-archive"}, strings.Repeat("c", 64), outbox)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func repair63LiveReplay(t *testing.T, ctx context.Context, service *Service, height int64) RebuildSnapshot {
	t.Helper()
	tx, err := service.authority.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
		t.Fatal(err)
	}
	// This is the live participant's actual producer, including canonical
	// receipt verification, dual-node parity and livePersistReplayTx.
	snapshot, err := service.liveRebuildTx(ctx, tx, realMarketCreator, realMarketToken, height)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.authority.epochGuard(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func repair63Active(t *testing.T, ctx context.Context, service *Service, expected RebuildSnapshot) {
	t.Helper()
	actual, err := service.Active(ctx, realMarketCreator, realMarketToken)
	if err != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("active pointer did not recover the committed producer snapshot: actual=%+v expected=%+v err=%v", actual, expected, err)
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var encoded []byte
	if err = service.authority.pool.QueryRow(ctx, `SELECT envelope FROM pnl.delivery_outbox WHERE subject=$1 AND event_id=$2`, RebuildSubject, delivery.Hash(raw)).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{RebuildSubject: true}, EventTypes: map[string]bool{RebuildSubject: true}}
	event, err := delivery.Decode(encoded, policy, RebuildSubject)
	if err != nil || event.PayloadHash != delivery.Hash(raw) || string(event.Payload) != string(raw) {
		t.Fatalf("active snapshot and transactional outbox disagree: %+v %v", event, err)
	}
}

func TestRepair_63_ActiveSnapshotUpdateGrant(t *testing.T) {
	_, role := repair63MigrationInputs(t)
	statements, err := role.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	want := `GRANT UPDATE ("generation", "snapshot_digest", "height") ON TABLE "pnl"."inventory_l16_active_v2" TO "pnl_runtime"`
	found := false
	for _, statement := range statements {
		found = found || statement == want
	}
	if !found {
		t.Fatalf("active-snapshot-update-grant: coordinator grants.json does not supply %s; apply the Domain 4 wiring request before runtime qualification", want)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	db, _ := repair63Database(t, ctx)
	service := repair63Service(t, ctx, db)
	// Both activation implementations use this same column-scoped contract.
	for _, column := range []string{"generation", "snapshot_digest", "height"} {
		var allowed bool
		if err := service.authority.pool.QueryRow(ctx, `SELECT has_column_privilege(current_user,'pnl.inventory_l16_active_v2',$1,'UPDATE')`, column).Scan(&allowed); err != nil || !allowed {
			t.Fatalf("active-snapshot-update-grant: UPDATE(%s) missing: %v", column, err)
		}
	}
	var partitionWrite, tableWrite, deletion bool
	if err := service.authority.pool.QueryRow(ctx, `SELECT has_column_privilege(current_user,'pnl.inventory_l16_active_v2','partition','UPDATE'),has_table_privilege(current_user,'pnl.inventory_l16_active_v2','UPDATE'),has_table_privilege(current_user,'pnl.inventory_l16_active_v2','DELETE')`).Scan(&partitionWrite, &tableWrite, &deletion); err != nil {
		t.Fatal(err)
	}
	if partitionWrite || tableWrite || deletion {
		t.Fatal("active pointer privileges exceed the three reviewed UPDATE columns")
	}
	first, err := service.RebuildArchive(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	repair63Active(t, ctx, service, first)
	later := repair63LiveReplay(t, ctx, service, 33963749)
	if later.Height <= first.Height || later.State == first.State || later.BlockHash == first.BlockHash {
		t.Fatal("recorded buy and later sell did not advance inventory", first, later)
	}
	repair63Active(t, ctx, service, later)
	service.authority.pool.Close()
	reopened := repair63Service(t, ctx, db)
	repair63Active(t, ctx, reopened, later)
	var snapshots, notices int
	if err = reopened.authority.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.inventory_l16_snapshots_v2),(SELECT count(*) FROM pnl.delivery_outbox WHERE subject=$1)`, RebuildSubject).Scan(&snapshots, &notices); err != nil || snapshots != 2 || notices != 2 {
		t.Fatalf("restart lost immutable snapshots or atomic outbox evidence: snapshots=%d notices=%d err=%v", snapshots, notices, err)
	}
}

func TestRepair_63_WalletRangeSchemaMigrated(t *testing.T) {
	inputs, role := repair63MigrationInputs(t)
	statements, err := role.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"inventory_l16_wallet_ranges_v3", "inventory_l16_wallet_intervals_v3"} {
		definition := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:"?pnl"?\.)?"?` + table + `"?\s*\(`)
		found := false
		for _, sql := range inputs {
			found = found || definition.MatchString(sql)
		}
		if !found {
			t.Fatalf("wallet-range-schema-not-migrated: no checksum-bound coordinator migration creates pnl.%s; apply the Domain 4 wiring request before runtime qualification", table)
		}
		want := `GRANT SELECT, INSERT ON TABLE "pnl"."` + table + `" TO "pnl_runtime"`
		found = false
		for _, statement := range statements {
			found = found || statement == want
		}
		if !found {
			t.Fatalf("wallet-range-schema-not-migrated: coordinator grants.json does not supply %s", want)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	db, admin := repair63Database(t, ctx)
	service := repair63Service(t, ctx, db)
	snapshot := repair63LiveReplay(t, ctx, service, 33963749)
	// PublishRebuiltWallet calls the real commitRebuiltHistoryTx, including
	// the range INSERT and interval COPY, in the projection/outbox transaction.
	summary, err := service.PublishRebuiltWallet(ctx, realMarketCreator, snapshot.Height, l17.Repository{Pool: service.authority.pool})
	if err != nil {
		t.Fatal(err)
	}
	var ranges, intervals, history int
	if err = service.authority.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.inventory_l16_wallet_ranges_v3 WHERE snapshot_id=$1),(SELECT count(*) FROM pnl.inventory_l16_wallet_intervals_v3 WHERE snapshot_id=$1),(SELECT count(*) FROM pnl.portfolio_history_rows WHERE snapshot_id=$1)`, summary.SnapshotID).Scan(&ranges, &intervals, &history); err != nil || ranges != 1 || intervals != 2 || history != 2 {
		t.Fatalf("wallet history producer did not persist both interval resolutions and the canonical buy/sell history: %d %d %d %v", ranges, intervals, history, err)
	}
	var payload []byte
	if err = service.authority.pool.QueryRow(ctx, `SELECT payload FROM pnl.inventory_l16_wallet_intervals_v3 WHERE snapshot_id=$1 AND interval='1h'`, summary.SnapshotID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var traded IntervalPnL
	if err = json.Unmarshal(payload, &traded); err != nil {
		t.Fatal(err)
	}
	if traded.TradeCount != 2 || traded.SaleCount != 1 || !traded.AccountingComplete || traded.RealizedPnlUsdlRaw == nil || *traded.RealizedPnlUsdlRaw != snapshot.State.Realized {
		t.Fatalf("persisted interval lost the producer's exact realized result: %+v state=%+v", traded, snapshot.State)
	}
	// The recorded head is within the trading hour. Read the complete earlier
	// hours and require the partial trading hour to remain unavailable.
	from, err := time.Parse("2006-01-02", snapshot.Checkpoints[0].Day)
	if err != nil {
		t.Fatal(err)
	}
	to := snapshot.ObservedThrough.Truncate(time.Hour)
	if !from.Before(to) {
		t.Fatal("recorded market must include a complete hourly prefix")
	}
	series, err := service.IntervalHistory(ctx, realMarketCreator, from, to, "1h", 24)
	if err != nil || series.SnapshotID != summary.SnapshotID || series.Watermark != summary.Watermark || len(series.Entries) != int(to.Sub(from)/time.Hour) || series.InputDigest != snapshot.InputDigest {
		t.Fatalf("interval consumer cannot read coordinator-migrated producer evidence: %+v %v", series, err)
	}
	for _, item := range series.Entries {
		if item.TradeCount != 0 || item.RealizedPnlUsdlRaw == nil || *item.RealizedPnlUsdlRaw != "0" || !item.AccountingComplete {
			t.Fatal("complete pre-trade interval is not an exact zero", item)
		}
	}
	if _, err = service.IntervalHistory(ctx, realMarketCreator, to, to.Add(time.Hour), "1h", 1); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatalf("partial trading hour admitted as complete: %v", err)
	}
	for _, table := range []string{"inventory_l16_wallet_ranges_v3", "inventory_l16_wallet_intervals_v3"} {
		for _, operation := range []string{"UPDATE pnl." + table + " SET snapshot_id=snapshot_id WHERE snapshot_id=$1", "DELETE FROM pnl." + table + " WHERE snapshot_id=$1"} {
			// The administrator bypasses runtime ACLs, so this checks the real
			// immutable trigger rather than an insufficient-privilege error.
			_, err = admin.Exec(ctx, operation, summary.SnapshotID)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || pgErr.Message != "immutable inventory evidence" {
				t.Fatalf("%s evidence is mutable: %v", table, err)
			}
		}
	}
	service.authority.pool.Close()
	reopened := repair63Service(t, ctx, db)
	recovered, err := reopened.IntervalHistory(ctx, realMarketCreator, from, to, "1h", 24)
	if err != nil || !reflect.DeepEqual(recovered, series) {
		t.Fatalf("wallet interval history changed across restart: %+v %v", recovered, err)
	}
	var indexed bool
	if err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='pnl' AND tablename='inventory_l16_wallet_ranges_v3' AND indexname='inventory_l16_wallet_ranges_v3_latest')`).Scan(&indexed); err != nil || !indexed {
		t.Fatalf("coordinator migration omitted the wallet range lookup index: %v", err)
	}
}
