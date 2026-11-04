package v562_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v562"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	legacy "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v562"
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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const (
	repair1263HistoricalHeight int64 = 10
	repair1263NextHeight       int64 = 20
	repair1263LiveHeight       int64 = 30
)

type repair1263Env struct {
	app        *repair1263Keepers
	ctx        sdk.Context
	caller     common.Address
	delegator  sdk.AccAddress
	recipient  sdk.AccAddress
	withdraw   common.Address
	validators []sdk.ValAddress
	denom      string
}

type repair1263Keepers struct {
	utils.EmptyKeepers
	AccountKeeper authkeeper.AccountKeeper
	BankKeeper    bankkeeper.BaseKeeper
	StakingKeeper stakingkeeper.Keeper
	DistrKeeper   distrkeeper.Keeper
	UpgradeKeeper upgradekeeper.Keeper
	EvmKeeper     evmkeeper.Keeper
	keys          map[string]*sdk.KVStoreKey
}

var _ utils.Keepers = (*repair1263Keepers)(nil)

func (k *repair1263Keepers) EVMK() utils.EVMKeeper                   { return &k.EvmKeeper }
func (k *repair1263Keepers) DistributionK() utils.DistributionKeeper { return k.DistrKeeper }
func (k *repair1263Keepers) GetKey(name string) *sdk.KVStoreKey      { return k.keys[name] }

func newRepair1263Keepers(t *testing.T) (*repair1263Keepers, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	multistore := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, upgradetypes.StoreKey, evmtypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		multistore.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	paramsTransient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	evmTransient := sdk.NewTransientStoreKey("transient_evm")
	multistore.MountStoreWithDB(paramsTransient, sdk.StoreTypeTransient, db)
	multistore.MountStoreWithDB(evmTransient, sdk.StoreTypeTransient, db)
	require.NoError(t, multistore.LoadLatestVersion())
	ctx := sdk.NewContext(multistore, tmtypes.Header{ChainID: "paxeer-test", Height: 2, Time: time.Unix(1, 0).UTC()}, false)
	ctx, _ = ctx.CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, keys[paramtypes.StoreKey], paramsTransient, name)
	}
	permissions := map[string][]string{
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		authtypes.FeeCollectorName:     nil,
	}
	k := &repair1263Keepers{keys: keys}
	k.AccountKeeper = authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	k.AccountKeeper.SetParams(ctx, authtypes.DefaultParams())
	for name, perms := range permissions {
		k.AccountKeeper.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, perms...))
	}
	k.BankKeeper = bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], k.AccountKeeper, subspace(banktypes.ModuleName), map[string]bool{})
	k.BankKeeper.SetParams(ctx, banktypes.DefaultParams())
	k.StakingKeeper = stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], k.AccountKeeper, k.BankKeeper, subspace(stakingtypes.ModuleName))
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = evmkeeper.BaseDenom
	k.StakingKeeper.SetParams(ctx, stakingParams)
	k.DistrKeeper = distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace(distrtypes.ModuleName), k.AccountKeeper, k.BankKeeper, &k.StakingKeeper, authtypes.FeeCollectorName, map[string]bool{})
	k.StakingKeeper.SetHooks(k.DistrKeeper.Hooks())
	k.DistrKeeper.InitGenesis(ctx, *distrtypes.DefaultGenesisState())
	k.UpgradeKeeper = upgradekeeper.NewKeeper(nil, keys[upgradetypes.StoreKey], cdc, t.TempDir(), nil)
	k.EvmKeeper = *evmkeeper.NewKeeper(keys[evmtypes.StoreKey], evmTransient, subspace(evmtypes.ModuleName), nil, k.BankKeeper, &k.AccountKeeper, &k.StakingKeeper, transferkeeper.Keeper{}, nil, nil, &k.UpgradeKeeper)
	k.EvmKeeper.SetParams(ctx, evmtypes.DefaultParams())
	k.EvmKeeper.SetAddressMapping(ctx, k.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName), evmkeeper.GetCoinbaseAddress())
	address := common.HexToAddress(legacy.DistrAddress)
	k.EvmKeeper.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: distribution.GetVersioned(pcommon.MergeUpgrade, k)}, pcommon.MergeUpgrade)
	return k, ctx
}

