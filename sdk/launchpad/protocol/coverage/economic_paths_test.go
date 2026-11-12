package coverage

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestReviewedFeeEventsMoveCreatorAirdropAndOpticalLiabilities(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	pool := addresses["SidioraPool"]
	accumulator := addresses["FeeAccumulator"]
	holder := common.HexToAddress("0xabcd")
	steps := []struct {
		name   string
		fields map[string]any
	}{
		{"FeeRecorded", map[string]any{"pool": pool, "feeAmount": big.NewInt(20), "protocolCut": big.NewInt(0), "poolCut": big.NewInt(20)}},
		{"AirdropTriggered", map[string]any{"pool": pool, "totalAmount": big.NewInt(10), "epoch": big.NewInt(2)}},
		{"AirdropClaimed", map[string]any{"pool": pool, "holder": holder, "amount": big.NewInt(4), "epoch": big.NewInt(2)}},
		{"FeesBurned", map[string]any{"pool": pool, "amount": big.NewInt(3)}},
		{"LpRewardsSent", map[string]any{"pool": pool, "amount": big.NewInt(2)}},
		{"FeesClaimed", map[string]any{"pool": pool, "recipient": holder, "amount": big.NewInt(5)}},
		{"OpticalSurplusRecorded", map[string]any{"pool": pool, "amount": big.NewInt(9)}},
		{"OpticalSurplusClaimed", map[string]any{"pool": pool, "recipient": holder, "amount": big.NewInt(9)}},
	}
	for i, step := range steps {
		log := eventLog(t, accumulator, abis["FeeAccumulator"].Events[step.name], step.fields)
		header, tx, receipt := receiptInputAt(t, uint64(20+i), log)
		result, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
		if err != nil {
			t.Fatal(step.name, err)
		}
		if len(result.Effects) != 1 {
			t.Fatalf("%s produced %d economic effects", step.name, len(result.Effects))
		}
		if field := map[string]string{
			"AirdropTriggered":      "totalAmount",
			"AirdropClaimed":        "amount",
			"FeesClaimed":           "amount",
			"OpticalSurplusClaimed": "amount",
		}[step.name]; field != "" {
			changed := *result
			changed.Effects = append([]Effect(nil), result.Effects...)
			changed.Effects[0].Fields = copyFields(result.Effects[0].Fields)
			changed.Effects[0].Fields[field] = big.NewInt(1000000)
			changed.seal = receiptSeal(&changed)
			if err := projection.ApplyReceipt(&changed); err == nil {
				t.Fatalf("%s exceeded recorded liability", step.name)
			}
		}
		if err := projection.ApplyReceipt(result); err != nil {
			t.Fatal(step.name, err)
		}
	}
	if projection.CreatorLiability[pool].Sign() != 0 || projection.AirdropLiability[pool.Hex()+"/2"].Cmp(big.NewInt(6)) != 0 || projection.OpticalLiability[pool].Sign() != 0 || len(projection.Claims) != 3 {
		t.Fatalf("fee accounting differs: %+v", projection)
	}
}

func TestReviewedOrderTransitionsRecordMinimumCancellationAndRefund(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	orders := addresses["PECOROrders"]
	steps := []struct {
		name   string
		fields map[string]any
	}{
		{"LimitOrderCreated", map[string]any{"orderId": big.NewInt(1)}},
		{"OrderMinimumOutputUpdated", map[string]any{"orderId": big.NewInt(1), "minimum": big.NewInt(7)}},
		{"LimitOrderCancelled", map[string]any{"orderId": big.NewInt(1)}},
		{"LimitOrderCreated", map[string]any{"orderId": big.NewInt(2)}},
		{"OrderExpiredAndRefunded", map[string]any{"orderId": big.NewInt(2), "amount": big.NewInt(11)}},
	}
	for i, step := range steps {
		log := eventLog(t, orders, abis["PECOROrders"].Events[step.name], step.fields)
		header, tx, receipt := receiptInputAt(t, uint64(30+i), log)
		result, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
		if err != nil {
			t.Fatal(step.name, err)
		}
		if field := map[string]string{
			"LimitOrderCreated":         "orderId",
			"OrderMinimumOutputUpdated": "minimum",
			"OrderExpiredAndRefunded":   "amount",
		}[step.name]; field != "" {
			changed := *result
			changed.Effects = append([]Effect(nil), result.Effects...)
			changed.Effects[0].Fields = copyFields(result.Effects[0].Fields)
			changed.Effects[0].Fields[field] = "noncanonical amount"
			changed.seal = receiptSeal(&changed)
			if err := projection.ApplyReceipt(&changed); err == nil {
				t.Fatalf("%s admitted a noncanonical %s", step.name, field)
			}
		}
		if err := projection.ApplyReceipt(result); err != nil {
			t.Fatal(step.name, err)
		}
	}
	first := projection.Lifecycle.Orders[fmt.Sprintf("125/%s/1", orders.Hex())]
	second := projection.Lifecycle.Orders[fmt.Sprintf("125/%s/2", orders.Hex())]
	if first.Active || first.Terminal != "cancelled" || first.MinimumOutput.Cmp(big.NewInt(7)) != 0 || second.Active || second.Terminal != effectExpiredRefunded || second.RefundAmount.Cmp(big.NewInt(11)) != 0 {
		t.Fatalf("order transitions differ: first=%+v second=%+v", first, second)
	}
}
