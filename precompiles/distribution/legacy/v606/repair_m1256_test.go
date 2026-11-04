package v606_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v606"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v606 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v606"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	distrkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const repairM1256HistoricalHeight int64 = 151573570

// The merge height is a committed store fixture, not a claimed network height.
const repairM1256MergeHeight = repairM1256HistoricalHeight + 10

// Tuple decoding follows ABI wire order, including nested tuple fields.
type repairM1256Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repairM1256Reward struct {
	Coins            []repairM1256Coin
	ValidatorAddress string
}

type repairM1256Rewards struct {
	Rewards []repairM1256Reward
	Total   []repairM1256Coin
}

type repairM1256Env struct {
	app       *repairM1256App
	ctx       sdk.Context
	caller    common.Address
	delegator sdk.AccAddress
	expected  repairM1256Rewards
}

// This assembly supplies real keepers to the production version registry.
// Unused interfaces are deliberately unbound; no test double handles a call.
type repairM1256App struct {
	utils.Keepers
	EvmKeeper     evmkeeper.Keeper
	UpgradeKeeper upgradekeeper.Keeper
	StakingKeeper stakingkeeper.Keeper
	BankKeeper    bankkeeper.BaseKeeper
	DistrKeeper   distrkeeper.Keeper
}

func (a *repairM1256App) EVMK() utils.EVMKeeper                   { return &a.EvmKeeper }
func (a *repairM1256App) DistributionK() utils.DistributionKeeper { return a.DistrKeeper }

func newRepairM1256App(t *testing.T) (*repairM1256App, sdk.Context) {
	t.Helper()
	unit, registered := sdk.GetDenomUnit("uhpx")
	require.True(t, registered, "production initialization must register uhpx")
	require.True(t, unit.Equal(sdk.NewDecWithPrec(1, 6)), "uhpx must retain its six-decimal unit")
	require.Equal(t, "uhpx", sdk.MustGetBaseDenom())
	root := store.NewCommitMultiStore(dbm.NewMemDB())
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, evmtypes.StoreKey, upgradetypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		root.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKeys(paramtypes.TStoreKey, evmtypes.TransientStoreKey)
	for _, key := range transient {
		root.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
	}
	require.NoError(t, root.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	ctx := sdk.NewContext(root.CacheMultiStore(), tmtypes.Header{ChainID: "repair-m1256", Height: 2, Time: time.Unix(1, 0).UTC()}, false)
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, keys[paramtypes.StoreKey], transient[paramtypes.TStoreKey], name)
	}
	permissions := map[string][]string{
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	}
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	ak.SetParams(ctx, authtypes.DefaultParams())
	for name, perms := range permissions {
		ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, perms...))
	}
	a := &repairM1256App{}
	a.BankKeeper = bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, subspace(banktypes.ModuleName), map[string]bool{})
	a.BankKeeper.InitGenesis(ctx, banktypes.DefaultGenesisState())
	a.StakingKeeper = stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, a.BankKeeper, subspace(stakingtypes.ModuleName))
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = "uhpx"
	a.StakingKeeper.SetParams(ctx, stakingParams)
	a.DistrKeeper = distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace(distrtypes.ModuleName), ak, a.BankKeeper, &a.StakingKeeper, authtypes.FeeCollectorName, map[string]bool{})
	a.DistrKeeper.SetParams(ctx, distrtypes.DefaultParams())
	a.DistrKeeper.SetFeePool(ctx, distrtypes.InitialFeePool())
	a.StakingKeeper.SetHooks(a.DistrKeeper.Hooks())
	a.UpgradeKeeper = upgradekeeper.NewKeeper(map[int64]bool{}, keys[upgradetypes.StoreKey], cdc, t.TempDir(), nil)
	a.EvmKeeper = *evmkeeper.NewKeeper(keys[evmtypes.StoreKey], transient[evmtypes.TransientStoreKey], subspace(evmtypes.ModuleName), nil, a.BankKeeper, &ak, &a.StakingKeeper, transferkeeper.Keeper{}, nil, nil, &a.UpgradeKeeper)
	a.EvmKeeper.SetParams(ctx, evmtypes.DefaultParams())
	a.EvmKeeper.SetAddressMapping(ctx, ak.GetModuleAddress(authtypes.FeeCollectorName), evmkeeper.GetCoinbaseAddress())
	address := common.HexToAddress(v606.DistrAddress)
	a.EvmKeeper.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: distribution.GetVersioned(pcommon.MergeUpgrade, a)}, pcommon.MergeUpgrade)
	return a, ctx
}

