package policy

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/testkernel"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type repair244Fixture struct {
	h         *harness
	accounts  []testkernel.Account
	pin       ConstructPin
	witnesses [][]byte
	state     *ConstructState
	budget    [32]byte
}

func repair244Open(t *testing.T) *repair244Fixture {
	t.Helper()
	return repair244OpenSource(t, false)
}

func repair244OpenSource(t *testing.T, native bool) *repair244Fixture {
	t.Helper()
	open := testkernel.Open
	if native {
		open = testkernel.OpenNative
	}
	if err := open(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(testkernel.Close)
	f := &repair244Fixture{h: newHarness(t), budget: [32]byte{0x82}}
	for i := uint(0); i < 6; i++ {
		a, err := testkernel.Info(i)
		if err != nil {
			t.Fatal(err)
		}
		f.accounts = append(f.accounts, a)
	}
	f.pin = ConstructPin{ChainID: 125, Account: common.HexToAddress(testAccount), BlockHash: common.HexToHash("0x244"), ActorDID: "did:key:owner", AuthorityKey: f.accounts[0].Authority}
	f.h.policy.doc.Defaults.RatePerMinute = new(uint32)
	*f.h.policy.doc.Defaults.RatePerMinute = 100
	f.h.policy.doc.Defaults.Caps[ConstructAsset(f.accounts[0].Asset)] = Cap{PerTransaction: "10", Daily: "15"}
	f.h.policy.doc.Defaults.Caps[ConstructAsset(f.accounts[3].Asset)] = Cap{PerTransaction: "10", Daily: "15"}
	for _, domain := range []string{"layerxpay", "layerxprograms"} {
		a, _ := evm.PrecompileAddress(domain)
		contract, err := evm.PrecompileABI(domain)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range contract.Methods {
			f.h.policy.doc.Defaults.Selectors[a.Hex()] = append(f.h.policy.doc.Defaults.Selectors[a.Hex()], "0x"+hex.EncodeToString(m.ID))
		}
	}
	f.refresh(t, false)
	f.h.policy.SetConstructStateReader(func(ctx Context) (*ConstructState, error) { return f.state, nil })
	return f
}

func (f *repair244Fixture) refresh(t *testing.T, budget bool) {
	t.Helper()
	var err error
	f.pin.StateRoot, err = testkernel.Root()
	if err != nil {
		t.Fatal(err)
	}
	f.witnesses = nil
	for _, a := range f.accounts {
		proof, err := testkernel.Proof(0, append([]byte{4}, a.ID[:]...))
		if err != nil {
			t.Fatal(err)
		}
		f.witnesses = append(f.witnesses, proof)
	}
	if budget {
		proof, err := testkernel.Proof(3, append([]byte("budget:"), f.budget[:]...))
		if err != nil {
			t.Fatal(err)
		}
		f.witnesses = append(f.witnesses, proof)
	}
	f.state, err = NewConstructState(f.pin, f.witnesses)
	if err != nil {
		t.Fatal(err)
	}
}

func repair244Pack(t *testing.T, domain, method string, args ...any) []byte {
	t.Helper()
	// The producer is the canonical precompile ABI, not a reconstructed selector.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "precompiles", domain, "abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	producer, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	data, err := producer.Pack(method, args...)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := evm.PrecompileABI(domain)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := consumer.Pack(method, args...)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, embedded) {
		t.Fatal("producer and consumer ABI differ")
	}
	return data
}
func (f *repair244Fixture) decide(t *testing.T, domain string, data []byte) Decision {
	t.Helper()
	to, _ := evm.PrecompileAddress(domain)
	return f.h.policy.Evaluate(testAccount, f.h.tx(t, to, big.NewInt(0), data), f.h.ledger)
}
func repair244Spend(t *testing.T, d Decision, asset [32]byte, n int64) {
	t.Helper()
	expect(t, d, CodeAllowed)
	if len(d.Spends) != 1 || d.Spends[0].Asset != ConstructAsset(asset) || d.Spends[0].Amount.Cmp(big.NewInt(n)) != 0 {
		t.Fatalf("spends %+v, want %d of %x exactly once", d.Spends, n, asset)
	}
}
func (f *repair244Fixture) create(t *testing.T, native bool, amount int64) []byte {
	t.Helper()
	method := "budgetCreate"
	args := []any{f.budget, f.accounts[1].ID, f.accounts[0].Asset, [32]byte{9}, big.NewInt(100), big.NewInt(0), big.NewInt(amount), uint64(1000), uint64(0), uint64(1000000), uint64(0), uint8(1)}
	if native {
		method = "budgetCreateNative"
		args[2] = f.accounts[3].Asset
		args = append(args, f.accounts[3].ID, uint64(0))
	}
	return repair244Pack(t, "layerxpay", method, args...)
}
func repair244Balance(t *testing.T, index uint, want uint64) {
	t.Helper()
	a, err := testkernel.Info(index)
	if err != nil {
		t.Fatal(err)
	}
	if a.Balance != want {
		t.Fatalf("native account %d balance %d, want %d", index, a.Balance, want)
	}
}