func newRepair1263Env(t *testing.T, height int64, tracing bool) *repair1263Env {
	t.Helper()
	a, ctx := newRepair1263Keepers(t)
	e := &repair1263Env{app: a, ctx: ctx, denom: a.StakingKeeper.GetParams(ctx).BondDenom}
	key := func(seed byte) *secp256k1.PrivKey {
		k := &secp256k1.PrivKey{Key: make([]byte, 32)}
		k.Key[31] = seed
		return k
	}
	callerKey := key(123)
	e.delegator = sdk.AccAddress(callerKey.PubKey().Address())
	ecdsaKey, err := crypto.ToECDSA(callerKey.Bytes())
	require.NoError(t, err)
	e.caller = crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	e.recipient = sdk.AccAddress(key(124).PubKey().Address())
	e.withdraw = common.HexToAddress("0x1263")
	a.EvmKeeper.SetAddressMapping(ctx, e.delegator, e.caller)
	a.EvmKeeper.SetAddressMapping(ctx, e.recipient, e.withdraw)
	a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, e.recipient))
	params := a.DistrKeeper.GetParams(ctx)
	params.WithdrawAddrEnabled = true
	a.DistrKeeper.SetParams(ctx, params)

	fund := func(addr sdk.AccAddress, amount int64) {
		coins := sdk.NewCoins(sdk.NewInt64Coin(e.denom, amount))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, addr, coins))
	}
	fund(e.delegator, 200)
	stakingServer := stakingkeeper.NewMsgServerImpl(a.StakingKeeper)
	for _, seed := range []byte{125, 126} {
		pub := key(seed).PubKey()
		validator := sdk.ValAddress(pub.Address())
		fund(sdk.AccAddress(validator), 100)
		commission := stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(5, 2), sdk.NewDecWithPrec(5, 2), sdk.ZeroDec())
		msg, err := stakingtypes.NewMsgCreateValidator(validator, pub, sdk.NewInt64Coin(e.denom, 100), stakingtypes.Description{Moniker: fmt.Sprintf("repair-1263-%d", seed)}, commission, sdk.OneInt())
		require.NoError(t, err)
		require.NoError(t, msg.ValidateBasic())
		_, err = stakingServer.CreateValidator(sdk.WrapSDKContext(ctx), msg)
		require.NoError(t, err)
		delegation := stakingtypes.NewMsgDelegate(e.delegator, validator, sdk.NewInt64Coin(e.denom, 100))
		require.NoError(t, delegation.ValidateBasic())
		_, err = stakingServer.Delegate(sdk.WrapSDKContext(ctx), delegation)
		require.NoError(t, err)
		val, found := a.StakingKeeper.GetValidator(ctx, validator)
		require.True(t, found)
		a.DistrKeeper.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin(e.denom, 1000)))
		e.validators = append(e.validators, validator)
	}
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, sdk.NewCoins(sdk.NewInt64Coin(e.denom, 2000))))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToModule(ctx, evmtypes.ModuleName, distrtypes.ModuleName, sdk.NewCoins(sdk.NewInt64Coin(e.denom, 2000))))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1263HistoricalHeight), "v5.6.2")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1263NextHeight), "v5.8.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1263LiveHeight), pcommon.MergeUpgrade)
	e.ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)
	return e
}

func (e *repair1263Env) args(method string) []interface{} {
	switch method {
	case legacy.SetWithdrawAddressMethod:
		return []interface{}{e.withdraw}
	case legacy.WithdrawDelegationRewardsMethod:
		return []interface{}{e.validators[0].String()}
	case legacy.WithdrawMultipleDelegationRewardsMethod:
		return []interface{}{[]string{e.validators[0].String(), e.validators[1].String()}}
	default:
		panic("unexpected distribution method: " + method)
	}
}

func (e *repair1263Env) stores(t *testing.T, ctx sdk.Context) map[string]map[string][]byte {
	t.Helper()
	result := make(map[string]map[string][]byte)
	for _, name := range []string{banktypes.StoreKey, distrtypes.StoreKey} {
		values := make(map[string][]byte)
		iterator := ctx.KVStore(e.app.GetKey(name)).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			values[string(iterator.Key())] = bytes.Clone(iterator.Value())
		}
		require.NoError(t, iterator.Close())
		result[name] = values
	}
	return result
}

func (e *repair1263Env) evm(t *testing.T) (*vm.EVM, *state.DBImpl, vm.PrecompiledContract) {
	t.Helper()
	k := &e.app.EvmKeeper
	db := state.NewDBImpl(e.ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	precompiles := k.CustomPrecompiles(e.ctx)
	address := common.HexToAddress(legacy.DistrAddress)
	require.Contains(t, precompiles, address)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(e.ctx)), vm.Config{}, precompiles)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db, precompiles[address]
}

func repair1263Methods() []string {
	return []string{legacy.SetWithdrawAddressMethod, legacy.WithdrawDelegationRewardsMethod, legacy.WithdrawMultipleDelegationRewardsMethod}
}

