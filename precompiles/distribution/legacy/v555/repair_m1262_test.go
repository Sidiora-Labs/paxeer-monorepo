package v555_test

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	pcommonv562 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v562"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v552 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v552"
	v555 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v555"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
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
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

//go:embed distribution.go
var repairM1262ReplaySource []byte

const (
	repairM1262HistoricalHeight int64  = 20
	repairM1262NextHeight       int64  = 30
	repairM1262LiveHeight       int64  = 40
	repairM1262Gas              uint64 = 10_000_000
)

type repairM1262Env struct {
	accounts    authkeeper.AccountKeeper
	bank        bankkeeper.BaseKeeper
	staking     stakingkeeper.Keeper
	distr       distrkeeper.Keeper
	keys        map[string]*sdk.KVStoreKey
	keeper      evmkeeper.Keeper
	ctx         sdk.Context
	versions    putils.VersionedPrecompiles
	caller      common.Address
	delegator   sdk.AccAddress
	withdraw    common.Address
	withdrawSDK sdk.AccAddress
	validator   sdk.ValAddress
}

type repairM1262Keepers struct {
	putils.EmptyKeepers
	evm   *evmkeeper.Keeper
	distr distrkeeper.Keeper
}

func (k *repairM1262Keepers) EVMK() putils.EVMKeeper                   { return k.evm }
func (k *repairM1262Keepers) DistributionK() putils.DistributionKeeper { return k.distr }

func newRepairM1262Env(t *testing.T) *repairM1262Env {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, evmtypes.StoreKey, upgradetypes.StoreKey, "repair_params")
	for _, key := range keys {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKey("repair_transient")
	ms.MountStoreWithDB(transient, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx, _ := sdk.NewContext(ms, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC(), ChainID: "paxeer"}, false).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys["repair_params"], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace("auth"), authtypes.ProtoBaseAccount, map[string][]string{
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		distrtypes.ModuleName:          nil,
		authtypes.FeeCollectorName:     nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	})
	accounts.SetParams(ctx, authtypes.DefaultParams())
	for _, name := range []string{evmtypes.ModuleName, distrtypes.ModuleName, authtypes.FeeCollectorName, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName} {
		require.NotNil(t, accounts.GetModuleAccount(ctx, name))
	}
	bank := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts, subspace("bank"), nil)
	bank.SetParams(ctx, banktypes.DefaultParams())
	staking := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], accounts, bank, subspace("staking"))
	staking.SetParams(ctx, stakingtypes.DefaultParams())
	distr := distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace("distribution"), accounts, bank, &staking, authtypes.FeeCollectorName, nil)
	distr.SetFeePool(ctx, distrtypes.InitialFeePool())
	params := distrtypes.DefaultParams()
	params.WithdrawAddrEnabled = true
	distr.SetParams(ctx, params)
	staking.SetHooks(distr.Hooks())
	upgrades := upgradekeeper.NewKeeper(nil, keys[upgradetypes.StoreKey], cdc, "", nil)
	k := evmkeeper.NewKeeper(keys[evmtypes.StoreKey], transient, subspace("evm"), nil, bank, &accounts, &staking, transferkeeper.Keeper{}, nil, nil, &upgrades)
	k.SetParams(ctx, evmtypes.DefaultParams())
	k.SetNextBaseFeePerGas(ctx, sdk.OneDec())
	feeCollector := accounts.GetModuleAddress(authtypes.FeeCollectorName)
	k.SetAddressMapping(ctx, feeCollector, common.BytesToAddress(feeCollector))
	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[31] = 126
	ethKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	delegator, caller := sdk.AccAddress(key.PubKey().Address()), crypto.PubkeyToAddress(ethKey.PublicKey)
	validator := sdk.ValAddress(delegator)
	denom := staking.GetParams(ctx).BondDenom
	funds := sdk.NewCoins(sdk.NewInt64Coin(denom, 100))
	require.NoError(t, bank.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, bank.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	commission := stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(5, 2), sdk.NewDecWithPrec(5, 2), sdk.ZeroDec())
	msg, err := stakingtypes.NewMsgCreateValidator(validator, key.PubKey(), funds[0], stakingtypes.Description{}, commission, sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(staking).CreateValidator(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	val, found := staking.GetValidator(ctx, validator)
	require.True(t, found)
	require.Equal(t, sdk.NewDecWithPrec(5, 2), val.GetCommission())
	rewards := sdk.NewCoins(sdk.NewInt64Coin(denom, 1000))
	require.NoError(t, bank.MintCoins(ctx, evmtypes.ModuleName, rewards))
	require.NoError(t, bank.SendCoinsFromModuleToModule(ctx, evmtypes.ModuleName, distrtypes.ModuleName, rewards))
	distr.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(rewards...))
	withdrawKey := &secp256k1.PrivKey{Key: make([]byte, 32)}
	withdrawKey.Key[31] = 127
	withdrawEthKey, err := crypto.ToECDSA(withdrawKey.Bytes())
	require.NoError(t, err)
	withdrawSDK, withdraw := sdk.AccAddress(withdrawKey.PubKey().Address()), crypto.PubkeyToAddress(withdrawEthKey.PublicKey)
	k.SetAddressMapping(ctx, delegator, caller)
	k.SetAddressMapping(ctx, withdrawSDK, withdraw)
	for version, height := range map[string]int64{
		"v5.5.2": 10, "v5.5.5": repairM1262HistoricalHeight,
		"v5.6.2": repairM1262NextHeight, pcommon.MergeUpgrade: repairM1262LiveHeight,
	} {
		upgrades.SetDone(ctx.WithBlockHeight(height), version)
	}
	versions := distribution.GetVersioned(pcommon.MergeUpgrade, &repairM1262Keepers{distr: distr, evm: k})
	k.SetCustomPrecompiles(map[common.Address]putils.VersionedPrecompiles{common.HexToAddress(v555.DistrAddress): versions}, pcommon.MergeUpgrade)
	return &repairM1262Env{accounts: accounts, bank: bank, staking: staking, distr: distr, keys: keys, keeper: *k, ctx: ctx, versions: versions, caller: caller, delegator: delegator, withdraw: withdraw, withdrawSDK: withdrawSDK, validator: validator}
}