func newRepairM1256Env(t *testing.T, height int64, tracing, mergeApplied bool) *repairM1256Env {
	t.Helper()
	a, ctx := newRepairM1256App(t)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1256HistoricalHeight), "v6.0.6")
	if mergeApplied {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1256MergeHeight), pcommon.MergeUpgrade)
	}

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 12, 56
	delegator := sdk.AccAddress(key.PubKey().Address())
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.StakingKeeper.GetParams(ctx).BondDenom, 100_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	commission := stakingtypes.NewCommissionRates(a.StakingKeeper.GetParams(ctx).MinCommissionRate, sdk.OneDec(), sdk.OneDec())
	message, err := stakingtypes.NewMsgCreateValidator(sdk.ValAddress(delegator), key.PubKey(), sdk.NewInt64Coin(funds[0].Denom, 100), stakingtypes.Description{}, commission, sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(a.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), message)
	require.NoError(t, err)
	validator, found := a.StakingKeeper.GetValidator(ctx, sdk.ValAddress(delegator))
	require.True(t, found)
	a.DistrKeeper.AllocateTokensToValidator(ctx, validator, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin("uhpx", 1000)))

	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing).WithClosestUpgradeName("v6.0.6")
	response, err := a.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	expected := repairM1256Rewards{Rewards: make([]repairM1256Reward, 0), Total: make([]repairM1256Coin, 0)}
	for _, reward := range response.Rewards {
		row := repairM1256Reward{ValidatorAddress: reward.ValidatorAddress, Coins: make([]repairM1256Coin, 0)}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, repairM1256Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
		}
		expected.Rewards = append(expected.Rewards, row)
	}
	for _, coin := range response.Total {
		expected.Total = append(expected.Total, repairM1256Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
	}
	return &repairM1256Env{app: a, ctx: ctx, caller: caller, delegator: delegator, expected: expected}
}

func (e *repairM1256Env) evm(t *testing.T, historical bool) (*vm.EVM, *state.DBImpl, abi.ABI) {
	t.Helper()
	k := &e.app.EvmKeeper
	address := common.HexToAddress(v606.DistrAddress)
	routes := k.CustomPrecompiles(e.ctx)
	route, found := routes[address]
	require.True(t, found)
	var contractABI abi.ABI
	if historical {
		require.Equal(t, "v6.0.6", k.GetCustomPrecompilesVersions(e.ctx)[address])
		legacy, ok := route.(*legacycommon.DynamicGasPrecompile)
		require.True(t, ok, "historical route must select the v606 wrapper, got %T", route)
		require.IsType(t, &v606.PrecompileExecutor{}, legacy.GetExecutor())
		contractABI = legacy.GetABI()
	} else {
		current, ok := route.(*pcommon.DynamicGasPrecompile)
		require.True(t, ok, "live route must select the current wrapper, got %T", route)
		require.IsType(t, &distribution.PrecompileExecutor{}, current.GetExecutor())
		contractABI = current.GetABI()
	}
	require.Equal(t, "view", contractABI.Methods[v606.RewardsMethod].StateMutability)
	db := state.NewDBImpl(e.ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(e.ctx)), vm.Config{}, routes)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db, contractABI
}

func (e *repairM1256Env) assertRewards(t *testing.T, contractABI abi.ABI, output []byte) {
	t.Helper()
	decoded, err := contractABI.Methods[v606.RewardsMethod].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	actual := abi.ConvertType(decoded[0], new(repairM1256Rewards)).(*repairM1256Rewards)
	require.Equal(t, e.expected, *actual)
	response, err := e.app.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(e.ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: e.delegator.String()})
	require.NoError(t, err)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive(), "query must leave rewards unwithdrawn")
}

