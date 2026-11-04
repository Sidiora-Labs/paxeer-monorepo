package v620_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	stakingv620 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking/legacy/v620"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

const repair1209Height int64 = 100

func repair1209Fixture(t *testing.T) (*node.App, sdk.Context, common.Address, sdk.AccAddress) {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).WithBlockHeight(repair1209Height).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).
		WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).CacheContext()
	key, err := crypto.ToECDSA(bytes.Repeat([]byte{0x12}, 32))
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(key.PublicKey)
	payer := sdk.AccAddress(caller.Bytes())
	a.EvmKeeper.SetAddressMapping(ctx, payer, caller)
	return a, ctx, caller, payer
}

func repair1209Fund(t *testing.T, a *node.App, ctx sdk.Context, address sdk.AccAddress, amount int64) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin(a.StakingKeeper.BondDenom(ctx), amount))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, address, coins))
}

func repair1209Input(t *testing.T, contractABI abi.ABI, key string) []byte {
	t.Helper()
	input, err := contractABI.Pack(staking.CreateValidatorMethod, key, "repair-1209", "0.1", "0.2", "0.05", big.NewInt(1))
	require.NoError(t, err)
	return input
}

func repair1209Call(t *testing.T, ctx sdk.Context, k *evmkeeper.Keeper, runner vm.PrecompiledContract, caller common.Address, input []byte) ([]byte, error, *state.DBImpl) {
	t.Helper()
	db := state.NewDBImpl(ctx, k, true)
	evm := &vm.EVM{StateDB: db}
	value := big.NewInt(1_000_000_000_000_000_000)
	if dynamic, ok := runner.(vm.DynamicGasPrecompiledContract); ok {
		ret, _, err := dynamic.RunAndCalculateGas(evm, caller, caller, input, 20_000_000, value, nil, false, false)
		return ret, err, db
	}
	ret, err := runner.Run(evm, caller, caller, input, value, false, false, nil)
	return ret, err, db
}

func TestRepairM_1209_InvalidKeyBeforePayment(t *testing.T) {
	for _, size := range []int{0, 1, 31, 33, 64} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			a, ctx, caller, payer := repair1209Fixture(t)
			a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
			p, err := staking.NewPrecompile(a.GetPrecompileKeepers())
			require.NoError(t, err)
			precompileAccount := a.EvmKeeper.GetPaxAddressOrDefault(ctx, p.Address())
			// Fund the actual payment source, so a missing guard reaches the
			// keeper's consensus-address construction rather than a bank error.
			repair1209Fund(t, a, ctx, precompileAccount, 1_000_000)
			balances := a.BankKeeper.GetAllBalances(ctx, payer)
			precompileBalances := a.BankKeeper.GetAllBalances(ctx, precompileAccount)
			validators := a.StakingKeeper.GetAllValidators(ctx)
			supply := a.BankKeeper.GetSupply(ctx, a.StakingKeeper.BondDenom(ctx))
			input := repair1209Input(t, p.GetABI(), hex.EncodeToString(bytes.Repeat([]byte{1}, size)))
			ret, err, db := repair1209Call(t, ctx, &a.EvmKeeper, p, caller, input)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Empty(t, ret)
			require.EqualError(t, db.GetPrecompileError(), "invalid public key length: expected 32 bytes")
			ctx = db.Ctx()
			require.Equal(t, balances, a.BankKeeper.GetAllBalances(ctx, payer))
			require.Equal(t, precompileBalances, a.BankKeeper.GetAllBalances(ctx, precompileAccount))
			require.Equal(t, supply, a.BankKeeper.GetSupply(ctx, a.StakingKeeper.BondDenom(ctx)))
			require.Equal(t, validators, a.StakingKeeper.GetAllValidators(ctx))
			_, found := a.StakingKeeper.GetDelegation(ctx, payer, sdk.ValAddress(payer))
			require.False(t, found)
		})
	}
}

func TestRepairM_1209_ValidKeyCreatesValidator(t *testing.T) {
	a, ctx, caller, payer := repair1209Fixture(t)
	a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
	p, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	precompileAccount := a.EvmKeeper.GetPaxAddressOrDefault(ctx, p.Address())
	repair1209Fund(t, a, ctx, precompileAccount, 1_000_000)
	key := ed25519.GenPrivKeyFromSecret([]byte("repair-1209-consensus-key")).PubKey()
	ret, err, db := repair1209Call(t, ctx, &a.EvmKeeper, p, caller, repair1209Input(t, p.GetABI(), hex.EncodeToString(key.Bytes())))
	require.NoError(t, err)
	ctx = db.Ctx()
	decoded, err := p.GetABI().Methods[staking.CreateValidatorMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{true}, decoded)
	validator, found := a.StakingKeeper.GetValidator(ctx, sdk.ValAddress(payer))
	require.True(t, found)
	storedKey, err := validator.ConsPubKey()
	require.NoError(t, err)
	require.Equal(t, key.Bytes(), storedKey.Bytes())
	require.Equal(t, "repair-1209", validator.Description.Moniker)
	require.Equal(t, sdk.NewInt(1_000_000), validator.Tokens)
	delegation, found := a.StakingKeeper.GetDelegation(ctx, payer, sdk.ValAddress(payer))
	require.True(t, found)
	require.Equal(t, sdk.NewDec(1_000_000), delegation.Shares)
	require.True(t, a.BankKeeper.GetAllBalances(ctx, precompileAccount).IsZero())
}

