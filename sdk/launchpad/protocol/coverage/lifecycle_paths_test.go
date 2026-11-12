package coverage

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReviewedImplementationAndPauseControlWriteAvailability(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	contract := addresses["PECOROrders"]
	version, ok := registry.DeploymentAt(125, contract, 12)
	if !ok {
		t.Fatal("reviewed orders deployment absent")
	}
	implementation := common.HexToAddress("0x9876")
	review := ImplementationReview{Contract: contract, Implementation: implementation, Family: version.Family, Version: version.Version, CodeHash: common.HexToHash("0x1234")}
	for _, invalid := range [][]ImplementationReview{
		{{}},
		{{Contract: contract, Implementation: implementation, Family: version.Family, Version: "unknown", CodeHash: review.CodeHash}},
		{review, review},
	} {
		if _, err := NewLifecycle(registry, invalid); err == nil {
			t.Fatal("unreviewed implementation admitted")
		}
	}
	lifecycle, err := NewLifecycle(registry, []ImplementationReview{review})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range []struct {
		name   string
		fields map[string]any
		want   bool
	}{
		{"Upgraded", map[string]any{"implementation": implementation}, true},
		{"PauseToggled", map[string]any{"paused": true}, false},
		{"PauseToggled", map[string]any{"paused": false}, true},
	} {
		log := eventLog(t, contract, abis["PECOROrders"].Events[step.name], step.fields)
		header, tx, receipt := receiptInputAt(t, uint64(40+i), log)
		result, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
		if err != nil {
			t.Fatal(step.name, err)
		}
		changed := *result
		changed.Effects = append([]Effect(nil), result.Effects...)
		if len(changed.Effects) != 1 {
			t.Fatalf("%s control effect absent", step.name)
		}
		changed.Effects[0].Fields = copyFields(result.Effects[0].Fields)
		if step.name == "Upgraded" {
			changed.Effects[0].Fields["implementation"] = common.Address{}
		} else {
			changed.Effects[0].Fields["paused"] = "noncanonical state"
		}
		changed.seal = receiptSeal(&changed)
		if err := projection.ApplyReceipt(&changed); err == nil {
			t.Fatalf("%s admitted incomplete control evidence", step.name)
		}
		if err := projection.ApplyReceipt(result); err != nil {
			t.Fatal(step.name, err)
		}
		if i == 1 {
			otherHeader, otherTx, otherReceipt := receiptInputAt(t, uint64(40+i), log)
			other, err := (&Correlator{Registry: registry}).Correlate(context.Background(), otherHeader, otherTx, otherReceipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := projection.ApplyReceipt(other); err == nil {
				t.Fatal("control events at the same canonical position reordered state")
			}
		}
		if projection.Lifecycle.WriteAllowed(contract, uint64(40+i)) != step.want || projection.Lifecycle.Controls[contract].Reattesting {
			t.Fatalf("reviewed control state differs after %s: %+v", step.name, projection.Lifecycle.Controls[contract])
		}
	}
}

func TestCanonicalReceiptCoordinatesAndFailedExecution(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	header, _, unsignedReceipt := receiptInput(t)
	target := common.HexToAddress("0x1234")
	unsignedTransaction := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(125), To: &target, Gas: 21000})
	unsignedReceipt.TxHash = unsignedTransaction.Hash()
	if _, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, unsignedTransaction, unsignedReceipt); err == nil {
		t.Fatal("unsigned chain transaction admitted as a receipt sender")
	}
	log := eventLog(t, addresses["SidioraERC20"], abis["SidioraERC20"].Events["Transfer"], map[string]any{"to": common.HexToAddress("0xab"), "value": big.NewInt(4)})
	for _, item := range []struct {
		name   string
		change func(*types.Receipt)
	}{
		{"MissingLog", func(r *types.Receipt) { r.Logs[0] = nil }},
		{"RemovedLog", func(r *types.Receipt) { r.Logs[0].Removed = true }},
		{"WrongBlock", func(r *types.Receipt) { r.Logs[0].BlockHash = common.HexToHash("0x12") }},
		{"WrongTransaction", func(r *types.Receipt) { r.Logs[0].TxHash = common.HexToHash("0x34") }},
		{"WrongIndex", func(r *types.Receipt) { r.Logs[0].TxIndex = 1 }},
		{"FailedWithLog", func(r *types.Receipt) { r.Status = types.ReceiptStatusFailed }},
	} {
		t.Run(item.name, func(t *testing.T) {
			header, tx, receipt := receiptInput(t, log)
			item.change(receipt)
			if _, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt); err == nil {
				t.Fatal("noncanonical receipt accepted")
			}
		})
	}
	header, tx, receipt := receiptInput(t)
	receipt.Status = types.ReceiptStatusFailed
	result, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
	if err != nil || result.Successful || len(result.Effects) != 0 {
		t.Fatalf("failed execution produced effects: %+v %v", result, err)
	}
}

