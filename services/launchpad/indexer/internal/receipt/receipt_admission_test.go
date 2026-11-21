package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestReceiptAdmissionPinsPrepaymentReceiptsAndEveryCodeEpoch(t *testing.T) {
	ctx := context.Background()
	original := receiptReplayInputsFromProducer(t)
	base, closeBase := receiptReplayOpen(t, ctx, original)
	defer closeBase()
	if err := base.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	base = base.current()
	bundle, err := base.Bundle(ctx, original.TenantSwap)
	if err != nil {
		t.Fatal(err)
	}

	priorBundle, err := base.Bundle(ctx, original.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	prepayment := priorBundle.Receipt
	actual := bundle
	actual.PrepayReceipts = []*types.Receipt{prepayment}

	if err = base.ValidateReceipt(ctx, bundle); err != nil {
		t.Fatalf("recorded receipt admission: %v", err)
	}
	if _, err = coordinates(actual); err != nil {
		t.Fatalf("prepayment coordinates height=%s index=%d logs=%d: %v", prepayment.BlockNumber, prepayment.TransactionIndex, len(prepayment.Logs), err)
	}
	if err = base.ValidateReceipt(ctx, actual); err != nil {
		t.Fatalf("pinned prepayment receipt: %v", err)
	}
	for _, mode := range []string{"prepayment block", "prepayment receipt", "untrusted signature", "code unavailable", "code changed"} {
		t.Run(mode, func(t *testing.T) {
			a, closeClient := receiptReplayOpen(t, ctx, original)
			defer closeClient()
			if err := a.Rehydrate(ctx, original.Markets); err != nil {
				t.Fatal(err)
			}
			a = a.current()
			b, err := clone(bundle)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "prepayment block", "prepayment receipt":
				prior, err := clone(prepayment)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "prepayment block" {
					prior.BlockHash = common.HexToHash("0xbad")
				} else {
					prior.GasUsed++
				}
				b.PrepayReceipts = []*types.Receipt{prior}
			case "untrusted signature":
				// A registry binding alone cannot qualify an emitter outside the authority epochs.
				excluded := b.Receipt.Logs[0].Address
				epochs := a.options.Epochs[:0]
				for _, e := range a.options.Epochs {
					if e.Deployment.Address != excluded {
						epochs = append(epochs, e)
					}
				}
				a.options.Epochs = epochs
			case "code unavailable", "code changed":
				input, err := clone(original)
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for key, response := range input.Responses {
					if !strings.HasPrefix(key, "eth_getCode ") || !strings.Contains(strings.ToLower(key), strings.ToLower(bundle.BlockHash.Hex())) {
						continue
					}
					if mode == "code unavailable" {
						response.Result = nil
						response.Error = json.RawMessage(`{"code":-32000,"message":"state unavailable"}`)
					} else {
						response.Error = nil
						response.Result = json.RawMessage(`"0x00"`)
					}
					input.Responses[key] = response
					count++
				}
				if count == 0 {
					t.Fatal("pinned code evidence absent")
				}
				input = receiptReplayReplay(t, input)
				q, closeChanged := receiptReplayOpen(t, ctx, input)
				defer closeChanged()
				a.client = q.client
			}
			if err = a.ValidateReceipt(ctx, b); err == nil {
				t.Fatal("unbound receipt evidence admitted")
			}
		})
	}
}

