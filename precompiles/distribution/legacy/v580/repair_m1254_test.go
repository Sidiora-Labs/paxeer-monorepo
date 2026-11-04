package v580_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	historicalcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v580"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v580 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v580"
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

// These are recorded upgrade heights in the isolated store, not mainnet heights.
const (
	repair1254HistoricalHeight int64 = 5
	repair1254MergeHeight      int64 = 10
)

// Decode in ABI wire order; the production packing structs use field names.
type repair1254Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repair1254Reward struct {
	Coins            []repair1254Coin
	ValidatorAddress string
}

type repair1254Rewards struct {
	Rewards []repair1254Reward
	Total   []repair1254Coin
}

type repair1254Env struct {
	app      *node.App
	ctx      sdk.Context
	caller   common.Address
	expected repair1254Rewards
}

func newRepair1254Env(t *testing.T, height int64, tracing, mergeApplied bool) *repair1254Env {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.NewContext(false, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1254HistoricalHeight), "v5.8.0")
	if mergeApplied {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1254MergeHeight), pcommon.MergeUpgrade)
	}

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[31] = 125
	validatorAddress := sdk.ValAddress(key.PubKey().Address())
	delegator := sdk.AccAddress(validatorAddress)
	bond := sdk.NewCoins(sdk.NewInt64Coin(a.StakingKeeper.GetParams(ctx).BondDenom, 100))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, bond))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, delegator, bond))
	staking := teststaking.NewHelper(t, ctx, a.StakingKeeper)
	staking.Handle(staking.CreateValidatorMsg(validatorAddress, key.PubKey(), bond[0].Amount), true)
	validator, found := a.StakingKeeper.GetValidator(ctx, validatorAddress)
	require.True(t, found)
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	backing := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1000))
	a.DistrKeeper.AllocateTokensToValidator(ctx, validator, sdk.NewDecCoinsFromCoins(backing...))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, backing))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, backing))
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 100_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)
	require.Equal(t, mergeApplied && height >= repair1254MergeHeight, pcommon.MergePlanActive(ctx, &a.EvmKeeper))

	response, err := a.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	expected := repair1254Rewards{Rewards: make([]repair1254Reward, 0), Total: make([]repair1254Coin, 0)}
	for _, reward := range response.Rewards {
		row := repair1254Reward{ValidatorAddress: reward.ValidatorAddress, Coins: make([]repair1254Coin, 0)}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, repair1254Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
		}
		expected.Rewards = append(expected.Rewards, row)
	}
	for _, coin := range response.Total {
		expected.Total = append(expected.Total, repair1254Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
	}
	return &repair1254Env{app: a, ctx: ctx, caller: caller, expected: expected}
}

