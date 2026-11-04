package v610_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v606"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v610 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v610"
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

// Heights are an in-memory upgrade history, not claims about mainnet heights.
const (
	repair1257HistoricalHeight int64 = 5
	repair1257MergeHeight      int64 = 10
)

type repair1257Env struct {
	ctx      sdk.Context
	keeper   *keeper.Keeper
	caller   common.Address
	address  common.Address
	method   abi.Method
	input    []byte
	expected []byte
}

// ABI decoding follows tuple order; the historical Go structs use a
// different field order even though packing resolves their fields by name.
type repair1257Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repair1257Reward struct {
	Coins            []repair1257Coin
	ValidatorAddress string
}

type repair1257Rewards struct {
	Rewards []repair1257Reward
	Total   []repair1257Coin
}

// This adapter supplies the two real keepers used by GetVersioned. It does
// not import the full node or replace any keeper operation with a test double.
type repair1257Keepers struct {
	utils.Keepers
	auth         authkeeper.AccountKeeper
	bank         bankkeeper.BaseKeeper
	staking      stakingkeeper.Keeper
	distribution distrkeeper.Keeper
	upgrades     upgradekeeper.Keeper
	evm          *keeper.Keeper
}

func (a *repair1257Keepers) DistributionK() utils.DistributionKeeper { return a.distribution }
func (a *repair1257Keepers) EVMK() utils.EVMKeeper                   { return a.evm }

func newRepair1257Store(t *testing.T) (*repair1257Keepers, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, upgradetypes.StoreKey, evmtypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKeys(paramtypes.TStoreKey, evmtypes.TransientStoreKey)
	for _, key := range transient {
		cms.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
	}
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}, false)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc, amino := codec.NewProtoCodec(registry), codec.NewLegacyAmino()
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
	a := &repair1257Keepers{}
	a.auth = authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	a.auth.SetParams(ctx, authtypes.DefaultParams())
	for name, perms := range permissions {
		a.auth.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, perms...))
	}
	a.bank = bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], a.auth, subspace(banktypes.ModuleName), map[string]bool{})
	a.bank.SetParams(ctx, banktypes.DefaultParams())
	a.staking = stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], a.auth, a.bank, subspace(stakingtypes.ModuleName))
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = "uhpx"
	a.staking.SetParams(ctx, stakingParams)
	a.distribution = distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace(distrtypes.ModuleName), a.auth, a.bank, a.staking, authtypes.FeeCollectorName, map[string]bool{})
	a.distribution.InitGenesis(ctx, *distrtypes.DefaultGenesisState())
	a.staking.SetHooks(a.distribution.Hooks())
	a.upgrades = upgradekeeper.NewKeeper(nil, keys[upgradetypes.StoreKey], cdc, "", nil)
	a.evm = keeper.NewKeeper(keys[evmtypes.StoreKey], transient[evmtypes.TransientStoreKey], subspace(evmtypes.ModuleName), nil, a.bank, &a.auth, &a.staking, transferkeeper.Keeper{}, nil, nil, &a.upgrades)
	a.evm.SetParams(ctx, evmtypes.DefaultParams())
	a.evm.SetAddressMapping(ctx, a.auth.GetModuleAddress(authtypes.FeeCollectorName), keeper.GetCoinbaseAddress())
	return a, ctx
}

func newRepair1257Env(t *testing.T, height int64, tracing, mergeApplied bool) *repair1257Env {
	t.Helper()
	a, ctx := newRepair1257Store(t)
	require.Zero(t, a.upgrades.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	a.upgrades.SetDone(ctx.WithBlockHeight(repair1257HistoricalHeight), "v6.1.0")
	if mergeApplied {
		a.upgrades.SetDone(ctx.WithBlockHeight(repair1257MergeHeight), pcommon.MergeUpgrade)
	}

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 12, 57
	delegator := sdk.AccAddress(key.PubKey().Address())
	validator := sdk.ValAddress(delegator)
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.evm.SetAddressMapping(ctx, delegator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.staking.GetParams(ctx).BondDenom, 100_000_000))
	require.NoError(t, a.bank.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.bank.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	commission := stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.OneDec(), sdk.NewDecWithPrec(1, 1))
	msg, err := stakingtypes.NewMsgCreateValidator(validator, key.PubKey(), sdk.NewInt64Coin("uhpx", 100), stakingtypes.Description{}, commission, sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(a.staking).CreateValidator(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	val, found := a.staking.GetValidator(ctx, validator)
	require.True(t, found)
	a.distribution.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin("uhpx", 1000)))
	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)

	k := a.evm
	address := common.HexToAddress(v610.DistrAddress)
	versions := distribution.GetVersioned(pcommon.MergeUpgrade, a)
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	historical, ok := versions["v6.1.0"].(*legacycommon.DynamicGasPrecompile)
	require.True(t, ok)
	require.IsType(t, &v610.PrecompileExecutor{}, historical.GetExecutor())
	method := historical.GetABI().Methods[v610.RewardsMethod]
	require.Equal(t, "view", method.StateMutability)
	require.Equal(t, "0700037d", common.Bytes2Hex(method.ID))
	input, err := historical.Pack(v610.RewardsMethod, caller)
	require.NoError(t, err)

	response, err := a.distribution.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	coins := func(input sdk.DecCoins) []v610.Coin {
		out := make([]v610.Coin, 0, len(input))
		for _, coin := range input {
			out = append(out, v610.Coin{Amount: coin.Amount.BigInt(), Denom: coin.Denom, Decimals: big.NewInt(sdk.Precision)})
		}
		return out
	}
	expected := v610.Rewards{Rewards: make([]v610.Reward, 0, len(response.Rewards)), Total: coins(response.Total)}
	for _, reward := range response.Rewards {
		expected.Rewards = append(expected.Rewards, v610.Reward{ValidatorAddress: reward.ValidatorAddress, Coins: coins(reward.Reward)})
	}
	packed, err := method.Outputs.Pack(expected)
	require.NoError(t, err)
	return &repair1257Env{ctx: ctx, keeper: k, caller: caller, address: address, method: method, input: input, expected: packed}
}

