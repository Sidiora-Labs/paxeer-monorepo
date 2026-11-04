package processor

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Native ABI codec -> approved registry -> transaction-bound read model ->
// typed SQL and HTTP consumers. Receipt authority admission of native Swaps is
// the separate repair 22.31.53; this test does not fabricate QualifiedReceipts.
func repair54NativeProjectionFixture(t *testing.T) (store.CanonicalBatch, []*decode.DecodedEnvelope, int64, common.Address, common.Address, common.Hash) {
	t.Helper()
	manifest := deployment.Current()
	reviewed, err := os.ReadFile(filepath.Join(repositoryRoot(), "sdk/launchpad/protocol/catalog/reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := runtimedocs.TrimCatalog(reviewed, manifest)
	if err != nil {
		t.Fatal(err)
	}
	deployments := []decode.Deployment{
		{Address: decode.LaunchpadPrecompileAddress, Role: decode.LaunchpadPrecompileRole, ABIRevision: catalog.Revisions["EventEmitter"], From: decode.Order{Block: manifest.ActivationHeight}},
		{Address: decode.LaunchpadPrecompileAddress, Role: "SidioraPool", ABIRevision: catalog.Revisions["SidioraPool"], From: decode.Order{Block: manifest.ActivationHeight}},
	}
	registry, err := decode.NewRegistry(catalog.JSON, deployments)
	if err != nil {
		t.Fatal(err)
	}
	rawABI, err := os.ReadFile(filepath.Join(repositoryRoot(), "precompiles/launchpad/abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	nativeABI, err := abi.JSON(strings.NewReader(string(rawABI)))
	if err != nil {
		t.Fatal(err)
	}
	poolAddress := common.HexToAddress(realMarketPool)
	token := common.HexToAddress(realMarketToken)
	caller := common.HexToAddress("0x00000000000000000000000000000000000000c1")
	poolID := common.HexToHash(poolIDFor(poolAddress))
	height := int64(manifest.ActivationHeight + 1)
	timestamp := int64(1700000000)
	batch := store.CanonicalBatch{ChainID: 125, StreamID: "native-repair54", Owner: "repair54", BatchID: "native-family", PreviousHeight: height - 1}
	blockHash := common.HexToHash("0x5401")
	txHash := common.HexToHash("0x5402")
	envelopes := []*decode.DecodedEnvelope{}
	encode := func(name string, values ...any) {
		t.Helper()
		event := nativeABI.Events[name]
		log := ethtypes.Log{Address: decode.LaunchpadPrecompileAddress, Topics: []common.Hash{event.ID}, BlockHash: blockHash, TxHash: txHash, BlockNumber: uint64(height), Index: uint(len(batch.Records))}
		data := []any{}
		for i, input := range event.Inputs {
			if !input.Indexed {
				data = append(data, values[i])
				continue
			}
			packed, err := (abi.Arguments{input}).Pack(values[i])
			if err != nil {
				t.Fatal(err)
			}
			log.Topics = append(log.Topics, common.BytesToHash(packed))
		}
		log.Data, err = event.Inputs.NonIndexed().Pack(data...)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := registry.Decode(log)
		if err != nil || envelope.Event == nil || envelope.Status != "decoded" {
			t.Fatalf("native %s: %+v %v", name, envelope, err)
		}
		envelopes = append(envelopes, envelope)
		raw := store.RawRecord{TransactionHash: strings.ToLower(txHash.Hex()), BlockHash: strings.ToLower(blockHash.Hex()), Emitter: strings.ToLower(log.Address.Hex()), Height: height, LogIndex: uint32(log.Index), Data: log.Data}
		for _, topic := range log.Topics {
			raw.Topics = append(raw.Topics, strings.ToLower(topic.Hex()))
		}
		batch.Records = append(batch.Records, raw)
	}
	n := big.NewInt
	encode("MarketCreated", poolID, token, caller, poolAddress, common.Address{}, n(timestamp), n(height))
	encode("Swap", poolID, caller, true, n(100), n(90), n(7), n(9), n(timestamp), n(height))
	encode("PoolStateUpdated", poolID, n(11), n(12), n(13), n(14), n(timestamp), n(height))
	encode("FeeDistributed", poolID, n(42), uint8(0), n(7), caller, n(timestamp), n(height))
	encode("FeeFlow", uint8(4), poolAddress, caller, n(7), n(42), n(1), n(0), n(timestamp), n(height))
	// A familiar signature at an unattested address never becomes a family.
	foreign := ethtypes.Log{Address: caller, Topics: []common.Hash{nativeABI.Events["Swap"].ID}, BlockHash: blockHash, TxHash: txHash, BlockNumber: uint64(height)}
	rejected, rejectErr := registry.Decode(foreign)
	if rejectErr == nil || rejected.Event != nil || rejected.FailureReason != "untrusted_emitter" {
		t.Fatalf("unattested native-shaped log accepted: %+v %v", rejected, rejectErr)
	}
	batch.Blocks = []store.BlockCompleteness{{Height: height, Hash: strings.ToLower(blockHash.Hex()), RawCount: len(batch.Records), FetchComplete: true, RequiredDecodeComplete: true}}
	return batch, envelopes, timestamp, poolAddress, token, txHash
}

func TestRepair_54_NativeProjectionIdentityGuards(t *testing.T) {
	batch, envelopes, timestamp, _, _, txHash := repair54NativeProjectionFixture(t)
	for _, envelope := range envelopes {
		if !nativeLegacyRole(envelope) {
			t.Fatalf("attested native %s role %s rejected", envelope.Event.EventName, envelope.EmitterRole)
		}
	}
	p := New(Deps{})
	timestamps := map[string]int64{strings.ToLower(txHash.Hex()): timestamp}
	duplicates := append(append([]*decode.DecodedEnvelope(nil), envelopes...), envelopes[0])
	if err := p.projectLegacyEnvelopes(context.Background(), batch, duplicates, timestamps, nil, nil); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("duplicate qualified identity: %v", err)
	}
	if err := p.projectLegacyEnvelopes(context.Background(), batch, envelopes, nil, nil, nil); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("missing qualified timestamp: %v", err)
	}
	batch.Records[0].Height++
	if err := p.projectLegacyEnvelopes(context.Background(), batch, envelopes, timestamps, nil, nil); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed qualified position: %v", err)
	}
}

func TestRepair54Postgres_NativeReadModelProjection(t *testing.T) {
	if os.Getenv("L06_POSTGRES_DSN") == "" {
		t.Skip("PostgreSQL integration coverage requires L06_POSTGRES_DSN; the in-memory repair gate runs independently")
	}
	ctx := context.Background()
	batch, envelopes, timestamp, poolAddress, token, txHash := repair54NativeProjectionFixture(t)
	height := batch.Blocks[0].Height
	var err error
	_, pool := canonicaltest.New(t)
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,$2,$2)`, batch.StreamID, height-1); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	batch.Fence, err = st.AcquireLease(ctx, 125, batch.StreamID, batch.Owner, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	project := func(c context.Context, canonical store.CanonicalTx) ([]store.OutboxIntent, error) {
		p := New(Deps{Store: canonical.Store, ChainID: 125, EventEmitterAddress: deployment.Current().Roles["EventEmitter"].Address})
		// The immutable factory proof normally seeds this cache in projectQualifiedLegacy.
		id := int64(42)
		p.nftCache[strings.ToLower(poolAddress.Hex())] = &id
		err := p.projectLegacyEnvelopes(c, batch, envelopes, map[string]int64{strings.ToLower(txHash.Hex()): timestamp}, nil, nil)
		return nil, err
	}
	// Fail at the final canonical write, after the real read-model handlers ran.
	if _, err = pool.Exec(ctx, `CREATE FUNCTION indexer.repair54_reject() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'repair54 atomic rollback'; END $$; CREATE TRIGGER repair54_reject BEFORE INSERT ON indexer.indexer_ingest_commits FOR EACH ROW EXECUTE FUNCTION indexer.repair54_reject()`); err != nil {
		t.Fatal(err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, project); err == nil || !strings.Contains(err.Error(), "repair54 atomic rollback") {
		t.Fatalf("rollback boundary: %v", err)
	}
	queries := sqlcdb.New(pool)
	if count, err := queries.GetPoolCount(ctx); err != nil || count != 0 {
		t.Fatalf("rolled-back pools=%d %v", count, err)
	}
	if count, err := queries.GetSwapCount(ctx); err != nil || count != 0 {
		t.Fatalf("rolled-back swaps=%d %v", count, err)
	}
	var escaped int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.pool_state_snapshots)+(SELECT count(*) FROM indexer.fee_distributions)+(SELECT count(*) FROM indexer.airdrop_claims)+(SELECT count(*) FROM indexer.indexer_raw_logs)+(SELECT count(*) FROM indexer.indexer_block_completeness)+(SELECT count(*) FROM indexer.indexer_ingest_commits)`).Scan(&escaped); err != nil || escaped != 0 {
		t.Fatalf("rolled-back effects=%d %v", escaped, err)
	}
	if watermark, err := st.GetCanonicalWatermark(ctx, 125, batch.StreamID); err != nil || watermark != height-1 {
		t.Fatalf("rollback watermark=%d %v", watermark, err)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER repair54_reject ON indexer.indexer_ingest_commits; DROP FUNCTION indexer.repair54_reject()`); err != nil {
		t.Fatal(err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, project); err != nil {
		t.Fatal(err)
	}
	market, err := queries.GetPoolByAddress(ctx, strings.ToLower(poolAddress.Hex()))
	if err != nil || market.PoolID != poolIDFor(poolAddress) || market.TokenAddress != strings.ToLower(token.Hex()) || market.NftID == nil || *market.NftID != 42 {
		t.Fatalf("typed pool=%+v %v", market, err)
	}
	var amount, virtual, fee, claim, eventID string
	if err = pool.QueryRow(ctx, `SELECT (SELECT amount_in FROM indexer.swaps), (SELECT virtual_reserve FROM indexer.pool_state_snapshots), (SELECT amount FROM indexer.fee_distributions), (SELECT amount FROM indexer.airdrop_claims), (SELECT id FROM indexer.swaps)`).Scan(&amount, &virtual, &fee, &claim, &eventID); err != nil {
		t.Fatal(err)
	}
	if amount != "100" || virtual != "11" || fee != "7" || claim != "7" || eventID != strings.ToLower(txHash.Hex())+"-1" {
		t.Fatalf("native rows: %s %s %s %s %s", amount, virtual, fee, claim, eventID)
	}
	router := chi.NewRouter()
	httpapi.RegisterStatus(router, httpapi.StatusDeps{Store: st, ChainID: 125, StartTime: time.Now()})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	var status struct {
		Pools int `json:"totalPoolsIndexed"`
		Swaps int `json:"totalSwapsIndexed"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &status); err != nil || response.Code != 200 || status.Pools != 1 || status.Swaps != 1 {
		t.Fatalf("HTTP consumer: %s %v", response.Body.String(), err)
	}
	if watermark, err := st.GetCanonicalWatermark(ctx, 125, batch.StreamID); err != nil || watermark != height {
		t.Fatalf("committed watermark=%d %v", watermark, err)
	}
	if committed, err := st.ReconcileCanonicalCommit(ctx, batch); err != nil || !committed {
		t.Fatalf("immutable replay: %v %v", committed, err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, project); !errors.Is(err, store.ErrWatermark) {
		t.Fatalf("duplicate batch: %v", err)
	}
	if count, err := queries.GetSwapCount(ctx); err != nil || count != 1 {
		t.Fatalf("duplicate swaps=%d %v", count, err)
	}
}