func TestRepair_244_TransferAssetAndOwnedEffects(t *testing.T) {
	f := repair244Open(t)
	asset := f.accounts[0].Asset
	legs := []policyTestTransferLeg{{From: f.accounts[0].ID, Asset: asset, To: f.accounts[2].ID, Amount: big.NewInt(4)}, {From: f.accounts[3].ID, Asset: asset, To: f.accounts[1].ID, Amount: big.NewInt(3)}}
	data := repair244Pack(t, "layerxprograms", "transfer", [32]byte{1}, legs)
	to, _ := evm.PrecompileAddress("layerxprograms")
	inspected, err := inspectCallContext(Context{ChainID: chainID, Account: f.pin.Account, ConstructState: f.state}, to, big.NewInt(0), data)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspected.Transfers) != 2 || inspected.Transfers[0].From != legs[0].From || inspected.Transfers[0].To != legs[0].To || !inspected.Transfers[0].FromOwned || inspected.Transfers[0].ToOwned || !inspected.Transfers[1].ToOwned || inspected.Transfers[0].Asset != asset {
		t.Fatalf("lost owned debit/credit effects: %+v", inspected.Transfers)
	}
	repair244Spend(t, f.decide(t, "layerxprograms", data), asset, 7)
	if err := testkernel.Execute("layerxprograms", data); err != nil {
		t.Fatal(err)
	}
	repair244Balance(t, 0, 996)
	repair244Balance(t, 3, 997)
	repair244Balance(t, 2, 4)
	repair244Balance(t, 1, 3)
	f.refresh(t, false)
	// Existing EVM wei caps never authorize native ledger asset units.
	delete(f.h.policy.doc.Defaults.Caps, ConstructAsset(asset))
	expect(t, f.decide(t, "layerxprograms", data), CodeNoCap)
	repair244Balance(t, 0, 996)
	f.h.policy.doc.Defaults.Caps[ConstructAsset(asset)] = Cap{PerTransaction: "6", Daily: "15"}
	expect(t, f.decide(t, "layerxprograms", data), CodeValueCap)
	f.h.policy.doc.Defaults.Caps[ConstructAsset(asset)] = Cap{PerTransaction: "10", Daily: "13"}
	expect(t, f.decide(t, "layerxprograms", data), CodeDailyCap)
	legs[0].From = f.accounts[2].ID
	data = repair244Pack(t, "layerxprograms", "transfer", [32]byte{1}, legs)
	expect(t, f.decide(t, "layerxprograms", data), CodeDestinationDenied)
}

func TestRepair_244_StorageFundingLegs(t *testing.T) {
	f := repair244Open(t)
	asset := f.accounts[0].Asset
	pack := func(amount int64, legs []policyTestTransferLeg) []byte {
		return repair244Pack(t, "layerxprograms", "sandboxFund", [32]byte{1}, f.accounts[0].ID, [32]byte{3}, f.accounts[4].ID, asset, big.NewInt(amount), uint64(1000000), uint32(1), [7]uint64{}, []byte{1}, [32]byte{3}, legs)
	}
	legs := []policyTestTransferLeg{{From: f.accounts[0].ID, Asset: asset, To: f.accounts[4].ID, Amount: big.NewInt(3)}, {From: f.accounts[3].ID, Asset: asset, To: f.accounts[4].ID, Amount: big.NewInt(4)}}
	data := pack(7, legs)
	repair244Spend(t, f.decide(t, "layerxprograms", data), asset, 7)
	if err := testkernel.Execute("layerxprograms", data); err != nil {
		t.Fatal(err)
	}
	repair244Balance(t, 0, 997)
	repair244Balance(t, 3, 996)
	repair244Balance(t, 4, 7)
	expect(t, f.decide(t, "layerxprograms", pack(8, legs)), CodeDecodeError)
	expect(t, f.decide(t, "layerxprograms", pack(7, nil)), CodeDecodeError)
	legs[0].Asset[0] ^= 1
	expect(t, f.decide(t, "layerxprograms", pack(7, legs)), CodeDecodeError)
}

