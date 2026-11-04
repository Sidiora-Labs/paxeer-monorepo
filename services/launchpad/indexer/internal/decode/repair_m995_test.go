package decode_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type repair995Recorded struct {
	Key    string          `json:"k"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type repair995Producer struct {
	Options    receipt.RPCOptions
	Markets    []receipt.DiscoveredMarket
	NativeSwap common.Hash
	Responses  map[string]repair995Recorded
}

func repair995Load(t *testing.T) repair995Producer {
	t.Helper()
	raw, err := os.ReadFile("../receipt/testdata/repair53-producer.json")
	if err != nil {
		t.Fatal(err)
	}
	var input repair995Producer
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input.Options.Manifest, deployment.Current()) || len(input.Markets) != 1 || input.NativeSwap == (common.Hash{}) {
		t.Fatal("approved native producer capture required")
	}
	return input
}

func repair995Key(method string, params []any) (string, error) {
	if params == nil {
		params = []any{}
	}
	if method == "eth_call" && len(params) == 2 {
		if call, ok := params[0].(map[string]any); ok && call["from"] == (common.Address{}).Hex() {
			delete(call, "from")
		}
	}
	raw, err := json.Marshal(params)
	return method + " " + string(raw), err
}

func repair995Replay(t *testing.T, input repair995Producer) string {
	t.Helper()
	responses := make(map[string]repair995Recorded, len(input.Responses))
	for key, response := range input.Responses {
		method, encoded, ok := strings.Cut(key, " ")
		var params []any
		if !ok || json.Unmarshal([]byte(encoded), &params) != nil {
			t.Fatalf("invalid recorded key: %s", key)
		}
		canonical, err := repair995Key(method, params)
		if err != nil {
			t.Fatal(err)
		}
		if prior, exists := responses[canonical]; exists && (!bytes.Equal(prior.Result, response.Result) || !bytes.Equal(prior.Error, response.Error)) {
			t.Fatalf("conflicting producer read: %s", canonical)
		}
		responses[canonical] = response
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage
			Method string
			Params []any
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		key, err := repair995Key(request.Method, request.Params)
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		response, exists := responses[key]
		if !exists {
			t.Errorf("unrecorded producer read: %s", key)
			http.Error(w, "unrecorded producer read", 502)
			return
		}
		result := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if len(response.Error) > 0 {
			result["error"] = response.Error
		} else {
			result["result"] = response.Result
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func repair995Bundle(t *testing.T, input repair995Producer) (*types.Transaction, *types.Receipt) {
	t.Helper()
	var rec types.Receipt
	key := " [\"" + input.NativeSwap.Hex() + "\"]"
	recorded, ok := input.Responses["eth_getTransactionReceipt"+key]
	if !ok || len(recorded.Result) == 0 || len(recorded.Error) != 0 {
		t.Fatal("native receipt absent from producer capture")
	}
	if err := json.Unmarshal(recorded.Result, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.BlockNumber == nil || !rec.BlockNumber.IsUint64() {
		t.Fatal("recorded native receipt has no bounded block number")
	}
	var block struct {
		Hash         common.Hash
		Number       string
		Transactions []*types.Transaction
	}
	blockNumber := "0x" + rec.BlockNumber.Text(16)
	blockKey := "eth_getBlockByNumber [\"" + blockNumber + "\",true]"
	recorded, ok = input.Responses[blockKey]
	if !ok || len(recorded.Result) == 0 || len(recorded.Error) != 0 {
		t.Fatalf("signed native block absent from producer capture: %s", blockKey)
	}
	if err := json.Unmarshal(recorded.Result, &block); err != nil {
		t.Fatal(err)
	}
	if block.Hash != rec.BlockHash || block.Number != blockNumber {
		t.Fatal("recorded native block differs from receipt identity")
	}
	var tx *types.Transaction
	for index, candidate := range block.Transactions {
		if candidate != nil && candidate.Hash() == input.NativeSwap {
			if tx != nil {
				t.Fatal("duplicate signed transaction in producer block")
			}
			if uint(index) != rec.TransactionIndex {
				t.Fatal("recorded transaction position differs from receipt identity")
			}
			tx = candidate
		}
	}
	if tx == nil {
		t.Fatal("signed native transaction absent from recorded block")
	}
	if tx.Hash() != input.NativeSwap || rec.TxHash != tx.Hash() || tx.ChainId().Cmp(big.NewInt(125)) != 0 {
		t.Fatal("native signed transaction identity lost")
	}
	return tx, &rec
}

func repair995Registry(t *testing.T, input repair995Producer, origin bool, poolActive bool) *decode.Registry {
	t.Helper()
	var catalog decode.Catalog
	if err := json.Unmarshal(input.Options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	revisions := map[string]string{}
	for _, c := range catalog.Contracts {
		revisions[c.Name] = c.ABIRevision
	}
	market := input.Markets[0]
	ds := []decode.Deployment{
		{Address: decode.LaunchpadPrecompileAddress, Role: decode.LaunchpadPrecompileRole, ABIRevision: revisions["EventEmitter"]},
		{Address: market.Token, Role: "SidioraERC20", ABIRevision: revisions["SidioraERC20"]},
		{Address: common.HexToAddress(input.Options.Manifest.Roles["USDL"].Address), Role: "SidioraERC20", ABIRevision: revisions["SidioraERC20"]},
	}
	if poolActive {
		ds = append(ds, decode.Deployment{Address: market.Pool, Role: "SidioraPool", ABIRevision: revisions["SidioraPool"]})
	}
	if origin {
		ds = append(ds, decode.Deployment{Address: decode.LaunchpadPrecompileAddress, Role: "SidioraPool", ABIRevision: revisions["SidioraPool"]})
	}
	r, err := decode.NewRegistry(input.Options.CatalogJSON, ds)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func repair995Markets(input repair995Producer) []decode.ActorMarketBinding {
	return []decode.ActorMarketBinding{{Pool: input.Markets[0].Pool, Token: input.Markets[0].Token, Settlement: common.HexToAddress(input.Options.Manifest.Roles["USDL"].Address)}}
}

func repair995Swap(t *testing.T, envelopes []*decode.DecodedEnvelope) *decode.DecodedEnvelope {
	t.Helper()
	var swap *decode.DecodedEnvelope
	for _, env := range envelopes {
		if env.EventName == "Swap" {
			if swap != nil {
				t.Fatal("duplicate primary Swap")
			}
			swap = env
		}
	}
	if swap == nil || swap.Status != "decoded" || swap.Event == nil || swap.Family == nil || !swap.Family.EconomicFill || swap.Family.Authority != "primary" || swap.Actors == nil {
		t.Fatal("native primary economic Swap lost")
	}
	return swap
}

func repair995AssertActors(t *testing.T, swap *decode.DecodedEnvelope, tx *types.Transaction, rec *types.Receipt) {
	t.Helper()
	sender, err := types.Sender(types.LatestSignerForChainID(big.NewInt(125)), tx)
	if err != nil {
		t.Fatal(err)
	}
	actor := swap.Actors
	output := rec.Logs[1]
	wantRecipient := strings.ToLower(common.BytesToAddress(output.Topics[2].Bytes()).Hex())
	if actor.TransactionOrigin == nil || *actor.TransactionOrigin != strings.ToLower(sender.Hex()) || actor.EventCaller == nil || *actor.EventCaller != swap.Event.Args["sender"] || actor.Recipient == nil || *actor.Recipient != wantRecipient || actor.Beneficiary == nil || *actor.Beneficiary != wantRecipient || actor.AttributionConfidence != "receipt_evidence" || actor.AttributionMethod != "unique_pool_output_transfer" || actor.Revision != "actor-v1" {
		t.Fatalf("native actor evidence did not reach consumer: %+v", actor)
	}
	want := []decode.RawIdentity{swap.Raw.Identity, {ChainID: 125, BlockHash: rec.BlockHash.Hex(), TransactionHash: rec.TxHash.Hex(), LogIndex: output.Index}}
	if !reflect.DeepEqual(actor.EvidenceIdentity, want) {
		t.Fatalf("receipt evidence identity changed: got %+v want %+v", actor.EvidenceIdentity, want)
	}
}

func TestRepairM_995_NativeReceiptAttribution(t *testing.T) {
	input := repair995Load(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, repair995Replay(t, input), "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	authority, err := receipt.NewRPCQualification(ctx, client, input.Options)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	batch, qualified, err := authority.FetchQualifiedBatch(ctx, bundle.Header.Number.Uint64(), bundle.Header.Number.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.ValidateBindings(ctx, batch, qualified); err != nil {
		t.Fatal(err)
	}
	if len(qualified) != 1 {
		t.Fatalf("expected one producer receipt, got %d", len(qualified))
	}
	envelopes, err := qualified[0].QualifiedEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	swap := repair995Swap(t, envelopes)
	if swap.Raw.Emitter != strings.ToLower(decode.LaunchpadPrecompileAddress.Hex()) || (swap.EmitterRole != decode.LaunchpadPrecompileRole && swap.EmitterRole != "SidioraPool") {
		t.Fatal("actual native emission did not reach qualified consumer")
	}
	repair995AssertActors(t, swap, bundle.Transaction, bundle.Receipt)
	fills, err := qualified[0].Fills(ctx)
	if err != nil || len(fills) != 1 || fills[0].ActualEmitter != swap.Raw.Emitter || fills[0].Position.TransactionHash != input.NativeSwap.Hex() || fills[0].TokenAmountRaw != swap.Event.Args["amountOut"] {
		t.Fatalf("qualified accounting consumer lost the native fill: %+v, %v", fills, err)
	}
	proof, err := qualified[0].SourceProvenance(ctx, decode.LaunchpadPrecompileAddress)
	if err != nil || proof.ABIRevision != swap.ABIRevision || proof.EmitterCodeEpoch == "" || fills[0].Provenance.EmitterCodeEpoch != proof.EmitterCodeEpoch {
		t.Fatal("native source provenance lost", err)
	}
}

func TestRepairM_995_NativeRoleProvenance(t *testing.T) {
	input := repair995Load(t)
	tx, rec := repair995Bundle(t, input)
	for _, origin := range []bool{false, true} {
		role := decode.LaunchpadPrecompileRole
		if origin {
			role = "SidioraPool"
		}
		t.Run(role, func(t *testing.T) {
			r := repair995Registry(t, input, origin, true)
			envelopes, err := r.DecodeTransactionReceipt(tx, rec, repair995Markets(input))
			if err != nil {
				t.Fatal(err)
			}
			swap := repair995Swap(t, envelopes)
			if swap.EmitterRole != role {
				t.Fatalf("verified native role changed: %s", swap.EmitterRole)
			}
			repair995AssertActors(t, swap, tx, rec)
		})
	}
}

func TestRepairM_995_ConservativeEvidence(t *testing.T) {
	input := repair995Load(t)
	for _, name := range []string{"absent", "ambiguous", "wrong_asset", "wrong_pool", "wrong_amount", "after_swap", "zero_recipient", "malformed_transfer", "unattested_pool", "missing_market"} {
		t.Run(name, func(t *testing.T) {
			tx, rec := repair995Bundle(t, input)
			output := rec.Logs[1]
			markets := repair995Markets(input)
			switch name {
			case "absent":
				rec.Logs = append(rec.Logs[:1], rec.Logs[2:]...)
			case "ambiguous":
				for _, l := range rec.Logs {
					l.Index *= 2
				}
				duplicate := *output
				duplicate.Index++
				rec.Logs = append(rec.Logs, &duplicate)
			case "wrong_asset":
				output.Address = markets[0].Settlement
			case "wrong_pool":
				output.Topics[1] = common.HexToHash("0x1234")
			case "wrong_amount":
				output.Data = common.LeftPadBytes(big.NewInt(1).Bytes(), 32)
			case "after_swap":
				output.Index = rec.Logs[len(rec.Logs)-1].Index + 1
			case "zero_recipient":
				output.Topics[2] = common.Hash{}
			case "malformed_transfer":
				output.Data = []byte{1}
			case "missing_market":
				markets = nil
			}
			r := repair995Registry(t, input, false, name != "unattested_pool")
			envelopes, err := r.DecodeTransactionReceipt(tx, rec, markets)
			if err != nil {
				t.Fatal(err)
			}
			swap := repair995Swap(t, envelopes)
			actor := swap.Actors
			if actor.Recipient != nil || actor.Beneficiary != nil || actor.AttributionConfidence != "unknown" || actor.AttributionMethod != "unavailable" || !reflect.DeepEqual(actor.EvidenceIdentity, []decode.RawIdentity{swap.Raw.Identity}) {
				t.Fatalf("%s produced unsupported attribution: %+v", name, actor)
			}
			if actor.TransactionOrigin == nil || actor.EventCaller == nil {
				t.Fatal("missing optional evidence removed authenticated actors")
			}
		})
	}
}

func TestRepairM_995_ReceiptIdentity(t *testing.T) {
	input := repair995Load(t)
	r := repair995Registry(t, input, false, true)
	for _, name := range []string{"transaction", "block_hash", "block_number", "transaction_index", "receipt_transaction", "failed_receipt", "conflicting_duplicate"} {
		t.Run(name, func(t *testing.T) {
			tx, rec := repair995Bundle(t, input)
			switch name {
			case "transaction":
				rec.Logs[1].TxHash = common.HexToHash("0x1234")
			case "block_hash":
				rec.Logs[1].BlockHash = common.HexToHash("0x1234")
			case "block_number":
				rec.Logs[1].BlockNumber++
			case "transaction_index":
				rec.Logs[1].TxIndex++
			case "receipt_transaction":
				rec.TxHash = common.HexToHash("0x1234")
			case "failed_receipt":
				rec.Status = types.ReceiptStatusFailed
			case "conflicting_duplicate":
				duplicate := *rec.Logs[1]
				duplicate.Data = common.LeftPadBytes(big.NewInt(1).Bytes(), 32)
				rec.Logs = append(rec.Logs, &duplicate)
			}
			if _, err := r.DecodeTransactionReceipt(tx, rec, repair995Markets(input)); err == nil {
				t.Fatalf("accepted mismatched receipt evidence: %s", name)
			}
		})
	}
	// An identical replay of one raw identity is one transfer, not ambiguity.
	tx, rec := repair995Bundle(t, input)
	rec.Logs = append(rec.Logs, rec.Logs[1])
	envelopes, err := r.DecodeTransactionReceipt(tx, rec, repair995Markets(input))
	if err != nil {
		t.Fatal(err)
	}
	repair995AssertActors(t, repair995Swap(t, envelopes), tx, rec)
}