// Replay an actual recorded block as the final head of this bounded recording.
// All bytes, including the node-reported block hash, remain unchanged.
func repair54RecordedResult(t *testing.T, key string) json.RawMessage {
	t.Helper()
	file, err := os.Open(filepath.Join(repositoryRoot(), "services/launchpad/indexer/internal/internaltest/chainfixture/testdata/paxeer125-market.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<20), 64<<20)
	for scanner.Scan() {
		var entry struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Key == key {
			return entry.Result
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("recorded RPC result %s missing", key)
	return nil
}

func repair54ReceiptInputs(t *testing.T) runtimedocs.Inputs {
	t.Helper()
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("historical deployment is not approved")
	}
	reviewed, err := os.ReadFile(filepath.Join(repositoryRoot(), "sdk/launchpad/protocol/catalog/reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Use the actual recorded token runtime and the production immutable mask,
	// avoiding a dependency on a separately built Solidity artifact.
	raw := repair54RecordedResult(t, `eth_getCode ["0x0f92fdd087c96dd3413ef98646b94d0d986192fa",{"blockHash":"0x81a18d822e327fbcbfe2be91b7f666bfa043016cb0e3f094c5bbabe91f9d192b"}]`)
	var encoded string
	if err = json.Unmarshal(raw, &encoded); err != nil {
		t.Fatal(err)
	}
	code, err := hex.DecodeString(strings.TrimPrefix(encoded, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	return runtimedocs.Inputs{Manifest: manifest, ReviewedCatalog: reviewed, ERC20RuntimeCode: code, PoolRuntimeHash: common.HexToHash("0xa9b304d6d0aa562f283b45978b84ced8840a9331f8ac946742670b1b02a2872d"), Anchor: realAnchor}
}

// This proves the recorded source and receipt path independently of the SQL
// qualification below. It does not substitute for canonical commit recovery.
func TestRepair_54_RecordedLiveQualification(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Setenv(chainfixture.RecordEnv, "")
	fixture := chainfixture.New(t)
	url := fixture.Serve(t)
	options, _, err := runtimedocs.ReceiptOptions(repair54ReceiptInputs(t))
	if err != nil {
		t.Fatal(err)
	}
	open := func() (*source.CommittedSource, *receipt.RPCQualification, func()) {
		t.Helper()
		client, closeClient, err := receipt.NewBoundedRPCClient(ctx, url, "")
		if err != nil {
			t.Fatal(err)
		}
		authority, err := receipt.NewRPCQualification(ctx, client, options)
		if err != nil {
			closeClient()
			t.Fatal(err)
		}
		committed, err := runtimedocs.CommittedOptions(options)
		if err != nil {
			closeClient()
			t.Fatal(err)
		}
		committed.Endpoints[runtimedocs.PrimaryEndpointID] = url
		rpcSource, err := source.NewCommittedSource(ctx, committed)
		if err != nil {
			closeClient()
			t.Fatal(err)
		}
		return rpcSource, authority, func() { _ = rpcSource.Close(); closeClient() }
	}
	rpcSource, authority, closeSource := open()
	checkpoint := realAnchor.Height
	priorHash := realAnchor.Hash
	live := &source.LiveQualifiedSource{RPC: rpcSource, Authority: authority,
		DurableHeight: func() uint64 { return checkpoint },
		PriorHash: func(_ context.Context, height uint64) (common.Hash, error) {
			if height != checkpoint {
				return common.Hash{}, store.ErrWatermark
			}
			return priorHash, nil
		},
	}
	creation, qualified, err := live.FetchQualifiedBatch(ctx, 33963716, 33963716)
	if err != nil {
		closeSource()
		t.Fatal(err)
	}
	if err := receipt.ValidateBindings(ctx, creation, qualified); err != nil {
		closeSource()
		t.Fatal(err)
	}
	facts, err := qualifiedCreationFacts(ctx, qualified)
	if err != nil || len(facts) != 1 || facts[0].Token != realMarketToken || facts[0].Pool != realMarketPool || facts[0].NFTID != "1" {
		closeSource()
		t.Fatalf("recorded factory facts=%+v: %v", facts, err)
	}
	checkpoint = uint64(creation.Blocks[len(creation.Blocks)-1].Height)
	priorHash = common.HexToHash(creation.Blocks[len(creation.Blocks)-1].Hash)
	closeSource()
	rpcSource, authority, closeSource = open()
	defer closeSource()
	transactionIndex, err := strconv.ParseUint(facts[0].Position.TransactionIndex, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.Rehydrate(ctx, []receipt.DiscoveredMarket{{Token: common.HexToAddress(facts[0].Token), Pool: common.HexToAddress(facts[0].Pool), Height: checkpoint, TransactionIndex: uint(transactionIndex)}}); err != nil {
		t.Fatal(err)
	}
	// Restore epoch-scoped metadata from its original recorded block.
	if _, _, err := authority.FetchQualifiedBatch(ctx, checkpoint, checkpoint); err != nil {
		t.Fatal(err)
	}
	live.RPC, live.Authority = rpcSource, authority
	fillIDs := make(map[string]bool)
	for _, end := range []uint64{33963733, 33963749} {
		batch, receipts, err := live.FetchQualifiedBatch(ctx, checkpoint+1, end)
		if err != nil {
			t.Fatal(err)
		}
		if err := receipt.ValidateBindings(ctx, batch, receipts); err != nil {
			t.Fatal(err)
		}
		fillsInBatch := 0
		for _, q := range receipts {
			fills, err := AccountQualified(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			for _, fill := range fills {
				if fill.PoolAddress != realMarketPool || fill.Token.Address != realMarketToken || fillIDs[fill.SourceFillID] {
					t.Fatalf("unexpected or duplicate recorded fill: %+v", fill)
				}
				fillIDs[fill.SourceFillID] = true
				fillsInBatch++
			}
		}
		if fillsInBatch != 1 {
			t.Fatalf("recorded batch ending %d has %d fills, want 1", end, fillsInBatch)
		}
		checkpoint = end
		priorHash = common.HexToHash(batch.Blocks[len(batch.Blocks)-1].Hash)
	}
	if len(fillIDs) != 2 {
		t.Fatalf("recorded fills=%d, want 2", len(fillIDs))
	}
}

func TestRepair_54_PendingBatchRetainedOnReconciliationCancellation(t *testing.T) {
	batch := fixtureBatch(t)
	batch.Fence = 1
	batch.Blocks = append(batch.Blocks, store.BlockCompleteness{Height: 102,
		Hash: common.HexToHash("0x5403").Hex(), FetchComplete: true, RequiredDecodeComplete: true})
	before, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	// An already-cancelled context exercises the real pgx/store failure path
	// without opening a database connection or replacing reconciliation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool, err := pgxpool.New(ctx, "postgres://unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	p := New(Deps{Store: store.New(pool), ChainID: 125,
		CanonicalStreamID: batch.StreamID, CanonicalOwner: batch.Owner,
		QualifiedBatchSource: &source.LiveQualifiedSource{},
	})
	p.pendingCanonical = &batch
	for _, end := range []int64{102, 101} {
		// Both an identical retry and a smaller range must reconcile the
		// original before the live source can reject a stale checkpoint.
		if err := p.ProcessBatch(ctx, 101, end); !errors.Is(err, context.Canceled) {
			t.Fatalf("retry end=%d: want reconciliation cancellation, got %v", end, err)
		}
		if p.pendingCanonical != &batch {
			t.Fatal("unresolved original batch was discarded or replaced")
		}
		after, err := json.Marshal(p.pendingCanonical)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("unresolved batch identity or contents changed: %s: %v", after, err)
		}
	}
	if acquired := pool.Stat().AcquireCount(); acquired != 0 {
		t.Fatalf("cancelled reconciliation acquired %d database connections", acquired)
	}
}

func repair54LiveProcessor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, url, headURL string) (*Processor, *receipt.RPCQualification, func()) {
	t.Helper()
	options, _, err := runtimedocs.ReceiptOptions(repair54ReceiptInputs(t))
	if err != nil {
		t.Fatal(err)
	}
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, url, "")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := receipt.NewRPCQualification(ctx, client, options)
	if err != nil {
		closeClient()
		t.Fatal(err)
	}
	committed, err := runtimedocs.CommittedOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	committed.Endpoints[runtimedocs.PrimaryEndpointID] = headURL
	rpcSource, err := source.NewCommittedSource(ctx, committed)
	if err != nil {
		t.Fatal(err)
	}
	closeSource := func() { _ = rpcSource.Close(); closeClient() }
	guard, err := queue.NewEpochGuard("recovery", 1, "railway-prod")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{publisher.CanonicalCreationSubject: true, accounting.MarketSubject: true, ReplaySubjectV2: true}, EventTypes: map[string]bool{publisher.CanonicalCreationEventType: true, accounting.MarketSubject: true, ReplaySubjectV2: true}}
	durable, err := delivery.NewStoreWithEpochGuard(pool, "indexer", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	stager, err := publisher.NewDurableStager(durable, options.Manifest.DeploymentVersion, map[string]string{publisher.CanonicalCreationEventType: publisher.CanonicalCreationSubject, accounting.MarketSubject: accounting.MarketSubject})
	if err != nil {
		t.Fatal(err)
	}
	live := &source.LiveQualifiedSource{RPC: rpcSource, Authority: authority, DurableHeight: func() uint64 {
		// Read the same durable checkpoint canonicalRuntimeHead loads on each poll.
		read, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		height, err := st.GetCanonicalWatermark(read, 125, realStream)
		if err != nil {
			panic(fmt.Sprintf("read durable checkpoint: %v", err))
		}
		return uint64(height)
	}, PriorHash: func(c context.Context, h uint64) (common.Hash, error) {
		if h == realAnchor.Height {
			return realAnchor.Hash, nil
		}
		var hash string
		err := pool.QueryRow(c, `SELECT block_hash FROM indexer.indexer_block_completeness WHERE chain_id=125 AND stream_id=$1 AND height=$2`, realStream, int64(h)).Scan(&hash)
		return common.HexToHash(hash), err
	}}
	p := New(Deps{Store: st, Head: rpcSource, Decoder: decode.NewDecoder(), EventEmitterAddress: deployment.Current().Roles["EventEmitter"].Address, ChainID: 125, QualifiedBatchSource: live, CanonicalStreamID: realStream, CanonicalOwner: "repair54-live", AccountingPartitions: partitions, Durable: stager, AccountingProjectionVersion: "accounting-market-v1", LegacyProjection: true, Reserves: authority, BatchSize: 17, PollInterval: 10 * time.Millisecond})
	return p, authority, closeSource
}

func TestRepair54Postgres_AmbiguousCommitLiveRecovery(t *testing.T) {
	if os.Getenv("L06_POSTGRES_DSN") == "" {
		t.Skip("PostgreSQL integration coverage requires L06_POSTGRES_DSN; the in-memory repair gate runs independently")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn, pool := canonicaltest.New(t)
	applyProductionSchema(t, ctx, pool)
	fixture := chainfixture.New(t)
	url := fixture.Serve(t)
	caughtUp := make(chan struct{}, 1)
	// Bind "latest" to the last recorded block for both the header and
	// full-transaction forms; delegate all responses to the actual recording.
	headServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read recorded head request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var call struct {
			JSONRPC string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err = json.Unmarshal(body, &call); err != nil {
			t.Errorf("decode recorded head request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"latest"` {
			call.Params[0] = json.RawMessage(`"0x2063ee5"`)
			body, err = json.Marshal(call)
			if err != nil {
				t.Errorf("bind recorded head: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			height, err := store.New(pool).GetCanonicalWatermark(ctx, 125, realStream)
			if err != nil {
				t.Errorf("observe durable head checkpoint: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if height == 33963749 {
				select {
				case caughtUp <- struct{}{}:
				default:
				}
			}
		}
		request := r.Clone(r.Context())
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		fixture.ServeHTTP(w, request)
	}))
	t.Cleanup(headServer.Close)
	headURL := headServer.URL
	seed, _, closeSeed := repair54LiveProcessor(t, ctx, pool, url, headURL)
	if err := seed.ProcessBatch(ctx, 33963716, 33963716); err != nil {
		t.Fatal(err)
	}
	if err := seed.deps.Store.ReleaseLease(ctx, 125, realStream, seed.deps.CanonicalOwner, seed.canonicalFence); err != nil {
		t.Fatal(err)
	}
	closeSeed()
	proxy, lost := canonicaltest.LostCommitResponse(t, dsn)
	uncertain, err := pgxpool.New(ctx, proxy)
	if err != nil {
		t.Fatal(err)
	}
	p, authority, closeLive := repair54LiveProcessor(t, ctx, uncertain, url, headURL)
	rehydrate := func(a *receipt.RPCQualification) {
		t.Helper()
		var token, poolAddress string
		var height int64
		var index int64
		if err := pool.QueryRow(ctx, `SELECT token,pool,height,transaction_index FROM indexer.indexer_canonical_creations`).Scan(&token, &poolAddress, &height, &index); err != nil {
			t.Fatal(err)
		}
		if err := a.Rehydrate(ctx, []receipt.DiscoveredMarket{{Token: common.HexToAddress(token), Pool: common.HexToAddress(poolAddress), Height: uint64(height), TransactionIndex: uint(index)}}); err != nil {
			t.Fatal(err)
		}
		// Qualify the genuine creation receipt at its recorded block. This
		// establishes the epoch-scoped metadata cache from recorded reads,
		// without inventing responses at later blocks absent from the recording.
		_, qualified, err := a.FetchQualifiedBatch(ctx, uint64(height), uint64(height))
		if err != nil {
			t.Fatalf("qualify recorded creation metadata: %v", err)
		}
		facts, err := qualifiedCreationFacts(ctx, qualified)
		if err != nil || len(facts) != 1 || facts[0].Token != token || facts[0].Pool != poolAddress {
			t.Fatalf("recorded creation facts=%+v: %v", facts, err)
		}
	}
	rehydrate(authority)
	runCtx, stop := context.WithCancel(ctx)
	result := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); result <- p.Run(runCtx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("processor did not stop")
		}
		closeLive()
		uncertain.Close()
	})
	select {
	case <-lost:
	case <-ctx.Done():
		t.Fatal("canonical COMMIT response was not dropped")
	}
	select {
	case <-caughtUp:
	case err := <-result:
		t.Fatalf("Run stopped before live recovery: %v", err)
	case <-ctx.Done():
		watermark, err := store.New(pool).GetCanonicalWatermark(context.Background(), 125, realStream)
		t.Fatalf("Run stalled after ambiguous commit at %d: %v", watermark, err)
	}
	stop()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run stop: %v", err)
	}
	if p.pendingCanonical != nil {
		t.Fatal("original committed batch remained pending")
	}
	closeLive()
	uncertain.Close()
	assertEffects := func(db *pgxpool.Pool) {
		t.Helper()
		var swaps, fills, marketFrames, commits int
		var originalEnd int64
		err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.swaps), (SELECT count(*) FROM indexer.indexer_accounting_fills), (SELECT count(*) FROM indexer.indexer_ingest_outbox WHERE event_kind=$1), (SELECT count(*) FROM indexer.indexer_ingest_commits), (SELECT last_complete_height FROM indexer.indexer_ingest_commits WHERE previous_height=33963716)`, accounting.MarketSubject).Scan(&swaps, &fills, &marketFrames, &commits, &originalEnd)
		if err != nil || swaps != 2 || fills != 2 || marketFrames != 2 || commits != 3 || originalEnd != 33963733 {
			t.Fatalf("exactly-once effects swaps/fills/frames/commits/end=%d/%d/%d/%d/%d %v", swaps, fills, marketFrames, commits, originalEnd, err)
		}
	}
	assertEffects(pool)
	// Stop and reopen both the PostgreSQL pool and the receipt/live components.
	reopened, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted, restartedAuthority, closeRestarted := repair54LiveProcessor(t, ctx, reopened, url, headURL)
	defer closeRestarted()
	rehydrate(restartedAuthority)
	restartCtx, stopRestart := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stopRestart()
	if err := restarted.Run(restartCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reopened Run: %v", err)
	}
	if watermark, err := restarted.deps.Store.GetCanonicalWatermark(ctx, 125, realStream); err != nil || watermark != 33963749 {
		t.Fatalf("recovered watermark=%d %v", watermark, err)
	}
	assertEffects(reopened)
}

// The persistence peer implements the processor's store contract in memory.
// Production receipt admission, projection handlers, canonical fact validation,
// partition sequencing and durable envelope encoding all run unchanged.
type repair54MemoryStore struct {
	*store.Store
	mu                   sync.Mutex
	db                   *repair54MemoryDB
	owner                string
	fence                int64
	stream               string
	lostResponse         bool
	rejectCommit         bool
	unavailableReconcile bool
	reconciled           []string
	guard                *queue.EpochGuard
}

type repair54MemoryState struct {
	height  int64
	tables  map[string]map[string][]any
	raw     map[string]store.RawRecord
	batches map[string]store.CanonicalBatch
	blocks  map[int64]string
}

type repair54MemoryDB struct{ state *repair54MemoryState }

func repair54NewMemoryStore(t *testing.T, height int64, stream string) *repair54MemoryStore {
	t.Helper()
	guard, err := queue.NewEpochGuard("recovery", 1, "railway-prod")
	if err != nil {
		t.Fatal(err)
	}
	db := &repair54MemoryDB{state: &repair54MemoryState{height: height, tables: map[string]map[string][]any{}, raw: map[string]store.RawRecord{}, batches: map[string]store.CanonicalBatch{}, blocks: map[int64]string{}}}
	return &repair54MemoryStore{Store: store.NewFromDBTX(db), db: db, stream: stream, guard: guard}
}

func (m *repair54MemoryStore) CanonicalEpochGuarded() bool { return m.guard != nil }
func (m *repair54MemoryStore) GetCanonicalWatermark(ctx context.Context, chain int32, stream string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if chain != 125 || stream != m.stream {
		return 0, store.ErrWatermark
	}
	return m.db.state.height, nil
}
func (m *repair54MemoryStore) AcquireLease(ctx context.Context, chain int32, stream, owner string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if chain != 125 || stream != m.stream || owner == "" || m.owner != "" || ttl <= 0 || ttl > 30*time.Second {
		return 0, store.ErrFenced
	}
	m.fence++
	m.owner = owner
	return m.fence, nil
}
func (m *repair54MemoryStore) RenewLease(ctx context.Context, chain int32, stream, owner string, fence int64, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if chain != 125 || stream != m.stream || owner != m.owner || fence != m.fence || ttl <= 0 || ttl > 30*time.Second {
		return store.ErrFenced
	}
	return nil
}
func (m *repair54MemoryStore) ReleaseLease(ctx context.Context, chain int32, stream, owner string, fence int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if chain != 125 || stream != m.stream || owner != m.owner || fence != m.fence {
		return store.ErrFenced
	}
	m.owner = ""
	return nil
}
func (m *repair54MemoryStore) ReconcileCanonicalCommit(ctx context.Context, b store.CanonicalBatch) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if m.unavailableReconcile {
		m.unavailableReconcile = false
		return false, io.ErrUnexpectedEOF
	}
	prior, ok := m.db.state.batches[b.BatchID]
	if !ok {
		return false, nil
	}
	// A replay may carry a new lease; the source and original range stay immutable.
	prior.Outbox = nil
	b.Outbox = nil
	prior.Owner = b.Owner
	prior.Fence = b.Fence
	if !reflect.DeepEqual(prior, b) {
		return false, store.ErrIntegrity
	}
	if m.db.state.height < b.Blocks[len(b.Blocks)-1].Height {
		return false, store.ErrIntegrity
	}
	m.reconciled = append(m.reconciled, b.BatchID)
	return true, nil
}
func (m *repair54MemoryStore) CommitProjectedCanonicalTx(ctx context.Context, b store.CanonicalBatch, project func(context.Context, store.CanonicalTx) ([]store.OutboxIntent, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.ChainID != 125 || b.StreamID != m.stream || b.Fence != m.fence || b.Owner != m.owner {
		return store.ErrFenced
	}
	if b.PreviousHeight != m.db.state.height || len(b.Blocks) == 0 {
		return store.ErrWatermark
	}
	original := m.db.state
	next := &repair54MemoryState{height: original.height, tables: map[string]map[string][]any{}, raw: map[string]store.RawRecord{}, batches: map[string]store.CanonicalBatch{}, blocks: map[int64]string{}}
	for table, rows := range original.tables {
		next.tables[table] = map[string][]any{}
		for key, row := range rows {
			next.tables[table][key] = append([]any(nil), row...)
		}
	}
	for k, v := range original.raw {
		next.raw[k] = v
	}
	for k, v := range original.batches {
		next.batches[k] = v
	}
	for k, v := range original.blocks {
		next.blocks[k] = v
	}
	counts := map[int64]int{}
	for _, raw := range b.Records {
		key := repair54Key(raw.TransactionHash, raw.LogIndex)
		if old, ok := next.raw[key]; ok && !reflect.DeepEqual(old, raw) {
			return store.ErrIntegrity
		}
		next.raw[key] = raw
		counts[raw.Height]++
	}
	for i, block := range b.Blocks {
		if block.Height != b.PreviousHeight+int64(i)+1 || !block.FetchComplete || !block.RequiredDecodeComplete || block.RawCount != counts[block.Height] {
			return store.ErrIntegrity
		}
		next.blocks[block.Height] = block.Hash
	}
	for _, raw := range b.Records {
		if next.blocks[raw.Height] != raw.BlockHash {
			return store.ErrIntegrity
		}
	}
	db := &repair54MemoryDB{state: next}
	tx := &repair54MemoryTx{db: db}
	if err := m.guard.Assert(ctx, tx); err != nil {
		return err
	}
	intents, err := project(ctx, store.CanonicalTx{Tx: tx, Store: store.NewFromDBTX(db)})
	if err != nil {
		return err
	}
	if b.Outbox != nil && !reflect.DeepEqual(b.Outbox, intents) {
		return store.ErrIntegrity
	}
	for _, intent := range intents {
		if _, ok := next.raw[repair54Key(intent.TransactionHash, intent.LogIndex)]; !ok || !json.Valid(intent.Payload) {
			return store.ErrIntegrity
		}
		if _, err := db.insert("indexer_ingest_outbox", repair54Key(intent.TransactionHash, intent.LogIndex, intent.Kind), []any{intent.TransactionHash, intent.LogIndex, intent.Kind, intent.Payload}); err != nil {
			return err
		}
	}
	if err = m.guard.Assert(ctx, tx); err != nil {
		return err
	}
	if m.rejectCommit {
		return errors.New("repair54 atomic rollback")
	}
	b.Outbox = intents
	next.batches[b.BatchID] = b
	next.height = b.Blocks[len(b.Blocks)-1].Height
	m.db.state = next
	if m.lostResponse {
		m.lostResponse = false
		return io.ErrUnexpectedEOF
	}
	return nil
}

func repair54Key(args ...any) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprint(a)
	}
	return strings.Join(parts, "\x00")
}
func (db *repair54MemoryDB) table(name string) map[string][]any {
	rows := db.state.tables[name]
	if rows == nil {
		rows = map[string][]any{}
		db.state.tables[name] = rows
	}
	return rows
}
func (db *repair54MemoryDB) insert(table, key string, args []any) (pgconn.CommandTag, error) {
	rows := db.table(table)
	if prior, ok := rows[key]; ok {
		if !reflect.DeepEqual(prior, args) {
			return pgconn.CommandTag{}, store.ErrIntegrity
		}
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	rows[key] = append([]any(nil), args...)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

var repair54InsertTable = regexp.MustCompile(`INSERT INTO (?:"?indexer"?\.)?(\w+)`)

func (db *repair54MemoryDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := ctx.Err(); err != nil {
		return pgconn.CommandTag{}, err
	}
	if strings.Contains(sql, `"recovery".assert_epoch`) {
		if len(args) != 2 || args[0] != int64(1) || args[1] != "railway-prod" {
			return pgconn.CommandTag{}, store.ErrFenced
		}
		return pgconn.NewCommandTag("SELECT 1"), nil
	}
	if strings.HasPrefix(sql, `UPDATE "indexer".delivery_partition_producers`) {
		key := repair54Key(args[:3]...)
		if _, ok := db.table("delivery_partition_producers")[key]; !ok {
			return pgconn.CommandTag{}, store.ErrIntegrity
		}
		db.table("delivery_partition_producers")[key] = append([]any(nil), args[3:]...)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	match := repair54InsertTable.FindStringSubmatch(sql)
	if len(match) != 2 {
		return pgconn.CommandTag{}, fmt.Errorf("unsupported in-memory Exec: %s", sql)
	}
	table := match[1]
	key := repair54Key(args[0])
	switch table {
	case "pools", "swaps", "pool_state_snapshots", "fee_distributions", "fee_events", "fee_strategy_changes", "airdrop_claims", "optical_executions", "token_for_token_swaps", "config_updates", "indexer_canonical_creations", "indexer_accounting_fills", "delivery_outbox":
	case "indexer_token_transfers":
		key = repair54Key(args[0], args[1])
	case "delivery_partition_producers":
		key = repair54Key(args[:3]...)
		if _, ok := db.table(table)[key]; ok {
			return pgconn.NewCommandTag("INSERT 0 0"), nil
		}
		return db.insert(table, key, []any{int64(0), "", []byte(nil)})
	case "delivery_partition_positions":
		key = repair54Key(args[0], args[1], args[2], args[4])
	default:
		return pgconn.CommandTag{}, fmt.Errorf("unsupported in-memory table: %s", table)
	}
	return db.insert(table, key, args)
}

type repair54MemoryRow struct {
	values []any
	err    error
}

func (r repair54MemoryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan columns: got %d want %d", len(dest), len(r.values))
	}
	for i, target := range dest {
		d := reflect.ValueOf(target).Elem()
		v := reflect.ValueOf(r.values[i])
		if !v.IsValid() {
			d.SetZero()
		} else if v.Type().AssignableTo(d.Type()) {
			d.Set(v)
		} else {
			return fmt.Errorf("scan column %d: %T into %T", i, r.values[i], target)
		}
	}
	return nil
}
func (db *repair54MemoryDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := func(v ...any) pgx.Row { return repair54MemoryRow{values: v} }
	missing := repair54MemoryRow{err: pgx.ErrNoRows}
	if err := ctx.Err(); err != nil {
		return repair54MemoryRow{err: err}
	}
	switch {
	case strings.Contains(sql, "-- name: GetPoolCount"):
		return row(int64(len(db.table("pools"))))
	case strings.Contains(sql, "-- name: GetSwapCount"):
		return row(int64(len(db.table("swaps"))))
	case strings.Contains(sql, "-- name: GetCursor"):
		return row(int32(125), db.state.height, pgtype.Timestamp{})
	case strings.Contains(sql, "-- name: GetPoolBy"):
		for _, values := range db.table("pools") {
			switch {
			case strings.Contains(sql, "GetPoolByPoolID") && values[4] == args[0]:
				return row(values...)
			case strings.Contains(sql, "GetPoolByAddress") && strings.EqualFold(values[0].(string), args[0].(string)):
				return row(values...)
			case strings.Contains(sql, "GetPoolByNftID") && values[5] != nil && *values[5].(*int64) == args[0].(int64):
				return row(values...)
			}
		}
		return missing
	case strings.Contains(sql, "FROM indexer.indexer_raw_logs"):
		// Both sourceBound and InsertAccountingFill verify the actual raw identity.
		var key string
		if strings.Contains(sql, "WHERE chain_id=125") {
			key = repair54Key(args[0], args[1])
		} else {
			key = repair54Key(args[1], args[2])
		}
		raw, ok := db.state.raw[key]
		if !ok {
			return missing
		}
		return row(fmt.Sprint(raw.Height) == fmt.Sprint(args[3]) && fmt.Sprint(raw.TransactionIndex) == fmt.Sprint(args[4]) && raw.BlockHash == args[5] && raw.Emitter == args[6])
	case strings.Contains(sql, "SELECT EXISTS(SELECT 1 FROM indexer.indexer_canonical_creations"):
		_, ok := db.table("indexer_canonical_creations")[repair54Key(args[0])]
		return row(ok)
	case strings.Contains(sql, "FROM indexer.indexer_canonical_creations c JOIN"):
		saved, ok := db.table("indexer_canonical_creations")[repair54Key(args[0])]
		if !ok {
			return missing
		}
		var fact store.CreationFact
		if err := json.Unmarshal(saved[13].([]byte), &fact); err != nil {
			return repair54MemoryRow{err: err}
		}
		raw, ok := db.state.raw[repair54Key(fact.Position.TransactionHash, fact.Position.LogIndex)]
		return row(ok && bytes.Equal(saved[13].([]byte), args[1].([]byte)) && fact.Pool == args[2] && fact.Creator == args[3] && fact.Factory == args[4] && fact.Position.Height == args[5] && fact.Position.BlockHash == args[6] && fact.Position.TransactionHash == args[7] && fact.Position.TransactionIndex == args[8] && fact.Position.LogIndex == args[9] && saved[11] == args[10] && fmt.Sprint(raw.Height) == fact.Position.Height && raw.BlockHash == fact.Position.BlockHash && fmt.Sprint(raw.TransactionIndex) == fact.Position.TransactionIndex && raw.Emitter == fact.Factory)
	case strings.Contains(sql, `FROM "indexer".delivery_partition_producers`):
		values, ok := db.table("delivery_partition_producers")[repair54Key(args...)]
		if !ok {
			return missing
		}
		return row(values...)
	case strings.Contains(sql, `FROM "indexer".delivery_partition_positions`):
		values, ok := db.table("delivery_partition_positions")[repair54Key(args...)]
		if !ok {
			return missing
		}
		return row(values[5])
	case strings.Contains(sql, `FROM "indexer".delivery_outbox`):
		values, ok := db.table("delivery_outbox")[repair54Key(args[0])]
		if !ok {
			return missing
		}
		return row(values[2], values[4])
	}
	return repair54MemoryRow{err: fmt.Errorf("unsupported in-memory QueryRow: %s", sql)}
}
func (db *repair54MemoryDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unsupported in-memory multi-row query")
}

// Only the SQL operations used by the real canonical projection are supported;
// any unexpected operation fails instead of returning a successful placeholder.
type repair54MemoryTx struct{ db *repair54MemoryDB }

func (tx *repair54MemoryTx) Exec(c context.Context, q string, a ...any) (pgconn.CommandTag, error) {
	return tx.db.Exec(c, q, a...)
}
func (tx *repair54MemoryTx) QueryRow(c context.Context, q string, a ...any) pgx.Row {
	return tx.db.QueryRow(c, q, a...)
}
func (tx *repair54MemoryTx) Query(c context.Context, q string, a ...any) (pgx.Rows, error) {
	return tx.db.Query(c, q, a...)
}
func (*repair54MemoryTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions unsupported")
}
func (*repair54MemoryTx) Commit(context.Context) error {
	return errors.New("the store owns the canonical commit")
}
func (*repair54MemoryTx) Rollback(context.Context) error {
	return errors.New("the store owns the canonical rollback")
}
func (*repair54MemoryTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy unsupported")
}
func (*repair54MemoryTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("batch unsupported")
}
func (*repair54MemoryTx) LargeObjects() pgx.LargeObjects { panic("large objects unsupported") }
func (*repair54MemoryTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare unsupported")
}
func (*repair54MemoryTx) Conn() *pgx.Conn { panic("connection access unsupported") }

func TestRepair_54_NativeReadModelProjection(t *testing.T) {
	ctx := context.Background()
	batch, envelopes, timestamp, poolAddress, token, txHash := repair54NativeProjectionFixture(t)
	height := batch.Blocks[0].Height
	memory := repair54NewMemoryStore(t, height-1, batch.StreamID)
	var err error
	batch.Fence, err = memory.AcquireLease(ctx, 125, batch.StreamID, batch.Owner, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	project := func(c context.Context, canonical store.CanonicalTx) ([]store.OutboxIntent, error) {
		p := New(Deps{Store: canonical.Store, ChainID: 125, EventEmitterAddress: deployment.Current().Roles["EventEmitter"].Address})
		id := int64(42)
		p.nftCache[strings.ToLower(poolAddress.Hex())] = &id
		return nil, p.projectLegacyEnvelopes(c, batch, envelopes, map[string]int64{strings.ToLower(txHash.Hex()): timestamp}, nil, nil)
	}
	memory.rejectCommit = true
	if err = memory.CommitProjectedCanonicalTx(ctx, batch, project); err == nil || !strings.Contains(err.Error(), "repair54 atomic rollback") {
		t.Fatalf("rollback boundary: %v", err)
	}
	queries := sqlcdb.New(memory.db)
	if count, err := queries.GetPoolCount(ctx); err != nil || count != 0 {
		t.Fatalf("rolled-back pools=%d %v", count, err)
	}
	if count, err := queries.GetSwapCount(ctx); err != nil || count != 0 {
		t.Fatalf("rolled-back swaps=%d %v", count, err)
	}
	if len(memory.db.state.raw) != 0 || len(memory.db.state.blocks) != 0 || len(memory.db.state.batches) != 0 {
		t.Fatal("canonical effects escaped rollback")
	}
	for _, table := range []string{"pool_state_snapshots", "fee_distributions", "airdrop_claims"} {
		if len(memory.db.table(table)) != 0 {
			t.Fatalf("%s escaped rollback", table)
		}
	}
	if watermark, err := memory.GetCanonicalWatermark(ctx, 125, batch.StreamID); err != nil || watermark != height-1 {
		t.Fatalf("rollback watermark=%d %v", watermark, err)
	}
	memory.rejectCommit = false
	if err = memory.CommitProjectedCanonicalTx(ctx, batch, project); err != nil {
		t.Fatal(err)
	}
	market, err := queries.GetPoolByAddress(ctx, strings.ToLower(poolAddress.Hex()))
	if err != nil || market.PoolID != poolIDFor(poolAddress) || market.TokenAddress != strings.ToLower(token.Hex()) || market.NftID == nil || *market.NftID != 42 {
		t.Fatalf("typed pool=%+v %v", market, err)
	}
	// Values below are the exact arguments the real generated SQL consumer wrote.
	swap := memory.db.table("swaps")[strings.ToLower(txHash.Hex())+"-1"]
	snapshot := memory.db.table("pool_state_snapshots")[fmt.Sprintf("%s-%d-2", poolIDFor(poolAddress), height)]
	fee := memory.db.table("fee_distributions")[strings.ToLower(txHash.Hex())+"-3"]
	claim := memory.db.table("airdrop_claims")[strings.ToLower(txHash.Hex())+"-4"]
	if len(swap) != 14 || len(snapshot) != 10 || len(fee) != 10 || len(claim) != 12 {
		t.Fatalf("missing native rows: swap=%v state=%v fee=%v claim=%v", swap, snapshot, fee, claim)
	}
	if swap[6] != "100" || snapshot[2] != "11" || fee[4] != "7" || claim[5] != "7" || swap[0] != strings.ToLower(txHash.Hex())+"-1" {
		t.Fatalf("native quantities or immutable key: %v %v %v %v", swap, snapshot, fee, claim)
	}
	router := chi.NewRouter()
	httpapi.RegisterStatus(router, httpapi.StatusDeps{Store: memory.Store, ChainID: 125, StartTime: time.Now()})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	var status struct {
		Pools int   `json:"totalPoolsIndexed"`
		Swaps int   `json:"totalSwapsIndexed"`
		Last  int64 `json:"lastProcessedBlock"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &status); err != nil || response.Code != 200 || status.Pools != 1 || status.Swaps != 1 || status.Last != height {
		t.Fatalf("HTTP consumer: %s %v", response.Body.String(), err)
	}
	if watermark, err := memory.GetCanonicalWatermark(ctx, 125, batch.StreamID); err != nil || watermark != height {
		t.Fatalf("committed watermark=%d %v", watermark, err)
	}
	if committed, err := memory.ReconcileCanonicalCommit(ctx, batch); err != nil || !committed {
		t.Fatalf("immutable replay: %v %v", committed, err)
	}
	if err = memory.CommitProjectedCanonicalTx(ctx, batch, project); !errors.Is(err, store.ErrWatermark) {
		t.Fatalf("duplicate batch: %v", err)
	}
	if count, err := queries.GetSwapCount(ctx); err != nil || count != 1 {
		t.Fatalf("duplicate swaps=%d %v", count, err)
	}
	changed := batch
	changed.Records = append([]store.RawRecord(nil), batch.Records...)
	changed.Records[0].Emitter = strings.ToLower(poolAddress.Hex())
	if _, err := memory.ReconcileCanonicalCommit(ctx, changed); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed immutable source accepted: %v", err)
	}
}

// This peer models app.canonicalRuntimeHead's checkpoint reload before its real
// committed RPC head read, including the live source's durable-height fence.
type repair54RuntimeHead struct {
	source.HeadSource
	memory  *repair54MemoryStore
	durable *atomic.Uint64
}

func (h *repair54RuntimeHead) Head(ctx context.Context) (int64, error) {
	height, err := h.memory.GetCanonicalWatermark(ctx, 125, realStream)
	if err != nil {
		return 0, err
	}
	h.durable.Store(uint64(height))
	return h.HeadSource.Head(ctx)
}

func repair54MemoryProcessor(t *testing.T, ctx context.Context, memory *repair54MemoryStore, url, headURL string) (*Processor, *receipt.RPCQualification, func()) {
	t.Helper()
	options, _, err := runtimedocs.ReceiptOptions(repair54ReceiptInputs(t))
	if err != nil {
		t.Fatal(err)
	}
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, url, "")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := receipt.NewRPCQualification(ctx, client, options)
	if err != nil {
		closeClient()
		t.Fatal(err)
	}
	committed, err := runtimedocs.CommittedOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	committed.Endpoints[runtimedocs.PrimaryEndpointID] = headURL
	rpcSource, err := source.NewCommittedSource(ctx, committed)
	if err != nil {
		t.Fatal(err)
	}
	// The real delivery writers only use the supplied transaction; this pool is
	// never connected. Any accidental pool operation fails on the closed pool.
	unusedPool, err := pgxpool.New(ctx, "postgres://unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	unusedPool.Close()
	policy := delivery.Policy{Subjects: map[string]bool{publisher.CanonicalCreationSubject: true, accounting.MarketSubject: true, ReplaySubjectV2: true}, EventTypes: map[string]bool{publisher.CanonicalCreationEventType: true, accounting.MarketSubject: true, ReplaySubjectV2: true}}
	durable, err := delivery.NewStoreWithEpochGuard(unusedPool, "indexer", policy, memory.guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(unusedPool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	stager, err := publisher.NewDurableStager(durable, options.Manifest.DeploymentVersion, map[string]string{publisher.CanonicalCreationEventType: publisher.CanonicalCreationSubject, accounting.MarketSubject: accounting.MarketSubject})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := memory.GetCanonicalWatermark(ctx, 125, realStream)
	if err != nil {
		t.Fatal(err)
	}
	durableHeight := new(atomic.Uint64)
	durableHeight.Store(uint64(checkpoint))
	live := &source.LiveQualifiedSource{RPC: rpcSource, Authority: authority, DurableHeight: durableHeight.Load, PriorHash: func(c context.Context, h uint64) (common.Hash, error) {
		if err := c.Err(); err != nil {
			return common.Hash{}, err
		}
		if h == realAnchor.Height {
			return realAnchor.Hash, nil
		}
		memory.mu.Lock()
		defer memory.mu.Unlock()
		hash, ok := memory.db.state.blocks[int64(h)]
		if !ok {
			return common.Hash{}, store.ErrWatermark
		}
		return common.HexToHash(hash), nil
	}}
	head := &repair54RuntimeHead{HeadSource: rpcSource, memory: memory, durable: durableHeight}
	p := New(Deps{Store: memory, Head: head, Decoder: decode.NewDecoder(), EventEmitterAddress: deployment.Current().Roles["EventEmitter"].Address, ChainID: 125, QualifiedBatchSource: live, CanonicalStreamID: realStream, CanonicalOwner: "repair54-memory", AccountingPartitions: partitions, Durable: stager, AccountingProjectionVersion: "accounting-market-v1", LegacyProjection: true, Reserves: authority, BatchSize: 17, PollInterval: 10 * time.Millisecond})
	return p, authority, func() { _ = rpcSource.Close(); closeClient() }
}

func TestRepair_54_AmbiguousCommitLiveRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Setenv(chainfixture.RecordEnv, "")
	memory := repair54NewMemoryStore(t, int64(realAnchor.Height), realStream)
	fixture := chainfixture.New(t)
	url := fixture.Serve(t)
	caughtUp := make(chan struct{}, 1)
	headServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read head: %v", err)
			http.Error(w, err.Error(), 400)
			return
		}
		var call struct {
			JSONRPC string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err = json.Unmarshal(body, &call); err != nil {
			t.Errorf("decode head: %v", err)
			http.Error(w, err.Error(), 400)
			return
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"latest"` {
			call.Params[0] = json.RawMessage(`"0x2063ee5"`)
			body, err = json.Marshal(call)
			if err != nil {
				t.Errorf("bind recorded head: %v", err)
				http.Error(w, err.Error(), 500)
				return
			}
			height, err := memory.GetCanonicalWatermark(r.Context(), 125, realStream)
			if err != nil {
				t.Errorf("read checkpoint: %v", err)
				http.Error(w, err.Error(), 500)
				return
			}
			if height == 33963749 {
				select {
				case caughtUp <- struct{}{}:
				default:
				}
			}
		}
		request := r.Clone(r.Context())
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		fixture.ServeHTTP(w, request)
	}))
	defer headServer.Close()
	seed, _, closeSeed := repair54MemoryProcessor(t, ctx, memory, url, headServer.URL)
	if err := seed.ProcessBatch(ctx, 33963716, 33963716); err != nil {
		closeSeed()
		t.Fatal(err)
	}
	if err := memory.ReleaseLease(ctx, 125, realStream, seed.deps.CanonicalOwner, seed.canonicalFence); err != nil {
		t.Fatal(err)
	}
	closeSeed()
	rehydrate := func(authority *receipt.RPCQualification) {
		t.Helper()
		memory.mu.Lock()
		rows := memory.db.table("indexer_canonical_creations")
		var fact store.CreationFact
		if len(rows) != 1 {
			memory.mu.Unlock()
			t.Fatalf("creation facts=%d, want 1", len(rows))
		}
		for _, values := range rows {
			if err := json.Unmarshal(values[13].([]byte), &fact); err != nil {
				memory.mu.Unlock()
				t.Fatal(err)
			}
		}
		memory.mu.Unlock()
		height, err := strconv.ParseUint(fact.Position.Height, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		index, err := strconv.ParseUint(fact.Position.TransactionIndex, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		if err := authority.Rehydrate(ctx, []receipt.DiscoveredMarket{{Token: common.HexToAddress(fact.Token), Pool: common.HexToAddress(fact.Pool), Height: height, TransactionIndex: uint(index)}}); err != nil {
			t.Fatal(err)
		}
		_, qualified, err := authority.FetchQualifiedBatch(ctx, height, height)
		if err != nil {
			t.Fatal(err)
		}
		facts, err := qualifiedCreationFacts(ctx, qualified)
		if err != nil || len(facts) != 1 || facts[0].Token != fact.Token || facts[0].Pool != fact.Pool {
			t.Fatalf("reopened factory proof=%+v: %v", facts, err)
		}
	}
	p, authority, closeLive := repair54MemoryProcessor(t, ctx, memory, url, headServer.URL)
	defer closeLive()
	rehydrate(authority)
	memory.lostResponse = true
	memory.unavailableReconcile = true
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	result := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); result <- p.Run(runCtx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("processor did not stop")
		}
	})
	select {
	case <-caughtUp:
	case err := <-result:
		t.Fatalf("Run stopped before live recovery: %v", err)
	case <-ctx.Done():
		t.Fatal("Run stalled after ambiguous commit")
	}
	stop()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run stop: %v", err)
	}
	if p.pendingCanonical != nil {
		t.Fatal("original committed batch remained pending")
	}
	assertEffects := func() {
		t.Helper()
		memory.mu.Lock()
		defer memory.mu.Unlock()
		state := memory.db.state
		if state.height != 33963749 || len(state.batches) != 3 || len(memory.db.table("swaps")) != 2 || len(memory.db.table("indexer_accounting_fills")) != 2 {
			t.Fatalf("exactly-once effects height/batches/swaps/fills=%d/%d/%d/%d", state.height, len(state.batches), len(memory.db.table("swaps")), len(memory.db.table("indexer_accounting_fills")))
		}
		originalEnd := int64(0)
		marketFrames := 0
		for _, b := range state.batches {
			if b.PreviousHeight == 33963716 {
				originalEnd = b.Blocks[len(b.Blocks)-1].Height
			}
		}
		for _, values := range memory.db.table("indexer_ingest_outbox") {
			if values[2] == accounting.MarketSubject {
				marketFrames++
			}
		}
		if originalEnd != 33963733 || marketFrames != 2 || len(memory.reconciled) != 1 {
			t.Fatalf("original range/reconciliations/frames=%d/%v/%d", originalEnd, memory.reconciled, marketFrames)
		}
		if len(memory.db.table("delivery_partition_positions")) != 2 || len(memory.db.table("delivery_outbox")) != 3 {
			t.Fatalf("durable positions/frames=%d/%d", len(memory.db.table("delivery_partition_positions")), len(memory.db.table("delivery_outbox")))
		}
		// Decode and validate the actual publisher envelopes as downstream readers do.
		for _, values := range memory.db.table("delivery_outbox") {
			wire := values[4].([]byte)
			subject := values[2].(string)
			policy := delivery.Policy{Subjects: map[string]bool{publisher.CanonicalCreationSubject: true, accounting.MarketSubject: true}, EventTypes: map[string]bool{publisher.CanonicalCreationEventType: true, accounting.MarketSubject: true}}
			envelope, err := delivery.Decode(wire, policy, subject)
			if err != nil {
				t.Fatal(err)
			}
			if values[2] == accounting.MarketSubject {
				publication, err := accounting.DecodePublicationV1(envelope.Payload)
				if err != nil {
					t.Fatal(err)
				}
				if publication.Fill == nil || publication.Fill.PoolAddress != realMarketPool || publication.Fill.Token.Address != realMarketToken {
					t.Fatalf("unexpected durable fill: %+v", publication)
				}
			}
		}
	}
	assertEffects()
	closeLive()
	// Restart the real processor and its RPC/receipt authority on the SAME store.
	restarted, restartedAuthority, closeRestarted := repair54MemoryProcessor(t, ctx, memory, url, headServer.URL)
	defer closeRestarted()
	rehydrate(restartedAuthority)
	restartCtx, stopRestart := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stopRestart()
	if err := restarted.Run(restartCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reopened Run: %v", err)
	}
	assertEffects()
}
