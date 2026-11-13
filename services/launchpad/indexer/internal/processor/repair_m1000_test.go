package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	pglite "github.com/elliots/go-pglite"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReceiptOptions' trimmed catalog predates historical Treasury admission.
// Supply its exact approved identity and ABI from the committed source; native
// generations continue to use their separately reviewed document.
func repairM1000ReceiptOptions(t *testing.T, inputs runtimedocs.Inputs) receipt.RPCOptions {
	t.Helper()
	options, _, err := runtimedocs.ReceiptOptions(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if inputs.Manifest.ActivationHeight >= deployment.Current().ActivationHeight {
		return options
	}
	options.CatalogJSON = inputs.ReviewedCatalog
	for i := range options.Discovery {
		if options.Discovery[i].Role == "SidioraPool" {
			options.Discovery[i].Kind = "beacon_proxy"
		}
	}
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "services/launchpad/indexer/internal/receipt/deployment_source.json"))
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Hash(raw) != inputs.Manifest.SourceManifestSHA256 {
		t.Fatal("historical source identity changed")
	}
	var source struct {
		Deployment struct {
			Records map[string]struct {
				Address, RuntimeHash, Status string
				Height                       uint64 `json:"blockNumber"`
			} `json:"records"`
		} `json:"_deployment"`
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	proxy, pok := source.Deployment.Records["Treasury_proxy"]
	implementation, iok := source.Deployment.Records["Treasury_impl"]
	if !pok || !iok || proxy.Status != "confirmed" || implementation.Status != "confirmed" || proxy.Height > inputs.Manifest.ActivationHeight || implementation.Height > inputs.Manifest.ActivationHeight {
		t.Fatal("approved historical Treasury unavailable")
	}
	var catalog decode.Catalog
	if err = json.Unmarshal(inputs.ReviewedCatalog, &catalog); err != nil {
		t.Fatal(err)
	}
	var revision string
	for _, contract := range catalog.Contracts {
		if contract.Name == "Treasury" {
			revision = contract.ABIRevision
		}
	}
	if revision == "" {
		t.Fatal("approved Treasury ABI unavailable")
	}
	options.Epochs = append(options.Epochs, receipt.RPCCodeEpoch{Deployment: decode.Deployment{Address: common.HexToAddress(proxy.Address), Role: "Treasury", ABIRevision: revision, From: decode.Order{Block: inputs.Manifest.ActivationHeight}}, Identity: deployment.Role{Address: proxy.Address, Kind: "erc1967", RuntimeHash: proxy.RuntimeHash, ImplementationAddress: implementation.Address, ImplementationHash: implementation.RuntimeHash}})
	return options
}