func (e *repairM1262Env) input(t *testing.T, method string) []byte {
	t.Helper()
	var arg interface{}
	switch method {
	case v555.SetWithdrawAddressMethod:
		arg = e.withdraw
	case v555.WithdrawDelegationRewardsMethod:
		arg = e.validator.String()
	case v555.WithdrawMultipleDelegationRewardsMethod:
		arg = []string{e.validator.String()}
	default:
		t.Fatalf("unknown mutation %s", method)
	}
	a := v555.GetABI()
	input, err := a.Pack(method, arg)
	require.NoError(t, err)
	live := e.versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
	liveInput, err := live.Pack(method, arg)
	require.NoError(t, err)
	require.Equal(t, input, liveInput)
	return input
}

func (e *repairM1262Env) execution(t *testing.T, height int64, historical bool) (*vm.EVM, *state.DBImpl, sdk.Context) {
	t.Helper()
	ctx, _ := e.ctx.WithBlockHeight(height).WithIsTracing(historical).CacheContext()
	precompiles := e.keeper.CustomPrecompiles(ctx)
	address := common.HexToAddress(v555.DistrAddress)
	if historical {
		require.IsType(t, &v555.Precompile{}, precompiles[address])
	} else {
		require.IsType(t, &pcommon.DynamicGasPrecompile{}, precompiles[address])
	}
	db := state.NewDBImpl(ctx, &e.keeper, false)
	t.Cleanup(db.Cleanup)
	block, err := e.keeper.GetVMBlockContext(db.Ctx(), e.keeper.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(e.keeper.ChainID(ctx)), vm.Config{}, precompiles)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db, ctx
}

