package processor

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/chainreader"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestNFTResolutionCachesTheRegistryResult(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "issued", true: "reverted"}[rejected], func(t *testing.T) {
			var calls atomic.Int64
			node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage
					Method string
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
				switch request.Method {
				case "eth_chainId":
					response["result"] = "0x7d"
				case "eth_call":
					calls.Add(1)
					if rejected {
						response["error"] = map[string]any{"code": -32000, "message": "execution reverted"}
					} else {
						response["result"] = common.BigToHash(big.NewInt(42)).Hex()
					}
				default:
					t.Errorf("unexpected registry method %s", request.Method)
					w.WriteHeader(400)
					return
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer node.Close()
			client, err := chain.NewClient(context.Background(), node.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			reader, err := chainreader.NewNFTReader(client, "0x00000000000000000000000000000000000000a1")
			if err != nil {
				t.Fatal(err)
			}
			p := New(Deps{NFT: reader})
			first := p.resolveNftID(context.Background(), "0x00000000000000000000000000000000000000ab")
			second := p.resolveNftID(context.Background(), "0x00000000000000000000000000000000000000AB")
			if calls.Load() != 1 || first != second || rejected && first != nil || !rejected && (first == nil || *first != 42) {
				t.Fatalf("registry cache first=%v second=%v calls=%d", first, second, calls.Load())
			}
		})
	}
}

func TestLegacyProjectionRejectsUnavailableRowsAndMissingTimestamps(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	p := processorFor(pool)
	b := fixtureBatch(t)
	r := sourceRegistry(t, common.HexToAddress(b.Records[0].Emitter))
	var envelopes []*decode.DecodedEnvelope
	timestamps := map[string]int64{}
	for _, raw := range b.Records {
		timestamps[raw.TransactionHash] = 123456
	}
	// Use envelopes from the same ABI and raw records as canonical ingestion.
	for _, raw := range b.Records {
		l := types.Log{Address: common.HexToAddress(raw.Emitter), BlockHash: common.HexToHash(raw.BlockHash), TxHash: common.HexToHash(raw.TransactionHash), BlockNumber: uint64(raw.Height), TxIndex: uint(raw.TransactionIndex), Index: uint(raw.LogIndex), Data: raw.Data}
		for _, topic := range raw.Topics {
			l.Topics = append(l.Topics, common.HexToHash(topic))
		}
		e, err := r.Decode(l)
		if err != nil {
			t.Fatal(err)
		}
		envelopes = append(envelopes, e)
	}
	for _, tc := range []struct {
		name    string
		edit    func([]*decode.DecodedEnvelope) []*decode.DecodedEnvelope
		missing bool
	}{
		{"unclassified", func(es []*decode.DecodedEnvelope) []*decode.DecodedEnvelope {
			return append([]*decode.DecodedEnvelope{nil, {Status: "opaque"}}, es...)
		}, false},
		{"unknown role", func(es []*decode.DecodedEnvelope) []*decode.DecodedEnvelope {
			e := *es[0]
			e.EmitterRole = "Unknown"
			return []*decode.DecodedEnvelope{&e}
		}, false},
		{"missing time", func(es []*decode.DecodedEnvelope) []*decode.DecodedEnvelope { return es }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			es := tc.edit(envelopes)
			ts := timestamps
			if tc.missing {
				ts = map[string]int64{}
			}
			err := p.projectLegacyEnvelopes(ctx, b, es, ts, nil, nil)
			if tc.missing {
				if !errors.Is(err, store.ErrIntegrity) {
					t.Fatalf("missing timestamp %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	if nativeLegacyRole(&decode.DecodedEnvelope{Raw: decode.RawLog{Emitter: strings.ToLower(decode.LaunchpadPrecompileAddress.Hex())}, EmitterRole: "Unknown"}) {
		t.Fatal("unknown native role admitted")
	}
	if err := (*Processor)(nil).RetainSourcePoison(ctx, source.FetchResult{}); err == nil {
		t.Fatal("absent processor retained input")
	}
	if err := New(Deps{}).RetainSourcePoison(ctx, source.FetchResult{}); err == nil {
		t.Fatal("unbound quarantine retained input")
	}
	if err := p.projectQualifiedLegacy(ctx, store.New(pool), b, []*receipt.QualifiedReceipt{nil}, nil); err == nil {
		t.Fatal("unissued receipt projected")
	}
	if err := p.ProcessCanonicalBatch(ctx, store.CanonicalBatch{ChainID: 1}, nil, nil); err == nil {
		t.Fatal("foreign chain committed")
	}
}

func TestRestoreCheckpointRequiresReadableStorageAndWritableDestination(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	for _, tc := range []struct {
		name string
		edit func(*IngestMeasurementSpec)
		path func() string
		want string
	}{
		{"source binding", func(s *IngestMeasurementSpec) { s.CandidateSHA256 = "invalid" }, func() string { return filepath.Join(t.TempDir(), "checkpoint.json") }, "candidate"},
		{"storage schema", func(s *IngestMeasurementSpec) { s.DeliverySchema = "absent" }, func() string { return filepath.Join(t.TempDir(), "checkpoint.json") }, ""},
		{"destination parent", func(*IngestMeasurementSpec) {}, func() string { return filepath.Join(t.TempDir(), "absent", "checkpoint.json") }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := h.spec(t)
			tc.edit(&s)
			path := tc.path()
			_, err := h.CaptureRestoreCheckpoint(ctx, s, 33963716, 33963716, path)
			if err == nil || tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("checkpoint admission %v", err)
			}
			if _, stat := os.Stat(path); !os.IsNotExist(stat) {
				t.Fatalf("rejected checkpoint exists: %v", stat)
			}
		})
	}
}