func TestReviewedOrderRefusesMissingAndRepeatedCreation(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	if _, err := NewLifecycle(nil, nil); err == nil {
		t.Fatal("lifecycle without reviewed registry admitted")
	}
	if _, err := orderKey(Effect{Fields: map[string]any{"orderId": big.NewInt(-1)}}); err == nil {
		t.Fatal("negative native order identity admitted")
	}
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	orders := addresses["PECOROrders"]
	makeResult := func(block uint64, event string) *ReceiptResult {
		t.Helper()
		log := eventLog(t, orders, abis["PECOROrders"].Events[event], map[string]any{"orderId": big.NewInt(17)})
		header, tx, receipt := receiptInputAt(t, block, log)
		result, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if err := projection.ApplyReceipt(makeResult(11, "LimitOrderCancelled")); err == nil {
		t.Fatal("cancellation without native order admitted")
	}
	if err := projection.ApplyReceipt(makeResult(12, "LimitOrderCreated")); err != nil {
		t.Fatal(err)
	}
	if err := projection.ApplyReceipt(makeResult(13, "LimitOrderCreated")); err == nil {
		t.Fatal("repeated native order creation admitted")
	}
	if err := projection.ApplyReceipt(makeResult(11, "OrderExecutionFailed")); err == nil {
		t.Fatal("out of order native transition admitted")
	}
	if !projection.Lifecycle.Orders["125/"+orders.Hex()+"/17"].Created {
		t.Fatal("rejected order transitions removed creation")
	}
	if lifecycle.WriteAllowed(common.HexToAddress("0xdead"), 12) {
		t.Fatal("unregistered contract allowed writes")
	}
	var empty *Lifecycle
	if empty.WriteAllowed(orders, 12) {
		t.Fatal("missing lifecycle allowed writes")
	}
}

func TestCanonicalReceiptRejectsConflictingTopicsAndOversizedPayload(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	log := eventLog(t, addresses["SidioraERC20"], abis["SidioraERC20"].Events["Transfer"], map[string]any{"value": big.NewInt(1)})
	header, tx, receipt := receiptInput(t, log)
	duplicate := *receipt.Logs[0]
	duplicate.Topics = append([]common.Hash(nil), duplicate.Topics...)
	duplicate.Topics[1] = common.HexToHash("0x12")
	receipt.Logs = append(receipt.Logs, &duplicate)
	if _, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt); err == nil {
		t.Fatal("conflicting indexed topic admitted")
	}
	header, tx, receipt = receiptInput(t, log)
	receipt.Logs[0].Data = make([]byte, 16*1024*1024+1)
	if _, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt); err == nil {
		t.Fatal("oversized raw receipt payload admitted")
	}
	if hashField(common.HexToHash("0x1234")) != common.HexToHash("0x1234") {
		t.Fatal("reviewed hash identity changed")
	}
	if (Position{Block: 12, Transaction: 1}).Before(Position{Block: 12, Transaction: 1}) {
		t.Fatal("equal canonical positions ordered")
	}
}