func TestRepairM_1209_UpgradeReplay(t *testing.T) {
	a, base, caller, _ := repair1209Fixture(t)
	k := a.EvmKeeper
	address := common.HexToAddress(staking.StakingAddress)
	versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(1), "v6.2.0")
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(base, pcommon.MergeUpgrade))
	archived, err := stakingv620.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	precompileAccount := k.GetPaxAddressOrDefault(base, address)
	repair1209Fund(t, a, base, precompileAccount, 1_000_000)
	input := repair1209Input(t, archived.GetABI(), hex.EncodeToString(bytes.Repeat([]byte{1}, 31)))
	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		switch phase {
		case "scheduled":
			require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(base, upgradetypes.Plan{Name: pcommon.MergeUpgrade, Height: repair1209Height}))
		case "applied":
			a.UpgradeKeeper.SetDone(base.WithBlockHeight(repair1209Height), pcommon.MergeUpgrade)
		}
		for _, height := range []int64{1, repair1209Height - 1, repair1209Height, repair1209Height + 1} {
			for _, tracing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, tracing), func(t *testing.T) {
					ctx, _ := base.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
					selected := k.CustomPrecompiles(ctx)[address]
					require.NotNil(t, selected)
					active := phase == "applied" && height >= repair1209Height
					wantVersion := "v6.2.0"
					if active {
						wantVersion = pcommon.MergeUpgrade
					}
					require.Equal(t, wantVersion, k.GetCustomPrecompilesVersions(ctx)[address])
					if active {
						_, err, db := repair1209Call(t, ctx, &k, selected, caller, input)
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.EqualError(t, db.GetPrecompileError(), "invalid public key length: expected 32 bytes")
					} else {
						require.PanicsWithValue(t, "pubkey is incorrect size", func() {
							repair1209Call(t, ctx, &k, selected, caller, input)
						})
					}
					// The archived snapshot retains the old behavior at every height.
					legacyCtx, _ := base.WithBlockHeight(height).CacheContext()
					require.PanicsWithValue(t, "pubkey is incorrect size", func() {
						repair1209Call(t, legacyCtx, &k, archived, caller, input)
					})
				})
			}
		}
	}
}

func TestRepairM_1209_EVMTransactionRefusal(t *testing.T) {
	for _, size := range []int{0, 31, 33} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			a, ctx, caller, payer := repair1209Fixture(t)
			a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
			k := a.EvmKeeper
			address := common.HexToAddress(staking.StakingAddress)
			versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
			k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
			p := versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
			repair1209Fund(t, a, ctx, payer, 2_000_000_000_000)
			balances := a.BankKeeper.GetAllBalances(ctx, payer)
			precompileAccount := k.GetPaxAddressOrDefault(ctx, address)
			precompileBalances := a.BankKeeper.GetAllBalances(ctx, precompileAccount)
			validators := a.StakingKeeper.GetAllValidators(ctx)
			key, err := crypto.ToECDSA(bytes.Repeat([]byte{0x12}, 32))
			require.NoError(t, err)
			signer := ethtypes.MakeSigner(evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(ctx)), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix()))
			tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
				GasPrice: big.NewInt(1_000_000_000_000), Gas: 2_000_000, To: &address,
				Value: big.NewInt(1_000_000_000_000_000_000),
				Data:  repair1209Input(t, p.GetABI(), hex.EncodeToString(bytes.Repeat([]byte{1}, size))),
			}), signer, key)
			require.NoError(t, err)
			require.Equal(t, caller, crypto.PubkeyToAddress(key.PublicKey))
			wrapped, err := ethtx.NewLegacyTx(tx)
			require.NoError(t, err)
			request, err := evmtypes.NewMsgEVMTransaction(wrapped)
			require.NoError(t, err)
			require.NoError(t, ante.Preprocess(ctx, request, k.ChainID(ctx), false))
			response, err := evmkeeper.NewMsgServerImpl(&k).EVMTransaction(sdk.WrapSDKContext(ctx), request)
			require.NoError(t, err)
			require.Equal(t, vm.ErrExecutionReverted.Error(), response.VmError)
			require.Empty(t, response.ReturnData)
			require.Empty(t, response.Logs)
			require.Equal(t, balances, a.BankKeeper.GetAllBalances(ctx, payer))
			require.Equal(t, precompileBalances, a.BankKeeper.GetAllBalances(ctx, precompileAccount))
			require.Equal(t, validators, a.StakingKeeper.GetAllValidators(ctx))
			_, found := a.StakingKeeper.GetValidator(ctx, sdk.ValAddress(payer))
			require.False(t, found)
		})
	}
}
