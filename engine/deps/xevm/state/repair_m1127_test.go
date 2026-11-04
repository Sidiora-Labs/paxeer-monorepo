package state_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/state"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1127_012_EmptyCodeReloadGasAndHash(t *testing.T) {
	checkEmptyCodeReloadM1127(t)
}

func checkEmptyCodeReloadM1127(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	h := openRepair95Keeper(t, dir)
	require.False(t, h.k.UseRegularStore, "exercise the Giga adapter")
	db := state.NewDBImpl(h.ctx, h.k, false)
	nilCode := common.HexToAddress("0x112701")
	emptyCode := common.HexToAddress("0x112702")
	cleared := common.HexToAddress("0x112703")
	absent := common.HexToAddress("0x112704")
	funded := common.HexToAddress("0x112705")
	nonce := common.HexToAddress("0x112706")
	nonempty := common.HexToAddress("0x112707")

	// Both the keeper producer and the StateDB entry point must emit the
	// representation that Empty and the VM consume after a reopen.
	h.k.SetCode(db.Ctx(), nilCode, nil)
	db.SetCode(emptyCode, []byte{})
	db.SetCode(cleared, []byte{byte(vm.STOP)})
	require.False(t, db.Empty(cleared))
	require.Equal(t, []byte{byte(vm.STOP)}, db.SetCode(cleared, nil))
	db.SetCode(funded, nil)
	db.AddBalance(funded, uint256.NewInt(1), tracing.BalanceChangeUnspecified)
	db.SetCode(nonce, nil)
	db.SetNonce(nonce, 1, tracing.NonceChangeUnspecified)
	db.SetCode(nonempty, []byte{byte(vm.STOP)})
	creator := common.HexToAddress("0x112708")
	code, deployed, _, err := newRepair95EVM(db).Create(creator,
		[]byte{byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.RETURN)},
		100_000, new(uint256.Int))
	require.NoError(t, err)
	require.Empty(t, code)
	require.Equal(t, crypto.CreateAddress(creator, 0), deployed)
	require.Equal(t, gethtypes.EmptyCodeHash, db.GetCodeHash(deployed))
	require.Equal(t, uint64(1), db.GetNonce(deployed))
	require.False(t, db.Empty(deployed))

	targets := []struct {
		name       string
		addr       common.Address
		empty      bool
		storedHash common.Hash
		vmHash     common.Hash
	}{
		{"nil", nilCode, true, gethtypes.EmptyCodeHash, common.Hash{}},
		{"zero length", emptyCode, true, gethtypes.EmptyCodeHash, common.Hash{}},
		{"cleared", cleared, true, gethtypes.EmptyCodeHash, common.Hash{}},
		{"absent", absent, true, common.Hash{}, common.Hash{}},
		{"balance", funded, false, gethtypes.EmptyCodeHash, gethtypes.EmptyCodeHash},
		{"nonce", nonce, false, gethtypes.EmptyCodeHash, gethtypes.EmptyCodeHash},
		{"zero byte deployment", deployed, false, gethtypes.EmptyCodeHash, gethtypes.EmptyCodeHash},
		{"code", nonempty, false, crypto.Keccak256Hash([]byte{byte(vm.STOP)}), crypto.Keccak256Hash([]byte{byte(vm.STOP)})},
	}
	for _, phase := range []string{"before close", "after reopen"} {
		t.Run(phase, func(t *testing.T) {
			for _, target := range targets {
				t.Run(target.name, func(t *testing.T) {
					require.Equal(t, target.empty, db.Empty(target.addr))
					require.Equal(t, target.storedHash, h.k.GetCodeHash(db.Ctx(), target.addr))
					require.Equal(t, target.vmHash, repair95CodeHash(t, db, target.addr), "real EXTCODEHASH")
					if target.addr != nonempty {
						require.Empty(t, db.GetCode(target.addr))
						require.Zero(t, db.GetCodeSize(target.addr))
					}
					snapshot := db.Snapshot()
					before := db.GetBalance(target.addr).ToBig()
					used := repair95CallGas(t, db, target.addr)
					// Seven pushes, warm CALL, value transfer, stipend refund,
					// and the memory/return sequence; EIP-161 adds 25000 for empties.
					want := uint64(7*3 + params.WarmStorageReadCostEIP2929 + params.CallValueTransferGas - params.CallStipend + 15)
					if target.empty {
						want += params.CallNewAccountGas
					}
					require.Equal(t, want, used)
					require.Equal(t, new(big.Int).Add(before, big.NewInt(1)),
						h.k.GetBalance(db.Ctx(), h.k.GetPaxAddressOrDefault(db.Ctx(), target.addr)))
					db.RevertToSnapshot(snapshot)
					require.Equal(t, before, db.GetBalance(target.addr).ToBig())
					require.Equal(t, target.empty, db.Empty(target.addr))
				})
			}
		})
		if phase == "before close" {
			h.commit(t, db)
			h.close(t)
			h = openRepair95Keeper(t, dir)
			db = state.NewDBImpl(h.ctx, h.k, false)
		}
	}
}

