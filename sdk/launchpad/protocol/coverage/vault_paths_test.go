package coverage

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestReviewedVaultWithdrawalsAndReserveSyncFollowDeposits(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	vault := addresses["PECORVault"]
	token := addresses["SidioraERC20"]
	party := common.HexToAddress("0xabcd")
	steps := []struct {
		name   string
		fields map[string]any
	}{
		{"Deposit", map[string]any{"token": token, "depositor": party, "amount": big.NewInt(7), "newReserve": big.NewInt(7)}},
		{"Withdrawal", map[string]any{"token": token, "recipient": party, "amount": big.NewInt(3), "newReserve": big.NewInt(4)}},
		{"ReservesSync", map[string]any{"token": token, "oldReserves": big.NewInt(4), "newReserves": big.NewInt(4)}},
		{"NativeDeposit", map[string]any{"depositor": party, "amount": big.NewInt(5), "newReserve": big.NewInt(5)}},
		{"NativeWithdrawal", map[string]any{"recipient": party, "amount": big.NewInt(2), "newReserve": big.NewInt(3)}},
	}
	for _, step := range steps {
		log := eventLog(t, vault, abis["PECORVault"].Events[step.name], step.fields)
		result := correlateInput(t, registry, log)
		if err := projection.ApplyReceipt(result); err != nil {
			t.Fatal(step.name, err)
		}
	}
	if projection.Funding[vault.Hex()+token.Hex()+party.Hex()].Cmp(big.NewInt(4)) != 0 || projection.Reserves[vault.Hex()+token.Hex()].Cmp(big.NewInt(4)) != 0 || projection.Funding[vault.Hex()+(common.Address{}).Hex()+party.Hex()].Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("vault accounting differs: %+v", projection)
	}
	bad := eventLog(t, vault, abis["PECORVault"].Events["Withdrawal"], map[string]any{"recipient": party, "amount": big.NewInt(1), "newReserve": big.NewInt(2)})
	if err := projection.ApplyReceipt(correlateInput(t, registry, bad)); err == nil {
		t.Fatal("unregistered vault token admitted")
	}
}

func TestReviewedNFTOwnershipRejectsDuplicateMintAndForeignBurn(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	nft := addresses["SidioraNFT"]
	owner := common.HexToAddress("0xa1")
	other := common.HexToAddress("0xb1")
	mint := eventLog(t, nft, abis["SidioraNFT"].Events["Transfer"], map[string]any{"to": owner, "tokenId": big.NewInt(7)})
	if err := projection.ApplyReceipt(correlateInput(t, registry, mint)); err != nil {
		t.Fatal(err)
	}
	duplicate := eventLog(t, nft, abis["SidioraNFT"].Events["Transfer"], map[string]any{"to": other, "tokenId": big.NewInt(7)})
	if err := projection.ApplyReceipt(correlateInput(t, registry, duplicate)); err == nil {
		t.Fatal("duplicate native NFT mint admitted")
	}
	foreign := eventLog(t, nft, abis["SidioraNFT"].Events["Transfer"], map[string]any{"from": other, "tokenId": big.NewInt(7)})
	if err := projection.ApplyReceipt(correlateInput(t, registry, foreign)); err == nil {
		t.Fatal("foreign native NFT burn admitted")
	}
	if projection.NFTOwners[nft.Hex()+"/7"] != owner {
		t.Fatal("rejected transfers changed ownership")
	}
}
