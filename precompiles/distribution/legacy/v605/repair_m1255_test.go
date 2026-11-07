package v605_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v605"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v605 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v605"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const (
	repair1255HistoricalHeight int64  = 10
	repair1255CurrentHeight    int64  = 20
	repair1255MergeHeight      int64  = 30
	repair1255Gas              uint64 = 1_000_000
)

type repair1255Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repair1255Reward struct {
	Coins            []repair1255Coin
	ValidatorAddress string
}

type repair1255Rewards struct {
	Rewards []repair1255Reward
	Total   []repair1255Coin
}

type repair1255Env struct {
	app       *node.App
	ctx       sdk.Context
	caller    common.Address
	delegator sdk.AccAddress
	abi       abi.ABI
}

func newRepair1255Env(t *testing.T, applied bool) *repair1255Env {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.NewContext(false, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1255HistoricalHeight), "v6.0.5")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1255CurrentHeight), "v6.4.0")
	if applied {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1255MergeHeight), pcommon.MergeUpgrade)
	}
	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 12, 55
	pub := key.PubKey()
	delegator := sdk.AccAddress(pub.Address())
	ethKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ethKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	denom := a.StakingKeeper.GetParams(ctx).BondDenom
	funds := sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	helper := teststaking.NewHelper(t, ctx, a.StakingKeeper)
	helper.Denom = denom
	helper.CreateValidator(sdk.ValAddress(delegator), pub, sdk.NewInt(100), true)
	validator, found := a.StakingKeeper.GetValidator(ctx, sdk.ValAddress(delegator))
	require.True(t, found)
	a.DistrKeeper.AllocateTokensToValidator(ctx, validator, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin(denom, 1000)))
	p, err := v605.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, "view", p.Methods[v605.RewardsMethod].StateMutability)
	return &repair1255Env{app: a, ctx: ctx, caller: caller, delegator: delegator, abi: p.GetABI()}
}

func (e *repair1255Env) evm(t *testing.T, height int64, historical bool) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	ctx := e.ctx.WithBlockHeight(height).WithTraceMode(historical)
	e.ctx = ctx
	k := &e.app.EvmKeeper
	contracts := k.CustomPrecompiles(ctx)
	address := common.HexToAddress(v605.DistrAddress)
	require.Contains(t, contracts, address)
	if historical {
		require.Equal(t, "v6.0.5", k.GetCustomPrecompilesVersions(ctx)[address])
		p, ok := contracts[address].(*legacycommon.DynamicGasPrecompile)
		require.True(t, ok, "historical dispatch must select the frozen v605 wrapper")
		require.IsType(t, &v605.PrecompileExecutor{}, p.GetExecutor())
	} else {
		p, ok := contracts[address].(*pcommon.DynamicGasPrecompile)
		require.True(t, ok, "live dispatch must select the current wrapper")
		require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
	}
	db := state.NewDBImpl(ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(ctx)), vm.Config{}, contracts)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db
}

func (e *repair1255Env) input(t *testing.T) []byte {
	t.Helper()
	input, err := e.abi.Pack(v605.RewardsMethod, e.caller)
	require.NoError(t, err)
	return input
}

