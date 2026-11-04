package v620_test

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
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v620"
	nextcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v630"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	legacy "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v620"
	next "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v630"
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

// These are heights in the isolated upgrade-store fixture, not network heights.
const (
	repairM1259LegacyHeight int64  = 20
	repairM1259NextHeight   int64  = 30
	repairM1259MergeHeight  int64  = 100
	repairM1259Gas          uint64 = 1_000_000
)

// This provider adapts concrete production keepers to the constructor's broad
// interface. Only EVMK and DistributionK are consumed by distribution's real
// version registry; no execution component is replaced by a test double.
type repairM1259Keepers struct {
	putils.Keepers
	EvmKeeper     evmkeeper.Keeper
	UpgradeKeeper upgradekeeper.Keeper
	BankKeeper    bankkeeper.BaseKeeper
	StakingKeeper stakingkeeper.Keeper
	DistrKeeper   distrkeeper.Keeper
}

func (k *repairM1259Keepers) EVMK() putils.EVMKeeper                   { return &k.EvmKeeper }
func (k *repairM1259Keepers) DistributionK() putils.DistributionKeeper { return k.DistrKeeper }

func newRepairM1259Keepers(t *testing.T) (*repairM1259Keepers, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, paramtypes.StoreKey, evmtypes.StoreKey, upgradetypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	paramsTransient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	evmTransient := sdk.NewTransientStoreKey("transient_evm")
	cms.MountStoreWithDB(paramsTransient, sdk.StoreTypeTransient, nil)
	cms.MountStoreWithDB(evmTransient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}, false).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, keys[paramtypes.StoreKey], paramsTransient, name)
	}
	perms := map[string][]string{
		minttypes.ModuleName:           {authtypes.Minter},
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		authtypes.FeeCollectorName:     {},
		distrtypes.ModuleName:          {},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	}
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, perms)
	ak.SetParams(ctx, authtypes.DefaultParams())
	for name, permissions := range perms {
		ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, permissions...))
	}
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, subspace(banktypes.ModuleName), map[string]bool{})
	bk.SetParams(ctx, banktypes.DefaultParams())
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, subspace(stakingtypes.ModuleName))
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = evmkeeper.BaseDenom
	sk.SetParams(ctx, stakingParams)
	dk := distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace(distrtypes.ModuleName), ak, bk, &sk, authtypes.FeeCollectorName, map[string]bool{})
	dk.InitGenesis(ctx, *distrtypes.DefaultGenesisState())
	sk.SetHooks(dk.Hooks())
	uk := upgradekeeper.NewKeeper(nil, keys[upgradetypes.StoreKey], cdc, t.TempDir(), nil)
	ek := evmkeeper.NewKeeper(keys[evmtypes.StoreKey], evmTransient, subspace(evmtypes.ModuleName), nil, bk, &ak, &sk, transferkeeper.Keeper{}, nil, nil, &uk)
	ek.SetParams(ctx, evmtypes.DefaultParams())
	ek.SetAddressMapping(ctx, ak.GetModuleAddress(authtypes.FeeCollectorName), evmkeeper.GetCoinbaseAddress())
	k := &repairM1259Keepers{EvmKeeper: *ek, UpgradeKeeper: uk, BankKeeper: bk, StakingKeeper: sk, DistrKeeper: dk}
	versions := distribution.GetVersioned(pcommon.MergeUpgrade, k)
	k.EvmKeeper.SetCustomPrecompiles(map[common.Address]putils.VersionedPrecompiles{common.HexToAddress(legacy.DistrAddress): versions}, pcommon.MergeUpgrade)
	return k, ctx
}

type repairM1259Env struct {
	app      *repairM1259Keepers
	ctx      sdk.Context
	caller   common.Address
	address  common.Address
	input    []byte
	expected []byte
}