func (e *repairM1256Env) call(t *testing.T, historical, reject bool, value int64) {
	t.Helper()
	evm, db, contractABI := e.evm(t, historical)
	input, err := contractABI.Pack(v606.RewardsMethod, e.caller)
	require.NoError(t, err)
	address := common.HexToAddress(v606.DistrAddress)
	callerBefore := db.GetBalance(e.caller).ToBig()
	precompileBefore := db.GetBalance(address).ToBig()
	output, gas, err := evm.Call(e.caller, address, input, 1_000_000, uint256.NewInt(uint64(value)))
	callerExpected := new(big.Int).Set(callerBefore)
	precompileExpected := new(big.Int).Set(precompileBefore)
	if reject {
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
		require.Empty(t, output)
	} else {
		require.NoError(t, err)
		require.NoError(t, db.GetPrecompileError())
		require.Positive(t, gas)
		require.Less(t, gas, uint64(1_000_000))
		e.assertRewards(t, contractABI, output)
		callerExpected.Sub(callerExpected, big.NewInt(value))
		precompileExpected.Add(precompileExpected, big.NewInt(value))
	}
	require.Equal(t, uint256.MustFromBig(callerExpected).Bytes32(), db.GetBalance(e.caller).Bytes32())
	require.Equal(t, uint256.MustFromBig(precompileExpected).Bytes32(), db.GetBalance(address).Bytes32())
	_, err = db.Finalize()
	require.NoError(t, err)
	reopened := state.NewDBImpl(e.ctx, &e.app.EvmKeeper, true)
	t.Cleanup(reopened.Cleanup)
	require.Equal(t, uint256.MustFromBig(callerExpected).Bytes32(), reopened.GetBalance(e.caller).Bytes32())
	require.Equal(t, uint256.MustFromBig(precompileExpected).Bytes32(), reopened.GetBalance(address).Bytes32())
}

func TestRepairM_1256_HistoricalRawCALL(t *testing.T) {
	for _, height := range []int64{repairM1256HistoricalHeight, repairM1256HistoricalHeight + 1, repairM1256MergeHeight - 1} {
		for _, applied := range []bool{false, true} {
			for _, value := range []int64{1, 1_000_000_000_000, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("height=%d/merge=%t/value=%d", height, applied, value), func(t *testing.T) {
					e := newRepairM1256Env(t, height, true, applied)
					require.False(t, pcommon.MergePlanActive(e.ctx, &e.app.EvmKeeper))
					e.call(t, true, false, value)
				})
			}
		}
	}
}

func TestRepairM_1256_LiveRoutingActivation(t *testing.T) {
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{repairM1256MergeHeight, repairM1256MergeHeight + 1} {
			for _, value := range []int64{1, 1_000_000_000_000, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("tracing=%t/height=%d/value=%d", tracing, height, value), func(t *testing.T) {
					e := newRepairM1256Env(t, height, tracing, true)
					require.True(t, pcommon.MergePlanActive(e.ctx, &e.app.EvmKeeper))
					require.Equal(t, pcommon.MergeUpgrade, e.app.EvmKeeper.GetCustomPrecompilesVersions(e.ctx)[common.HexToAddress(v606.DistrAddress)])
					e.call(t, false, true, value)
				})
			}
		}
	}
	for _, applied := range []bool{false, true} {
		t.Run(fmt.Sprintf("inactive-merge=%t", applied), func(t *testing.T) {
			height := repairM1256MergeHeight - 1
			if !applied {
				height = repairM1256MergeHeight + 1
			}
			e := newRepairM1256Env(t, height, false, applied)
			require.False(t, pcommon.MergePlanActive(e.ctx, &e.app.EvmKeeper))
			e.call(t, false, false, 1_000_000_000_001)
		})
	}
}

func TestRepairM_1256_ZeroValueQueries(t *testing.T) {
	for _, historical := range []bool{false, true} {
		for _, static := range []bool{false, true} {
			t.Run(fmt.Sprintf("historical=%t/static=%t", historical, static), func(t *testing.T) {
				height := repairM1256MergeHeight
				if historical {
					height = repairM1256HistoricalHeight
				}
				e := newRepairM1256Env(t, height, historical, true)
				if !static {
					e.call(t, historical, false, 0)
					return
				}
				evm, db, contractABI := e.evm(t, historical)
				input, err := contractABI.Pack(v606.RewardsMethod, e.caller)
				require.NoError(t, err)
				address := common.HexToAddress(v606.DistrAddress)
				callerBefore := db.GetBalance(e.caller).ToBig()
				precompileBefore := db.GetBalance(address).ToBig()
				output, _, err := evm.StaticCall(e.caller, address, input, 1_000_000)
				require.NoError(t, err)
				e.assertRewards(t, contractABI, output)
				require.Equal(t, callerBefore, db.GetBalance(e.caller).ToBig())
				require.Equal(t, precompileBefore, db.GetBalance(address).ToBig())
			})
		}
	}
}