func (e *repair1255Env) assertRewards(t *testing.T, output []byte) {
	t.Helper()
	response, err := e.app.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(e.ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: e.delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.False(t, response.Total.IsZero())
	decoded, err := e.abi.Methods[v605.RewardsMethod].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	actual := abi.ConvertType(decoded[0], new(repair1255Rewards)).(*repair1255Rewards)
	require.Len(t, actual.Rewards, len(response.Rewards))
	assertCoins := func(actual []repair1255Coin, expected sdk.DecCoins) {
		require.Len(t, actual, len(expected))
		for i, coin := range expected {
			require.Equal(t, coin.Denom, actual[i].Denom)
			require.Zero(t, coin.Amount.BigInt().Cmp(actual[i].Amount))
			require.Zero(t, big.NewInt(sdk.Precision).Cmp(actual[i].Decimals))
		}
	}
	for i, reward := range response.Rewards {
		require.Equal(t, reward.ValidatorAddress, actual.Rewards[i].ValidatorAddress)
		assertCoins(actual.Rewards[i].Coins, reward.Reward)
	}
	assertCoins(actual.Total, response.Total)
	encoded, err := e.abi.Methods[v605.RewardsMethod].Outputs.Pack(*actual)
	require.NoError(t, err)
	require.Equal(t, output, encoded)
}

func TestRepairM_1255_HistoricalRawCALL(t *testing.T) {
	for _, height := range []int64{repair1255HistoricalHeight, repair1255HistoricalHeight + 1} {
		for _, amount := range []uint64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height_%d/value_%d", height, amount), func(t *testing.T) {
				e := newRepair1255Env(t, true)
				evm, db := e.evm(t, height, true)
				address := common.HexToAddress(v605.DistrAddress)
				beforeCaller, beforeTarget := db.GetBalance(e.caller).ToBig(), db.GetBalance(address).ToBig()
				value := new(big.Int).SetUint64(amount)
				output, left, err := evm.Call(e.caller, address, e.input(t), repair1255Gas, uint256.NewInt(amount))
				require.NoError(t, err)
				require.Positive(t, left)
				require.Less(t, left, repair1255Gas)
				e.assertRewards(t, output)
				require.NoError(t, db.GetPrecompileError())
				require.Zero(t, new(big.Int).Sub(beforeCaller, value).Cmp(db.GetBalance(e.caller).ToBig()))
				require.Zero(t, new(big.Int).Add(beforeTarget, value).Cmp(db.GetBalance(address).ToBig()))
				_, err = db.Finalize()
				require.NoError(t, err)
				reopened := state.NewDBImpl(db.Ctx(), &e.app.EvmKeeper, true)
				defer reopened.Cleanup()
				require.Zero(t, new(big.Int).Sub(beforeCaller, value).Cmp(reopened.GetBalance(e.caller).ToBig()))
				require.Zero(t, new(big.Int).Add(beforeTarget, value).Cmp(reopened.GetBalance(address).ToBig()))
			})
		}
	}
}

func TestRepairM_1255_LiveRoutingActivation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		height  int64
		applied bool
		refuse  bool
	}{
		{"before_merge", repair1255MergeHeight - 1, true, false},
		{"at_merge", repair1255MergeHeight, true, true},
		{"after_merge", repair1255MergeHeight + 1, true, true},
		{"unapplied_merge", repair1255MergeHeight + 1, false, false},
	} {
		for _, amount := range []uint64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("%s/value_%d", tc.name, amount), func(t *testing.T) {
				e := newRepair1255Env(t, tc.applied)
				evm, db := e.evm(t, tc.height, false)
				address := common.HexToAddress(v605.DistrAddress)
				beforeCaller, beforeTarget := db.GetBalance(e.caller).ToBig(), db.GetBalance(address).ToBig()
				output, _, err := evm.Call(e.caller, address, e.input(t), repair1255Gas, uint256.NewInt(amount))
				if tc.refuse {
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.Empty(t, output)
					require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
					require.Zero(t, beforeCaller.Cmp(db.GetBalance(e.caller).ToBig()))
					require.Zero(t, beforeTarget.Cmp(db.GetBalance(address).ToBig()))
				} else {
					require.NoError(t, err)
					e.assertRewards(t, output)
					value := new(big.Int).SetUint64(amount)
					require.Zero(t, new(big.Int).Sub(beforeCaller, value).Cmp(db.GetBalance(e.caller).ToBig()))
					require.Zero(t, new(big.Int).Add(beforeTarget, value).Cmp(db.GetBalance(address).ToBig()))
				}
			})
		}
	}
}

func TestRepairM_1255_ZeroValueQueries(t *testing.T) {
	for _, historical := range []bool{true, false} {
		for _, static := range []bool{false, true} {
			t.Run(fmt.Sprintf("historical_%t/static_%t", historical, static), func(t *testing.T) {
				e := newRepair1255Env(t, true)
				height := repair1255MergeHeight
				if historical {
					height = repair1255HistoricalHeight
				}
				evm, db := e.evm(t, height, historical)
				address := common.HexToAddress(v605.DistrAddress)
				beforeCaller, beforeTarget := db.GetBalance(e.caller).ToBig(), db.GetBalance(address).ToBig()
				var output []byte
				var err error
				if static {
					output, _, err = evm.StaticCall(e.caller, address, e.input(t), repair1255Gas)
				} else {
					output, _, err = evm.Call(e.caller, address, e.input(t), repair1255Gas, uint256.NewInt(0))
				}
				require.NoError(t, err)
				e.assertRewards(t, output)
				require.Zero(t, beforeCaller.Cmp(db.GetBalance(e.caller).ToBig()))
				require.Zero(t, beforeTarget.Cmp(db.GetBalance(address).ToBig()))
			})
		}
	}
}
