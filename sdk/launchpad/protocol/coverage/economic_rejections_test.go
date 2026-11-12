package coverage

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestProjectionPreservesCommittedEffectIdentityAndSupply(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	if _, err := NewProjection(nil); err == nil {
		t.Fatal("projection without lifecycle admitted")
	}
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	token := addresses["SidioraERC20"]
	owner := common.HexToAddress("0xabcd")
	mint := eventLog(t, token, abis["SidioraERC20"].Events["Transfer"], map[string]any{"from": common.Address{}, "to": owner, "value": big.NewInt(7)})
	result := correlateInput(t, registry, mint)
	if err := projection.ApplyReceipt(result); err != nil {
		t.Fatal(err)
	}
	if projection.Supply[token].Cmp(big.NewInt(7)) != 0 || projection.Balances[token.Hex()+owner.Hex()].Cmp(big.NewInt(7)) != 0 {
		t.Fatal("mint did not credit supply and holder")
	}
	changed := *result
	changed.Effects = append([]Effect(nil), result.Effects...)
	changed.Effects[0].Fields = copyFields(result.Effects[0].Fields)
	changed.Effects[0].Fields["value"] = big.NewInt(8)
	changed.seal = receiptSeal(&changed)
	if err := projection.ApplyReceipt(&changed); err == nil {
		t.Fatal("different effect bytes reused a committed identity")
	}
	changed = *result
	changed.Effects = append([]Effect(nil), result.Effects...)
	changed.Effects[0].ID = "different"
	changed.seal = receiptSeal(&changed)
	fresh, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.ApplyReceipt(&changed); err == nil {
		t.Fatal("effect without its canonical identity admitted")
	}
	if projection.Supply[token].Cmp(big.NewInt(7)) != 0 || projection.Balances[token.Hex()+owner.Hex()].Cmp(big.NewInt(7)) != 0 {
		t.Fatal("rejected effect changed committed balances")
	}
	burn := eventLog(t, token, abis["SidioraERC20"].Events["Transfer"], map[string]any{"from": owner, "to": common.Address{}, "value": big.NewInt(8)})
	header, tx, receipt := receiptInput(t, burn)
	result, err = (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := projection.ApplyReceipt(result); err == nil {
		t.Fatal("burn beyond committed balance admitted")
	}
	if projection.Supply[token].Cmp(big.NewInt(7)) != 0 {
		t.Fatal("rejected burn changed supply")
	}
}

func TestProjectionRejectsMalformedReviewedEconomicFields(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	pool := addresses["SidioraPool"]
	party := common.HexToAddress("0x1234")
	token := addresses["SidioraERC20"]
	bad := "noncanonical amount"
	for _, item := range []struct {
		name, role, event string
		fields            map[string]any
		change            func(map[string]any)
	}{
		{"TransferParties", "SidioraERC20", "Transfer", map[string]any{"to": party, "value": big.NewInt(1)}, func(fields map[string]any) { fields["to"] = common.Address{} }},
		{"TransferAmount", "SidioraERC20", "Transfer", map[string]any{"to": party, "value": big.NewInt(1)}, func(fields map[string]any) { fields["value"] = bad }},
		{"NFTIdentity", "SidioraNFT", "Transfer", map[string]any{"to": party, "tokenId": big.NewInt(1)}, func(fields map[string]any) { fields["tokenId"] = bad }},
		{"FeeRecordedCut", "FeeAccumulator", "FeeRecorded", map[string]any{"pool": pool, "feeAmount": big.NewInt(5), "poolCut": big.NewInt(5)}, func(fields map[string]any) { fields["poolCut"] = bad }},
		{"FeeClaimAmount", "FeeAccumulator", "FeesClaimed", map[string]any{"pool": pool, "recipient": party, "amount": big.NewInt(5)}, func(fields map[string]any) { fields["amount"] = bad }},
		{"OpticalRecordAmount", "FeeAccumulator", "OpticalSurplusRecorded", map[string]any{"pool": pool, "amount": big.NewInt(5)}, func(fields map[string]any) { fields["amount"] = bad }},
		{"OpticalClaimAmount", "FeeAccumulator", "OpticalSurplusClaimed", map[string]any{"pool": pool, "recipient": party, "amount": big.NewInt(5)}, func(fields map[string]any) { fields["amount"] = bad }},
		{"AirdropTotal", "FeeAccumulator", "AirdropTriggered", map[string]any{"pool": pool, "totalAmount": big.NewInt(5), "epoch": big.NewInt(1)}, func(fields map[string]any) { fields["totalAmount"] = bad }},
		{"AirdropEpoch", "FeeAccumulator", "AirdropTriggered", map[string]any{"pool": pool, "totalAmount": big.NewInt(5), "epoch": big.NewInt(1)}, func(fields map[string]any) { fields["epoch"] = bad }},
		{"AirdropClaimAmount", "FeeAccumulator", "AirdropClaimed", map[string]any{"pool": pool, "holder": party, "amount": big.NewInt(5), "epoch": big.NewInt(1)}, func(fields map[string]any) { fields["amount"] = bad }},
		{"AirdropClaimEpoch", "FeeAccumulator", "AirdropClaimed", map[string]any{"pool": pool, "holder": party, "amount": big.NewInt(5), "epoch": big.NewInt(1)}, func(fields map[string]any) { fields["epoch"] = bad }},
		{"VaultReserve", "PECORVault", "Deposit", map[string]any{"token": token, "depositor": party, "amount": big.NewInt(5), "newReserve": big.NewInt(5)}, func(fields map[string]any) { fields["newReserve"] = bad }},
		{"VaultParty", "PECORVault", "Deposit", map[string]any{"token": token, "depositor": party, "amount": big.NewInt(5), "newReserve": big.NewInt(5)}, func(fields map[string]any) { fields["depositor"] = common.Address{} }},
		{"VaultAmount", "PECORVault", "Deposit", map[string]any{"token": token, "depositor": party, "amount": big.NewInt(5), "newReserve": big.NewInt(5)}, func(fields map[string]any) { fields["amount"] = bad }},
	} {
		t.Run(item.name, func(t *testing.T) {
			lifecycle, err := NewLifecycle(registry, nil)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := NewProjection(lifecycle)
			if err != nil {
				t.Fatal(err)
			}
			log := eventLog(t, addresses[item.role], abis[item.role].Events[item.event], item.fields)
			result := correlateInput(t, registry, log)
			if len(result.Effects) != 1 {
				t.Fatalf("reviewed %s effect absent", item.event)
			}
			item.change(result.Effects[0].Fields)
			result.seal = receiptSeal(result)
			if err := projection.ApplyReceipt(result); err == nil {
				t.Fatal("malformed economic evidence admitted")
			}
			if len(projection.seen) != 0 || len(projection.Balances) != 0 || len(projection.Claims) != 0 || len(projection.Funding) != 0 || len(projection.Reserves) != 0 {
				t.Fatal("rejected evidence changed projection state")
			}
		})
	}
}
