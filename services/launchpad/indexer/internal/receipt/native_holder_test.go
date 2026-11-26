package receipt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func nativeHolderReplay(t *testing.T, tenant bool) (Bundle, []*decode.DecodedEnvelope, accounting.FillV1) {
	t.Helper()
	ctx := context.Background()
	input := receiptReplayInputsFromProducer(t)
	authority, closeClient := receiptReplayOpen(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	hash := input.NativeSwap
	if tenant {
		hash = input.TenantSwap
	}
	fill, _ := receiptReplayFill(t, ctx, authority, hash)
	bundle, err := authority.Bundle(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	decoded := make([]*decode.DecodedEnvelope, len(bundle.Receipt.Logs))
	for i, log := range bundle.Receipt.Logs {
		decoded[i], err = authority.Registry().Decode(*log)
		if err != nil {
			t.Fatal(err)
		}
	}
	return bundle, decoded, fill
}

func nativeHolderAssertAttribution(t *testing.T, bundle Bundle, fill accounting.FillV1, a accounting.Attribution) {
	t.Helper()
	sender := fill.Attribution.TransactionSender
	if a.Quality != "qualified_calldata" || a.TransactionSender != sender {
		t.Fatalf("direct trade attribution: %+v", a)
	}
	for _, actor := range []*string{a.Payer, a.Recipient, a.Trader, a.RouterCaller} {
		if actor == nil || *actor != sender {
			t.Fatalf("direct trade actor: %+v", a)
		}
	}
	if len(a.Evidence) != 3 || a.Evidence[0].Kind != "router-calldata" {
		t.Fatalf("direct trade evidence: %+v", a.Evidence)
	}
	calldataHash := sha256.Sum256(bundle.Transaction.Data())
	if a.Evidence[0].ArtifactSHA256 != hex.EncodeToString(calldataHash[:]) || a.Evidence[0].ArtifactID != fill.Position.TransactionHash || a.Evidence[0].LogIndex != nil {
		t.Fatalf("calldata proof: %+v", a.Evidence[0])
	}
	receiptHash, err := digest(bundle.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"input-transfer", "output-transfer"} {
		proof := a.Evidence[i+1]
		index := i
		if fill.Side == "sell" {
			index = 1 - i
		}
		want := strconv.FormatUint(uint64(bundle.Receipt.Logs[index].Index), 10)
		if proof.Kind != kind || proof.LogIndex == nil || *proof.LogIndex != want || proof.ArtifactID != fill.Position.TransactionHash || proof.ArtifactSHA256 != hex.EncodeToString(receiptHash[:]) {
			t.Fatalf("settlement proof: %+v, want log %s", proof, want)
		}
	}
}

func TestNativeHolderAttributionRecordedSwaps(t *testing.T) {
	for name, tenant := range map[string]bool{"native": false, "tenant": true} {
		t.Run(name, func(t *testing.T) {
			bundle, _, fill := nativeHolderReplay(t, tenant)
			sender, err := coordinates(bundle)
			if err != nil || sender != fill.Attribution.TransactionSender {
				t.Fatalf("signed transaction sender: %s, %v", sender, err)
			}
			nativeHolderAssertAttribution(t, bundle, fill, fill.Attribution)
		})
	}
}

func nativeHolderAttribute(t *testing.T, b Bundle, decoded []*decode.DecodedEnvelope, f accounting.FillV1, fills ...accounting.FillV1) accounting.Attribution {
	t.Helper()
	f.Attribution = accounting.Attribution{TransactionSender: f.Attribution.TransactionSender, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}
	hash, err := digest(b.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	return attribute(b, decoded, f, f.Attribution.TransactionSender, hash, fills...)
}

func nativeHolderTransaction(b *Bundle, data []byte) {
	b.Transaction = types.NewTx(&types.LegacyTx{To: b.Transaction.To(), Gas: b.Transaction.Gas(), GasPrice: b.Transaction.GasPrice(), Value: b.Transaction.Value(), Data: data})
}

func TestNativeHolderAttributionRejectsChangedReceipt(t *testing.T) {
	for name, tenant := range map[string]bool{"native": false, "tenant": true} {
		t.Run(name, func(t *testing.T) {
			original, decoded, fill := nativeHolderReplay(t, tenant)
			for _, change := range []string{"settlement to pool", "settlement to other", "settlement amount", "duplicate settlement", "token from other", "swap sender", "long calldata"} {
				t.Run(change, func(t *testing.T) {
					b, err := clone(original)
					if err != nil {
						t.Fatal(err)
					}
					envelopes, err := clone(decoded)
					if err != nil {
						t.Fatal(err)
					}
					other := common.HexToAddress(fill.Token.Address)
					changedFill := fill
					switch change {
					case "settlement to pool":
						b.Receipt.Logs[0].Topics[2] = common.HexToHash(fill.PoolAddress)
					case "settlement to other":
						b.Receipt.Logs[0].Topics[2] = common.BytesToHash(other.Bytes())
					case "settlement amount":
						amount := new(big.Int).SetBytes(b.Receipt.Logs[0].Data)
						b.Receipt.Logs[0].Data = amount.Add(amount, big.NewInt(1)).FillBytes(make([]byte, 32))
					case "duplicate settlement":
						duplicate, err := clone(b.Receipt.Logs[0])
						if err != nil {
							t.Fatal(err)
						}
						for i, log := range b.Receipt.Logs {
							if log.Index >= 2 {
								log.Index++
							}
							if envelopes[i].Family != nil && envelopes[i].Family.EconomicFill {
								changedFill.Position.LogIndex = strconv.FormatUint(uint64(log.Index), 10)
							}
						}
						duplicate.Index = 2
						b.Receipt.Logs = append(b.Receipt.Logs[:2], append([]*types.Log{duplicate}, b.Receipt.Logs[2:]...)...)
						envelopes = append(envelopes[:2], append([]*decode.DecodedEnvelope{decoded[0]}, envelopes[2:]...)...)
					case "token from other":
						b.Receipt.Logs[1].Topics[1] = common.BytesToHash(other.Bytes())
					case "swap sender":
						for i, e := range envelopes {
							if e.Family != nil && e.Family.EconomicFill {
								e.Event.Args["sender"] = strings.ToLower(other.Hex())
								b.Receipt.Logs[i].Topics[2] = common.BytesToHash(other.Bytes())
							}
						}
					case "long calldata":
						nativeHolderTransaction(&b, append(b.Transaction.Data(), make([]byte, 32)...))
					}
					a := nativeHolderAttribute(t, b, envelopes, changedFill)
					if a.Quality != "unavailable" || a.Payer != nil || a.Recipient != nil || len(a.Evidence) != 0 {
						t.Fatalf("changed receipt attributed: %+v", a)
					}
				})
			}
		})
	}
}

func nativeHolderSell(t *testing.T, b *Bundle, decoded []*decode.DecodedEnvelope, f *accounting.FillV1) {
	t.Helper()
	f.Side = "sell"
	sender, pool := common.HexToHash(f.Attribution.TransactionSender), common.HexToHash(f.PoolAddress)
	b.Receipt.Logs[0].Topics[1], b.Receipt.Logs[0].Topics[2] = common.BytesToHash(decode.LaunchpadModuleAddress.Bytes()), sender
	b.Receipt.Logs[1].Topics[1], b.Receipt.Logs[1].Topics[2] = sender, pool
	input, err := accounting.Uint(f.TokenAmountRaw)
	if err != nil {
		t.Fatal(err)
	}
	output, err := accounting.Uint(f.USDLAmountRaw)
	if err != nil {
		t.Fatal(err)
	}
	data := b.Transaction.Data()
	copy(data[:4], crypto.Keccak256([]byte("sell(address,uint256,uint256,uint256)"))[:4])
	input.FillBytes(data[36:68])
	nativeHolderTransaction(b, data)
	for i, e := range decoded {
		log := b.Receipt.Logs[i]
		if e.Family != nil && e.Family.EconomicFill {
			e.Event.Args["isBuy"], e.Event.Args["amountIn"], e.Event.Args["amountOut"] = false, input.String(), output.String()
			copy(log.Data[:32], make([]byte, 32))
			input.FillBytes(log.Data[32:64])
			output.FillBytes(log.Data[64:96])
		}
		if e.EmitterRole == "Router" && e.EventName == "Buy" {
			e.EventName, e.Event.EventName = "Sell", "Sell"
			e.Event.Sig = "Sell(address,address,uint256,uint256)"
			delete(e.Event.Args, "buyer")
			delete(e.Event.Args, "usdlIn")
			delete(e.Event.Args, "tokensOut")
			e.Event.Args["seller"], e.Event.Args["tokensIn"], e.Event.Args["usdlOut"] = f.Attribution.TransactionSender, input.String(), output.String()
			log.Topics[0] = crypto.Keccak256Hash([]byte("Sell(address,address,uint256,uint256)"))
			input.FillBytes(log.Data[:32])
			output.FillBytes(log.Data[32:64])
		}
	}
}

func TestNativeHolderAttributionNativeSell(t *testing.T) {
	b, decoded, fill := nativeHolderReplay(t, false)
	nativeHolderSell(t, &b, decoded, &fill)
	a := nativeHolderAttribute(t, b, decoded, fill)
	nativeHolderAssertAttribution(t, b, fill, a)
	b.Receipt.Logs[0].Topics[1] = common.HexToHash(fill.PoolAddress)
	if a := nativeHolderAttribute(t, b, decoded, fill); a.Quality != "unavailable" {
		t.Fatalf("native sell paid by pool attributed: %+v", a)
	}
}

func TestNativeHolderAttributionContractBuyAndSell(t *testing.T) {
	input := receiptReplayInputsFromProducer(t)
	for _, side := range []string{"buy", "sell"} {
		t.Run(side, func(t *testing.T) {
			b, decoded, fill := nativeHolderReplay(t, false)
			if side == "sell" {
				nativeHolderSell(t, &b, decoded, &fill)
			}
			fill.ActualEmitter = input.Options.Manifest.Roles["EventEmitter"].Address
			for i, e := range decoded {
				if e.Family != nil && e.Family.EconomicFill {
					b.Receipt.Logs[i].Address = common.HexToAddress(fill.ActualEmitter)
				}
			}
			if a := nativeHolderAttribute(t, b, decoded, fill); a.Quality != "unavailable" {
				t.Fatalf("contract settlement held by module attributed: %+v", a)
			}
			settlementTopic := 2
			if side == "sell" {
				settlementTopic = 1
			}
			b.Receipt.Logs[0].Topics[settlementTopic] = common.HexToHash(fill.PoolAddress)
			a := nativeHolderAttribute(t, b, decoded, fill)
			nativeHolderAssertAttribution(t, b, fill, a)
			if reflect.DeepEqual(a, fill.Attribution) {
				t.Fatal("changed receipt must bind its own transfer evidence")
			}
		})
	}
}
