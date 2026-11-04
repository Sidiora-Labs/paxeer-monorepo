package abi_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	spotkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const repair19Gas = 30_000_000

type repair19Env struct {
	t        *testing.T
	app      *app.App
	ctx      sdk.Context
	keeper   *spotkeeper.Keeper
	contract abi.ABI
	pointer  common.Address
	holder   common.Address
	denom    string
}

func repair19Address(name string) common.Address {
	return common.BytesToAddress(crypto.Keccak256([]byte(name))[12:])
}

func newRepair19Env(t *testing.T) *repair19Env {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	pricefeed, ctx := pricefeedtestutil.NewKeeper(a, ctx)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key)
	k := spotkeeper.NewKeeper(key, a.BankKeeper, &a.EvmKeeper, pricefeed)
	contract, err := spotabi.Share()
	require.NoError(t, err)
	e := &repair19Env{t: t, app: a, ctx: ctx, keeper: k, contract: contract,
		holder: repair19Address(t.Name() + " holder")}
	token := repair19Address(t.Name() + " underlying")
	e.denom = k.ShareDenom(token)
	e.run(func(ctx sdk.Context, evm *vm.EVM) error {
		var err error
		e.pointer, err = k.CreateShareToken(ctx, evm, token, "USDL", 6)
		return err
	})
	pointer, _, exists := a.EvmKeeper.GetERC20NativePointer(e.ctx, e.denom)
	require.True(t, exists)
	require.Equal(t, e.pointer, pointer)
	require.NotEmpty(t, a.EvmKeeper.GetCode(ctx, pointer))
	return e
}

func (e *repair19Env) run(fn func(sdk.Context, *vm.EVM) error) []*ethtypes.Log {
	e.t.Helper()
	var logs []*ethtypes.Log
	err := e.app.EvmKeeper.RunWithOneOffEVMInstance(e.ctx, func(evm *vm.EVM) error {
		db := state.GetDBImpl(evm.StateDB)
		require.NotNil(e.t, db)
		if err := fn(db.Ctx(), evm); err != nil {
			return err
		}
		logs = db.GetAllLogs()
		return nil
	}, func(string, string) {})
	require.NoError(e.t, err)
	return logs
}

func (e *repair19Env) call(contract abi.ABI, caller common.Address, method string, args ...interface{}) ([]byte, error) {
	e.t.Helper()
	input, err := contract.Pack(method, args...)
	require.NoError(e.t, err)
	var ret []byte
	err = e.app.EvmKeeper.RunWithOneOffEVMInstance(e.ctx, func(evm *vm.EVM) error {
		var callErr error
		ret, _, callErr = evm.Call(caller, e.pointer, input, repair19Gas, uint256.NewInt(0))
		return callErr
	}, func(string, string) {})
	return ret, err
}

func (e *repair19Env) view(method string, args ...interface{}) interface{} {
	e.t.Helper()
	input, err := e.contract.Pack(method, args...)
	require.NoError(e.t, err)
	ret, err := e.app.EvmKeeper.StaticCallEVM(e.ctx, spotkeeper.CustodyAddress(), &e.pointer, input)
	require.NoError(e.t, err)
	values, err := e.contract.Unpack(method, ret)
	require.NoError(e.t, err)
	require.Len(e.t, values, 1)
	return values[0]
}

func (e *repair19Env) mint(amount int64) []*ethtypes.Log {
	return e.run(func(ctx sdk.Context, evm *vm.EVM) error {
		return e.keeper.MintShares(ctx, evm, e.denom, e.holder, sdk.NewInt(amount))
	})
}

func (e *repair19Env) requireTransferLog(logs []*ethtypes.Log, from, to common.Address, amount int64) {
	e.t.Helper()
	event := e.contract.Events["Transfer"]
	var found []*ethtypes.Log
	for _, log := range logs {
		if log.Address == e.pointer && len(log.Topics) > 0 && log.Topics[0] == event.ID {
			found = append(found, log)
		}
	}
	require.Len(e.t, found, 1)
	require.Equal(e.t, []common.Hash{event.ID, common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())}, found[0].Topics)
	values, err := event.Inputs.NonIndexed().Unpack(found[0].Data)
	require.NoError(e.t, err)
	require.Equal(e.t, []interface{}{big.NewInt(amount)}, values)
}