func (e *repair1257Env) evm(t *testing.T, historical bool) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	routes := e.keeper.CustomPrecompiles(e.ctx)
	require.Contains(t, routes, e.address)
	if historical {
		p, ok := routes[e.address].(*legacycommon.DynamicGasPrecompile)
		require.True(t, ok, "historical routing must serve the v610 wrapper")
		require.IsType(t, &v610.PrecompileExecutor{}, p.GetExecutor())
		require.Equal(t, "v6.1.0", e.keeper.GetCustomPrecompilesVersions(e.ctx)[e.address])
	} else {
		p, ok := routes[e.address].(*pcommon.DynamicGasPrecompile)
		require.True(t, ok, "live routing must serve the repaired wrapper")
		require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
	}
	db := state.NewDBImpl(e.ctx, e.keeper, false)
	t.Cleanup(db.Cleanup)
	block, err := e.keeper.GetVMBlockContext(db.Ctx(), e.keeper.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(e.keeper.ChainID(e.ctx)), vm.Config{}, routes)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db
}

func (e *repair1257Env) assertRewards(t *testing.T, output []byte) {
	t.Helper()
	require.Equal(t, e.expected, output, "ABI response must contain the real keeper's validator rewards and totals")
	decoded, err := e.method.Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	actual := abi.ConvertType(decoded[0], new(repair1257Rewards)).(*repair1257Rewards)
	require.NotEmpty(t, actual.Rewards)
	require.NotEmpty(t, actual.Total)
	require.Positive(t, actual.Total[0].Amount.Sign())
}

// historical-rewards-value-v610: a raw CALL really transfers the payment;
// marking rewards as view in the ABI never enforced nonpayability here.
func TestRepairM_1257_HistoricalRawCALL(t *testing.T) {
	for _, height := range []int64{repair1257HistoricalHeight, repair1257MergeHeight - 1} {
		for _, value := range []int64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height_%d/value_%d", height, value), func(t *testing.T) {
				e := newRepair1257Env(t, height, true, true)
				require.False(t, pcommon.MergePlanActive(e.ctx, e.keeper))
				evm, db := e.evm(t, true)
				callerBefore, targetBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
				payment := big.NewInt(value)
				output, remaining, err := evm.Call(e.caller, e.address, e.input, 1_000_000, uint256.MustFromBig(payment))
				require.NoError(t, err)
				require.Positive(t, remaining)
				e.assertRewards(t, output)
				callerAfter := new(big.Int).Sub(callerBefore, payment)
				targetAfter := new(big.Int).Add(targetBefore, payment)
				require.Equal(t, callerAfter, db.GetBalance(e.caller).ToBig())
				require.Equal(t, targetAfter, db.GetBalance(e.address).ToBig())
				_, err = db.Finalize()
				require.NoError(t, err)
				reopened := state.NewDBImpl(e.ctx, e.keeper, true)
				t.Cleanup(reopened.Cleanup)
				require.Equal(t, callerAfter, reopened.GetBalance(e.caller).ToBig())
				require.Equal(t, targetAfter, reopened.GetBalance(e.address).ToBig())
			})
		}
	}
}

func TestRepairM_1257_LiveActivationBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		height  int64
		tracing bool
		applied bool
		active  bool
	}{
		{"before_activation", repair1257MergeHeight - 1, false, true, false},
		{"at_activation", repair1257MergeHeight, false, true, true},
		{"after_activation", repair1257MergeHeight + 1, false, true, true},
		{"replay_at_activation", repair1257MergeHeight, true, true, true},
		{"replay_after_activation", repair1257MergeHeight + 1, true, true, true},
		{"unapplied_plan", repair1257MergeHeight + 1, false, false, false},
	} {
		for _, value := range []int64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("%s/value_%d", tc.name, value), func(t *testing.T) {
				e := newRepair1257Env(t, tc.height, tc.tracing, tc.applied)
				require.Equal(t, tc.active, pcommon.MergePlanActive(e.ctx, e.keeper))
				evm, db := e.evm(t, false)
				callerBefore, targetBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
				payment := big.NewInt(value)
				output, _, err := evm.Call(e.caller, e.address, e.input, 1_000_000, uint256.MustFromBig(payment))
				if tc.active {
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.Empty(t, output)
					require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
					require.Equal(t, callerBefore, db.GetBalance(e.caller).ToBig())
					require.Equal(t, targetBefore, db.GetBalance(e.address).ToBig())
				} else {
					require.NoError(t, err)
					e.assertRewards(t, output)
					require.Equal(t, new(big.Int).Sub(callerBefore, payment), db.GetBalance(e.caller).ToBig())
					require.Equal(t, new(big.Int).Add(targetBefore, payment), db.GetBalance(e.address).ToBig())
				}
			})
		}
	}
}

func TestRepairM_1257_ZeroValueQueries(t *testing.T) {
	for _, historical := range []bool{true, false} {
		for _, static := range []bool{false, true} {
			t.Run(fmt.Sprintf("historical_%t/static_%t", historical, static), func(t *testing.T) {
				height := repair1257MergeHeight
				if historical {
					height = repair1257HistoricalHeight
				}
				e := newRepair1257Env(t, height, historical, true)
				evm, db := e.evm(t, historical)
				callerBefore, targetBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(e.address).ToBig()
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
				require.Equal(t, targetBefore, db.GetBalance(e.address).ToBig())
			})
		}
	}
}