func TestRepairM_1263_HistoricalStaticCallStateDiff(t *testing.T) {
	for _, height := range []int64{repair1263HistoricalHeight, repair1263NextHeight - 1} {
		for _, method := range repair1263Methods() {
			t.Run(fmt.Sprintf("%d/%s", height, method), func(t *testing.T) {
				e := newRepair1263Env(t, height, true)
				evm, db, selected := e.evm(t)
				p, ok := selected.(*legacycommon.DynamicGasPrecompile)
				require.True(t, ok, "historical-height dispatch must select the v562 wrapper")
				require.IsType(t, &legacy.PrecompileExecutor{}, p.GetExecutor())
				require.Equal(t, "v5.6.2", e.app.EvmKeeper.GetCustomPrecompilesVersions(e.ctx)[p.Address()])
				input, err := p.Pack(method, e.args(method)...)
				require.NoError(t, err)
				before := e.stores(t, db.Ctx())
				expected, _ := e.ctx.CacheContext()
				if method == legacy.SetWithdrawAddressMethod {
					require.NoError(t, e.app.DistrKeeper.SetWithdrawAddr(expected, e.delegator, e.recipient))
				} else {
					validators := e.validators[:1]
					if method == legacy.WithdrawMultipleDelegationRewardsMethod {
						validators = e.validators
					}
					for _, validator := range validators {
						coins, err := e.app.DistrKeeper.WithdrawDelegationRewards(expected, e.delegator, validator)
						require.NoError(t, err)
						require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(e.denom, 475)), coins)
					}
				}
				want := e.stores(t, expected)
				require.NotEqual(t, before[distrtypes.StoreKey], want[distrtypes.StoreKey])
				output, _, err := evm.StaticCall(e.caller, p.Address(), input, 20_000_000)
				require.NoError(t, err)
				require.NoError(t, db.GetPrecompileError())
				decoded, err := p.Methods[method].Outputs.Unpack(output)
				require.NoError(t, err)
				require.Equal(t, []interface{}{true}, decoded)
				require.Equal(t, want, e.stores(t, db.Ctx()), "replay must retain the exact historical keeper writes")
				if method == legacy.SetWithdrawAddressMethod {
					require.Equal(t, before[banktypes.StoreKey], want[banktypes.StoreKey])
					require.Equal(t, e.recipient, e.app.DistrKeeper.GetDelegatorWithdrawAddr(db.Ctx(), e.delegator))
				} else {
					require.NotEqual(t, before[banktypes.StoreKey], want[banktypes.StoreKey])
					amount := int64(475)
					if method == legacy.WithdrawMultipleDelegationRewardsMethod {
						amount *= 2
					}
					require.Equal(t, sdk.NewInt64Coin(e.denom, amount), e.app.BankKeeper.GetBalance(db.Ctx(), e.delegator, e.denom))
				}
				_, err = db.Finalize()
				require.NoError(t, err)
				require.Equal(t, want, e.stores(t, e.ctx), "replay finalization must retain the state diff")
			})
		}
	}
}

func TestRepairM_1263_LiveStaticCallPreservesState(t *testing.T) {
	for _, height := range []int64{repair1263LiveHeight, repair1263LiveHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, method := range repair1263Methods() {
				t.Run(fmt.Sprintf("%d/tracing=%t/%s", height, tracing, method), func(t *testing.T) {
					e := newRepair1263Env(t, height, tracing)
					evm, db, selected := e.evm(t)
					p, ok := selected.(*pcommon.DynamicGasPrecompile)
					require.True(t, ok, "live dispatch must select the current wrapper")
					require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
					historicalABI := legacy.GetABI()
					input, err := historicalABI.Pack(method, e.args(method)...)
					require.NoError(t, err)
					liveInput, err := p.Pack(method, e.args(method)...)
					require.NoError(t, err)
					require.Equal(t, input, liveInput, "both executors must receive identical ABI calldata")
					before := e.stores(t, db.Ctx())
					_, _, err = evm.StaticCall(e.caller, p.Address(), input, 20_000_000)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.EqualError(t, db.GetPrecompileError(), "cannot call distr precompile from staticcall")
					require.Equal(t, before, e.stores(t, db.Ctx()))
					_, err = db.Finalize()
					require.NoError(t, err)
					require.Equal(t, before, e.stores(t, e.ctx), "refused writes must remain absent after finalization")
				})
			}
		}
	}
}

func TestRepairM_1263_LegacyConstructorIsolation(t *testing.T) {
	e := newRepair1263Env(t, repair1263LiveHeight, false)
	keepers := e.app
	for _, latest := range []string{pcommon.MergeUpgrade, "v8.0"} {
		versions := distribution.GetVersioned(latest, keepers)
		historical, ok := versions["v5.6.2"].(*legacycommon.DynamicGasPrecompile)
		require.True(t, ok)
		require.IsType(t, &legacy.PrecompileExecutor{}, historical.GetExecutor())
		current, ok := versions[latest].(*pcommon.DynamicGasPrecompile)
		require.True(t, ok)
		require.IsType(t, &distribution.PrecompileExecutor{}, current.GetExecutor())
		for version, precompile := range versions {
			if p, ok := precompile.(*legacycommon.DynamicGasPrecompile); ok {
				if _, historical := p.GetExecutor().(*legacy.PrecompileExecutor); historical {
					require.Equal(t, "v5.6.2", version, "legacy constructor must remain confined to its replay mapping")
				}
			}
		}
	}
}