func TestTenantAdmissionRequiresAvailableParentRegistryState(t *testing.T) {
	ctx := context.Background()
	original := receiptReplayInputsFromProducer(t)
	base, closeBase := receiptReplayOpen(t, ctx, original)
	defer closeBase()
	bundle, err := base.Bundle(ctx, original.TenantSwap)
	if err != nil {
		t.Fatal(err)
	}
	if err = base.ValidateSignedDomain(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"header", "receipt", "parent unavailable", "parent identity", "registry unavailable", "registry encoding", "registry identity"} {
		t.Run(mode, func(t *testing.T) {
			input, err := clone(original)
			if err != nil {
				t.Fatal(err)
			}
			b, err := clone(bundle)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "header" {
				b.Header.Time++
			}
			if mode == "receipt" {
				b.Receipt.GasUsed++
			}
			changed := 0
			selector := common.Bytes2Hex(crypto.Keccak256([]byte("getChain(uint32)"))[:4])
			for key, response := range input.Responses {
				parent := strings.Contains(strings.ToLower(key), strings.ToLower(bundle.Header.ParentHash.Hex()))
				if (mode == "parent unavailable" || mode == "parent identity") && parent && strings.HasPrefix(key, "eth_getBlockByHash ") {
					if mode == "parent unavailable" {
						response.Result = nil
						response.Error = json.RawMessage(`{"code":-32000,"message":"parent unavailable"}`)
					} else {
						var h map[string]json.RawMessage
						if err = json.Unmarshal(response.Result, &h); err != nil {
							t.Fatal(err)
						}
						h["number"] = json.RawMessage(`"0x1"`)
						response.Result, err = json.Marshal(h)
						if err != nil {
							t.Fatal(err)
						}
					}
					input.Responses[key] = response
					changed++
				}
				if strings.HasPrefix(mode, "registry") && parent && strings.HasPrefix(key, "eth_call ") && strings.Contains(key, selector) {
					switch mode {
					case "registry unavailable":
						response.Result = nil
						response.Error = json.RawMessage(`{"code":-32000,"message":"registry unavailable"}`)
					case "registry encoding":
						response.Result = json.RawMessage(`"0x01"`)
					case "registry identity":
						response.Result = json.RawMessage(`"0x` + strings.Repeat("0", 11*64) + `"`)
					}
					input.Responses[key] = response
					changed++
				}
			}
			if mode != "header" && mode != "receipt" && changed == 0 {
				t.Fatal("parent admission evidence absent")
			}
			input = receiptReplayReplay(t, input)
			a, closeClient := receiptReplayOpen(t, ctx, input)
			defer closeClient()
			if err = a.ValidateSignedDomain(ctx, b); err == nil {
				t.Fatal("unbound tenant domain admitted")
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = base.ValidateSignedDomain(canceled, bundle); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled domain: %v", err)
	}
}

func TestEmitterOutsideEpochsReceiptKeepsQualifiedEmitters(t *testing.T) {
	ctx := context.Background()
	original := receiptReplayInputsFromProducer(t)
	a, closeClient := receiptReplayOpen(t, ctx, original)
	defer closeClient()
	if err := a.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	a = a.current()
	b, err := a.Bundle(ctx, original.TenantSwap)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Qualify(ctx, a.registry, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Receipt.Logs) < 2 {
		t.Fatal("receipt must contain multiple emitters")
	}
	excluded := common.HexToAddress(a.options.Manifest.Roles["FeeAccumulator"].Address)
	known := false
	for _, l := range b.Receipt.Logs {
		if l.Address != excluded {
			continue
		}
		env, err := a.registry.Decode(*l)
		if err != nil || env.Status != "decoded" || env.Event == nil || env.EventSignature == "" {
			t.Fatalf("qualified catalog event required: %+v, %v", env, err)
		}
		known = true
	}
	if !known {
		t.Fatal("receipt must contain a qualified fee accumulator event")
	}
	var epochs []RPCCodeEpoch
	var deployments []decode.Deployment
	removed := 0
	for _, e := range a.options.Epochs {
		if e.Deployment.Address == excluded {
			removed++
			continue
		}
		epochs = append(epochs, e)
		deployments = append(deployments, e.Deployment)
	}
	if removed == 0 {
		t.Fatal("emitter must have a qualified epoch")
	}
	a.options.Epochs = epochs
	a.registry, err = decode.NewRegistry(a.options.CatalogJSON, deployments)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range b.Receipt.Logs {
		if l.Address != excluded {
			continue
		}
		if _, err := a.epoch(excluded, decode.Order{Block: l.BlockNumber, Transaction: l.TxIndex, Log: l.Index}); err == nil {
			t.Fatal("emitter retains an authority epoch")
		}
		env, err := a.registry.Decode(*l)
		if err != nil || env.Status != "opaque" || !env.Opaque || env.Event != nil || env.Family != nil || env.FailureReason != "untrusted_emitter" {
			t.Fatalf("unbound receipt log: %+v, %v", env, err)
		}
	}
	if err = a.ValidateReceipt(ctx, b); err != nil {
		t.Fatalf("receipt with an unbound emitter: %v", err)
	}
	after, err := Qualify(ctx, a.registry, a, b)
	if err != nil {
		t.Fatal(err)
	}
	fills, err := after.Fills(ctx)
	if err != nil || len(fills) == 0 || !reflect.DeepEqual(fills, before.fills) {
		t.Fatalf("qualified fills changed: %+v, %v", fills, err)
	}
	for _, f := range fills {
		if strings.EqualFold(f.ActualEmitter, excluded.Hex()) {
			t.Fatal("unbound emitter supplies a fill")
		}
	}
	var wantEvents []*decode.Event
	var wantEnvelopes []*decode.DecodedEnvelope
	wantSelected := map[uint32]bool{}
	for _, env := range before.envelopes {
		if strings.EqualFold(env.Raw.Emitter, excluded.Hex()) {
			if after.selected[uint32(env.Raw.Identity.LogIndex)] {
				t.Fatal("unbound emitter selected")
			}
			continue
		}
		wantEnvelopes = append(wantEnvelopes, env)
		wantSelected[uint32(env.Raw.Identity.LogIndex)] = true
		if env.Family != nil && (env.Family.Authority == "primary" || env.Family.Authority == valueNative) {
			wantEvents = append(wantEvents, env.Event)
		}
		proof, err := after.SourceProvenance(ctx, common.HexToAddress(env.Raw.Emitter))
		if err != nil || proof != before.provenance[env.Raw.Emitter] {
			t.Fatalf("qualified emitter provenance changed: %+v, %v", proof, err)
		}
	}
	if len(wantEnvelopes) == 0 || len(wantEnvelopes) >= len(before.envelopes) {
		t.Fatal("receipt must retain decoded events beside the unbound emitter")
	}
	events, err := after.Events(ctx)
	if err != nil || !reflect.DeepEqual(events, wantEvents) || !reflect.DeepEqual(after.selected, wantSelected) {
		t.Fatalf("qualified events or selection changed: %+v, %v", events, err)
	}
	envelopes, err := after.QualifiedEvents(ctx)
	if err != nil || len(envelopes) != len(wantEnvelopes) {
		t.Fatalf("qualified envelope count: %d, %v", len(envelopes), err)
	}
	for i, env := range envelopes {
		want := wantEnvelopes[i]
		if env.Status != "decoded" || !reflect.DeepEqual(env.Raw, want.Raw) || !reflect.DeepEqual(env.Event, want.Event) || !reflect.DeepEqual(env.Family, want.Family) || env.EmitterRole != want.EmitterRole || env.ABIRevision != want.ABIRevision {
			t.Fatalf("qualified envelope changed: %+v", env)
		}
	}
	if _, err = after.SourceProvenance(ctx, excluded); !errors.Is(err, ErrQualification) || len(after.provenance) != len(before.provenance)-1 {
		t.Fatalf("unbound emitter provenance: %v", err)
	}
	logs, err := after.SourceLogs(ctx)
	originalLogs, originalErr := before.SourceLogs(ctx)
	if err != nil || originalErr != nil || !reflect.DeepEqual(logs, originalLogs) {
		t.Fatal("receipt raw logs changed", err, originalErr)
	}
}

func TestEmitterOutsideEpochsReceiptAdmission(t *testing.T) {
	ctx := context.Background()
	original := receiptReplayInputsFromProducer(t)
	a, closeClient := receiptReplayOpen(t, ctx, original)
	defer closeClient()
	if err := a.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	a = a.current()
	b, err := a.Bundle(ctx, original.TenantSwap)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.ValidateReceipt(ctx, b); err != nil {
		t.Fatal(err)
	}
	if len(b.Receipt.Logs) == 0 {
		t.Fatal("receipt logs required")
	}
	excluded := b.Receipt.Logs[0].Address
	env, err := a.registry.Decode(*b.Receipt.Logs[0])
	if err != nil || env.Status != "decoded" || env.EventName != "Transfer" || env.Event == nil {
		t.Fatalf("qualified Transfer required: %+v, %v", env, err)
	}
	var epochs []RPCCodeEpoch
	var deployments []decode.Deployment
	for _, e := range a.options.Epochs {
		if e.Deployment.Address != excluded {
			epochs = append(epochs, e)
			deployments = append(deployments, e.Deployment)
		}
	}
	if len(epochs) == len(a.options.Epochs) {
		t.Fatal("emitter epoch required")
	}
	a.options.Epochs = epochs
	a.registry, err = decode.NewRegistry(a.options.CatalogJSON, deployments)
	if err != nil {
		t.Fatal(err)
	}
	env, err = a.registry.Decode(*b.Receipt.Logs[0])
	if err != nil || env.Status != "opaque" || !env.Opaque || env.Event != nil || env.FailureReason != "untrusted_emitter" {
		t.Fatalf("unbound Transfer stays opaque: %+v, %v", env, err)
	}
	if err = a.ValidateReceipt(ctx, b); err != nil {
		t.Fatal(err)
	}
}
