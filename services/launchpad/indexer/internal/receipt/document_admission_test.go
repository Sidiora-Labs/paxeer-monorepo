package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestQualificationDocumentsRejectUnencodableAndOversizedValues(t *testing.T) {
	if _, err := digest(math.Inf(1)); !errors.Is(err, ErrQualification) {
		t.Fatalf("non-finite document: %v", err)
	}
	if _, err := clone(make(chan int)); !errors.Is(err, ErrQualification) {
		t.Fatalf("unencodable document: %v", err)
	}
	if _, err := clone(&types.Header{}); err == nil {
		t.Fatal("incomplete header roundtrip admitted")
	}
	if _, err := clone(make([]byte, maxDocumentBytes)); !errors.Is(err, ErrQualification) {
		t.Fatalf("oversized document: %v", err)
	}
	if _, err := provenanceHeaderDigest(&types.Header{Extra: make([]byte, maxDocumentBytes)}); !errors.Is(err, ErrQualification) {
		t.Fatalf("oversized header provenance: %v", err)
	}
	if err := verifyHistoricalProvenance(nil, accounting.Provenance{}, accounting.Provenance{}); !errors.Is(err, ErrQualification) {
		t.Fatalf("absent historical header: %v", err)
	}
	envelope := &decode.DecodedEnvelope{Status: valueDecoded, EventName: "Swap", EmitterRole: "unlisted", Event: &decode.Event{}, Family: &decode.FamilyDTO{EconomicFill: true, Authority: "primary"}}
	if qualifiedSwapFamily(envelope, &types.Log{}) {
		t.Fatal("unknown swap emitter role admitted")
	}
	if receiptEqual(nil, nil) || receiptEqual(&types.Receipt{}, nil) {
		t.Fatal("absent receipts compared equal")
	}
	address, hash := virtualChainRegistry, common.HexToHash("0x125")
	for _, tc := range []struct {
		address   common.Address
		hash      common.Hash
		signature string
		args      []common.Address
	}{{common.Address{}, hash, "name()", nil}, {address, common.Hash{}, "name()", nil}, {address, hash, strings.Repeat("x", 129), nil}, {address, hash, "name()", []common.Address{address, address}}} {
		if _, err := (*RPCQualification)(nil).word(context.Background(), tc.address, tc.signature, tc.hash, tc.args...); !errors.Is(err, ErrQualification) {
			t.Fatalf("invalid pinned call: %v", err)
		}
	}
	refused := qualificationRefusal("Pool", "direct", address, "coverage")
	if qualificationContext(errors.Join(errors.New("read"), refused), "Token", "direct", address, "identity") != refused {
		t.Fatal("qualification context replaced the original refusal")
	}
	if discoveryKind(valueSidioraPool, "") != valueBeaconProxy || (&RPCQualification{}).discoveryKind("unlisted") != valueUnknown {
		t.Fatal("discovery defaults differ")
	}
}

func TestIdentifiedHeadersRejectInvalidHashNumberAndTransactionDocuments(t *testing.T) {
	authority := &RPCQualification{}
	header := &types.Header{Number: big.NewInt(1), Difficulty: new(big.Int), Time: 1}
	raw, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ field, value string }{{"hash", "1"}, {"number", `"0x10000000000000000"`}, {"transactions", "12"}} {
		t.Run(tc.field, func(t *testing.T) {
			var document map[string]json.RawMessage
			if err := json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			document[tc.field] = json.RawMessage(tc.value)
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := authority.parseBlock(encoded); !errors.Is(err, ErrQualification) {
				t.Fatalf("invalid block document admitted: %v", err)
			}
		})
	}
}

func TestReadAheadRejectsCanceledAndIncompleteEntries(t *testing.T) {
	state := &rpcReadState{}
	if state.readaheadDepth() != DefaultArchiveReadahead {
		t.Fatal("default readahead differs")
	}
	var absent *RPCQualification
	absent.ReadAhead(1, 1)
	(&RPCQualification{reads: state}).ReadAhead(2, 1)
	var absentCache *rpcReadState
	absentCache.storeBlock(common.Hash{}, cachedBlock{})
	if _, ok := absentCache.block(common.Hash{}); ok {
		t.Fatal("absent block cache returned material")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	state.ahead = map[uint64]*pinnedAhead{1: {done: make(chan struct{})}}
	if _, ok := state.takeAhead(canceled, 1); ok || len(state.ahead) != 0 {
		t.Fatal("canceled pin retained or returned")
	}
	for _, entry := range []*pinnedAhead{{done: make(chan struct{})}, {done: make(chan struct{}), pinned: pinnedHeight{header: &types.Header{Number: big.NewInt(2)}}}} {
		close(entry.done)
		state.ahead[1] = entry
		if _, ok := state.takeAhead(context.Background(), 1); ok || len(state.ahead) != 0 || state.aheadHits.Load() != 0 {
			t.Fatal("incomplete or mismatched pin admitted")
		}
	}
}

func TestShellReceiptMembershipRequiresCompleteUniqueTransactions(t *testing.T) {
	for _, name := range []string{"absent block", "nil transaction", "duplicate transaction", "missing receipt", "compact index"} {
		t.Run(name, func(t *testing.T) {
			txs, receipts, hash, height := shellReceiptFixture(t, true)
			bloom := types.MergeBloom(listedShellFixtureReceipts(receipts))
			switch name {
			case "absent block":
				hash = common.Hash{}
			case "nil transaction":
				txs[0] = nil
			case "duplicate transaction":
				txs = append(txs, txs[0])
			case "missing receipt":
				receipts = nil
			case "compact index":
				receipts[0].TransactionIndex++
				for _, log := range receipts[0].Logs {
					log.TxIndex = receipts[0].TransactionIndex
				}
			}
			if _, _, err := SeparateShellReceipts(txs, receipts, hash, height, bloom, common.Hash{}, true); !errors.Is(err, ErrQualification) {
				t.Fatalf("incomplete transaction membership admitted: %v", err)
			}
		})
	}
}
