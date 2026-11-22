package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func deploymentBoundsRegistry(t *testing.T) (*decode.Registry, abi.ABI, abi.ABI) {
	t.Helper()
	body, err := os.ReadFile("../../../../../sdk/launchpad/protocol/contracts/v1/chain/abi-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog decode.Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		t.Fatal(err)
	}
	var factory, emitter abi.ABI
	var epochs []decode.Deployment
	for _, contract := range catalog.Contracts {
		if contract.Name != "SidioraFactory" && contract.Name != "EventEmitter" {
			continue
		}
		parsed, err := abi.JSON(bytes.NewReader(contract.ABI))
		if err != nil {
			t.Fatal(err)
		}
		address := common.HexToAddress("0xf1")
		if contract.Name == "EventEmitter" {
			address, emitter = common.HexToAddress("0xe1"), parsed
		} else {
			factory = parsed
		}
		epochs = append(epochs, decode.Deployment{Address: address, Role: contract.Name, ABIRevision: contract.ABIRevision, EnabledOptionalEvents: []string{"BestRouteSwap"}})
	}
	r, err := decode.NewRegistry(body, epochs)
	if err != nil {
		t.Fatal(err)
	}
	return r, factory, emitter
}

func deploymentBoundsSourceLog(event abi.Event, address common.Address, index uint, data []byte) types.Log {
	return types.Log{Address: address, Topics: []common.Hash{event.ID, common.HexToHash("0xa1"), common.HexToHash("0xb1"), common.HexToHash("0xc1")}, Data: data, BlockNumber: 42, BlockHash: common.HexToHash("0x1111"), TxHash: common.HexToHash("0x2222"), Index: index}
}

func TestDeploymentLogBoundsCreationFacts(t *testing.T) {
	r, factory, emitter := deploymentBoundsRegistry(t)
	marketData, err := factory.Events["MarketCreated"].Inputs.NonIndexed().Pack(big.NewInt(7), common.Address{})
	if err != nil {
		t.Fatal(err)
	}
	marketLog := deploymentBoundsSourceLog(factory.Events["MarketCreated"], common.HexToAddress("0xf1"), 0, marketData)
	market, err := r.Decode(marketLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, tokenName, symbol string
		quarantined             bool
	}{
		{"withheld name", string([]byte{0xff}), "COIN", false},
		{"zero character", "coin\x00", "COIN", false},
		{"withheld symbol", "Coin", strings.Repeat("x", decode.MaxFieldBytes+1), false},
		{"empty fields", "", "", false},
		{"quarantined token", "Coin", "COIN", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := emitter.Events["TokenDeployed"].Inputs.NonIndexed().Pack([32]byte{}, tc.tokenName, tc.symbol, uint8(6), big.NewInt(1000000), big.NewInt(1700000000), big.NewInt(42))
			if err != nil {
				t.Fatal(err)
			}
			if tc.quarantined {
				data = make([]byte, decode.MaxEnvelopeBytes+1)
			}
			log := deploymentBoundsSourceLog(emitter.Events["TokenDeployed"], common.HexToAddress("0xe1"), 1, data)
			mirror, err := r.Decode(log)
			if err != nil {
				t.Fatal(err)
			}
			prove := func(address common.Address) (accounting.Provenance, error) {
				if address == common.HexToAddress(market.Raw.Emitter) {
					return accounting.Provenance{ABIRevision: market.ABIRevision}, nil
				}
				if address == log.Address && !tc.quarantined {
					return accounting.Provenance{ABIRevision: mirror.ABIRevision}, nil
				}
				t.Fatal("metadata provenance requested for an unavailable event")
				return accounting.Provenance{}, store.ErrIntegrity
			}
			facts, err := creationFacts([]*decode.DecodedEnvelope{market, mirror}, 1700000000, prove)
			if err != nil || len(facts) != 1 || facts[0].Token != strings.ToLower(common.HexToAddress("0xa1").Hex()) || facts[0].Pool != strings.ToLower(common.HexToAddress("0xb1").Hex()) {
				t.Fatalf("creation: %+v: %v", facts, err)
			}
			m := facts[0].Metadata
			if tc.quarantined {
				if m != nil || tokenMirror(mirror) {
					t.Fatalf("quarantined metadata: %+v", m)
				}
				return
			}
			name, symbol := tc.tokenName, tc.symbol
			if tc.name == "withheld name" || tc.name == "zero character" {
				name = ""
			}
			if tc.name == "withheld symbol" {
				symbol = ""
			}
			if m == nil || m.Name != name || m.Symbol != symbol || m.Decimals != 6 || m.TotalSupply != "1000000" {
				t.Fatalf("creation metadata: %+v", m)
			}
			changed := *mirror
			changed.Status = "quarantined"
			if tokenMirror(&changed) {
				t.Fatal("nondecoded event supplies metadata")
			}
			facts, err = creationFacts([]*decode.DecodedEnvelope{market, &changed}, 1700000000, prove)
			if err != nil || len(facts) != 1 || facts[0].Metadata != nil {
				t.Fatalf("status-gated metadata: %+v: %v", facts, err)
			}
		})
	}
}

