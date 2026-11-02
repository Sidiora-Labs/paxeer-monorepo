package evm_test

import (
	"encoding/hex"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
)

func shippedPolicy(t *testing.T) *policy.Document {
	t.Helper()
	doc, err := policy.LoadFile(filepath.Join("..", "..", "..", "..", "deploy", "attestor-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestShippedWalletSelectors(t *testing.T) {
	doc := shippedPolicy(t)
	for name, signatures := range map[string][]string{
		"perps": {
			"deposit(address,uint256)", "withdraw(address,uint256)",
			"openPosition(uint256,address,uint256,uint256,bool)", "cancelOrder(uint256)", "closePosition(uint256)",
			"placeLimitOrder(uint256,bool,uint256,uint256,uint256,address,uint256)",
			"placeStopLimitOrder(uint256,bool,uint256,uint256,uint256,uint256,address,uint256)",
			"placeTakeProfitOrder(uint256,uint256)", "placeStopLossOrder(uint256,uint256)",
			"setPositionTpSl(uint256,uint256,uint256)", "partialClose(uint256,uint256)",
			"addCollateral(uint256,uint256)", "removeCollateral(uint256,uint256)",
		},
		"launchpad": {
			"buy(address,uint256,uint256,uint256)", "sell(address,uint256,uint256,uint256)",
			"createMarket(string,string,uint8,address)", "setFeeStrategy(uint256,uint8)",
			"claimFees(uint256)", "claimAirdrop(uint256)", "executeAirdrop(uint256)", "executeBurn(uint256)", "executeLpRewards(uint256)",
		},
		"xweb": {"request(uint8,bytes,uint64)", "refund(uint64)"},
	} {
		t.Run(name, func(t *testing.T) {
			address, ok := evm.PrecompileAddress(name)
			if !ok {
				t.Fatal("missing precompile")
			}
			contract, err := evm.PrecompileABI(name)
			if err != nil {
				t.Fatal(err)
			}
			selectors := doc.Defaults.Selectors[strings.ToLower(address.Hex())]
			if len(selectors) != len(signatures) {
				t.Fatalf("%s selectors %v, want exactly %v", name, selectors, signatures)
			}
			found := map[string]bool{}
			for _, sig := range signatures {
				selector := crypto.Keccak256([]byte(sig))[:4]
				method, err := contract.MethodById(selector)
				if err != nil || method.Sig != sig {
					t.Fatalf("%s: %v", sig, err)
				}
				found["0x"+hex.EncodeToString(selector)] = true
			}
			for _, selector := range selectors {
				if !found[selector] {
					t.Fatalf("unexpected or duplicate %s selector %s", name, selector)
				}
				delete(found, selector)
			}
			if len(found) != 0 {
				t.Fatalf("missing selectors %v", found)
			}
		})
	}
}

func TestShippedLaunchpadAndPerpsAccountPolicy(t *testing.T) {
	doc := shippedPolicy(t)
	const account = "0x1111111111111111111111111111111111111111"
	usdl := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	launchpad, _ := evm.PrecompileAddress("launchpad")
	perps, _ := evm.PrecompileAddress("perps")
	now := time.Now()
	ledger := policy.NewMemoryLedger(func() time.Time { return now })
	engine := policy.New(doc)
	evaluate := func(to common.Address, data []byte, want string) policy.Decision {
		t.Helper()
		tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(125), GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 500000, To: &to, Value: big.NewInt(0), Data: data})
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		view, err := evm.DecodeTransaction(raw, big.NewInt(125))
		if err != nil {
			t.Fatal(err)
		}
		decision := engine.Evaluate(account, policy.Request{Kind: policy.KindEVMTransaction, View: view}, ledger)
		if decision.Code != want || decision.Allowed != (want == policy.CodeAllowed) {
			t.Fatalf("decision %+v, want %s", decision, want)
		}
		return decision
	}
	erc20, err := evm.PrecompileABI("erc20")
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(25_000_000)
	for _, spender := range []common.Address{launchpad, perps} {
		approve, err := erc20.Pack("approve", spender, amount)
		if err != nil {
			t.Fatal(err)
		}
		decision := evaluate(usdl, approve, policy.CodeAllowed)
		if spender == launchpad {
			if len(decision.Spends) != 1 || decision.Spends[0].Asset != policy.TokenAsset(usdl) || decision.Spends[0].Amount.Cmp(amount) != 0 {
				t.Fatalf("launchpad approval accounting: %+v", decision.Spends)
			}
		} else if len(decision.Spends) != 0 {
			t.Fatalf("perps approval counted before pull: %+v", decision.Spends)
		}
		unlimited := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
		data, err := erc20.Pack("approve", spender, unlimited)
		if err != nil {
			t.Fatal(err)
		}
		evaluate(usdl, data, policy.CodeUnlimitedApproval)
	}
	launchpadABI, err := evm.PrecompileABI("launchpad")
	if err != nil {
		t.Fatal(err)
	}
	buy, err := launchpadABI.Pack("buy", common.HexToAddress("0x2222222222222222222222222222222222222222"), amount, big.NewInt(1), big.NewInt(1_900_000_000))
	if err != nil {
		t.Fatal(err)
	}
	evaluate(launchpad, buy, policy.CodeAllowed)
	// An account override must continue to refuse a write admitted by defaults.
	doc.Accounts = map[string]policy.Rules{account: {Selectors: map[string][]string{strings.ToLower(launchpad.Hex()): {}}}}
	engine = policy.New(doc)
	evaluate(launchpad, buy, policy.CodeSelectorNotAllowed)
	perpsABI, err := evm.PrecompileABI("perps")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int64{25_000_000, 1_000_000_001} {
		deposit, err := perpsABI.Pack("deposit", usdl, big.NewInt(n))
		if err != nil {
			t.Fatal(err)
		}
		want := policy.CodeAllowed
		if n > 1_000_000_000 {
			want = policy.CodeValueCap
		}
		decision := evaluate(perps, deposit, want)
		if decision.Allowed && (len(decision.Spends) != 1 || decision.Spends[0].Amount.Cmp(big.NewInt(n)) != 0) {
			t.Fatalf("perps pull accounting: %+v", decision.Spends)
		}
	}
}

func TestXwebWalletSelectorsExcludeFulfilment(t *testing.T) {
	doc := shippedPolicy(t)
	contract, err := evm.PrecompileABI("xweb")
	if err != nil {
		t.Fatal(err)
	}
	address, _ := evm.PrecompileAddress("xweb")
	selectors := doc.Defaults.Selectors[strings.ToLower(address.Hex())]
	for _, name := range []string{"fulfil", "fulfilProgram"} {
		method, ok := contract.Methods[name]
		if !ok {
			t.Fatalf("missing %s ABI", name)
		}
		for _, selector := range selectors {
			if selector == "0x"+hex.EncodeToString(method.ID) {
				t.Fatalf("wallet policy admits attestor fulfilment %s", name)
			}
		}
	}
	for _, tc := range []struct {
		name string
		args []any
	}{
		{"request", []any{uint8(2), []byte("paxeer"), uint64(0)}},
		{"refund", []any{uint64(7)}},
	} {
		data, err := contract.Pack(tc.name, tc.args...)
		if err != nil {
			t.Fatal(err)
		}
		call, err := evm.DecodeCalldata(address, data)
		if err != nil || call.Method != tc.name || call.Kind != evm.CallPrecompile {
			t.Fatalf("wallet %s: %+v %v", tc.name, call, err)
		}
	}
}