func (e *repair1254Env) call(t *testing.T, value int64, static, historical, rejected bool) {
	t.Helper()
	k := &e.app.EvmKeeper
	address := common.HexToAddress(v580.DistrAddress)
	// Use the application's registered version table and the keeper's real
	// height/tracing selector, rather than installing the executor under test.
	routes := k.CustomPrecompiles(e.ctx)
	route, found := routes[address]
	require.True(t, found)
	var servedABI abi.ABI
	if historical {
		p, ok := route.(*historicalcommon.DynamicGasPrecompile)
		require.True(t, ok, "historical route is %T", route)
		require.IsType(t, &v580.PrecompileExecutor{}, p.GetExecutor())
		require.Equal(t, "v5.8.0", k.GetCustomPrecompilesVersions(e.ctx)[address])
		servedABI = p.GetABI()
	} else {
		p, ok := route.(*pcommon.DynamicGasPrecompile)
		require.True(t, ok, "live route is %T", route)
		require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
		servedABI = p.GetABI()
	}
	require.Equal(t, "view", servedABI.Methods[v580.RewardsMethod].StateMutability)
	input, err := servedABI.Pack(v580.RewardsMethod, e.caller)
	require.NoError(t, err)
	db := state.NewDBImpl(e.ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(e.ctx)), vm.Config{}, routes)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	callerBefore := db.GetBalance(e.caller).ToBig()
	precompileBefore := db.GetBalance(address).ToBig()
	var output []byte
	if static {
		require.Zero(t, value)
		output, _, err = evm.StaticCall(e.caller, address, input, 1_000_000)
	} else {
		output, _, err = evm.Call(e.caller, address, input, 1_000_000, uint256.NewInt(uint64(value)))
	}
	paid := big.NewInt(value)
	if rejected {
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
		require.Empty(t, output)
		paid = big.NewInt(0)
	} else {
		require.NoError(t, err)
		require.NoError(t, db.GetPrecompileError())
		decoded, err := servedABI.Methods[v580.RewardsMethod].Outputs.Unpack(output)
		require.NoError(t, err)
		require.Len(t, decoded, 1)
		require.Equal(t, e.expected, *abi.ConvertType(decoded[0], new(repair1254Rewards)).(*repair1254Rewards))
	}
	// Compare numeric balances; big.Int can represent zero with nil or empty limbs.
	callerAfter := new(big.Int).Sub(callerBefore, paid)
	precompileAfter := new(big.Int).Add(precompileBefore, paid)
	require.Zero(t, callerAfter.Cmp(db.GetBalance(e.caller).ToBig()), "balance must equal %s", callerAfter)
	require.Zero(t, precompileAfter.Cmp(db.GetBalance(address).ToBig()), "balance must equal %s", precompileAfter)
	_, err = db.Finalize()
	require.NoError(t, err)
	// Reopen the state adapter to prove the accepted payment (or rollback)
	// reached the SDK store, including the sub-unit wei remainder.
	reopened := state.NewDBImpl(e.ctx, k, true)
	t.Cleanup(reopened.Cleanup)
	require.Zero(t, callerAfter.Cmp(reopened.GetBalance(e.caller).ToBig()), "balance must equal %s", callerAfter)
	require.Zero(t, precompileAfter.Cmp(reopened.GetBalance(address).ToBig()), "balance must equal %s", precompileAfter)
}

func TestRepairM_1254_HistoricalRawCALLPayment(t *testing.T) {
	for _, height := range []int64{repair1254HistoricalHeight, repair1254MergeHeight - 1} {
		for _, value := range []int64{1, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height_%d/value_%d", height, value), func(t *testing.T) {
				newRepair1254Env(t, height, true, true).call(t, value, false, true, false)
			})
		}
	}
}

func TestRepairM_1254_LiveRoutingRejectsPayment(t *testing.T) {
	for _, height := range []int64{repair1254MergeHeight, repair1254MergeHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, value := range []int64{1, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("height_%d/tracing_%t/value_%d", height, tracing, value), func(t *testing.T) {
					newRepair1254Env(t, height, tracing, true).call(t, value, false, false, true)
				})
			}
		}
	}
}

func TestRepairM_1254_ZeroValueRewards(t *testing.T) {
	for _, height := range []int64{repair1254HistoricalHeight, repair1254MergeHeight - 1, repair1254MergeHeight, repair1254MergeHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, static := range []bool{false, true} {
				t.Run(fmt.Sprintf("height_%d/tracing_%t/static_%t", height, tracing, static), func(t *testing.T) {
					newRepair1254Env(t, height, tracing, true).call(t, 0, static, tracing && height < repair1254MergeHeight, false)
				})
			}
		}
	}
}

func TestRepairM_1254_PreActivationCompatibility(t *testing.T) {
	for _, applied := range []bool{false, true} {
		height := repair1254MergeHeight + 1
		if applied {
			height = repair1254MergeHeight - 1
		}
		t.Run(fmt.Sprintf("merge_applied_%t", applied), func(t *testing.T) {
			newRepair1254Env(t, height, false, applied).call(t, 1_000_000_000_001, false, false, false)
		})
	}
}
