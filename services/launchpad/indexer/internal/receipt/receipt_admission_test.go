package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestReceiptAdmissionPinsPrepaymentReceiptsAndEveryCodeEpoch(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
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
	for _, mode := range []string{"prepayment block", "prepayment receipt", "missing epoch", "untrusted signature", "code unavailable", "code changed"} {
		t.Run(mode, func(t *testing.T) {
			a, closeClient := repair53Open(t, ctx, original)
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
			case "missing epoch", "untrusted signature":
				// Removing a binding must never authorize the original known signature.
				excluded := b.Receipt.Logs[0].Address
				epochs := a.options.Epochs[:0]
				for _, e := range a.options.Epochs {
					if e.Deployment.Address != excluded {
						epochs = append(epochs, e)
					}
				}
				a.options.Epochs = epochs
				if mode == "missing epoch" {
					var ds []decode.Deployment
					for _, e := range epochs {
						ds = append(ds, e.Deployment)
					}
					a.registry, err = decode.NewRegistry(a.options.CatalogJSON, ds)
					if err != nil {
						t.Fatal(err)
					}
				}
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
				input = repair53Replay(t, input)
				q, closeChanged := repair53Open(t, ctx, input)
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
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
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
			input = repair53Replay(t, input)
			a, closeClient := repair53Open(t, ctx, input)
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
