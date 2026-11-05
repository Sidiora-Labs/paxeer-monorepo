package source_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

const (
	repair56Creation = uint64(33963716)
	repair56Buy      = uint64(33963733)
	repair56Sell     = uint64(33963749)
	repair56Token    = "0x0f92fdd087c96dd3413ef98646b94d0d986192fa"
	repair56Pool     = "0xce20da50f92d21d8bee497910c782e9aa25b6263"
)

type repair56Fixture struct {
	ctx                    context.Context
	endpoint, reader, role string
	options                receipt.RPCOptions
	committed              source.CommittedOptions
	logReads               atomic.Uint64
}

func repair56RPCSetup(t *testing.T) *repair56Fixture {
	t.Helper()
	t.Setenv(chainfixture.RecordEnv, "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	f := &repair56Fixture{ctx: ctx}
	recording := chainfixture.New(t)
	recordedURL := recording.Serve(t)
	client, err := ethclient.DialContext(ctx, recordedURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	// The recording's latest block has no transactions. Its exact response is
	// therefore also the full-transaction response needed by the committed source.
	var head struct{ Transactions []json.RawMessage }
	if err = client.Client().CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false); err != nil || head.Transactions == nil || len(head.Transactions) != 0 {
		t.Fatalf("recorded empty head required: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			t.Error(err)
			http.Error(w, "request", 400)
			return
		}
		var call struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err = json.Unmarshal(body, &call); err != nil {
			t.Error(err)
			http.Error(w, "request", 400)
			return
		}
		if call.Method == "eth_getLogs" {
			f.logReads.Add(1)
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"latest"` && string(call.Params[1]) == "true" {
			call.Params[1] = json.RawMessage("false")
			body, err = json.Marshal(call)
			if err != nil {
				t.Error(err)
				http.Error(w, "request", 400)
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		recording.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	f.endpoint = server.URL
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved historical manifest unavailable")
	}
	reviewed, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "protocol", "catalog", "reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, err := client.CodeAtHash(ctx, common.HexToAddress(repair56Token), common.HexToHash("0x81a18d822e327fbcbfe2be91b7f666bfa043016cb0e3f094c5bbabe91f9d192b"))
	if err != nil {
		t.Fatal(err)
	}
	f.options, _, err = runtimedocs.ReceiptOptions(runtimedocs.Inputs{
		Manifest: manifest, ReviewedCatalog: reviewed, ERC20RuntimeCode: code,
		PoolRuntimeHash: common.HexToHash("0xa9b304d6d0aa562f283b45978b84ced8840a9331f8ac946742670b1b02a2872d"),
		Anchor:          receipt.BlockAnchor{Height: repair56Creation - 1, Hash: common.HexToHash("0x82477110c5fc791f6bef5a7d96636dc457e83daf59c8b802e264bada02f13395")},
	})
	if err != nil {
		t.Fatal(err)
	}
	// This recorded generation uses beacon pools; its approved manifest pins
	// the beacon and implementation, which the real qualifier checks.
	for i := range f.options.Discovery {
		if f.options.Discovery[i].Role == "SidioraPool" {
			f.options.Discovery[i].Kind = "beacon_proxy"
		}
	}
	f.committed, err = runtimedocs.CommittedOptions(f.options)
	if err != nil {
		t.Fatal(err)
	}
	f.committed.Endpoints[runtimedocs.PrimaryEndpointID] = f.endpoint

	return f
}

func repair56Setup(t *testing.T) *repair56Fixture {
	t.Helper()
	f := repair56RPCSetup(t)
	ctx := f.ctx
	client, err := ethclient.DialContext(ctx, f.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	dsn, pool := internaltest.NewPostgres(t)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	f.role = "repair56_" + strings.TrimPrefix(u.Path, "/")
	role := pgx.Identifier{f.role}.Sanitize()
	if _, err = pool.Exec(ctx, `CREATE TABLE public.blocks(number bigint,hash bytea,parent_hash bytea,timestamp timestamptz,consensus boolean);
CREATE TABLE public.transactions(hash bytea,block_hash bytea,"index" bigint,from_address_hash bytea);
CREATE TABLE public.logs(address_hash bytea,first_topic bytea,second_topic bytea,third_topic bytea,fourth_topic bytea,data bytea,transaction_hash bytea,block_hash bytea,block_number bigint,"index" bigint,transaction_index bigint);
CREATE ROLE `+role+` LOGIN PASSWORD 'repair56-reader' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT USAGE ON SCHEMA public TO `+role+`;
GRANT SELECT ON public.blocks,public.transactions,public.logs TO `+role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		if _, err := pool.Exec(cleanup, "DROP OWNED BY "+role+"; DROP ROLE "+role); err != nil {
			t.Error(err)
		}
	})
	u.User = url.UserPassword(f.role, "repair56-reader")
	f.reader = u.String()
	for height := repair56Creation; height <= repair56Sell; height++ {
		var block struct {
			Hash, ParentHash common.Hash
			Timestamp        hexutil.Uint64
			Transactions     []struct {
				Hash common.Hash
				From common.Address
			}
		}
		if err = client.Client().CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.EncodeUint64(height), true); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO public.blocks VALUES($1,$2,$3,$4,true)`, int64(height), block.Hash.Bytes(), block.ParentHash.Bytes(), time.Unix(int64(block.Timestamp), 0).UTC()); err != nil {
			t.Fatal(err)
		}
		for index, transaction := range block.Transactions {
			if _, err = pool.Exec(ctx, `INSERT INTO public.transactions VALUES($1,$2,$3,$4)`, transaction.Hash.Bytes(), block.Hash.Bytes(), index, transaction.From.Bytes()); err != nil {
				t.Fatal(err)
			}
		}
		var receipts []*ethtypes.Receipt
		if err = client.Client().CallContext(ctx, &receipts, "eth_getBlockReceipts", map[string]any{"blockHash": block.Hash.Hex(), "requireCanonical": true}); err != nil {
			t.Fatal(err)
		}
		for _, receipt := range receipts {
			for _, log := range receipt.Logs {
				var topics [4]any
				for i, topic := range log.Topics {
					topics[i] = topic.Bytes()
				}
				if _, err = pool.Exec(ctx, `INSERT INTO public.logs VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, log.Address.Bytes(), topics[0], topics[1], topics[2], topics[3], append([]byte{}, log.Data...), log.TxHash.Bytes(), log.BlockHash.Bytes(), int64(log.BlockNumber), int64(log.Index), int64(log.TxIndex)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return f
}

func (f *repair56Fixture) open(t *testing.T) (*source.HistoricalQualifiedSource, func()) {
	t.Helper()
	client, closeClient, err := receipt.NewBoundedRPCClient(f.ctx, f.endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeClient)
	authority, err := receipt.NewRPCQualification(f.ctx, client, f.options)
	if err != nil {
		t.Fatal(err)
	}
	rpc, err := source.NewCommittedSource(f.ctx, f.committed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rpc.Close() })
	explorer, err := source.NewCommittedPaxscan(f.ctx, f.reader, source.CommittedPaxscanOptions{SourceID: "repair56-explorer", SchemaVersion: source.ExplorerSchemaV1, ExpectedRole: f.role, ManifestDigest: f.committed.ManifestDigest, Epochs: f.committed.Epochs, RPC: rpc})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = explorer.Close() })
	history := &source.HistoricalSource{RPC: rpc, Explorer: explorer}
	return &source.HistoricalQualifiedSource{History: history, Authority: authority, DurableHeight: func() uint64 { return repair56Creation - 1 }}, func() {
		if err := history.Close(); err != nil {
			t.Fatal(err)
		}
		closeClient()
	}
}

func repair56Fetch(t *testing.T, f *repair56Fixture, qualified *source.HistoricalQualifiedSource, from, to uint64) (store.CanonicalBatch, []*receipt.QualifiedReceipt) {
	t.Helper()
	beforeQueries := qualified.History.Explorer.Metrics().Queries
	beforeLogs := f.logReads.Load()
	batch, receipts, err := qualified.FetchQualifiedBatch(f.ctx, from, to)
	if err != nil {
		t.Fatalf("explorer qualify %d..%d: %v", from, to, err)
	}
	if qualified.History.Explorer.Metrics().Queries <= beforeQueries || f.logReads.Load() != beforeLogs {
		t.Fatal("qualification must read actual PostgreSQL without fetching RPC logs")
	}
	if err = receipt.ValidateBindings(f.ctx, batch, receipts); err != nil {
		t.Fatal(err)
	}
	return batch, receipts
}

func (f *repair56Fixture) poolSwap(emitter string, topics []string) bool {
	return emitter == strings.ToLower(f.options.Manifest.Roles["EventEmitter"].Address) &&
		len(topics) == 3 &&
		topics[0] == crypto.Keccak256Hash([]byte("Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)")).Hex() &&
		topics[1] == common.BytesToHash(common.HexToAddress(repair56Pool).Bytes()).Hex()
}

func TestRepair56Postgres_ExplorerDynamicDiscoveries(t *testing.T) {
	f := repair56Setup(t)
	qualified, stop := f.open(t)
	if len(qualified.Authority.DiscoveredEpochs()) != 0 {
		t.Fatal("cold authority already discovered markets")
	}
	creation, receipts := repair56Fetch(t, f, qualified, repair56Creation, repair56Creation)
	if qualified.History.Explorer.Metrics().Queries != 2 || len(qualified.Authority.DiscoveredEpochs()) != 2 || len(receipts) != 1 {
		t.Fatal("market creation must grow discovery epochs and retry the explorer query")
	}
	tokenLogs := 0
	for _, raw := range creation.Records {
		if raw.Emitter == repair56Token {
			tokenLogs++
		}
	}
	if tokenLogs != 2 {
		t.Fatalf("creation transaction must include both token transfers: %d", tokenLogs)
	}
	events, err := receipts[0].QualifiedEvents(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	market := false
	for _, event := range events {
		if event.EmitterRole == "SidioraFactory" && event.EventName == "MarketCreated" {
			market = true
		}
	}
	if !market {
		t.Fatal("qualified factory market creation missing")
	}
	buy, buyReceipts := repair56Fetch(t, f, qualified, repair56Creation+1, repair56Buy)
	token, pool := false, false
	for _, raw := range buy.Records {
		token = token || raw.Emitter == repair56Token
		pool = pool || f.poolSwap(raw.Emitter, raw.Topics)
	}
	if !token || !pool || len(buyReceipts) != 1 {
		t.Fatal("subsequent history lost discovered token or pool events")
	}
	fills, err := buyReceipts[0].Fills(f.ctx)
	if err != nil || len(fills) != 1 || fills[0].Side != "buy" || fills[0].PoolAddress != repair56Pool || fills[0].Token.Address != repair56Token {
		t.Fatalf("real qualified consumer did not recover buy: %+v %v", fills, err)
	}
	for _, interval := range []struct {
		from, to uint64
		expected store.CanonicalBatch
	}{{repair56Creation, repair56Creation, creation}, {repair56Creation + 1, repair56Buy, buy}} {
		live := &source.LiveQualifiedSource{RPC: qualified.History.RPC, Authority: qualified.Authority, DurableHeight: func() uint64 { return interval.from - 1 }}
		rpcBatch, _, err := live.FetchQualifiedBatch(f.ctx, interval.from, interval.to)
		if err != nil || !reflect.DeepEqual(interval.expected.Blocks, rpcBatch.Blocks) || !reflect.DeepEqual(interval.expected.Records, rpcBatch.Records) {
			t.Fatalf("RPC and PostgreSQL canonical history differ: %v", err)
		}
	}
	stop()
	qualified, _ = f.open(t)
	if len(qualified.Authority.DiscoveredEpochs()) != 0 {
		t.Fatal("restart retained process-local discoveries")
	}
	recoveredCreation, _ := repair56Fetch(t, f, qualified, repair56Creation, repair56Creation)
	recoveredBuy, _ := repair56Fetch(t, f, qualified, repair56Creation+1, repair56Buy)
	if !reflect.DeepEqual(creation, recoveredCreation) || !reflect.DeepEqual(buy, recoveredBuy) {
		t.Fatal("reopened source and qualifier did not recover identical canonical history")
	}
}

func TestRepair56Postgres_DiscoveryEpochBounds(t *testing.T) {
	f := repair56Setup(t)
	qualified, _ := f.open(t)
	_, _ = repair56Fetch(t, f, qualified, repair56Creation, repair56Creation)
	discoveries := source.DiscoveredAddressEpochs(qualified.Authority, f.committed.ManifestDigest)
	creation := source.FetchOptions{FromBlock: int64(repair56Creation), ToBlock: int64(repair56Creation)}
	buy := source.FetchOptions{FromBlock: int64(repair56Buy), ToBlock: int64(repair56Buy)}
	for _, discovery := range discoveries {
		if discovery.ActivationHeight != repair56Creation || discovery.CreationTxIndex != 0 {
			t.Fatal("authority discovery lost its creation bounds")
		}
	}
	t.Run("creation_transaction", func(t *testing.T) {
		bounded := append([]source.AddressEpoch(nil), discoveries...)
		for i := range bounded {
			bounded[i].CreationTxIndex = 1
		}
		result, err := qualified.History.FetchHistoricalWith(f.ctx, creation, 0, bounded)
		var refused *source.SourceError
		if !errors.As(err, &refused) || refused.Kind != source.IntegrityError || result.Complete {
			t.Fatalf("pre-creation transaction admitted: %v", err)
		}
		result, err = qualified.History.FetchHistoricalWith(f.ctx, buy, 0, bounded)
		if err != nil || !result.Complete {
			t.Fatalf("creation tx bound incorrectly applied to later block: %v", err)
		}
		found := map[string]bool{}
		for _, log := range result.Logs {
			found[strings.ToLower(log.Address.Hex())] = true
			topics := make([]string, len(log.Topics))
			for i, topic := range log.Topics {
				topics[i] = topic.Hex()
			}
			if f.poolSwap(strings.ToLower(log.Address.Hex()), topics) {
				found[repair56Pool] = true
			}
		}
		if !found[repair56Token] || !found[repair56Pool] {
			t.Fatal("later transaction-zero events missing")
		}
	})
	for _, kind := range []string{"activation", "retirement"} {
		t.Run(kind, func(t *testing.T) {
			bounded := append([]source.AddressEpoch(nil), discoveries...)
			interval := creation
			for i := range bounded {
				if kind == "activation" {
					bounded[i].ActivationHeight = repair56Creation + 1
				} else {
					retired := repair56Buy
					bounded[i].RetirementHeight = &retired
					interval = buy
				}
			}
			result, err := qualified.History.FetchHistoricalWith(f.ctx, interval, 0, bounded)
			if err != nil || !result.Complete {
				t.Fatal(err)
			}
			for _, log := range result.Logs {
				if log.Address == common.HexToAddress(repair56Token) || log.Address == common.HexToAddress(repair56Pool) {
					t.Fatal("inactive discovery selected")
				}
			}
		})
	}
	t.Run("invalid_authority", func(t *testing.T) {
		invalid := append([]source.AddressEpoch(nil), discoveries...)
		invalid[0].ManifestDigest = "unapproved"
		for _, epochs := range [][]source.AddressEpoch{invalid, {f.committed.Epochs[0]}, {discoveries[0], discoveries[0]}} {
			queries := qualified.History.Explorer.Metrics().Queries
			result, err := qualified.History.FetchHistoricalWith(f.ctx, creation, 0, epochs)
			var refused *source.SourceError
			if !errors.As(err, &refused) || refused.Kind != source.IdentityError || result.Complete || qualified.History.Explorer.Metrics().Queries != queries {
				t.Fatalf("invalid epoch must fail before explorer SQL: %v", err)
			}
		}
	})
	initial, err := qualified.History.Explorer.FetchLogs(f.ctx, creation)
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range initial.Logs {
		if log.Address == common.HexToAddress(repair56Token) || log.Address == common.HexToAddress(repair56Pool) {
			t.Fatal("request discoveries mutated initial explorer options")
		}
	}
}
