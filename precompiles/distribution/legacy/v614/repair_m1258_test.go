package v614_test

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	historicalcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v614"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v614 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v614"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
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

// These are deterministic store fixture heights, not network activation heights.
const (
	repairM1258HistoricalHeight int64 = 10
	repairM1258MergeHeight      int64 = 20
)

//go:embed distribution.go abi.json
var repairM1258Frozen embed.FS

// Field order follows the wire ABI, including nested tuples.
type repairM1258Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repairM1258Reward struct {
	Coins            []repairM1258Coin
	ValidatorAddress string
}

type repairM1258Rewards struct {
	Rewards []repairM1258Reward
	Total   []repairM1258Coin
}

type repairM1258Env struct {
	keeper   *keeper.Keeper
	ctx      sdk.Context
	caller   common.Address
	address  common.Address
	versions utils.VersionedPrecompiles
	abi      abi.ABI
	input    []byte
	expected repairM1258Rewards
}

func newRepairM1258Env(t *testing.T, height int64, tracing, applied bool) *repairM1258Env {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.NewContext(false, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(5), "v6.1.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1258HistoricalHeight), "v6.1.4")
	if applied {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1258MergeHeight), pcommon.MergeUpgrade)
	}

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 4, 234
	pub := key.PubKey()
	validator := sdk.ValAddress(pub.Address())
	delegator := sdk.AccAddress(validator)
	bond := sdk.NewCoins(sdk.NewInt64Coin(a.StakingKeeper.GetParams(ctx).BondDenom, 100))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, bond))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, delegator, bond))
	helper := teststaking.NewHelper(t, ctx, a.StakingKeeper)
	helper.Handle(helper.CreateValidatorMsg(validator, pub, bond[0].Amount), true)
	val, found := a.StakingKeeper.GetValidator(ctx, validator)
	require.True(t, found)
	a.DistrKeeper.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin("uhpx", 1000)))
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 100_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))

	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)
	// Copy the real keeper's routing fields so this fixture never replaces the
	// shared application's registry. The keepers still use the real SDK stores.
	k := a.EvmKeeper
	versions := distribution.GetVersioned(node.LatestUpgrade, a.GetPrecompileKeepers())
	address := common.HexToAddress(v614.DistrAddress)
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, node.LatestUpgrade)
	historical, ok := versions["v6.1.4"].(*historicalcommon.DynamicGasPrecompile)
	require.True(t, ok)
	require.IsType(t, &v614.PrecompileExecutor{}, historical.GetExecutor())
	require.Equal(t, "view", historical.Methods[v614.RewardsMethod].StateMutability)
	input, err := historical.Pack(v614.RewardsMethod, caller)
	require.NoError(t, err)
	response, err := a.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	expected := repairM1258Rewards{Rewards: make([]repairM1258Reward, 0), Total: make([]repairM1258Coin, 0)}
	for _, reward := range response.Rewards {
		row := repairM1258Reward{ValidatorAddress: reward.ValidatorAddress, Coins: make([]repairM1258Coin, 0)}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, repairM1258Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
		}
		expected.Rewards = append(expected.Rewards, row)
	}
	for _, coin := range response.Total {
		expected.Total = append(expected.Total, repairM1258Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
	}
	return &repairM1258Env{keeper: &k, ctx: ctx, caller: caller, address: address, versions: versions, abi: historical.ABI, input: input, expected: expected}
}

func (e *repairM1258Env) evm(t *testing.T) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	db := state.NewDBImpl(e.ctx, e.keeper, false)
	t.Cleanup(db.Cleanup)
	block, err := e.keeper.GetVMBlockContext(db.Ctx(), e.keeper.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(e.keeper.ChainID(e.ctx)), vm.Config{}, e.keeper.CustomPrecompiles(e.ctx))
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db
}

func (e *repairM1258Env) assertRewards(t *testing.T, output []byte) {
	t.Helper()
	decoded, err := e.abi.Methods[v614.RewardsMethod].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	actual := abi.ConvertType(decoded[0], new(repairM1258Rewards)).(*repairM1258Rewards)
	require.Equal(t, e.expected, *actual)
}

func (e *repairM1258Env) assertStoredBalances(t *testing.T, db *state.DBImpl, caller, precompile *big.Int) {
	t.Helper()
	require.Equal(t, caller, db.GetBalance(e.caller).ToBig())
	require.Equal(t, precompile, db.GetBalance(e.address).ToBig())
	_, err := db.Finalize()
	require.NoError(t, err)
	reopened := state.NewDBImpl(e.ctx, e.keeper, true)
	t.Cleanup(reopened.Cleanup)
	require.Equal(t, caller, reopened.GetBalance(e.caller).ToBig())
	require.Equal(t, precompile, reopened.GetBalance(e.address).ToBig())
}