func (e *repairM1262Env) stores(t *testing.T, ctx sdk.Context) map[string]map[string]string {
	t.Helper()
	result := make(map[string]map[string]string)
	for _, name := range []string{distrtypes.StoreKey, banktypes.StoreKey} {
		entries := make(map[string]string)
		it := ctx.KVStore(e.keys[name]).Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			entries[string(it.Key())] = string(it.Value())
		}
		require.NoError(t, it.Close())
		result[name] = entries
	}
	return result
}

func (e *repairM1262Env) assertMutation(t *testing.T, before, after sdk.Context, method string) {
	t.Helper()
	if method == v555.SetWithdrawAddressMethod {
		require.Equal(t, e.delegator, e.distr.GetDelegatorWithdrawAddr(before, e.delegator))
		require.Equal(t, e.withdrawSDK, e.distr.GetDelegatorWithdrawAddr(after, e.delegator))
		require.Equal(t, e.stores(t, before)[banktypes.StoreKey], e.stores(t, after)[banktypes.StoreKey])
		return
	}
	denom := e.staking.GetParams(before).BondDenom
	balanceBefore := e.bank.GetBalance(before, e.delegator, denom).Amount
	balanceAfter := e.bank.GetBalance(after, e.delegator, denom).Amount
	require.Equal(t, sdk.NewInt(950), balanceAfter.Sub(balanceBefore))
	module := e.accounts.GetModuleAddress(distrtypes.ModuleName)
	moduleBefore := e.bank.GetBalance(before, module, denom).Amount
	moduleAfter := e.bank.GetBalance(after, module, denom).Amount
	require.Equal(t, sdk.NewInt(950), moduleBefore.Sub(moduleAfter))
	require.Equal(t, sdk.NewDec(1000), e.distr.GetValidatorOutstandingRewardsCoins(before, e.validator).AmountOf(denom))
	require.Equal(t, sdk.NewDec(50), e.distr.GetValidatorOutstandingRewardsCoins(after, e.validator).AmountOf(denom))
	require.Equal(t, uint64(after.BlockHeight()), e.distr.GetDelegatorStartingInfo(after, e.validator, e.delegator).Height)
}

var repairM1262Mutations = []string{
	v555.SetWithdrawAddressMethod,
	v555.WithdrawDelegationRewardsMethod,
	v555.WithdrawMultipleDelegationRewardsMethod,
}

func TestRepairM_1262_HistoricalStaticCallStateDiff(t *testing.T) {
	for _, height := range []int64{repairM1262HistoricalHeight, repairM1262NextHeight - 1} {
		for _, method := range repairM1262Mutations {
			t.Run(fmt.Sprintf("%d/%s", height, method), func(t *testing.T) {
				e := newRepairM1262Env(t)
				input := e.input(t, method)
				var callState map[string]map[string]string
				for _, static := range []bool{false, true} {
					evm, db, root := e.execution(t, height, true)
					before := e.stores(t, db.Ctx())
					var output []byte
					var remaining uint64
					var err error
					if static {
						output, remaining, err = evm.StaticCall(e.caller, common.HexToAddress(v555.DistrAddress), input, repairM1262Gas)
					} else {
						output, remaining, err = evm.Call(e.caller, common.HexToAddress(v555.DistrAddress), input, repairM1262Gas, uint256.NewInt(0))
					}
					require.NoError(t, err)
					require.NoError(t, db.GetPrecompileError())
					require.Greater(t, remaining, uint64(0))
					require.Less(t, remaining, repairM1262Gas)
					a := v555.GetABI()
					decoded, err := a.Methods[method].Outputs.Unpack(output)
					require.NoError(t, err)
					require.Equal(t, []interface{}{true}, decoded)
					after := e.stores(t, db.Ctx())
					require.NotEqual(t, before[distrtypes.StoreKey], after[distrtypes.StoreKey])
					e.assertMutation(t, root, db.Ctx(), method)
					if static {
						require.Equal(t, callState, after)
					} else {
						callState = after
					}
					_, err = db.Finalize()
					require.NoError(t, err)
					require.Equal(t, after, e.stores(t, root))
				}
			})
		}
	}
}

