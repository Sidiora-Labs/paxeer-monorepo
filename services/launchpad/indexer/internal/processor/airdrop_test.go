package processor

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
)

const (
	airdropPool   = "0x1111111111111111111111111111111111111111"
	airdropHolder = "0x2222222222222222222222222222222222222222"
	airdropOther  = "0x3333333333333333333333333333333333333333"
	airdropPoolID = "0x0000000000000000000000001111111111111111111111111111111111111111"
	airdropTx     = "0x" + "ab" + "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
	airdropBlock  = "0x" + "ef" + "efefefefefefefefefefefefefefefefefefefefefefefefefefefefefefef"
)

// encodedLog builds an authentic EVM log for one ABI event: topic0 is the
// canonical signature hash, indexed inputs are packed as topics and the
// non-indexed inputs are ABI-packed into data, exactly as the chain emits them.
func encodedLog(t *testing.T, meta *bind.MetaData, name string, index uint, indexed []any, data ...any) ethtypes.Log {
	t.Helper()
	parsed, err := meta.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := parsed.Events[name]
	if !ok {
		t.Fatalf("event %s absent from ABI", name)
	}
	var indexedArgs abi.Arguments
	for _, in := range ev.Inputs {
		if in.Indexed {
			indexedArgs = append(indexedArgs, in)
		}
	}
	topics, err := abi.MakeTopics(append([]any{}, indexed...))
	if err != nil {
		t.Fatal(err)
	}
	if len(indexedArgs) != len(indexed) || len(topics) != 1 || len(topics[0]) != len(indexed) {
		t.Fatalf("indexed arity for %s", name)
	}
	packed, err := ev.Inputs.NonIndexed().Pack(data...)
	if err != nil {
		t.Fatal(err)
	}
	return ethtypes.Log{Address: common.HexToAddress("0x" + strings.Repeat("9", 40)), Topics: append([]common.Hash{ev.ID}, topics[0]...), Data: packed, BlockNumber: 777, TxHash: common.HexToHash(airdropTx), TxIndex: 2, BlockHash: common.HexToHash(airdropBlock), Index: index}
}

func airdropFixture(t *testing.T) (*Processor, *pgxpool.Pool) {
	t.Helper()
	_, pool := internaltest.NewPostgres(t)
	st := store.New(pool)
	nft := int64(42)
	if err := st.InsertPool(context.Background(), sqlcdb.InsertPoolParams{PoolAddress: "0x" + strings.ToUpper(airdropPool[2:6]) + airdropPool[6:], TokenAddress: "0x" + strings.Repeat("4", 40), Creator: "0x" + strings.Repeat("5", 40), Optical: "0x" + strings.Repeat("6", 40), PoolID: airdropPoolID, NftID: &nft, CreatedAt: 1760000000, CreatedBlock: 700, TxHash: "0x" + strings.Repeat("7", 64)}); err != nil {
		t.Fatal(err)
	}
	return New(Deps{Store: st, Decoder: decode.NewDecoder(), ChainID: 125}), pool
}

type airdropRow struct {
	poolID, poolAddress *string
	nftID               *int64
	holder, amount      string
	epoch               *string
	eventName, txHash   string
	logIndex            int32
}