func newRepairM1259Env(t *testing.T, height int64, tracing, applied bool) *repairM1259Env {
	t.Helper()
	a, ctx := newRepairM1259Keepers(t)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1259LegacyHeight), "v6.2.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1259NextHeight), "v6.3.0")
	if applied {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repairM1259MergeHeight), pcommon.MergeUpgrade)
	}

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 12, 59
	validator := sdk.ValAddress(key.PubKey().Address())
	delegator := sdk.AccAddress(validator)
	bondDenom := a.StakingKeeper.GetParams(ctx).BondDenom
	selfBond := sdk.NewCoins(sdk.NewInt64Coin(bondDenom, 100))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, selfBond))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, delegator, selfBond))
	msg, err := stakingtypes.NewMsgCreateValidator(validator, key.PubKey(), selfBond[0], stakingtypes.Description{}, stakingtypes.NewCommissionRates(a.StakingKeeper.GetParams(ctx).MinCommissionRate, sdk.OneDec(), sdk.OneDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(a.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	val, found := a.StakingKeeper.GetValidator(ctx, validator)
	require.True(t, found)
	a.DistrKeeper.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin(bondDenom, 1000)))
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 100_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)
	if tracing {
		version := "v6.2.0"
		if height >= repairM1259NextHeight {
			version = "v6.3.0"
		}
		if applied && height >= repairM1259MergeHeight {
			version = pcommon.MergeUpgrade
		}
		ctx = ctx.WithClosestUpgradeName(version)
	}

	// Obtain the ABI independently of the router, so the replay call remains
	// encoded with the frozen historical selector and tuple layout.
	historical, err := legacy.NewPrecompile(a)
	require.NoError(t, err)
	require.Equal(t, "view", historical.Methods[legacy.RewardsMethod].StateMutability)
	require.Equal(t, "rewards(address)", historical.Methods[legacy.RewardsMethod].Sig)
	input, err := historical.Pack(legacy.RewardsMethod, caller)
	require.NoError(t, err)
	response, err := a.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf(bondDenom).IsPositive())
	expected := legacy.Rewards{Rewards: make([]legacy.Reward, 0), Total: make([]legacy.Coin, 0)}
	for _, reward := range response.Rewards {
		row := legacy.Reward{ValidatorAddress: reward.ValidatorAddress, Coins: make([]legacy.Coin, 0)}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, legacy.Coin{Amount: coin.Amount.BigInt(), Denom: coin.Denom, Decimals: big.NewInt(sdk.Precision)})
		}
		expected.Rewards = append(expected.Rewards, row)
	}
	for _, coin := range response.Total {
		expected.Total = append(expected.Total, legacy.Coin{Amount: coin.Amount.BigInt(), Denom: coin.Denom, Decimals: big.NewInt(sdk.Precision)})
	}
	encoded, err := historical.Methods[legacy.RewardsMethod].Outputs.Pack(expected)
	require.NoError(t, err)
	return &repairM1259Env{app: a, ctx: ctx, caller: caller, address: historical.Address(), input: input, expected: encoded}
}

func (e *repairM1259Env) evm(t *testing.T) (*vm.EVM, *state.DBImpl, vm.PrecompiledContract) {
	t.Helper()
	k := &e.app.EvmKeeper
	// The production distribution registry was installed on the real keeper.
	// Let its height/tracing selection choose the map handed to the VM.
	registered := k.CustomPrecompiles(e.ctx)
	selected, found := registered[e.address]
	require.True(t, found)
	require.NotNil(t, selected)
	db := state.NewDBImpl(e.ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(e.ctx)), vm.Config{}, registered)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db, selected
}

func (e *repairM1259Env) assertBalances(t *testing.T, db *state.DBImpl, caller, recipient *big.Int) {
	t.Helper()
	require.Equal(t, uint256.MustFromBig(caller), db.GetBalance(e.caller))
	require.Equal(t, uint256.MustFromBig(recipient), db.GetBalance(e.address))
	_, err := db.Finalize()
	require.NoError(t, err)
	reopened := state.NewDBImpl(e.ctx, &e.app.EvmKeeper, true)
	defer reopened.Cleanup()
	require.Equal(t, uint256.MustFromBig(caller), reopened.GetBalance(e.caller))
	require.Equal(t, uint256.MustFromBig(recipient), reopened.GetBalance(e.address))
}

func TestRepairM_1259_HistoricalRawCALL(t *testing.T) {
	for _, height := range []int64{repairM1259LegacyHeight, repairM1259NextHeight - 1} {
		for _, value := range []uint64{0, 1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height_%d/value_%d", height, value), func(t *testing.T) {
				// The merge marker is already present: replay below it must
				// still select v620 and retain the historical payment hazard.
				e := newRepairM1259Env(t, height, true, true)
				require.Equal(t, "v6.2.0", e.app.EvmKeeper.GetCustomPrecompilesVersions(e.ctx)[e.address])
				evm, db, selected := e.evm(t)
				p, ok := selected.(*legacycommon.DynamicGasPrecompile)
				require.True(t, ok, "historical route selected %T", selected)
				require.IsType(t, &legacy.PrecompileExecutor{}, p.GetExecutor())
				callerBefore := db.GetBalance(e.caller).ToBig()
				recipientBefore := db.GetBalance(e.address).ToBig()
				output, remaining, err := evm.Call(e.caller, e.address, e.input, repairM1259Gas, uint256.NewInt(value))
				require.NoError(t, err)
				require.Positive(t, remaining)
				require.Less(t, remaining, repairM1259Gas)
				require.Equal(t, e.expected, output)
				require.NoError(t, db.GetPrecompileError())
				amount := new(big.Int).SetUint64(value)
				e.assertBalances(t, db, new(big.Int).Sub(callerBefore, amount), new(big.Int).Add(recipientBefore, amount))
			})
		}
	}
}