func TestRepair_19_NativeShareDeploymentInterface(t *testing.T) {
	e := newRepair19Env(t)
	producer := native.GetParsedABI()
	require.Equal(t, producer.Constructor.Sig, e.contract.Constructor.Sig)
	require.Equal(t, producer.Methods, e.contract.Methods)
	require.Equal(t, producer.Events, e.contract.Events)
	require.Equal(t, producer.Errors, e.contract.Errors)
	for name, want := range map[string]interface{}{
		"name": "Spot Vault Share USDL", "symbol": "sUSDL", "decimals": uint8(6),
		"nname": "Spot Vault Share USDL", "ssymbol": "sUSDL", "ddecimals": uint8(6),
		"denom": e.denom, "BankPrecompile": common.HexToAddress("0x1001"),
	} {
		require.Equal(t, want, e.view(name), name)
	}
	require.Zero(t, e.view("totalSupply").(*big.Int).Sign())
	e.requireTransferLog(e.mint(100), common.Address{}, e.holder, 100)
	require.Equal(t, big.NewInt(100), e.view("balanceOf", e.holder))
	require.Equal(t, big.NewInt(100), e.view("totalSupply"))
	spender, receiver := repair19Address(t.Name()+" spender"), repair19Address(t.Name()+" receiver")
	for _, call := range []struct {
		caller common.Address
		method string
		args   []interface{}
	}{
		{e.holder, "approve", []interface{}{spender, big.NewInt(40)}},
		{spender, "transferFrom", []interface{}{e.holder, receiver, big.NewInt(30)}},
		{e.holder, "transfer", []interface{}{receiver, big.NewInt(20)}},
	} {
		ret, err := e.call(e.contract, call.caller, call.method, call.args...)
		require.NoError(t, err)
		values, err := e.contract.Unpack(call.method, ret)
		require.NoError(t, err)
		require.Equal(t, []interface{}{true}, values)
	}
	require.Equal(t, big.NewInt(10), e.view("allowance", e.holder, spender))
	require.Equal(t, big.NewInt(50), e.view("balanceOf", e.holder))
	require.Equal(t, big.NewInt(50), e.view("balanceOf", receiver))
	logs := e.run(func(ctx sdk.Context, evm *vm.EVM) error {
		return e.keeper.BurnShares(ctx, evm, e.denom, e.holder, sdk.NewInt(10))
	})
	e.requireTransferLog(logs, e.holder, common.Address{}, 10)
	require.Equal(t, big.NewInt(40), e.view("balanceOf", e.holder))
	require.Equal(t, big.NewInt(90), e.view("totalSupply"))
	require.Equal(t, big.NewInt(40), e.app.BankKeeper.GetBalance(e.ctx,
		e.app.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.holder), e.denom).Amount.BigInt())
}

func TestRepair_19_ShareAdministrationUnserved(t *testing.T) {
	e := newRepair19Env(t)
	e.mint(100)
	historical, err := spotabi.HistoricalSolidityShare()
	require.NoError(t, err)
	for _, method := range []struct {
		name     string
		selector string
		args     []interface{}
	}{
		{"mint", "0x40c10f19", []interface{}{e.holder, big.NewInt(1)}},
		{"burn", "0x9dc29fac", []interface{}{e.holder, big.NewInt(1)}},
		{"owner", "0x8da5cb5b", nil},
		{"renounceOwnership", "0x715018a6", nil},
		{"transferOwnership", "0xf2fde38b", []interface{}{e.holder}},
	} {
		t.Run(method.name, func(t *testing.T) {
			require.NotContains(t, e.contract.Methods, method.name)
			require.Equal(t, common.FromHex(method.selector), historical.Methods[method.name].ID)
			for _, caller := range []common.Address{e.holder, common.HexToAddress(types.PECORVaultAddress), types.GovernanceEVMAddress()} {
				ret, err := e.call(historical, caller, method.name, method.args...)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Empty(t, ret)
				require.Equal(t, big.NewInt(100), e.view("balanceOf", e.holder))
				require.Equal(t, big.NewInt(100), e.view("totalSupply"))
			}
		})
	}
	require.NotContains(t, e.contract.Events, "OwnershipTransferred")
	require.Contains(t, historical.Events, "OwnershipTransferred")
}

func TestRepair_19_ShareBalanceRevert(t *testing.T) {
	for _, method := range []string{"transfer", "transferFrom"} {
		t.Run(method, func(t *testing.T) {
			e := newRepair19Env(t)
			e.mint(100)
			spender, receiver := repair19Address(t.Name()+" spender"), repair19Address(t.Name()+" receiver")
			caller := e.holder
			args := []interface{}{receiver, big.NewInt(101)}
			if method == "transferFrom" {
				_, err := e.call(e.contract, e.holder, "approve", spender, big.NewInt(101))
				require.NoError(t, err)
				require.Equal(t, big.NewInt(101), e.view("allowance", e.holder, spender))
				caller = spender
				args = []interface{}{e.holder, receiver, big.NewInt(101)}
			}
			ret, err := e.call(e.contract, caller, method, args...)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, big.NewInt(100), e.view("balanceOf", e.holder))
			require.Zero(t, e.view("balanceOf", receiver).(*big.Int).Sign())
			require.Equal(t, big.NewInt(100), e.view("totalSupply"))
			if method == "transferFrom" {
				require.Equal(t, big.NewInt(101), e.view("allowance", e.holder, spender))
			}
			declared := e.contract.Errors["ERC20InsufficientBalance"]
			require.Equal(t, common.FromHex("0xe450d38c"), declared.ID.Bytes()[:4])
			require.GreaterOrEqual(t, len(ret), 4)
			require.Equal(t, declared.ID.Bytes()[:4], ret[:4], "native pointer must preserve the frozen custom error; actual revert: %x", ret)
			values, err := declared.Unpack(ret)
			require.NoError(t, err)
			require.Equal(t, []interface{}{e.holder, big.NewInt(100), big.NewInt(101)}, values)
		})
	}
}