func TestRepairM_1127_013_RefuseRealChainsBeforeFunding(t *testing.T) {
	for _, chainID := range []string{
		"hyperpax_125-1", "125", "hyperpax_125-2", "pacific-1",
		"atlantic-2", "arctic-1", "", "unknown-chain", "test-chain",
		"mock-balances-local-2", "mock-balances-local-1 ", "MOCK-BALANCES-LOCAL-1",
	} {
		t.Run(chainID, func(t *testing.T) {
			h := openRepair95Keeper(t, t.TempDir())
			db := state.NewDBImpl(h.ctx.WithChainID(chainID), h.k, false)
			addr := common.HexToAddress("0x112713")
			paxAddr := h.k.GetPaxAddressOrDefault(db.Ctx(), addr)
			bank := h.k.BankKeeper()
			denom := h.k.GetBaseDenom(db.Ctx())
			minimum, sufficient := state.RepairM1127MockBalanceHooks(db)
			for _, hook := range []struct {
				name string
				call func()
			}{
				{"GetBalance hook", func() { minimum(addr) }},
				{"SubBalance hook", func() { sufficient(addr, big.NewInt(1)) }},
				{"zero debit hook", func() { sufficient(addr, new(big.Int)) }},
			} {
				t.Run(hook.name, func(t *testing.T) {
					supply := bank.GetSupply(db.Ctx(), denom)
					events := len(db.Ctx().EventManager().Events())
					require.False(t, h.k.AccountKeeper().HasAccount(db.Ctx(), paxAddr))
					require.PanicsWithValue(t,
						"FATAL: mock_balances requires the disposable mock-balances-local-1 chain", hook.call)
					require.Equal(t, big.NewInt(0), h.k.GetBalance(db.Ctx(), paxAddr))
					require.Equal(t, supply, bank.GetSupply(db.Ctx(), denom))
					require.Len(t, db.Ctx().EventManager().Events(), events)
					require.False(t, h.k.AccountKeeper().HasAccount(db.Ctx(), paxAddr))
					require.NoError(t, db.Err())
				})
			}
		})
	}
}

func TestRepairM_1127_013_DisposableChainFunding(t *testing.T) {
	h := openRepair95Keeper(t, t.TempDir())
	db := state.NewDBImpl(h.ctx.WithChainID("mock-balances-local-1"), h.k, false)
	minimum, sufficient := state.RepairM1127MockBalanceHooks(db)
	bank := h.k.BankKeeper()
	denom := h.k.GetBaseDenom(db.Ctx())
	topOff := new(big.Int).Mul(big.NewInt(100), big.NewInt(1_000_000_000_000_000_000))
	require.Equal(t, topOff, state.TopOffAmount)
	addr := common.HexToAddress("0x112714")
	paxAddr := h.k.GetPaxAddressOrDefault(db.Ctx(), addr)
	supply := bank.GetSupply(db.Ctx(), denom).Amount
	minimum(addr)
	require.Equal(t, topOff, h.k.GetBalance(db.Ctx(), paxAddr))
	require.True(t, h.k.AccountKeeper().HasAccount(db.Ctx(), paxAddr))
	uhpx, _ := state.SplitUhpxWeiAmount(topOff)
	require.True(t, bank.GetSupply(db.Ctx(), denom).Amount.Equal(supply.Add(uhpx).Add(sdk.OneInt())))
	supply = bank.GetSupply(db.Ctx(), denom).Amount
	minimum(addr)
	require.True(t, bank.GetSupply(db.Ctx(), denom).Amount.Equal(supply), "funded reads do not mint again")

	// The debit hook must work independently, without a preceding read top-off.
	debitAddr := common.HexToAddress("0x112715")
	debitPax := h.k.GetPaxAddressOrDefault(db.Ctx(), debitAddr)
	debit := new(big.Int).Add(topOff, big.NewInt(19))
	sufficient(debitAddr, debit)
	want := new(big.Int).Add(topOff, debit)
	require.Equal(t, want, h.k.GetBalance(db.Ctx(), debitPax))
	previous := db.SubBalance(debitAddr, uint256.MustFromBig(debit), tracing.BalanceDecreaseGasBuy)
	require.Equal(t, want, previous.ToBig())
	require.NoError(t, db.Err())
	require.Equal(t, topOff, db.GetBalance(debitAddr).ToBig())
	require.Equal(t, topOff, h.k.GetBalance(db.Ctx(), debitPax))
	supply = bank.GetSupply(db.Ctx(), denom).Amount
	sufficient(debitAddr, big.NewInt(1))
	require.True(t, bank.GetSupply(db.Ctx(), denom).Amount.Equal(supply), "sufficient funds do not mint again")
}

func TestRepairM_1127_013_OrdinaryBuildDoesNotFund(t *testing.T) {
	for _, chainID := range []string{"hyperpax_125-1", "mock-balances-local-1"} {
		t.Run(chainID, func(t *testing.T) {
			h := openRepair95Keeper(t, t.TempDir())
			db := state.NewDBImpl(h.ctx.WithChainID(chainID), h.k, false)
			addr := common.HexToAddress("0x112716")
			bank := h.k.BankKeeper()
			denom := h.k.GetBaseDenom(db.Ctx())
			supply := bank.GetSupply(db.Ctx(), denom)
			require.True(t, db.GetBalance(addr).IsZero())
			db.SubBalance(addr, uint256.NewInt(1), tracing.BalanceDecreaseGasBuy)
			require.Error(t, db.Err(), "an ordinary build must reject an unfunded debit")
			require.Equal(t, supply, bank.GetSupply(db.Ctx(), denom))
			require.True(t, db.GetBalance(addr).IsZero())
		})
	}
}