func TestRepair_244_BudgetCreationAndResolvedFunding(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "legacy"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			f := repair244OpenSource(t, native)
			source := uint(0)
			if native {
				source = 3
			}
			asset := f.accounts[source].Asset
			// The same initial debit is rejected by the per-transaction cap first.
			expect(t, f.decide(t, "layerxpay", f.create(t, native, 11)), CodeValueCap)
			data := f.create(t, native, 10)
			repair244Spend(t, f.decide(t, "layerxpay", data), asset, 10)
			if err := testkernel.Execute("layerxpay", data); err != nil {
				t.Fatal(err)
			}
			repair244Balance(t, source, 990)
			repair244Balance(t, 1, 10)
			if native {
				repair244Balance(t, 0, 1000)
			}
			method := "budgetFund"
			args := []any{f.budget, big.NewInt(5)}
			if native {
				method = "budgetFundNative"
				args = append(args, uint64(1))
			}
			funding := repair244Pack(t, "layerxpay", method, args...)
			// Calldata contains no asset; absence of its state proof must refuse it.
			expect(t, f.decide(t, "layerxpay", funding), CodeMissingField)
			f.refresh(t, true)
			repair244Spend(t, f.decide(t, "layerxpay", funding), asset, 5)
			if err := testkernel.Execute("layerxpay", funding); err != nil {
				t.Fatal(err)
			}
			repair244Balance(t, source, 985)
			repair244Balance(t, 1, 15)
			f.refresh(t, true)
			args[1] = big.NewInt(1)
			if native {
				args[2] = uint64(2)
			}
			expect(t, f.decide(t, "layerxpay", repair244Pack(t, "layerxpay", method, args...)), CodeDailyCap)
			f.h.now = f.h.now.Add(SpendWindow + time.Second)
			repair244Spend(t, f.decide(t, "layerxpay", repair244Pack(t, "layerxpay", method, args...)), asset, 1)
		})
	}
}

func TestRepair_244_EscrowReservation(t *testing.T) {
	f := repair244Open(t)
	asset := f.accounts[0].Asset
	pack := func(n int64) []byte {
		return repair244Pack(t, "layerxpay", "escrowOpen", [32]byte{1}, f.accounts[0].ID, f.accounts[4].ID, f.accounts[2].ID, [32]byte{2}, asset, big.NewInt(n), uint64(1000000), uint64(1000), [32]byte{3}, [32]byte{4})
	}
	expect(t, f.decide(t, "layerxpay", pack(11)), CodeValueCap)
	data := pack(10)
	repair244Spend(t, f.decide(t, "layerxpay", data), asset, 10)
	if err := testkernel.Execute("layerxpay", data); err != nil {
		t.Fatal(err)
	}
	repair244Balance(t, 0, 990)
	repair244Balance(t, 4, 10)
	expect(t, f.decide(t, "layerxpay", pack(6)), CodeDailyCap)
	delete(f.h.policy.doc.Defaults.Caps, ConstructAsset(asset))
	expect(t, f.decide(t, "layerxpay", pack(1)), CodeNoCap)
}