func TestRepairM_1262_LiveStaticCallRefusesSameWrites(t *testing.T) {
	for _, height := range []int64{repairM1262LiveHeight, repairM1262LiveHeight + 1} {
		for _, method := range repairM1262Mutations {
			t.Run(fmt.Sprintf("%d/%s", height, method), func(t *testing.T) {
				e := newRepairM1262Env(t)
				input := e.input(t, method)
				evm, db, root := e.execution(t, height, false)
				before := e.stores(t, db.Ctx())
				output, remaining, err := evm.StaticCall(e.caller, common.HexToAddress(v555.DistrAddress), input, repairM1262Gas)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.EqualError(t, db.GetPrecompileError(), "cannot call distr precompile from staticcall")
				require.Empty(t, output)
				require.Zero(t, remaining)
				require.Equal(t, before, e.stores(t, db.Ctx()))
				_, err = db.Finalize()
				require.NoError(t, err)
				require.Equal(t, before, e.stores(t, root))

				evm, db, root = e.execution(t, height, false)
				output, _, err = evm.Call(e.caller, common.HexToAddress(v555.DistrAddress), input, repairM1262Gas, uint256.NewInt(0))
				require.NoError(t, err)
				a := v555.GetABI()
				decoded, err := a.Methods[method].Outputs.Unpack(output)
				require.NoError(t, err)
				require.Equal(t, []interface{}{true}, decoded)
				e.assertMutation(t, root, db.Ctx(), method)
			})
		}
	}
}

func TestRepairM_1262_ReplayAndActivationIsolation(t *testing.T) {
	require.Equal(t, "d1d8873dbf48da4c75dad806184abf2817fe1924d1a57c6ea9e10ec9c611ec64", fmt.Sprintf("%x", sha256.Sum256(repairM1262ReplaySource)))
	e := newRepairM1262Env(t)
	address := common.HexToAddress(v555.DistrAddress)
	for _, tc := range []struct {
		height   int64
		version  string
		contract vm.PrecompiledContract
	}{
		{repairM1262HistoricalHeight - 1, "v5.5.2", &v552.Precompile{}},
		{repairM1262HistoricalHeight, "v5.5.5", &v555.Precompile{}},
		{repairM1262NextHeight - 1, "v5.5.5", &v555.Precompile{}},
		{repairM1262NextHeight, "v5.6.2", &pcommonv562.DynamicGasPrecompile{}},
		{repairM1262LiveHeight, pcommon.MergeUpgrade, &pcommon.DynamicGasPrecompile{}},
		{repairM1262LiveHeight + 1, pcommon.MergeUpgrade, &pcommon.DynamicGasPrecompile{}},
	} {
		ctx := e.ctx.WithBlockHeight(tc.height).WithIsTracing(true)
		require.Equal(t, tc.version, e.keeper.GetCustomPrecompilesVersions(ctx)[address])
		require.IsType(t, tc.contract, e.keeper.CustomPrecompiles(ctx)[address])
		require.Same(t, e.versions[tc.version], e.keeper.CustomPrecompiles(ctx)[address])
		require.Same(t, e.versions[pcommon.MergeUpgrade], e.keeper.CustomPrecompiles(ctx.WithIsTracing(false))[address])
	}
	for _, latest := range []string{pcommon.MergeUpgrade, "v7.1"} {
		versions := distribution.GetVersioned(latest, &repairM1262Keepers{distr: e.distr, evm: &e.keeper})
		require.IsType(t, &pcommon.DynamicGasPrecompile{}, versions[latest])
		for version, contract := range versions {
			_, historical := contract.(*v555.Precompile)
			require.Equal(t, version == "v5.5.5", historical, version)
		}
	}
}
