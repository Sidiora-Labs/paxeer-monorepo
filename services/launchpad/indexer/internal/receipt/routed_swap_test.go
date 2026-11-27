package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func routedSwapReplay(t *testing.T, contract bool) (Bundle, []*decode.DecodedEnvelope, []accounting.FillV1) {
	t.Helper()
	b, original, first := nativeHolderReplay(t, false)
	nativeHolderSell(t, &b, original, &first)
	second, err := clone(first)
	if err != nil {
		t.Fatal(err)
	}
	second.Side, second.FeeDisposition = "buy", "distributed_usdl"
	second.Token.Address = strings.ToLower(common.BytesToAddress(crypto.Keccak256(common.HexToAddress(first.Token.Address).Bytes())).Hex())
	second.PoolAddress = strings.ToLower(common.BytesToAddress(crypto.Keccak256(common.HexToAddress(first.PoolAddress).Bytes())).Hex())
	if contract {
		emitter := receiptReplayInputsFromProducer(t).Options.Manifest.Roles["EventEmitter"].Address
		first.ActualEmitter, second.ActualEmitter = emitter, emitter
	}
	fills := []accounting.FillV1{first, second}
	var swap, route, mirror int
	for i, e := range original {
		switch e.EventName {
		case "Swap":
			swap = i
		case "Sell":
			route = i
		case "RouterTrade":
			mirror = i
		}
	}
	logs := make([]*types.Log, 10)
	envelopes := make([]*decode.DecodedEnvelope, 10)
	for i, source := range []int{1, 0, swap, 0, 1, 2, 3, swap, route, mirror} {
		logs[i], err = clone(b.Receipt.Logs[source])
		if err != nil {
			t.Fatal(err)
		}
		envelopes[i], err = clone(original[source])
		if err != nil {
			t.Fatal(err)
		}
		logs[i].Index = uint(i)
	}
	b.Receipt.Logs = logs
	setTransfer := func(index int, asset, from, to, amount string) {
		logs[index].Address = common.HexToAddress(asset)
		logs[index].Topics = []common.Hash{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")), common.HexToHash(from), common.HexToHash(to)}
		value, err := accounting.Uint(amount)
		if err != nil {
			t.Fatal(err)
		}
		logs[index].Data = value.FillBytes(make([]byte, 32))
	}
	sender := first.Attribution.TransactionSender
	holderOut, holderIn, relay := first.PoolAddress, second.PoolAddress, strings.ToLower(b.Transaction.To().Hex())
	if !contract {
		holderOut, holderIn, relay = strings.ToLower(decode.LaunchpadModuleAddress.Hex()), strings.ToLower(decode.LaunchpadModuleAddress.Hex()), second.PoolAddress
	}
	setTransfer(0, first.Token.Address, sender, first.PoolAddress, first.TokenAmountRaw)
	setTransfer(1, first.Settlement.Address, holderOut, relay, first.USDLAmountRaw)
	setTransfer(3, first.Settlement.Address, relay, holderIn, first.USDLAmountRaw)
	setTransfer(4, second.Token.Address, second.PoolAddress, sender, second.TokenAmountRaw)
	for i, index := range []int{2, 7} {
		fill := &fills[i]
		fill.Position.LogIndex = strconv.Itoa(index)
		fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
		fill.Attribution = accounting.Attribution{TransactionSender: sender, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}
		input, output := fill.TokenAmountRaw, fill.USDLAmountRaw
		if i == 1 {
			input, output = output, input
		}
		e := envelopes[index]
		e.Event.Args["poolId"], e.Event.Args["sender"] = common.HexToHash(fill.PoolAddress).Hex(), sender
		e.Event.Args["isBuy"], e.Event.Args["amountIn"], e.Event.Args["amountOut"] = i == 1, input, output
		logs[index].Address = common.HexToAddress(fill.ActualEmitter)
		logs[index].Topics[1], logs[index].Topics[2] = common.HexToHash(fill.PoolAddress), common.HexToHash(sender)
		clear(logs[index].Data[:32])
		if i == 1 {
			logs[index].Data[31] = 1
		}
		in, _ := accounting.Uint(input)
		out, _ := accounting.Uint(output)
		in.FillBytes(logs[index].Data[32:64])
		out.FillBytes(logs[index].Data[64:96])
		if contract {
			e.EmitterRole = valueEventEmitter
		}
	}
	e := envelopes[8]
	e.EventName, e.Event.EventName = "MultihopSwap", "MultihopSwap"
	e.Event.Sig = "MultihopSwap(address,address,address,uint256,uint256,uint256)"
	e.Event.Args = map[string]any{"sender": sender, "tokenIn": first.Token.Address, "tokenOut": second.Token.Address, "amountIn": first.TokenAmountRaw, "intermediateUsdl": first.USDLAmountRaw, "amountOut": second.TokenAmountRaw}
	logs[8].Topics = []common.Hash{crypto.Keccak256Hash([]byte(e.Event.Sig)), common.HexToHash(sender), common.HexToHash(first.Token.Address), common.HexToHash(second.Token.Address)}
	logs[8].Data = make([]byte, 96)
	for i, amount := range []string{first.TokenAmountRaw, first.USDLAmountRaw, second.TokenAmountRaw} {
		value, _ := accounting.Uint(amount)
		value.FillBytes(logs[8].Data[i*32 : (i+1)*32])
	}
	data := make([]byte, 164)
	copy(data[:4], crypto.Keccak256([]byte("swapTokenForToken(address,address,uint256,uint256,uint256)"))[:4])
	copy(data[16:36], common.HexToAddress(first.Token.Address).Bytes())
	copy(data[48:68], common.HexToAddress(second.Token.Address).Bytes())
	amount, _ := accounting.Uint(first.TokenAmountRaw)
	amount.FillBytes(data[68:100])
	amount, _ = accounting.Uint(second.TokenAmountRaw)
	amount.FillBytes(data[100:132])
	new(big.Int).SetUint64(b.Header.Time).FillBytes(data[132:164])
	nativeHolderTransaction(&b, data)
	return b, envelopes, fills
}

func TestRoutedSwapAttributionBothForms(t *testing.T) {
	for _, contract := range []bool{false, true} {
		t.Run(strconv.FormatBool(contract), func(t *testing.T) {
			b, decoded, fills := routedSwapReplay(t, contract)
			for i, fill := range fills {
				a := nativeHolderAttribute(t, b, decoded, fill, fills...)
				sender := fill.Attribution.TransactionSender
				if a.Quality != "qualified_calldata" || a.TransactionSender != sender || len(a.Evidence) != 3 {
					t.Fatalf("routed fill %d attribution: %+v", i, a)
				}
				for _, actor := range []*string{a.RouterCaller, a.Trader, a.Payer, a.Recipient} {
					if actor == nil || *actor != sender {
						t.Fatalf("routed fill %d actor: %+v", i, a)
					}
				}
				calldataHash := sha256.Sum256(b.Transaction.Data())
				receiptHash, err := digest(b.Receipt)
				if err != nil {
					t.Fatal(err)
				}
				if a.Evidence[0].Kind != "router-calldata" || a.Evidence[0].ArtifactSHA256 != hex.EncodeToString(calldataHash[:]) || a.Evidence[0].ArtifactID != fill.Position.TransactionHash || a.Evidence[0].LogIndex != nil {
					t.Fatalf("routed calldata evidence: %+v", a.Evidence)
				}
				for j, kind := range []string{"input-transfer", "output-transfer"} {
					proof := a.Evidence[j+1]
					want := [2][2]string{{"0", "1"}, {"3", "4"}}[i][j]
					if proof.Kind != kind || proof.LogIndex == nil || *proof.LogIndex != want || proof.ArtifactSHA256 != hex.EncodeToString(receiptHash[:]) || proof.ArtifactID != fill.Position.TransactionHash {
						t.Fatalf("routed fill %d evidence: %+v, want %s", i, proof, want)
					}
				}
			}
		})
	}
}

func TestRoutedSwapAttributionRejectsChangedReceipt(t *testing.T) {
	for _, contract := range []bool{false, true} {
		t.Run(strconv.FormatBool(contract), func(t *testing.T) {
			original, decoded, originalFills := routedSwapReplay(t, contract)
			changes := []string{"short calldata", "long calldata", "selector", "value", "expired", "minimum", "amount in", "token in", "token out", "token in padding", "token out padding", "router missing", "router duplicate", "router address", "router role", "router sender", "router token in", "router token out", "router amount in", "router intermediate", "router amount out", "sell sender", "buy sender", "third fill", "one fill", "settlement amounts", "settlement asset", "same pool", "fill order", "token input missing", "token input duplicate", "token output missing", "token output duplicate", "settlement out missing", "settlement out duplicate", "settlement in missing", "settlement in duplicate", "relay", "settlement to sender", "settlement from sender", "token source", "token recipient", "permit", "multicall", "indirect call"}
			for _, change := range changes {
				t.Run(change, func(t *testing.T) {
					b, err := clone(original)
					if err != nil {
						t.Fatal(err)
					}
					envelopes, err := clone(decoded)
					if err != nil {
						t.Fatal(err)
					}
					fills, err := clone(originalFills)
					if err != nil {
						t.Fatal(err)
					}
					data := b.Transaction.Data()
					other := fills[1].Token.Address
					duplicate := func(index int) {
						log, err := clone(b.Receipt.Logs[index])
						if err != nil {
							t.Fatal(err)
						}
						env, err := clone(envelopes[index])
						if err != nil {
							t.Fatal(err)
						}
						log.Index = uint(len(b.Receipt.Logs))
						b.Receipt.Logs = append(b.Receipt.Logs, log)
						envelopes = append(envelopes, env)
					}
					remove := func(index int) {
						b.Receipt.Logs = append(b.Receipt.Logs[:index], b.Receipt.Logs[index+1:]...)
						envelopes = append(envelopes[:index], envelopes[index+1:]...)
					}
					switch change {
					case "short calldata":
						data = data[:163]
					case "long calldata":
						data = append(data, 0)
					case "selector":
						data[0] ^= 1
					case "value":
						b.Transaction = types.NewTx(&types.LegacyTx{To: b.Transaction.To(), Value: big.NewInt(1), Data: data})
					case "expired":
						new(big.Int).SetUint64(b.Header.Time - 1).FillBytes(data[132:164])
					case "minimum":
						amount := new(big.Int).SetBytes(data[100:132])
						amount.Add(amount, big.NewInt(1)).FillBytes(data[100:132])
					case "amount in":
						amount := new(big.Int).SetBytes(data[68:100])
						amount.Add(amount, big.NewInt(1)).FillBytes(data[68:100])
					case "token in":
						data[35] ^= 1
					case "token out":
						data[67] ^= 1
					case "token in padding":
						data[4] = 1
					case "token out padding":
						data[36] = 1
					case "router missing":
						remove(8)
					case "router duplicate":
						duplicate(8)
					case "router address":
						b.Receipt.Logs[8].Address = common.HexToAddress(other)
					case "router role":
						envelopes[8].EmitterRole = valueEventEmitter
					case "router sender", "router token in", "router token out":
						key := map[string]string{"router sender": "sender", "router token in": "tokenIn", "router token out": "tokenOut"}[change]
						envelopes[8].Event.Args[key] = fills[0].PoolAddress
					case "router amount in", "router intermediate", "router amount out":
						key := map[string]string{"router amount in": "amountIn", "router intermediate": "intermediateUsdl", "router amount out": "amountOut"}[change]
						amount, err := accounting.Uint(envelopes[8].Event.Args[key].(string))
						if err != nil {
							t.Fatal(err)
						}
						envelopes[8].Event.Args[key] = amount.Add(amount, big.NewInt(1)).String()
					case "sell sender":
						envelopes[2].Event.Args["sender"] = other
					case "buy sender":
						envelopes[7].Event.Args["sender"] = other
					case "third fill":
						duplicate(7)
					case "one fill":
						remove(7)
					case "settlement amounts":
						amount, err := accounting.Uint(fills[1].USDLAmountRaw)
						if err != nil {
							t.Fatal(err)
						}
						fills[1].USDLAmountRaw = amount.Add(amount, big.NewInt(1)).String()
						envelopes[7].Event.Args["amountIn"] = fills[1].USDLAmountRaw
					case "settlement asset":
						fills[1].Settlement.Address = other
					case "same pool":
						fills[1].PoolAddress = fills[0].PoolAddress
					case "fill order":
						envelopes[2], envelopes[7] = envelopes[7], envelopes[2]
					case "token input missing":
						remove(0)
					case "token input duplicate":
						duplicate(0)
					case "token output missing":
						remove(4)
					case "token output duplicate":
						duplicate(4)
					case "settlement out missing":
						remove(1)
					case "settlement out duplicate":
						duplicate(1)
					case "settlement in missing":
						remove(3)
					case "settlement in duplicate":
						duplicate(3)
					case "relay":
						b.Receipt.Logs[3].Topics[1] = common.HexToHash(other)
					case "settlement to sender", "settlement from sender":
						duplicate(1)
						index := 2
						if change == "settlement from sender" {
							index = 1
						}
						b.Receipt.Logs[len(b.Receipt.Logs)-1].Topics[index] = common.HexToHash(fills[0].Attribution.TransactionSender)
					case "token source":
						b.Receipt.Logs[0].Topics[1] = common.HexToHash(other)
					case "token recipient":
						b.Receipt.Logs[4].Topics[2] = common.HexToHash(other)
					case "permit":
						copy(data[:4], crypto.Keccak256([]byte("swapTokenForTokenWithPermit(address,address,uint256,uint256,uint256,(uint256,uint256,uint8,bytes32,bytes32))"))[:4])
					case "multicall":
						copy(data[:4], crypto.Keccak256([]byte("multicall(bytes[])"))[:4])
					case "indirect call":
						to := common.HexToAddress(other)
						b.Transaction = types.NewTx(&types.LegacyTx{To: &to, Data: data})
					}
					nativeHolderTransaction(&b, data)
					for _, fill := range fills {
						a := nativeHolderAttribute(t, b, envelopes, fill, fills...)
						if a.Quality != "unavailable" || a.Trader != nil || a.RouterCaller != nil || a.Payer != nil || a.Recipient != nil || len(a.Evidence) != 0 {
							t.Fatalf("changed routed swap attributed: %+v", a)
						}
					}
				})
			}
		})
	}
}