type repairM1000Recorded struct {
	Key    string          `json:"k"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// Replay source-owned responses, never generating state reads. Log queries
// can select a different approved address set only over complete recorded
// blocks whose every transaction has its original receipt in this packet.
func repairM1000RecordedRPC(t *testing.T, latest ...uint64) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "services/launchpad/indexer/internal/processor/testdata/repair_m1000_batches.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Responses map[string]repairM1000Recorded
	}
	if err = json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(repositoryRoot(), "services/launchpad/indexer/internal/processor/testdata/repair_m1000_treasury.json"))
	if err != nil {
		t.Fatal(err)
	}
	var treasury struct {
		Responses map[string]repairM1000Recorded
	}
	if err = json.Unmarshal(raw, &treasury); err != nil {
		t.Fatal(err)
	}
	for encoded, response := range treasury.Responses {
		if prior, exists := packet.Responses[encoded]; exists && (!reflect.DeepEqual(prior.Result, response.Result) || !reflect.DeepEqual(prior.Error, response.Error)) {
			t.Fatal("archival Treasury response conflicts with committed producer:", encoded)
		}
		packet.Responses[encoded] = response
	}
	key := func(method string, params []json.RawMessage) string {
		if method == "eth_call" && len(params) == 2 {
			var call map[string]any
			if json.Unmarshal(params[0], &call) == nil && call["from"] == "0x0000000000000000000000000000000000000000" {
				delete(call, "from")
				params = append([]json.RawMessage(nil), params...)
				params[0], _ = json.Marshal(call)
			}
		}
		return chainfixture.Key(method, params)
	}
	responses := map[string]repairM1000Recorded{}
	headers := map[uint64][]common.Hash{}
	receipts := map[common.Hash]*types.Receipt{}
	for encoded, response := range packet.Responses {
		method, args, ok := strings.Cut(encoded, " ")
		var params []json.RawMessage
		if !ok || json.Unmarshal([]byte(args), &params) != nil {
			t.Fatal("invalid recorded RPC key")
		}
		canonical := key(method, params)
		if prior, exists := responses[canonical]; exists && (!reflect.DeepEqual(prior.Result, response.Result) || !reflect.DeepEqual(prior.Error, response.Error)) {
			t.Fatal("conflicting recorded RPC result:", canonical)
		}
		responses[canonical] = response
		if method == "eth_getTransactionReceipt" && len(response.Error) == 0 {
			var receipt types.Receipt
			if err = json.Unmarshal(response.Result, &receipt); err != nil {
				t.Fatal(err)
			}
			receipts[receipt.TxHash] = &receipt
		}
		if method == "eth_getBlockReceipts" && len(response.Error) == 0 {
			var blockReceipts []*types.Receipt
			if err = json.Unmarshal(response.Result, &blockReceipts); err != nil {
				t.Fatal(err)
			}
			for _, receipt := range blockReceipts {
				if receipt == nil {
					t.Fatal("null recorded block receipt")
				}
				receipts[receipt.TxHash] = receipt
			}
		}
		if method == "eth_getBlockByNumber" && len(response.Error) == 0 {
			var block struct {
				Number       string
				Transactions []json.RawMessage
			}
			if err = json.Unmarshal(response.Result, &block); err != nil {
				t.Fatal(err)
			}
			height, err := strconv.ParseUint(strings.TrimPrefix(block.Number, "0x"), 16, 64)
			if err != nil {
				t.Fatal(err)
			}
			hashes := make([]common.Hash, len(block.Transactions))
			for i, raw := range block.Transactions {
				if len(raw) > 0 && raw[0] == '"' {
					if err = json.Unmarshal(raw, &hashes[i]); err != nil {
						t.Fatal(err)
					}
				} else {
					var tx struct{ Hash common.Hash }
					if err = json.Unmarshal(raw, &tx); err != nil {
						t.Fatal(err)
					}
					hashes[i] = tx.Hash
				}
			}
			headers[height] = hashes
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if len(latest) == 1 && call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"latest"` {
			call.Params[0] = json.RawMessage(strconv.Quote(fmt.Sprintf("0x%x", latest[0])))
		}
		canonical := key(call.Method, call.Params)
		response, exists := responses[canonical]
		if !exists && call.Method == "eth_getLogs" && len(call.Params) == 1 {
			var filter struct {
				FromBlock, ToBlock string
				Address            []common.Address
				Topics             []json.RawMessage
			}
			if json.Unmarshal(call.Params[0], &filter) == nil && len(filter.Topics) == 0 {
				from, e1 := strconv.ParseUint(strings.TrimPrefix(filter.FromBlock, "0x"), 16, 64)
				to, e2 := strconv.ParseUint(strings.TrimPrefix(filter.ToBlock, "0x"), 16, 64)
				if e1 == nil && e2 == nil && to >= from && to-from < 64 {
					logs := []*types.Log{}
					complete := true
					for h := from; h <= to; h++ {
						hashes, ok := headers[h]
						if !ok {
							complete = false
							break
						}
						for _, hash := range hashes {
							receipt, ok := receipts[hash]
							if !ok {
								complete = false
								break
							}
							for _, log := range receipt.Logs {
								for _, address := range filter.Address {
									if log.Address == address {
										logs = append(logs, log)
										break
									}
								}
							}
						}
					}
					if complete {
						sort.Slice(logs, func(i, j int) bool {
							a, b := logs[i], logs[j]
							if a.BlockNumber != b.BlockNumber {
								return a.BlockNumber < b.BlockNumber
							}
							if a.TxIndex != b.TxIndex {
								return a.TxIndex < b.TxIndex
							}
							return a.Index < b.Index
						})
						response.Result, _ = json.Marshal(logs)
						exists = true
					}
				}
			}
		}
		if !exists {
			t.Logf("missing committed producer response: %s", canonical)
			http.Error(w, "unrecorded producer read: "+canonical, 502)
			return
		}
		out := map[string]any{"jsonrpc": "2.0", "id": call.ID}
		if len(response.Error) > 0 {
			out["error"] = response.Error
		} else {
			out["result"] = response.Result
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// PostgreSQL's WASI engine is the persistence peer. The production Store,
// generated queries, transaction handling and durable publisher are unchanged.
func repairM1000Database(t *testing.T) (*pgxpool.Pool, func() *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	var engine *pglite.PGlite
	var pool *pgxpool.Pool
	open := func() *pgxpool.Pool {
		t.Helper()
		var err error
		engine, err = pglite.New(pglite.Config{DataDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		pool, err = engine.Pool(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return pool
	}
	open()
	// goose uses its own short-lived connection, before the pool acquires one.
	if err := migrate.Up(ctx, engine.ConnString()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "migrations/60000.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	return pool, func() *pgxpool.Pool {
		pool.Close()
		if err := engine.Close(); err != nil {
			t.Fatal(err)
		}
		return open()
	}
}

func TestRepairM_1000_NativeFeeFlow(t *testing.T) { repairM1000NativeFeeFlow(t) }

func repairM1000NativeFeeFlow(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, reopen := repairM1000Database(t)
	inputs := repair54ReceiptInputs(t)
	inputs.Manifest = deployment.Current()
	reviewed, err := os.ReadFile(filepath.Join(repositoryRoot(), "sdk/launchpad/protocol/catalog/reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	inputs.ReviewedCatalog = reviewed
	height := inputs.Manifest.ActivationHeight + 1
	parent := &types.Header{Number: new(big.Int).SetUint64(height - 1), Time: 1760000499, GasLimit: 30000000, Difficulty: big.NewInt(0)}
	inputs.Anchor = receipt.BlockAnchor{Height: height - 1, Hash: parent.Hash()}
	options, _, err := runtimedocs.ReceiptOptions(inputs)
	if err != nil {
		t.Fatal(err)
	}
	code, err := os.ReadFile(filepath.Join(repositoryRoot(), "precompiles/launchpad/abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	native, err := abi.JSON(strings.NewReader(string(code)))
	if err != nil {
		t.Fatal(err)
	}
	event := native.Events["FeeFlow"]
	if event.Name != "FeeFlow" {
		t.Fatal("native FeeFlow ABI absent")
	}
	key, err := crypto.HexToECDSA(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	party := crypto.PubkeyToAddress(key.PublicKey)
	tx, err := types.SignNewTx(key, types.LatestSignerForChainID(big.NewInt(125)), &types.LegacyTx{To: &decode.LaunchpadPrecompileAddress, Gas: 200000, GasPrice: big.NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	logs := []*types.Log{}
	// Production ABI encoding supplies the wire bytes, including both the
	// claim kind and a non-claim kind. No decoded Event or QualifiedReceipt
	// is constructed by this test.
	for i, kind := range []uint8{4, 1} {
		values := []any{kind, common.HexToAddress(airdropPool), party, big.NewInt(1500000), big.NewInt(0), big.NewInt(0), big.NewInt(3), big.NewInt(1760000500), new(big.Int).SetUint64(height)}
		log := &types.Log{Address: decode.LaunchpadPrecompileAddress, Topics: []common.Hash{event.ID}, BlockNumber: height, TxHash: tx.Hash(), Index: uint(i)}
		data := []any{}
		for j, argument := range event.Inputs {
			if !argument.Indexed {
				data = append(data, values[j])
				continue
			}
			encoded, err := (abi.Arguments{argument}).Pack(values[j])
			if err != nil {
				t.Fatal(err)
			}
			log.Topics = append(log.Topics, common.BytesToHash(encoded))
		}
		log.Data, err = event.Inputs.NonIndexed().Pack(data...)
		if err != nil {
			t.Fatal(err)
		}
		logs = append(logs, log)
	}
	r := &types.Receipt{Type: tx.Type(), Status: types.ReceiptStatusSuccessful, CumulativeGasUsed: 100000, GasUsed: 100000, TxHash: tx.Hash(), BlockNumber: new(big.Int).SetUint64(height), Logs: logs}
	r.Bloom = types.CreateBloom(r)
	block := types.NewBlock(&types.Header{ParentHash: parent.Hash(), Number: new(big.Int).SetUint64(height), Time: 1760000500, GasLimit: 30000000, GasUsed: 100000, Difficulty: big.NewInt(0)}, &types.Body{Transactions: []*types.Transaction{tx}}, []*types.Receipt{r}, trie.NewStackTrie(nil))
	r.BlockHash = block.Hash()
	for _, log := range logs {
		log.BlockHash = block.Hash()
	}
	blockJSON := func(header *types.Header, transactions any) map[string]any {
		raw, err := json.Marshal(header)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		result["hash"], result["transactions"], result["uncles"] = header.Hash(), transactions, []any{}
		return result
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var call struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(req.Body).Decode(&call); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		var result any
		switch call.Method {
		case "eth_chainId":
			result = "0x7d"
		case "eth_getBlockByNumber", "eth_getBlockByHash":
			var selector string
			if len(call.Params) == 0 || json.Unmarshal(call.Params[0], &selector) != nil {
				http.Error(w, "missing block selector", 400)
				return
			}
			full := len(call.Params) > 1 && string(call.Params[1]) == "true"
			if selector == parent.Hash().Hex() || selector == fmt.Sprintf("0x%x", height-1) {
				result = blockJSON(parent, []any{})
			} else if selector == block.Hash().Hex() || selector == fmt.Sprintf("0x%x", height) {
				if full {
					result = blockJSON(block.Header(), []*types.Transaction{tx})
				} else {
					result = blockJSON(block.Header(), []common.Hash{tx.Hash()})
				}
			} else {
				http.Error(w, "unrecorded block selector", 400)
				return
			}
		case "eth_getBlockReceipts":
			result = []*types.Receipt{r}
		case "eth_getTransactionReceipt":
			if len(call.Params) != 1 || string(call.Params[0]) != `"`+tx.Hash().Hex()+`"` {
				http.Error(w, "unrecorded receipt", 400)
				return
			}
			result = r
		default:
			http.Error(w, "unrecorded method "+call.Method, 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result})
	}))
	defer server.Close()
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	authority, err := receipt.NewRPCQualification(ctx, client, options)
	if err != nil {
		t.Fatal(err)
	}
	batch, qualified, err := authority.FetchQualifiedBatch(ctx, height, height)
	if err != nil || len(qualified) != 1 {
		t.Fatalf("native receipt qualification: %v (%d)", err, len(qualified))
	}
	batch.StreamID, batch.Owner, batch.BatchID = "native-m1000", "repair-m1000", "native-feeflow"
	batch.PreviousHeight = int64(height) - 1
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,$2,$2)`, batch.StreamID, batch.PreviousHeight); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	batch.Fence, err = st.AcquireLease(ctx, 125, batch.StreamID, batch.Owner, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	project := func(c context.Context, canonical store.CanonicalTx) ([]store.OutboxIntent, error) {
		if err := receipt.ValidateBindings(c, batch, qualified); err != nil {
			return nil, err
		}
		p := New(Deps{Store: canonical.Store, ChainID: 125, EventEmitterAddress: inputs.Manifest.Roles["EventEmitter"].Address})
		return nil, p.projectQualifiedLegacy(c, canonical.Store, batch, qualified, nil)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION indexer.reject_m1000() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'm1000 rollback'; END $$; CREATE TRIGGER reject_m1000 BEFORE INSERT ON indexer.indexer_ingest_commits FOR EACH ROW EXECUTE FUNCTION indexer.reject_m1000()`); err != nil {
		t.Fatal(err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, project); err == nil || !strings.Contains(err.Error(), "m1000 rollback") {
		t.Fatalf("native rollback: %v", err)
	}
	if got := airdropRows(t, pool); len(got) != 0 {
		t.Fatalf("claim escaped rollback: %+v", got)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER reject_m1000 ON indexer.indexer_ingest_commits; DROP FUNCTION indexer.reject_m1000()`); err != nil {
		t.Fatal(err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, project); err != nil {
		t.Fatal(err)
	}
	want := airdropRows(t, pool)
	if len(want) != 1 || want[0].eventName != "FeeFlow" || want[0].amount != "1500000" || want[0].epoch == nil || *want[0].epoch != "3" || want[0].holder != strings.ToLower(party.Hex()) || want[0].txHash != strings.ToLower(tx.Hash().Hex()) || want[0].logIndex != 0 || want[0].poolAddress == nil || *want[0].poolAddress != airdropPool {
		t.Fatalf("native SQL claim: %+v", want)
	}
	pool = reopen()
	st = store.New(pool)
	if got := airdropRows(t, pool); !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered claims %+v != %+v", got, want)
	}
	if committed, err := st.ReconcileCanonicalCommit(ctx, batch); err != nil || !committed {
		t.Fatalf("replay identity: %v %v", committed, err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, project); !errors.Is(err, store.ErrWatermark) {
		t.Fatalf("native replay: %v", err)
	}
	if got := airdropRows(t, pool); !reflect.DeepEqual(got, want) {
		t.Fatalf("replay changed claims %+v", got)
	}
}

func TestRepairM_1000_TreasuryCodeEpoch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	options := repairM1000ReceiptOptions(t, repair54ReceiptInputs(t))
	url := repairM1000RecordedRPC(t)
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, url, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	authority, err := receipt.NewRPCQualification(ctx, client, options)
	if err != nil {
		t.Fatal(err)
	}
	treasury := common.HexToAddress("0xBb82d74E071e03d8E68F1d78Fae9EFe8b6b7A723")
	for _, span := range [][2]uint64{{33963716, 33963716}, {33963717, 33963733}} {
		batch, qualified, err := authority.FetchQualifiedBatch(ctx, span[0], span[1])
		if err != nil {
			t.Fatalf("recorded Treasury code epoch at %d..%d: %v", span[0], span[1], err)
		}
		if err = receipt.ValidateBindings(ctx, batch, qualified); err != nil {
			t.Fatal(err)
		}
		if len(batch.Blocks) != int(span[1]-span[0]+1) || len(qualified) != 1 {
			t.Fatalf("incomplete recorded batch %d..%d: blocks=%d receipts=%d", span[0], span[1], len(batch.Blocks), len(qualified))
		}
		var treasuryLogs int
		for _, q := range qualified {
			logs, err := q.SourceLogs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, log := range logs {
				if log.Address == treasury {
					if log.BlockNumber != span[1] {
						t.Fatalf("Treasury log at %d, want %d", log.BlockNumber, span[1])
					}
					if _, err := q.SourceProvenance(ctx, treasury); err != nil {
						t.Fatal("Treasury log lacks qualified code provenance:", err)
					}
					treasuryLogs++
				}
			}
		}
		if treasuryLogs == 0 {
			t.Fatalf("recorded batch %d..%d omitted Treasury source logs", span[0], span[1])
		}
	}
}

func TestRepairM_1000_LiveGeneration(t *testing.T) {
	current := deployment.Current()
	ranges, err := liveGenerationRanges(realAnchor.Height+1, current.ActivationHeight+1)
	if err != nil || len(ranges) != 2 {
		t.Fatalf("approved ranges %+v: %v", ranges, err)
	}
	if ranges[0].From != realAnchor.Height+1 || ranges[0].To != current.ActivationHeight-1 || ranges[1].From != current.ActivationHeight || ranges[1].To != current.ActivationHeight+1 || ranges[1].Manifest.DeploymentVersion != current.DeploymentVersion {
		t.Fatalf("activation split %+v", ranges)
	}
	if _, err = liveGenerationRanges(0, 1); err == nil {
		t.Fatal("unanchored range accepted")
	}
	manifest, err := liveManifestAt(realAnchor.Height)
	if err != nil || manifest.ActivationHeight != ranges[0].Manifest.ActivationHeight {
		t.Fatalf("historical anchor generation %+v: %v", manifest, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, _ := repairM1000Database(t)
	applyProductionSchema(t, ctx, pool)
	t.Setenv(chainfixture.RecordEnv, "")
	url := repairM1000RecordedRPC(t)
	l := newLiveIngestor(t, ctx, pool, url, url, realAnchor)
	// Replay the original capture boundaries: creation first, then the buy.
	// The authority retains the creation metadata across the canonical commits.
	_, _ = l.ingest(t, ctx, 33963716, 33963716)
	_, _ = l.ingest(t, ctx, 33963717, 33963733)
	var creations, fills int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_canonical_creations),(SELECT count(*) FROM indexer.indexer_accounting_fills)`).Scan(&creations, &fills); err != nil || creations != 1 || fills != 1 {
		t.Fatalf("historical real source SQL creations=%d fills=%d: %v", creations, fills, err)
	}
	if watermark, err := l.store.GetCanonicalWatermark(ctx, 125, realStream); err != nil || watermark != 33963733 {
		t.Fatalf("historical watermark %d: %v", watermark, err)
	}
}

func TestRepairM_1000_InventoryDurableFrames(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t.Setenv(chainfixture.RecordEnv, "")
	pool, reopen := repairM1000Database(t)
	h := newInventoryHarnessOnPool(t, ctx, 0, pool)
	h.batch(t, ctx, 33963716, 33963716)
	baseline := h.issueBaseline(t, ctx, realMarketTrader, realMarketToken)
	h.batch(t, ctx, 33963717, 33963733)
	frames := h.frames(t, ctx, accounting.InventorySubject)
	if len(frames) != 1 {
		t.Fatalf("inventory frames=%d", len(frames))
	}
	// Decode the actual producer frame, rather than reconstructing a publication.
	policy := delivery.Policy{Subjects: map[string]bool{accounting.InventorySubject: true}, EventTypes: map[string]bool{accounting.InventorySubject: true}}
	envelope, err := delivery.Decode(frames[0].wire, policy, frames[0].subject)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := accounting.DecodePublicationV1(envelope.Payload)
	if err != nil || publication.Facet != "inventory" || publication.Enrichment == nil || publication.Positions[0].PartitionKey != baseline.att.Partition {
		t.Fatalf("real inventory frame: %+v %v", publication, err)
	}
	if err = h.proc.deps.Store.ReleaseLease(ctx, 125, realStream, inventoryTestOwner, h.proc.canonicalFence); err != nil {
		t.Fatal(err)
	}
	pool = reopen()
	h.pool = pool
	if recovered := h.frames(t, ctx, accounting.InventorySubject); !reflect.DeepEqual(recovered, frames) {
		t.Fatal("outbox frame changed after database restart")
	}
	st := store.New(pool)
	fence, err := st.AcquireLease(ctx, 125, realStream, "m1000-restarted", 30*time.Second)
	if err != nil || fence <= baseline.att.SourceEpoch {
		t.Fatalf("source fence %d: %v", fence, err)
	}
	if err = st.RenewLease(ctx, 125, realStream, inventoryTestOwner, baseline.att.SourceEpoch, 30*time.Second); err == nil {
		t.Fatal("stale source fence renewed")
	}
	if recovered := h.frames(t, ctx, accounting.InventorySubject); !reflect.DeepEqual(recovered, frames) {
		t.Fatal("source fence change rewrote in-flight frame")
	}
}

func repairM1000MeasurementSpec(t *testing.T) IngestMeasurementSpec {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate.go")
	_, ownFile, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(ownFile), "measurement.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	digest := delivery.Hash(body)
	topology, err := json.Marshal(IngestTopologyEvidence{Region: "local-fixture", Database: "embedded-postgres", ApprovedAuthorityID: "committed-test-topology", CapturedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Zones: []string{"a", "b", "c"}, NodeIDs: []string{"n1", "n2", "n3"}})
	if err != nil {
		t.Fatal(err)
	}
	topologyPath := filepath.Join(t.TempDir(), "topology.json")
	if err = os.WriteFile(topologyPath, topology, 0600); err != nil {
		t.Fatal(err)
	}
	return IngestMeasurementSpec{CaptureMaxBytes: 16 << 20, RunID: "missing-real-providers", CandidateSHA256: delivery.Hash([]byte(fmt.Sprintf("%d:%s:%s\n", len(path), path, digest))), Profile: "steady", Region: "local-fixture", SourceFiles: map[string]string{path: digest}, Duration: 5 * time.Minute, BatchBlocks: 1, MaxMemoryBytes: 1 << 30, MaxDatabaseConnections: 1, DeliverySchema: "indexer", HotPool: airdropPool, OutputPath: filepath.Join(t.TempDir(), "capture.ndjson"), TopologyPath: topologyPath, TopologySHA256: delivery.Hash(topology)}
}

func TestRepairM_1000_MeasurementRefusals(t *testing.T) { repairM1000MeasurementRefusals(t) }

func repairM1000MeasurementRefusals(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	spec := repairM1000MeasurementSpec(t)
	if err := validateMeasurementSource(spec); err != nil {
		t.Fatal("valid pinned source:", err)
	}
	pool, _ := repairM1000Database(t)
	guard, err := queue.NewEpochGuard("recovery", 1, "railway-prod")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(chainfixture.RecordEnv, "")
	url := repairM1000RecordedRPC(t)
	options := repairM1000ReceiptOptions(t, repair54ReceiptInputs(t))
	committed, err := runtimedocs.CommittedOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	committed.Endpoints[runtimedocs.PrimaryEndpointID] = url
	rpcSource, err := source.NewCommittedSource(ctx, committed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rpcSource.Close() })
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, url, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeClient)
	authority, err := receipt.NewRPCQualification(ctx, client, options)
	if err != nil {
		t.Fatal(err)
	}
	live := &source.LiveQualifiedSource{RPC: rpcSource, Authority: authority, DurableHeight: func() uint64 { return realAnchor.Height }, PriorHash: func(c context.Context, h uint64) (common.Hash, error) {
		if h == realAnchor.Height {
			return realAnchor.Hash, c.Err()
		}
		var hash string
		err := pool.QueryRow(c, `SELECT block_hash FROM indexer.indexer_block_completeness WHERE chain_id=125 AND stream_id=$1 AND height=$2`, realStream, int64(h)).Scan(&hash)
		return common.HexToHash(hash), err
	}}
	trace := new(IngestSQLMeasurementTracer)
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = trace
	// Provider admission must not open a connection. This pool uses the same
	// real SQL transport, with its tracer configured before construction.
	observed, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(observed.Close)
	makeHarness := func() *IngestMeasurementHarness {
		p := New(Deps{Store: st, Head: rpcSource, QualifiedBatchSource: live, CanonicalStreamID: realStream, CanonicalOwner: inventoryTestOwner})
		return &IngestMeasurementHarness{SQLTrace: trace, Pool: observed, Processor: p, Store: st}
	}
	if err := validateMeasurementProviders(makeHarness()); err != nil {
		t.Fatal("complete real provider wiring:", err)
	}
	cases := []struct {
		name, want string
		omit       func(*IngestMeasurementHarness)
	}{
		{"tracer", "measurement SQL tracer required", func(h *IngestMeasurementHarness) { h.SQLTrace = nil }},
		{"pool", "measurement database pool required", func(h *IngestMeasurementHarness) { h.Pool = nil }},
		{"tracer-binding", "measurement database tracer binding required", func(h *IngestMeasurementHarness) { h.SQLTrace = new(IngestSQLMeasurementTracer) }},
		{"processor", "measurement processor required", func(h *IngestMeasurementHarness) { h.Processor = nil }},
		{"readiness", "measurement processor readiness fence required", func(h *IngestMeasurementHarness) { h.Processor.readyFence = nil }},
		{"store", "measurement store required", func(h *IngestMeasurementHarness) { h.Store = nil }},
		{"guard", "measurement guarded store required", func(h *IngestMeasurementHarness) { h.Store = store.New(pool) }},
		{"qualified-source", "measurement qualified source required", func(h *IngestMeasurementHarness) { h.Processor.deps.QualifiedBatchSource = nil }},
		{"committed-head", "measurement committed head required", func(h *IngestMeasurementHarness) { h.Processor.deps.Head = nil }},
		{"store-binding", "measurement processor store binding required", func(h *IngestMeasurementHarness) { h.Processor.deps.Store = store.New(pool) }},
		{"stream", "measurement canonical stream required", func(h *IngestMeasurementHarness) { h.Processor.deps.CanonicalStreamID = "" }},
		{"owner", "measurement canonical owner required", func(h *IngestMeasurementHarness) { h.Processor.deps.CanonicalOwner = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := makeHarness()
			tc.omit(h)
			report, err := h.Run(ctx, spec)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("exact provider refusal: %v, want %q", err, tc.want)
			}
			if !report.StartedAt.IsZero() || report.CaptureSHA256 != "" || report.Transactions != 0 {
				t.Fatalf("refused provider generated evidence: %+v", report)
			}
			if _, err = os.Stat(spec.OutputPath); !os.IsNotExist(err) {
				t.Fatalf("refusal wrote capture: %v", err)
			}
		})
	}
	invalidPool := spec
	invalidPool.HotPool = "unknown"
	if report, err := makeHarness().Run(ctx, invalidPool); err == nil || !strings.Contains(err.Error(), "hot pool") || report.CaptureSHA256 != "" {
		t.Fatalf("unbound hot pool admitted: %+v %v", report, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	report, err := makeHarness().Run(cancelled, spec)
	if err == nil || report.CaptureSHA256 == "" || len(report.Violations) == 0 || report.FinishedAt.IsZero() {
		t.Fatalf("cancelled storage measurement lacked a captured result: %+v %v", report, err)
	}
	if body, readErr := os.ReadFile(spec.OutputPath); readErr != nil || !strings.Contains(string(body), `"kind":"report"`) {
		t.Fatalf("captured report unavailable: %v", readErr)
	}
}

func TestRepairM_1000_ChangedCandidate(t *testing.T) { repairM1000ChangedCandidate(t) }
func repairM1000ChangedCandidate(t *testing.T) {
	spec := repairM1000MeasurementSpec(t)
	if err := validateMeasurementSource(spec); err != nil {
		t.Fatal(err)
	}
	for path := range spec.SourceFiles {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString("\n")
		if err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateMeasurementSource(spec); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed candidate refusal: %v", err)
	}
	if _, err := os.Stat(spec.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("changed candidate wrote evidence: %v", err)
	}
}
