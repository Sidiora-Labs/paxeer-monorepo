package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type repair28Producer struct {
	Endpoint                                              string
	Addresses                                             map[string]common.Address
	Account                                               common.Address
	Created, Funded, Direct, Routed, Forged, Orphan, Sold common.Hash
	Lists                                                 []struct {
		Transactions                 [][]byte
		Receipts                     types.Receipts
		TransactionRoot, ReceiptRoot common.Hash
	}
	stop func()
}

func repair28StartProducer(t *testing.T) repair28Producer {
	t.Helper()
	root, err := filepath.Abs("../../../../contracts/launchpad/contracts")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancel)
	manifest, err := os.ReadFile("testdata/repair28producer/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	modfile := filepath.Join(t.TempDir(), "producer.mod")
	if err := os.WriteFile(modfile, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "go", "run", "-modfile="+modfile, "-mod=mod", ".", root)
	command.Dir = "testdata/repair28producer"
	stderr, err := os.Create(filepath.Join(t.TempDir(), "producer.log"))
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			stdin.Close()
			if err := command.Wait(); err != nil {
				stderr.Close()
				raw, _ := os.ReadFile(stderr.Name())
				t.Errorf("real contract producer: %v\n%s", err, raw)
			} else {
				stderr.Close()
			}
		})
	}
	t.Cleanup(stop)
	var producer repair28Producer
	if err := json.NewDecoder(stdout).Decode(&producer); err != nil {
		cancel()
		raw, _ := os.ReadFile(stderr.Name())
		t.Fatalf("real producer did not start: %v\n%s", err, raw)
	}
	producer.stop = stop
	return producer
}