func TestRepair_244_NativeBudgetAcrossGoroutines(t *testing.T) {
	f := repair244OpenSource(t, true)
	asset := f.accounts[3].Asset
	created := f.create(t, true, 10)
	repair244Spend(t, f.decide(t, "layerxpay", created), asset, 10)
	execute := func(data []byte) {
		t.Helper()
		result := make(chan error, 1)
		go func() { result <- testkernel.Execute("layerxpay", data) }()
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	execute(created)
	f.refresh(t, true)
	funded := repair244Pack(t, "layerxpay", "budgetFundNative", f.budget, big.NewInt(5), uint64(1))
	repair244Spend(t, f.decide(t, "layerxpay", funded), asset, 5)
	execute(funded)
	f.refresh(t, true)
	repair244Balance(t, 0, 1000)
	repair244Balance(t, 3, 985)
	repair244Balance(t, 1, 15)
	expect(t, f.decide(t, "layerxpay", repair244Pack(t, "layerxpay", "budgetFundNative", f.budget, big.NewInt(1), uint64(2))), CodeDailyCap)
}

func TestRepair_244_StateProofAndFailClosed(t *testing.T) {
	f := repair244Open(t)
	for name, mutate := range map[string]func(*ConstructPin){
		"root":      func(p *ConstructPin) { p.StateRoot[0] ^= 1 },
		"authority": func(p *ConstructPin) { p.AuthorityKey[0] ^= 1 },
		"actor":     func(p *ConstructPin) { p.ActorDID = "did:key:foreign" },
		"unpinned":  func(p *ConstructPin) { p.BlockHash = common.Hash{} },
	} {
		t.Run(name, func(t *testing.T) {
			pin := f.pin
			mutate(&pin)
			if s, err := NewConstructState(pin, f.witnesses); err == nil || s != nil {
				t.Fatal("accepted unbound state")
			}
		})
	}
	raw := append([]byte(nil), f.witnesses[0]...)
	raw[len(raw)-1] ^= 1
	if s, err := NewConstructState(f.pin, [][]byte{raw}); err == nil || s != nil {
		t.Fatal("accepted corrupted state proof")
	}
	// The state transport returns decoded real kernel account bytes, with no
	// boolean or caller-supplied asset replacing proof verification.
	w, err := codec.DecodeStateWitness(f.witnesses[0])
	if err != nil {
		t.Fatal(err)
	}
	account, err := codec.DecodeAccountValue(f.accounts[0].ID, w.Value)
	if err != nil || account.AssetID != f.accounts[0].Asset {
		t.Fatalf("account codec: %+v %v", account, err)
	}
	data := f.create(t, false, 1)
	f.h.policy.SetConstructStateReader(nil)
	expect(t, f.decide(t, "layerxpay", data), CodeMissingField)
	f.h.policy.SetConstructStateReader(func(Context) (*ConstructState, error) {
		pin := f.pin
		pin.ChainID++
		return NewConstructState(pin, f.witnesses)
	})
	expect(t, f.decide(t, "layerxpay", data), CodeMissingField)
	f.h.policy.SetConstructStateReader(func(Context) (*ConstructState, error) {
		pin := f.pin
		pin.Account = common.HexToAddress(friend)
		return NewConstructState(pin, f.witnesses)
	})
	expect(t, f.decide(t, "layerxpay", data), CodeMissingField)
	for _, tc := range []struct {
		domain, method string
		args           []any
	}{
		{"layerxpay", "budgetSpend", []any{f.budget, f.accounts[2].ID, big.NewInt(1)}},
		{"layerxpay", "budgetPull", []any{f.budget, f.accounts[2].ID, big.NewInt(1), []byte{}}},
		{"layerxprograms", "call", []any{[32]byte{1}, uint16(1), "execute", []byte{}, []byte{}, []byte{}, uint32(0), [7]uint64{}, big.NewInt(100)}},
	} {
		expect(t, f.decide(t, tc.domain, repair244Pack(t, tc.domain, tc.method, tc.args...)), CodeSelectorNotAllowed)
	}
}

func TestRepair_244_SponsoredBatchUsesOnePinAndAggregates(t *testing.T) {
	f := repair244Open(t)
	reads := 0
	f.h.policy.SetConstructStateReader(func(Context) (*ConstructState, error) { reads++; return f.state, nil })
	to, _ := evm.PrecompileAddress("layerxprograms")
	request := func(first, second int64) Request {
		t.Helper()
		batch := sponsoredBatch()
		batch.Quote.TokenAmount = big.NewInt(0)
		batch.Calls = nil
		for _, n := range []int64{first, second} {
			data := repair244Pack(t, "layerxprograms", "transfer", [32]byte{1}, []policyTestTransferLeg{{From: f.accounts[0].ID, Asset: f.accounts[0].Asset, To: f.accounts[2].ID, Amount: big.NewInt(n)}})
			batch.Calls = append(batch.Calls, evm.BatchCall{To: to, Value: big.NewInt(0), Data: data})
		}
		digest, err := evm.SponsoredBatchDigest(batch)
		if err != nil {
			t.Fatal(err)
		}
		return Request{Kind: KindSponsoredBatch, View: &evm.SponsoredBatchClaim{Batch: batch, ClaimedDigest: digest}}
	}
	expect(t, f.h.policy.Evaluate(testAccount, request(6, 6), f.h.ledger), CodeValueCap)
	if reads != 1 {
		t.Fatalf("two calls used %d state pins, want one", reads)
	}
	repair244Spend(t, f.h.policy.Evaluate(testAccount, request(4, 3), f.h.ledger), f.accounts[0].Asset, 7)
	if reads != 2 {
		t.Fatalf("evaluation did not refresh its pin: %d reads", reads)
	}
	expect(t, f.h.policy.Evaluate(testAccount, request(5, 4), f.h.ledger), CodeDailyCap)
	if reads != 3 {
		t.Fatalf("batch did not keep one snapshot: %d reads", reads)
	}
}