func TestRepairM_1259_LiveRoutingActivation(t *testing.T) {
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{repairM1259MergeHeight, repairM1259MergeHeight + 1} {
			for _, value := range []uint64{0, 1, 1_000_000_000_000, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("tracing_%t/height_%d/value_%d", tracing, height, value), func(t *testing.T) {
					e := newRepairM1259Env(t, height, tracing, true)
					require.True(t, pcommon.MergePlanActive(e.ctx, &e.app.EvmKeeper))
					evm, db, selected := e.evm(t)
					p, ok := selected.(*pcommon.DynamicGasPrecompile)
					require.True(t, ok, "live route selected %T", selected)
					require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
					callerBefore := db.GetBalance(e.caller).ToBig()
					recipientBefore := db.GetBalance(e.address).ToBig()
					output, _, err := evm.Call(e.caller, e.address, e.input, repairM1259Gas, uint256.NewInt(value))
					if value == 0 {
						require.NoError(t, err)
						require.Equal(t, e.expected, output)
						require.NoError(t, db.GetPrecompileError())
					} else {
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
					}
					e.assertBalances(t, db, callerBefore, recipientBefore)
				})
			}
		}
	}
}

func TestRepairM_1259_PreActivationCompatibility(t *testing.T) {
	for _, applied := range []bool{false, true} {
		height := repairM1259MergeHeight - 1
		if !applied {
			height = repairM1259MergeHeight + 1
		}
		t.Run(fmt.Sprintf("applied_%t", applied), func(t *testing.T) {
			e := newRepairM1259Env(t, height, false, applied)
			require.False(t, pcommon.MergePlanActive(e.ctx, &e.app.EvmKeeper))
			evm, db, selected := e.evm(t)
			p, ok := selected.(*pcommon.DynamicGasPrecompile)
			require.True(t, ok)
			require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
			callerBefore := db.GetBalance(e.caller).ToBig()
			recipientBefore := db.GetBalance(e.address).ToBig()
			value := uint256.NewInt(1_000_000_000_001)
			output, _, err := evm.Call(e.caller, e.address, e.input, repairM1259Gas, value)
			require.NoError(t, err)
			require.Equal(t, e.expected, output)
			e.assertBalances(t, db, new(big.Int).Sub(callerBefore, value.ToBig()), new(big.Int).Add(recipientBefore, value.ToBig()))
		})
	}
}

func TestRepairM_1259_HistoricalBoundaryAndStaticQuery(t *testing.T) {
	for _, height := range []int64{repairM1259NextHeight - 1, repairM1259NextHeight, repairM1259MergeHeight} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			e := newRepairM1259Env(t, height, true, true)
			evm, db, selected := e.evm(t)
			version := e.app.EvmKeeper.GetCustomPrecompilesVersions(e.ctx)[e.address]
			switch height {
			case repairM1259NextHeight - 1:
				require.Equal(t, "v6.2.0", version)
				require.IsType(t, &legacy.PrecompileExecutor{}, selected.(*legacycommon.DynamicGasPrecompile).GetExecutor())
			case repairM1259NextHeight:
				require.Equal(t, "v6.3.0", version)
				require.IsType(t, &next.PrecompileExecutor{}, selected.(*nextcommon.DynamicGasPrecompile).GetExecutor())
			case repairM1259MergeHeight:
				require.IsType(t, &distribution.PrecompileExecutor{}, selected.(*pcommon.DynamicGasPrecompile).GetExecutor())
			}
			callerBefore := db.GetBalance(e.caller).ToBig()
			recipientBefore := db.GetBalance(e.address).ToBig()
			output, _, err := evm.StaticCall(e.caller, e.address, e.input, repairM1259Gas)
			require.NoError(t, err)
			require.Equal(t, e.expected, output)
			e.assertBalances(t, db, callerBefore, recipientBefore)
		})
	}
}