func TestRepair_28_EmitterCallerEvidence(t *testing.T) {
	producer := repair28StartProducer(t)
	ctx := context.Background()
	client, err := DialMeasurementRPC(ctx, producer.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	var deployments []catalog.Deployment
	for _, bundle := range bundles {
		if address, ok := producer.Addresses[bundle.Name]; ok {
			deployments = append(deployments, catalog.Deployment{ChainID: 125, Address: address, Family: bundle.Name, Version: bundle.ABIHash, Attested: true})
		}
		if bundle.Name == "SidioraERC20" {
			deployments = append(deployments, catalog.Deployment{ChainID: 125, Address: producer.Addresses["USDL"], Family: bundle.Name, Version: bundle.ABIHash, Attested: true})
		}
	}
	registry, err := catalog.New(deployments)
	if err != nil {
		t.Fatal(err)
	}
	selector, ok := TypedSelector("Swap")
	if !ok {
		t.Fatal("Swap selector absent")
	}
	source := &RPCSource{RPC: client, Registry: registry, PoolRegistry: producer.Addresses["PoolRegistry"],
		Permissions: []PermissionEpoch{{ChainID: 125, Emitter: producer.Addresses["EventEmitter"], Caller: producer.Addresses["SidioraPool"], Selector: selector}},
		Traces:      &RPCTraceSource{RPC: client.Client()}, NativeReceipts: &RPCNativeReceiptSource{RPC: client},
	}
	load := func(hash common.Hash) (*types.Header, *types.Transaction, *types.Receipt) {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err != nil {
			t.Fatal(err)
		}
		header, err := client.HeaderByHash(ctx, receipt.BlockHash)
		if err != nil {
			t.Fatal(err)
		}
		tx, pending, err := client.TransactionByHash(ctx, hash)
		if err != nil || pending {
			t.Fatalf("signed producer transaction unavailable: %v", err)
		}
		return header, tx, receipt
	}
	swapLog := func(receipt *types.Receipt) *types.Log {
		for _, log := range receipt.Logs {
			d := registry.Decode(125, *log)
			if d.EmitterRole == "EventEmitter" && strings.HasPrefix(d.Signature, "Swap(") {
				return log
			}
		}
		t.Fatal("real pool emitted no Swap")
		return nil
	}
	for _, item := range []struct {
		Name   string
		Hash   common.Hash
		Target common.Address
	}{
		{"PoolSwap", producer.Direct, producer.Addresses["SidioraPool"]},
		{"RouterSwap", producer.Routed, producer.Addresses["Router"]},
	} {
		t.Run(item.Name, func(t *testing.T) {
			header, tx, receipt := load(item.Hash)
			if tx.To() == nil || *tx.To() != item.Target || *tx.To() == producer.Addresses["EventEmitter"] {
				t.Fatal("producer did not use the pool/router interface")
			}
			caller, gotSelector, _, err := source.CallerAt(ctx, identity(125, *swapLog(receipt)), position(*swapLog(receipt)))
			if err != nil || caller != producer.Addresses["SidioraPool"] || caller == producer.Account || gotSelector != selector {
				t.Fatalf("internal call provenance lost: caller=%s selector=%x err=%v", caller, gotSelector, err)
			}
			out, err := (&Correlator{Registry: registry, Source: source}).Correlate(ctx, header, tx, receipt)
			if err != nil {
				t.Fatal(err)
			}
			fills := 0
			for _, effect := range out.Effects {
				if effect.Kind != "core_fill" {
					continue
				}
				fills++
				if effect.Fields["pool"] != producer.Addresses["SidioraPool"] || effect.Fields["token"] != producer.Addresses["SidioraERC20"] || effect.Fields["settlement"] != producer.Addresses["USDL"] || effect.Fields["sender"] != producer.Account || !equalNumber(effect.Fields["amountIn"], big.NewInt(100000000)) || effect.Actors.TxFrom.Address != producer.Account || effect.Actors.TxTo.Address != item.Target {
					t.Fatalf("consumer fill differs: %+v", effect)
				}
				amount, ok := effect.Fields["amountOut"].(*big.Int)
				if !ok || amount.Sign() <= 0 || !effect.Actors.Recipient.Known || effect.Actors.Recipient.Address != producer.Account {
					t.Fatal("real transfer output did not reach the consumer")
				}
				if item.Name == "RouterSwap" && len(effect.Companions) != 1 {
					t.Fatal("router receipt lost its fill association")
				}
			}
			if fills != 1 {
				t.Fatalf("valid swap produced %d fills; diagnostics=%v records=%+v", fills, out.Diagnostics, out.Records)
			}
		})
	}
	t.Run("ForgedCaller", func(t *testing.T) {
		header, tx, receipt := load(producer.Forged)
		log := swapLog(receipt)
		if _, _, _, err := source.CallerAt(ctx, identity(125, *log), position(*log)); err == nil {
			t.Fatal("unpermitted EOA caller authenticated")
		}
		// Even a genuine emitter permission does not authorize an EOA to
		// impersonate the registered pool named by the event's poolId.
		forgedSource := *source
		forgedSource.Permissions = append(append([]PermissionEpoch(nil), source.Permissions...), PermissionEpoch{ChainID: 125, Emitter: log.Address, Caller: producer.Account, Selector: selector})
		out, err := (&Correlator{Registry: registry, Source: &forgedSource}).Correlate(ctx, header, tx, receipt)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Records) != 1 || out.Records[0].Caller != producer.Account || len(out.Effects) != 0 || !containsDiagnostic(out.Diagnostics, "unauthorized_pool_swap") {
			t.Fatalf("forged pool caller admitted: %+v", out)
		}
	})
	t.Run("TraceRefusals", func(t *testing.T) {
		_, tx, receipt := load(producer.Routed)
		log := swapLog(receipt)
		traceLog := CallFrameLog{Address: log.Address, Topics: append([]common.Hash(nil), log.Topics...), Data: append([]byte(nil), log.Data...)}
		if !sameRawLog(traceLog, log) {
			t.Fatal("committed trace log differs from receipt")
		}
		traceLog.Topics[0] = common.Hash{}
		if sameRawLog(traceLog, log) {
			t.Fatal("different trace topic matched receipt")
		}
		frame, err := source.Traces.TraceTransactionInBlock(ctx, receipt.BlockHash, tx.Hash())
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			Name   string
			Mutate func(*CallFrame)
		}{
			{"OuterSender", func(f *CallFrame) { f.From = producer.Addresses["SidioraPool"] }},
			{"OuterTarget", func(f *CallFrame) { f.To = producer.Addresses["SidioraPool"] }},
			{"OuterSelector", func(f *CallFrame) { f.Input[0] ^= 1 }},
			{"MissingValue", func(f *CallFrame) { f.Value = nil }},
			{"WrongValue", func(f *CallFrame) { (*big.Int)(f.Value).SetInt64(1) }},
			{"WrongRootType", func(f *CallFrame) { f.Type = "DELEGATECALL" }},
			{"FailedRoot", func(f *CallFrame) { f.Error = "execution reverted" }},
			{"TruncatedLogs", func(f *CallFrame) { f.Calls = nil; f.Logs = nil }},
			{"InvalidSelfDestruct", func(f *CallFrame) {
				f.Calls = append(f.Calls, CallFrame{Type: "SELFDESTRUCT", From: f.To, Logs: []CallFrameLog{{Address: log.Address}}})
			}},
		}
		var visit func(*CallFrame, func(*CallFrame))
		visit = func(f *CallFrame, change func(*CallFrame)) {
			for _, l := range f.Logs {
				if l.Address == log.Address && len(l.Topics) > 0 && l.Topics[0] == log.Topics[0] {
					change(f)
					return
				}
			}
			for i := range f.Calls {
				visit(&f.Calls[i], change)
			}
		}
		cases = append(cases,
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"ForgedInternalCaller", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.From = producer.Account }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"ForgedLogPayload", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.Logs[0].Data[0] ^= 1 }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"MissingLogAssociation", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.Logs[0].Position = nil }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"RevertedLogFrame", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.Error = "execution reverted" }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"StaticLogFrame", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.Type = "STATICCALL" }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"UnsupportedLogFrame", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.Type = "UNKNOWN" }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"ExcessTopics", func(f *CallFrame) {
				visit(f, func(frame *CallFrame) {
					frame.Logs[0].Topics = append(frame.Logs[0].Topics, common.Hash{}, common.Hash{}, common.Hash{}, common.Hash{}, common.Hash{})
				})
			}},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"OutOfRangeLogPosition", func(f *CallFrame) {
				visit(f, func(frame *CallFrame) {
					position := *frame.Logs[0].Position + 1000
					frame.Logs[0].Position = &position
				})
			}},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"MissingEmitterSelector", func(f *CallFrame) { visit(f, func(frame *CallFrame) { frame.Input = nil }) }},
			struct {
				Name   string
				Mutate func(*CallFrame)
			}{"WrongLogAddress", func(f *CallFrame) {
				visit(f, func(frame *CallFrame) { frame.Logs[0].Address = common.HexToAddress("0xdead") })
			}},
		)
		for _, item := range cases {
			t.Run(item.Name, func(t *testing.T) {
				var changed CallFrame
				if err := json.Unmarshal(raw, &changed); err != nil {
					t.Fatal(err)
				}
				item.Mutate(&changed)
				if _, _, err := callerFromTrace(&changed, tx, receipt, identity(125, *log)); err == nil {
					t.Fatal("forged frame evidence accepted")
				}
			})
		}
		if _, err := source.Traces.TraceTransactionInBlock(ctx, receipt.BlockHash, producer.Direct); err == nil {
			t.Fatal("trace crossed block identity")
		}
		if _, err := source.Traces.TraceTransactionInBlock(ctx, common.HexToHash("0x9876"), producer.Routed); err == nil {
			t.Fatal("uncommitted block supplied execution trace")
		}
		wrong := position(*log)
		wrong.Log++
		if _, _, _, err := source.CallerAt(ctx, identity(125, *log), wrong); err == nil {
			t.Fatal("trace crossed log identity")
		}
		absent := *source
		absent.Traces = nil
		if _, _, _, err := absent.CallerAt(ctx, identity(125, *log), position(*log)); err == nil {
			t.Fatal("missing trace silently used the EOA")
		}
		copied := *receipt
		copied.Logs = append([]*types.Log(nil), receipt.Logs...)
		altered := *log
		altered.Data = append([]byte(nil), log.Data...)
		altered.Data[0] ^= 1
		for i, l := range copied.Logs {
			if l.Index == log.Index {
				copied.Logs[i] = &altered
			}
		}
		header, _, _ := load(producer.Routed)
		if _, err := (&Correlator{Registry: registry, Source: source}).Correlate(ctx, header, tx, &copied); err == nil {
			t.Fatal("consumer changed authenticated receipt payload")
		}
	})
	t.Run("VerifiedNativeReceiptEvidence", func(t *testing.T) {
		header, tx, receipt := load(producer.Routed)
		verifier := &RPCNativeReceiptSource{RPC: client}
		if err := verifier.VerifyNativeReceipt(ctx, header, tx, receipt); err != nil {
			t.Fatal("committed producer receipt refused", err)
		}
		log := swapLog(receipt)
		caller, selector, err := source.nativeCaller(ctx, header, tx, receipt, *log)
		wantSelector, ok := TypedSelector("Swap")
		if err != nil || !ok || caller != producer.Addresses["SidioraPool"] || selector != wantSelector {
			t.Fatalf("committed pool mirror caller differs: %s %x %v", caller, selector, err)
		}
		if _, _, err := source.nativeCaller(ctx, header, tx, receipt, *receipt.Logs[0]); err == nil {
			t.Fatal("unrelated committed log gained swap caller authority")
		}
		alteredLog := *log
		alteredLog.Data = append([]byte(nil), log.Data...)
		alteredLog.Data[len(alteredLog.Data)-64+31] ^= 1
		if _, _, err := source.nativeCaller(ctx, header, tx, receipt, alteredLog); err == nil {
			t.Fatal("mirror with different block timestamp gained native caller")
		}
		alteredLog = *log
		alteredLog.Topics = append([]common.Hash(nil), log.Topics...)
		alteredLog.Topics[1] = common.HexToHash("0xdead")
		if _, _, err := source.nativeCaller(ctx, header, tx, receipt, alteredLog); err == nil {
			t.Fatal("mirror with unknown pool gained native caller")
		}
		withoutNative := *source
		withoutNative.NativeReceipts = nil
		if _, _, err := withoutNative.nativeCaller(ctx, header, tx, receipt, *log); err == nil {
			t.Fatal("caller without receipt inclusion gained authority")
		}
		if err := source.AuthenticateReceipt(ctx, nil); err == nil {
			t.Fatal("missing authenticated receipt admitted")
		}
		unknown := *receipt
		unknown.TxHash = common.HexToHash("0x9876")
		if err := source.AuthenticateReceipt(ctx, &unknown); err == nil {
			t.Fatal("uncommitted receipt returned authenticated bytes")
		}
		modified := *receipt
		modified.Status = types.ReceiptStatusFailed
		if err := verifier.VerifyNativeReceipt(ctx, header, tx, &modified); err == nil {
			t.Fatal("modified native receipt accepted")
		}
		if err := source.AuthenticateReceipt(ctx, &modified); err == nil {
			t.Fatal("different authenticated receipt bytes admitted")
		}
		if err := verifier.VerifyNativeReceipt(ctx, nil, tx, receipt); err == nil {
			t.Fatal("native evidence without header admitted")
		}
		wrongIndex := *receipt
		wrongIndex.TransactionIndex++
		if err := verifier.VerifyNativeReceipt(ctx, header, tx, &wrongIndex); err == nil {
			t.Fatal("native receipt with foreign transaction index admitted")
		}
		wrongData := *receipt
		wrongData.Logs = append([]*types.Log(nil), receipt.Logs...)
		firstLog := *receipt.Logs[0]
		firstLog.Data = append([]byte(nil), firstLog.Data...)
		firstLog.Data[0] ^= 1
		wrongData.Logs[0] = &firstLog
		if err := verifier.VerifyNativeReceipt(ctx, header, tx, &wrongData); err == nil {
			t.Fatal("native receipt with different log data admitted")
		}
		otherHeader := *header
		otherHeader.ReceiptHash = common.Hash{}
		if err := verifier.VerifyNativeReceipt(ctx, &otherHeader, tx, receipt); err == nil {
			t.Fatal("uncommitted receipt root accepted")
		}
		_, otherTx, _ := load(producer.Direct)
		if err := verifier.VerifyNativeReceipt(ctx, header, otherTx, receipt); err == nil {
			t.Fatal("native receipt attached to foreign transaction")
		}
		for i, list := range producer.Lists {
			t.Run(fmt.Sprintf("ConsensusRoots%d", i), func(t *testing.T) {
				txs := make(types.Transactions, len(list.Transactions))
				for i, raw := range list.Transactions {
					txs[i] = new(types.Transaction)
					if err := txs[i].UnmarshalBinary(raw); err != nil {
						t.Fatal(err)
					}
				}
				if consensusListRoot(txs) != list.TransactionRoot || consensusListRoot(list.Receipts) != list.ReceiptRoot {
					t.Fatalf("receipt/transaction roots differ from geth for %d entries", len(txs))
				}
			})
		}
	})
}

func containsDiagnostic(diagnostics []string, expected string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic == expected {
			return true
		}
	}
	return false
}