func TestRepairM_1258_HistoricalBytes(t *testing.T) {
	for path, digest := range map[string]string{
		"distribution.go": "0103c76e87ef82ac28a4b1171ebdd0f3a5dd99631baeb50176e13320393b0620",
		"abi.json":        "e4e32d4555e10a681284546e630dda6f1b7a006e516987a40d02802e65e2d869",
	} {
		data, err := repairM1258Frozen.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, digest, fmt.Sprintf("%x", sha256.Sum256(data)), "historical replay bytes: %s", path)
	}
}

func TestRepairM_1258_HistoricalRawCALL(t *testing.T) {
	for _, height := range []int64{repairM1258HistoricalHeight, repairM1258HistoricalHeight + 1, repairM1258MergeHeight - 1} {
		for _, value := range []uint64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height=%d/value=%d", height, value), func(t *testing.T) {
				e := newRepairM1258Env(t, height, true, true)
				require.Equal(t, "v6.1.4", e.keeper.GetCustomPrecompilesVersions(e.ctx)[e.address])
				require.Same(t, e.versions["v6.1.4"], e.keeper.CustomPrecompiles(e.ctx)[e.address])
				require.Same(t, e.versions["v6.1.0"], e.keeper.CustomPrecompiles(e.ctx.WithBlockHeight(repairM1258HistoricalHeight - 1))[e.address])
				evm, db := e.evm(t)
				callerBefore, precompileBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
				output, _, err := evm.Call(e.caller, e.address, e.input, 1_000_000, uint256.NewInt(value))
				require.NoError(t, err)
				e.assertRewards(t, output)
				payment := new(big.Int).SetUint64(value)
				e.assertStoredBalances(t, db, new(big.Int).Sub(callerBefore, payment), new(big.Int).Add(precompileBefore, payment))
			})
		}
	}
}

func TestRepairM_1258_LiveRoutingRejectsValue(t *testing.T) {
	for _, height := range []int64{repairM1258MergeHeight, repairM1258MergeHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, value := range []uint64{1, 1_000_000_000_000, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("height=%d/tracing=%t/value=%d", height, tracing, value), func(t *testing.T) {
					e := newRepairM1258Env(t, height, tracing, true)
					selected := e.keeper.CustomPrecompiles(e.ctx)[e.address]
					require.Same(t, e.versions[node.LatestUpgrade], selected)
					current, ok := selected.(*pcommon.DynamicGasPrecompile)
					require.True(t, ok)
					require.IsType(t, &distribution.PrecompileExecutor{}, current.GetExecutor())
					evm, db := e.evm(t)
					callerBefore, precompileBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
					_, _, err := evm.Call(e.caller, e.address, e.input, 1_000_000, uint256.NewInt(value))
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
					e.assertStoredBalances(t, db, callerBefore, precompileBefore)
				})
			}
		}
	}
}

func TestRepairM_1258_ZeroValueRewards(t *testing.T) {
	for _, height := range []int64{repairM1258HistoricalHeight, repairM1258MergeHeight - 1, repairM1258MergeHeight, repairM1258MergeHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, static := range []bool{false, true} {
				t.Run(fmt.Sprintf("height=%d/tracing=%t/static=%t", height, tracing, static), func(t *testing.T) {
					e := newRepairM1258Env(t, height, tracing, true)
					evm, db := e.evm(t)
					callerBefore, precompileBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
					var output []byte
					var err error
					if static {
						output, _, err = evm.StaticCall(e.caller, e.address, e.input, 1_000_000)
					} else {
						output, _, err = evm.Call(e.caller, e.address, e.input, 1_000_000, uint256.NewInt(0))
					}
					require.NoError(t, err)
					e.assertRewards(t, output)
					require.Equal(t, callerBefore, db.GetBalance(e.caller).ToBig())
					require.Equal(t, precompileBefore, db.GetBalance(e.address).ToBig())
				})
			}
		}
	}
}

func TestRepairM_1258_UnappliedActivation(t *testing.T) {
	e := newRepairM1258Env(t, repairM1258MergeHeight+1, false, false)
	require.False(t, pcommon.MergePlanActive(e.ctx, e.keeper))
	require.Same(t, e.versions[node.LatestUpgrade], e.keeper.CustomPrecompiles(e.ctx)[e.address])
	evm, db := e.evm(t)
	callerBefore, precompileBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
	value := uint256.NewInt(1_000_000_000_001)
	output, _, err := evm.Call(e.caller, e.address, e.input, 1_000_000, value)
	require.NoError(t, err)
	e.assertRewards(t, output)
	e.assertStoredBalances(t, db, new(big.Int).Sub(callerBefore, value.ToBig()), new(big.Int).Add(precompileBefore, value.ToBig()))
}