func airdropRows(t *testing.T, pool *pgxpool.Pool) []airdropRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT pool_id,pool_address,nft_id,holder,amount,epoch,event_name,tx_hash,log_index FROM indexer.airdrop_claims ORDER BY log_index`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []airdropRow{}
	for rows.Next() {
		var r airdropRow
		if err := rows.Scan(&r.poolID, &r.poolAddress, &r.nftID, &r.holder, &r.amount, &r.epoch, &r.eventName, &r.txHash, &r.logIndex); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	return out
}

func decodeAndProcess(t *testing.T, p *Processor, l ethtypes.Log) {
	t.Helper()
	ev, err := p.deps.Decoder.Decode(l)
	if err != nil {
		t.Fatal(err)
	}
	if ev == nil {
		t.Fatalf("log %d not a registered protocol event", l.Index)
	}
	processDecoded(t, p, ev)
}

func processDecoded(t *testing.T, p *Processor, ev *decode.Event) {
	t.Helper()
	wev, err := p.processEvent(context.Background(), ev, 1760000500, nil)
	if err != nil {
		t.Fatalf("process %s: %v", ev.EventName, err)
	}
	if wev != nil {
		t.Fatalf("%s must not fan out a webhook", ev.EventName)
	}
}

// feeFlowMirror is the IEventEmitter.FeeFlow(kind,pool,party,amount,protocolCut,
// poolCut,epoch,timestamp,blockNumber) event in the humanized shape the decoder
// hands processEvent (addresses lowercased, uint values as decimal strings).
// The checked-in protocol/bindings EventEmitter ABI predates FeeFlow, so the
// mirror covers the direct processor unit route only; the native ABI,
// receipt authority and SQL path are covered by TestAirdropNativeFeeFlowQualified.
// The processor route is
// exercised on the decoded event type directly.
func feeFlowMirror(kind uint8, pool, party string, amount, epoch int64, index uint) *decode.Event {
	return &decode.Event{
		EventName:   "FeeFlow",
		Sig:         "FeeFlow(uint8,address,address,uint256,uint256,uint256,uint256,uint256,uint256)",
		Args:        map[string]any{"kind": big.NewInt(int64(kind)).String(), "pool": pool, "party": party, "amount": big.NewInt(amount).String(), "protocolCut": "0", "poolCut": "0", "epoch": big.NewInt(epoch).String(), "timestamp": "1760000500", "blockNumber": "777"},
		BlockNumber: 777,
		TxHash:      airdropTx,
		LogIndex:    index,
	}
}

func TestAirdropClaimedDecodedAndPersistedFromEveryEmittedShape(t *testing.T) {
	p, pool := airdropFixture(t)
	accumulator := encodedLog(t, bindings.FeeAccumulatorMetaData, "AirdropClaimed", 1, []any{common.HexToAddress(airdropPool), common.HexToAddress(airdropHolder)}, big.NewInt(2500000), big.NewInt(3))
	router := encodedLog(t, bindings.FeesRouterMetaData, "AirdropClaimed", 3, []any{big.NewInt(42), common.HexToAddress(airdropHolder)}, big.NewInt(4000000))
	unknownPool := encodedLog(t, bindings.FeeAccumulatorMetaData, "AirdropClaimed", 4, []any{common.HexToAddress(airdropOther), common.HexToAddress(airdropHolder)}, big.NewInt(1), big.NewInt(9))
	decodeAndProcess(t, p, accumulator)
	processDecoded(t, p, feeFlowMirror(4, airdropPool, airdropOther, 1500000, 3, 2))
	decodeAndProcess(t, p, router)
	decodeAndProcess(t, p, unknownPool)
	processDecoded(t, p, feeFlowMirror(1, airdropPool, airdropHolder, 1, 0, 5))
	decodeAndProcess(t, p, accumulator)
	processDecoded(t, p, feeFlowMirror(4, airdropPool, airdropOther, 1500000, 3, 2))
	rows := airdropRows(t, pool)
	if len(rows) != 4 {
		t.Fatalf("rows=%d want 4 (idempotent replay, kind!=4 FeeFlow ignored): %+v", len(rows), rows)
	}
	checksummed := "0x" + strings.ToUpper(airdropPool[2:6]) + airdropPool[6:]
	expect := func(r airdropRow, eventName, holder, amount string, epoch *string, logIndex int32, attributed bool, nft *int64) {
		t.Helper()
		if r.eventName != eventName || r.holder != holder || r.amount != amount || r.logIndex != logIndex || r.txHash != airdropTx {
			t.Fatalf("row %+v", r)
		}
		if (epoch == nil) != (r.epoch == nil) || (epoch != nil && *epoch != *r.epoch) {
			t.Fatalf("epoch %+v", r)
		}
		if attributed {
			if r.poolID == nil || *r.poolID != airdropPoolID || r.poolAddress == nil || *r.poolAddress != checksummed || r.nftID == nil || *r.nftID != 42 {
				t.Fatalf("attribution %+v", r)
			}
			return
		}
		if r.poolID != nil || r.poolAddress == nil || *r.poolAddress != airdropOther || r.nftID != nil {
			t.Fatalf("unattributed claim must keep the emitted pool and no invented identity: %+v", r)
		}
		_ = nft
	}
	three, nine := "3", "9"
	expect(rows[0], "AirdropClaimed", airdropHolder, "2500000", &three, 1, true, nil)
	expect(rows[1], "FeeFlow", airdropOther, "1500000", &three, 2, true, nil)
	expect(rows[2], "AirdropClaimed", airdropHolder, "4000000", nil, 3, true, nil)
	expect(rows[3], "AirdropClaimed", airdropHolder, "1", &nine, 4, false, nil)
}

func TestAirdropClaimedRejectsShapesWithoutSettlementEvidence(t *testing.T) {
	p, pool := airdropFixture(t)
	ctx := context.Background()
	for name, args := range map[string]map[string]any{
		"no holder": {"pool": airdropPool, "amount": "1"},
		"no amount": {"pool": airdropPool, "holder": airdropHolder},
		"no pool":   {"holder": airdropHolder, "amount": "1"},
		"bad nft":   {"nftId": "x", "holder": airdropHolder, "amount": "1"},
	} {
		if err := p.handleAirdropClaimed(ctx, args, "AirdropClaimed", airdropTx+"-9", 777, 1760000500, 9); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if rows := airdropRows(t, pool); len(rows) != 0 {
		t.Fatalf("rows persisted for rejected shapes: %+v", rows)
	}
}

// TestProofLaunchpadSdkProtocolGoIndexerConsumer passes the eight real metadata
// producers through the same encodedLog -> Decoder.Decode path used above.
// It deliberately stops before database persistence: these ABI producers carry
// no persistent state and this proof requires no provisioned infrastructure.
func TestProofLaunchpadSdkProtocolGoIndexerConsumer(t *testing.T) {
	pool, holder := common.HexToAddress(airdropPool), common.HexToAddress(airdropHolder)
	token, creator := common.HexToAddress("0x3333"), common.HexToAddress("0x4444")
	optical := common.HexToAddress("0x5555")
	key := [32]byte(common.HexToHash("0x1234"))
	address := func(a common.Address) string { return strings.ToLower(a.Hex()) }
	cases := []struct {
		role          string
		meta          *bind.MetaData
		event         string
		indexed, data []any
		want          map[string]any
	}{
		{"FeeAccumulator", bindings.FeeAccumulatorMetaData, "AirdropClaimed", []any{pool, holder}, []any{big.NewInt(2500000), big.NewInt(3)}, map[string]any{"pool": address(pool), "holder": address(holder), "amount": "2500000", "epoch": "3"}},
		{"FeesRouter", bindings.FeesRouterMetaData, "AirdropClaimed", []any{big.NewInt(42), holder}, []any{big.NewInt(4000000)}, map[string]any{"nftId": "42", "holder": address(holder), "amount": "4000000"}},
		{"PoolRegistry", bindings.PoolRegistryMetaData, "PoolRegistered", []any{pool, token, creator}, []any{optical, big.NewInt(42), big.NewInt(1760000000)}, map[string]any{"pool": address(pool), "token": address(token), "creator": address(creator), "optical": address(optical), "nftId": "42", "timestamp": "1760000000"}},
		{"ProtocolConfig", bindings.ProtocolConfigMetaData, "ConfigUpdated", []any{key}, []any{big.NewInt(10), big.NewInt(20)}, map[string]any{"key": common.Hash(key).Hex(), "oldValue": "10", "newValue": "20"}},
		{"Quoter", bindings.QuoterMetaData, "Initialized", nil, []any{uint64(2)}, map[string]any{"version": uint64(2)}},
		{"Router", bindings.RouterMetaData, "Buy", []any{pool, holder}, []any{big.NewInt(100), big.NewInt(90)}, map[string]any{"pool": address(pool), "buyer": address(holder), "usdlIn": "100", "tokensOut": "90"}},
		{"SidioraFactory", bindings.SidioraFactoryMetaData, "MarketCreated", []any{token, pool, creator}, []any{big.NewInt(42), optical}, map[string]any{"token": address(token), "pool": address(pool), "creator": address(creator), "nftId": "42", "optical": address(optical)}},
		{"SidioraNFT", bindings.SidioraNFTMetaData, "Transfer", []any{pool, holder, big.NewInt(42)}, nil, map[string]any{"from": address(pool), "to": address(holder), "tokenId": "42"}},
	}
	decoder := decode.NewDecoder()
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			for round := uint(1); round <= 2; round++ {
				raw := encodedLog(t, tc.meta, tc.event, round, tc.indexed, tc.data...)
				observed, err := decoder.Decode(raw)
				if err != nil || observed == nil {
					t.Fatalf("decode %s: %v %v", tc.role, observed, err)
				}
				if observed.EventName != tc.event || observed.BlockNumber != raw.BlockNumber || observed.TxHash != strings.ToLower(raw.TxHash.Hex()) || observed.LogIndex != round || !reflect.DeepEqual(observed.Args, tc.want) {
					t.Fatalf("consumer observed %+v, want args %+v at log %d", observed, tc.want, round)
				}
				// A cached output map would leak this mutation into the next decode.
				for field := range observed.Args {
					observed.Args[field] = "caller mutation"
				}
				if len(raw.Data) > 0 {
					raw.Data = raw.Data[:len(raw.Data)-1]
				} else {
					raw.Topics = raw.Topics[:len(raw.Topics)-1]
				}
				if decoded, err := decoder.Decode(raw); err == nil || decoded != nil {
					t.Fatalf("malformed %s producer result accepted: %+v %v", tc.role, decoded, err)
				}
			}
		})
	}
	if observed, err := decoder.Decode(ethtypes.Log{Topics: []common.Hash{common.HexToHash("0xdeadbeef")}}); err != nil || observed != nil {
		t.Fatalf("unknown identifier must remain unrecognized: %+v %v", observed, err)
	}
}

func TestAirdropNativeFeeFlowQualified(t *testing.T) {
	repairM1000NativeFeeFlow(t)
}