func TestDeploymentLogBoundsRetention(t *testing.T) {
	ctx, _, pool := replayPostgres(t)
	r, _, emitter := deploymentBoundsRegistry(t)
	p := processorFor(pool)
	p.deps.Quarantine = &QuarantinePolicy{Registry: r, SourceStream: "deployment-bounds"}
	hops := make([]common.Address, 24000)
	for i := range hops {
		hops[i] = common.HexToAddress("0xa1")
	}
	arrayData, err := emitter.Events["BestRouteSwap"].Inputs.NonIndexed().Pack([32]byte{}, big.NewInt(1), big.NewInt(2), hops, big.NewInt(0), big.NewInt(1700000000), big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	logs := []types.Log{
		deploymentBoundsSourceLog(emitter.Events["TokenDeployed"], common.HexToAddress("0xe1"), 0, make([]byte, decode.MaxEnvelopeBytes+1)),
		deploymentBoundsSourceLog(emitter.Events["BestRouteSwap"], common.HexToAddress("0xe1"), 1, arrayData),
	}
	for _, log := range logs {
		result := source.FetchResult{Logs: []types.Log{log}, BlockTimestamps: map[uint64]int64{42: 1700000000}}
		for i := 0; i < 2; i++ {
			if err := p.RetainSourcePoison(ctx, result); err != nil {
				t.Fatalf("bounds retention: %v", err)
			}
		}
		var required bool
		var requirement, code string
		var data []byte
		if err := pool.QueryRow(ctx, `SELECT q.required,q.requirement,q.error_code,r.data FROM indexer.indexer_quarantine q JOIN indexer.indexer_raw_logs r USING(chain_id,transaction_hash,log_index) WHERE q.stream_id=$1 AND q.log_index=$2`, p.deps.Quarantine.SourceStream, log.Index).Scan(&required, &requirement, &code, &data); err != nil {
			t.Fatal(err)
		}
		if required || requirement != "optional.deployment_log_bounds" || code != "bounds" || !bytes.Equal(data, log.Data) {
			t.Fatalf("retained classification: required=%t requirement=%s code=%s", required, requirement, code)
		}
	}
	var degraded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_capability_degradation WHERE stream_id=$1 AND requirement='optional.deployment_log_bounds'`, p.deps.Quarantine.SourceStream).Scan(&degraded); err != nil || degraded != 2 {
		t.Fatalf("optional records: %d: %v", degraded, err)
	}
	malformed := deploymentBoundsSourceLog(emitter.Events["TokenDeployed"], common.HexToAddress("0xe1"), 2, []byte{1})
	result := source.FetchResult{Logs: []types.Log{malformed}, BlockTimestamps: map[uint64]int64{42: 1700000000}}
	if err := p.RetainSourcePoison(ctx, result); !errors.Is(err, store.ErrRequiredPoison) {
		t.Fatalf("required malformed event: %v", err)
	}
	var required bool
	var requirement string
	if err := pool.QueryRow(ctx, `SELECT required,requirement FROM indexer.indexer_quarantine WHERE stream_id=$1 AND log_index=2`, p.deps.Quarantine.SourceStream).Scan(&required, &requirement); err != nil || !required || requirement != "D02_021" {
		t.Fatalf("required classification: %t %s: %v", required, requirement, err)
	}
	missing := logs[0]
	missing.Index = 3
	if err := p.RetainSourcePoison(ctx, source.FetchResult{Logs: []types.Log{missing}}); !errors.Is(err, store.ErrRequiredPoison) {
		t.Fatalf("missing source timestamp remains required: %v", err)
	}
}

func TestDeploymentLogBoundsFactoryCreations(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	n := h.node
	manifest := strings.Repeat("a", 64)
	epochs := make([]source.AddressEpoch, 0, len(n.options.Epochs))
	for _, e := range n.options.Epochs {
		epochs = append(epochs, source.AddressEpoch{Address: strings.ToLower(e.Deployment.Address.Hex()), Role: e.Deployment.Role, ABIRevision: e.Deployment.ABIRevision, ManifestDigest: manifest, ActivationHeight: e.Deployment.From.Block})
	}
	committed, err := source.NewCommittedSource(n.ctx, source.CommittedOptions{Endpoints: map[string]string{"primary": n.endpoint}, GenesisHash: n.options.GenesisHash, ManifestDigest: manifest, Epochs: epochs})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := committed.Close(); err != nil {
			t.Error(err)
		}
	})
	durable := h.created.BlockNumber.Uint64()
	for _, tc := range []struct{ name, tokenName, symbol, cause string }{
		{"invalid UTF8", string([]byte{0xff, 0xfe}), "COIN", "invalid_utf8"},
		{"long name", strings.Repeat("\x00", decode.MaxFieldBytes+1), "COIN", "max_field_bytes"},
		{"zero character", "coin\x00", "COIN", "nul"},
		{"empty fields", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cause == "max_field_bytes" {
				tokenABI, code := creationSnapshotArtifact(t, "SidioraERC20")
				constructor, err := tokenABI.Pack("", tc.tokenName, tc.symbol, big.NewInt(1), common.HexToAddress(n.roles["SidioraFactory"].Address))
				if err != nil {
					t.Fatal(err)
				}
				if size := len(code) + len(constructor); size <= params.MaxInitCodeSize {
					t.Fatalf("oversized text init code: %d <= %d", size, params.MaxInitCodeSize)
				}
				auth := *n.auth
				auth.GasLimit = 12000000
				tx, err := n.contracts["SidioraFactory"].Transact(&auth, "createMarketFor", n.owner, tc.tokenName, tc.symbol, uint8(0), common.Address{})
				if err != nil {
					t.Fatal(err)
				}
				created, err := bind.WaitMined(n.ctx, n.client, tx)
				if err != nil {
					t.Fatal(err)
				}
				if created.Status != types.ReceiptStatusFailed || len(created.Logs) != 0 {
					t.Fatalf("init code limit must prevent creation: status=%d logs=%d", created.Status, len(created.Logs))
				}
				return
			}
			authority, created, address := n.createNamed(t, n.options, false, tc.tokenName, tc.symbol)
			qualifiedSource := &source.LiveQualifiedSource{RPC: committed, Authority: authority, DurableHeight: func() uint64 { return durable }}
			h.proc.deps.QualifiedBatchSource, h.proc.deps.Reserves = qualifiedSource, authority
			batch, receipts, err := qualifiedSource.FetchQualifiedBatch(n.ctx, durable+1, created.BlockNumber.Uint64())
			if err != nil || len(receipts) != 1 || len(batch.Records) == 0 {
				t.Fatalf("qualified factory source: receipts=%d records=%d: %v", len(receipts), len(batch.Records), err)
			}
			facts, err := qualifiedCreationFacts(n.ctx, receipts)
			if err != nil || len(facts) != 1 || facts[0].Metadata == nil {
				t.Fatalf("qualified factory creation: %+v: %v", facts, err)
			}
			if m := facts[0].Metadata; m.Name != "" || m.Symbol != tc.symbol || m.Decimals != 6 || m.TotalSupply == "" {
				t.Fatalf("factory metadata: %+v", m)
			}
			envelopes, err := receipts[0].QualifiedEvents(n.ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, env := range envelopes {
				if env.Status != "decoded" || env.EventName != "TokenDeployed" {
					continue
				}
				found = true
				var alerts []string
				if tc.cause != "" {
					alerts = []string{"withheld:name:" + tc.cause}
				}
				if env.Event == nil || !reflect.DeepEqual(env.Alerts, alerts) || env.Event.Args["name"] != "" {
					t.Fatalf("token envelope: %+v", env)
				}
				for _, raw := range created.Logs {
					if raw.Index == env.Raw.Identity.LogIndex && env.Raw.Data != "0x"+common.Bytes2Hex(raw.Data) {
						t.Fatal("factory raw metadata changed")
					}
				}
			}
			if !found {
				t.Fatal("factory TokenDeployed absent")
			}
			if err := h.proc.ProcessBatch(n.ctx, int64(durable)+1, created.BlockNumber.Int64()); err != nil {
				t.Fatal(err)
			}
			var wire []byte
			if err := h.pool.QueryRow(n.ctx, `SELECT payload FROM indexer.indexer_canonical_creations WHERE pool=$1`, strings.ToLower(address.Hex())).Scan(&wire); err != nil {
				t.Fatal(err)
			}
			var stored store.CreationFact
			if err := json.Unmarshal(wire, &stored); err != nil || !reflect.DeepEqual(stored, facts[0]) {
				t.Fatalf("stored factory creation: %+v: %v", stored, err)
			}
			var published []byte
			if err := h.pool.QueryRow(n.ctx, `SELECT envelope FROM indexer.delivery_outbox WHERE subject=$1 AND convert_from(envelope,'UTF8')::jsonb->'payload'->>'pool'=$2`, publisher.CanonicalCreationSubject, strings.ToLower(address.Hex())).Scan(&published); err != nil {
				t.Fatal(err)
			}
			var envelope delivery.Envelope
			if err := json.Unmarshal(published, &envelope); err != nil {
				t.Fatal(err)
			}
			var delivered store.CreationFact
			if err := json.Unmarshal(envelope.Payload, &delivered); err != nil || !reflect.DeepEqual(delivered, stored) {
				t.Fatalf("published factory creation: %+v: %v", delivered, err)
			}
			watermark, err := h.proc.deps.Store.GetCanonicalWatermark(n.ctx, 125, realStream)
			if err != nil || watermark != created.BlockNumber.Int64() {
				t.Fatalf("creation watermark: %d: %v", watermark, err)
			}
			durable = created.BlockNumber.Uint64()
		})
	}
}

func TestDeploymentLogBoundsCreationWithoutMetadata(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	ctx := h.node.ctx
	q := h.receipts[0]
	envelopes, err := q.QualifiedEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	available := make([]*decode.DecodedEnvelope, 0, len(envelopes))
	quarantined := 0
	for _, env := range envelopes {
		if env.EventName != "TokenDeployed" {
			available = append(available, env)
			continue
		}
		for _, raw := range h.created.Logs {
			if raw.Index != env.Raw.Identity.LogIndex {
				continue
			}
			log := *raw
			log.Data = make([]byte, decode.MaxEnvelopeBytes+1)
			bounded, err := h.authority.Registry().Decode(log)
			if err != nil || bounded.Status != "quarantined" || bounded.FailureReason != "bounds" || bounded.Event != nil || bounded.RawSummary == nil || bounded.RawSummary.DataBytes != len(log.Data) || bounded.Raw.Identity != env.Raw.Identity {
				t.Fatalf("bounded token metadata: %+v: %v", bounded, err)
			}
			available = append(available, bounded)
			quarantined++
		}
	}
	if quarantined != 1 || len(available) != len(envelopes) {
		t.Fatalf("creation envelope coverage: quarantined=%d available=%d total=%d", quarantined, len(available), len(envelopes))
	}
	_, _, timestamp, err := q.SourceContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := creationFacts(available, timestamp, func(address common.Address) (accounting.Provenance, error) { return q.SourceProvenance(ctx, address) })
	if err != nil || len(facts) != 1 || facts[0].Metadata != nil {
		t.Fatalf("creation without token metadata: %+v: %v", facts, err)
	}
	fact := facts[0]
	logIndex, err := strconv.ParseUint(fact.Position.LogIndex, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(fact)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.proc.deps.Store.CommitProjectedCanonicalTx(ctx, h.batch, func(ctx context.Context, tx store.CanonicalTx) ([]store.OutboxIntent, error) {
		if err := tx.InsertCreation(ctx, fact); err != nil {
			return nil, err
		}
		if err := h.proc.deps.Durable.StageCanonicalCreationTx(ctx, tx.Tx, fact); err != nil {
			return nil, err
		}
		return []store.OutboxIntent{{TransactionHash: fact.Position.TransactionHash, LogIndex: uint32(logIndex), Kind: publisher.CanonicalCreationSubject, Payload: wire}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	var stored, published []byte
	if err := h.pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_canonical_creations WHERE token=$1`, fact.Token).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT envelope FROM indexer.delivery_outbox WHERE subject=$1`, publisher.CanonicalCreationSubject).Scan(&published); err != nil {
		t.Fatal(err)
	}
	var decodedFact store.CreationFact
	if err := json.Unmarshal(stored, &decodedFact); err != nil || !reflect.DeepEqual(decodedFact, fact) {
		t.Fatalf("stored metadata unavailable: %+v: %v", decodedFact, err)
	}
	var envelope delivery.Envelope
	if err := json.Unmarshal(published, &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope.Payload, &decodedFact); err != nil || !reflect.DeepEqual(decodedFact, fact) {
		t.Fatalf("published metadata unavailable: %+v: %v", decodedFact, err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil || len(payload) != 10 || string(payload["metadata"]) != "null" {
		t.Fatalf("creation payload shape: %s: %v", envelope.Payload, err)
	}
}