func TestRepair_19_ShareBalanceRevertFullWidth(t *testing.T) {
	for _, method := range []string{"transfer", "transferFrom"} {
		t.Run(method, func(t *testing.T) {
			e := newRepair19Env(t)
			e.mint(100)
			needed := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
			spender := repair19Address(t.Name() + " spender")
			receiver := repair19Address(t.Name() + " receiver")
			caller := e.holder
			args := []interface{}{receiver, needed}
			if method == "transferFrom" {
				_, err := e.call(e.contract, e.holder, "approve", spender, needed)
				require.NoError(t, err)
				caller = spender
				args = []interface{}{e.holder, receiver, needed}
			}
			ret, err := e.call(e.contract, caller, method, args...)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			declared := e.contract.Errors["ERC20InsufficientBalance"]
			require.GreaterOrEqual(t, len(ret), 4)
			require.Equal(t, common.FromHex("0xe450d38c"), ret[:4])
			values, err := declared.Unpack(ret)
			require.NoError(t, err)
			require.Equal(t, []interface{}{e.holder, big.NewInt(100), needed}, values)
			require.Equal(t, big.NewInt(100), e.view("balanceOf", e.holder))
			require.Zero(t, e.view("balanceOf", receiver).(*big.Int).Sign())
			require.Equal(t, big.NewInt(100), e.view("totalSupply"))
			if method == "transferFrom" {
				require.Equal(t, needed, e.view("allowance", e.holder, spender))
			}
		})
	}
}

func TestRepair_19_ShareBalanceErrorPrecedence(t *testing.T) {
	e := newRepair19Env(t)
	e.mint(100)
	spender, receiver := repair19Address(t.Name()+" spender"), repair19Address(t.Name()+" receiver")
	for _, test := range []struct {
		name   string
		caller common.Address
		method string
		args   []interface{}
		want   []byte
	}{
		{"allowance-before-balance", spender, "transferFrom", []interface{}{e.holder, receiver, big.NewInt(101)},
			types.ERC20InsufficientAllowance(spender, uint256.NewInt(0), uint256.NewInt(101)).Data()},
		{"receiver-before-balance", e.holder, "transfer", []interface{}{common.Address{}, big.NewInt(101)},
			types.ERC20InvalidReceiver(common.Address{}).Data()},
		{"invalid-sender", common.Address{}, "transfer", []interface{}{receiver, big.NewInt(0)},
			types.ERC20InvalidSender(common.Address{}).Data()},
		{"invalid-approver-before-sender", spender, "transferFrom", []interface{}{common.Address{}, receiver, big.NewInt(0)},
			types.ERC20InvalidApprover(common.Address{}).Data()},
		{"invalid-spender", e.holder, "approve", []interface{}{common.Address{}, big.NewInt(1)},
			types.ERC20InvalidSpender(common.Address{}).Data()},
	} {
		t.Run(test.name, func(t *testing.T) {
			ret, err := e.call(e.contract, test.caller, test.method, test.args...)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, test.want, ret)
			require.Equal(t, big.NewInt(100), e.view("balanceOf", e.holder))
			require.Zero(t, e.view("balanceOf", receiver).(*big.Int).Sign())
			require.Equal(t, big.NewInt(100), e.view("totalSupply"))
			require.Zero(t, e.view("allowance", e.holder, spender).(*big.Int).Sign())
		})
	}
	// The boundary must report a zero bank balance without confusing the
	// spender with the token owner, and must leave subsequent calls usable.
	ret, err := e.call(e.contract, receiver, "transfer", e.holder, big.NewInt(1))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	declared := e.contract.Errors["ERC20InsufficientBalance"]
	require.GreaterOrEqual(t, len(ret), 4)
	require.Equal(t, common.FromHex("0xe450d38c"), ret[:4])
	values, err := declared.Unpack(ret)
	require.NoError(t, err)
	decoded, ok := values.([]interface{})
	require.True(t, ok)
	require.Len(t, decoded, 3)
	require.Equal(t, receiver, decoded[0])
	require.Zero(t, decoded[1].(*big.Int).Sign())
	require.Equal(t, big.NewInt(1), decoded[2])
	ret, err = e.call(e.contract, e.holder, "transfer", receiver, big.NewInt(100))
	require.NoError(t, err)
	values, err = e.contract.Unpack("transfer", ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{true}, values)
	require.Zero(t, e.view("balanceOf", e.holder).(*big.Int).Sign())
	require.Equal(t, big.NewInt(100), e.view("balanceOf", receiver))
	require.Equal(t, big.NewInt(100), e.view("totalSupply"))
}
