package v640_test

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v640"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v640 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v640"
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
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
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

const (
	repair1261HistoricalHeight int64 = 10
	repair1261MergeHeight      int64 = 30
)

//go:embed distribution.go abi.json
var repair1261HistoricalFiles embed.FS

type repair1261Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repair1261Reward struct {
	Coins            []repair1261Coin
	ValidatorAddress string
}

type repair1261Rewards struct {
	Rewards []repair1261Reward
	Total   []repair1261Coin
}

type repair1261Env struct {
	app    *repair1261Keepers
	ctx    sdk.Context
	caller common.Address
	abi    abi.ABI
	input  []byte
	want   repair1261Rewards
}

// This adapter exposes the real keepers consumed by every distribution
// constructor. Unrelated domains are absent and cannot be called through it.
type repair1261Keepers struct {
	utils.Keepers
	EvmKeeper     *evmkeeper.Keeper
	DistrKeeper   distrkeeper.Keeper
	BankKeeper    bankkeeper.BaseKeeper
	StakingKeeper stakingkeeper.Keeper
	UpgradeKeeper upgradekeeper.Keeper
}

func (k *repair1261Keepers) EVMK() utils.EVMKeeper                   { return k.EvmKeeper }
func (k *repair1261Keepers) DistributionK() utils.DistributionKeeper { return k.DistrKeeper }

func newRepair1261Keepers(t *testing.T) (*repair1261Keepers, sdk.Context) {
	t.Helper()
	database := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	stores := store.NewCommitMultiStore(database)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, evmtypes.StoreKey, upgradetypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		stores.MountStoreWithDB(key, sdk.StoreTypeIAVL, database)
	}
	transient := sdk.NewTransientStoreKeys(paramtypes.TStoreKey, evmtypes.TransientStoreKey)
	for _, key := range transient {
		stores.MountStoreWithDB(key, sdk.StoreTypeTransient, database)
	}
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmtypes.Header{ChainID: "pax-test", Height: 2, Time: time.Unix(1_800_000_000, 0).UTC()}, false)
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	stakingtypes.RegisterLegacyAminoCodec(amino)
	params := paramskeeper.NewKeeper(cdc, amino, keys[paramtypes.StoreKey], transient[paramtypes.TStoreKey])
	permissions := map[string][]string{
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], params.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	accounts.SetParams(ctx, authtypes.DefaultParams())
	for name, permissions := range permissions {
		accounts.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, permissions...))
	}
	bank := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts, params.Subspace(banktypes.ModuleName), map[string]bool{})
	bank.SetParams(ctx, banktypes.DefaultParams())
	staking := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], accounts, bank, params.Subspace(stakingtypes.ModuleName))
	staking.SetParams(ctx, stakingtypes.DefaultParams())
	distributionKeeper := distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], params.Subspace(distrtypes.ModuleName), accounts, bank, &staking, authtypes.FeeCollectorName, map[string]bool{})
	staking.SetHooks(distributionKeeper.Hooks())
	distributionKeeper.InitGenesis(ctx, *distrtypes.DefaultGenesisState())
	upgrades := upgradekeeper.NewKeeper(map[int64]bool{}, keys[upgradetypes.StoreKey], cdc, t.TempDir(), nil)
	evm := evmkeeper.NewKeeper(keys[evmtypes.StoreKey], transient[evmtypes.TransientStoreKey], params.Subspace(evmtypes.ModuleName), nil, bank, &accounts, &staking, transferkeeper.Keeper{}, nil, nil, &upgrades)
	evm.SetParams(ctx, evmtypes.DefaultParams())
	bank.RegisterRecipientChecker(evm.CanAddressReceive)
	evm.SetAddressMapping(ctx, accounts.GetModuleAddress(authtypes.FeeCollectorName), evmkeeper.GetCoinbaseAddress())
	keepers := &repair1261Keepers{EvmKeeper: evm, DistrKeeper: distributionKeeper, BankKeeper: bank, StakingKeeper: staking, UpgradeKeeper: upgrades}
	tags, err := os.ReadFile("../../../../node/tags")
	require.NoError(t, err)
	versions := strings.Fields(string(tags))
	require.NotEmpty(t, versions)
	latest := versions[len(versions)-1]
	require.Equal(t, pcommon.MergeUpgrade, latest)
	address := common.HexToAddress(v640.DistrAddress)
	evm.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: distribution.GetVersioned(latest, keepers)}, latest)
	return keepers, ctx
}

func newRepair1261Env(t *testing.T) *repair1261Env {
	t.Helper()
	a, ctx := newRepair1261Keepers(t)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(5), "v6.3.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1261HistoricalHeight), "v6.4.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair1261MergeHeight), pcommon.MergeUpgrade)
	require.Equal(t, repair1261HistoricalHeight, a.UpgradeKeeper.GetDoneHeight(ctx, "v6.4.0"))
	require.Equal(t, repair1261MergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 12, 61
	validator := sdk.ValAddress(key.PubKey().Address())
	delegator := sdk.AccAddress(validator)
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 100_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	msg, err := stakingtypes.NewMsgCreateValidator(validator, key.PubKey(), sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 100), stakingtypes.Description{}, stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.NewDecWithPrec(2, 1), sdk.NewDecWithPrec(1, 2)), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(a.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	val, found := a.StakingKeeper.GetValidator(ctx, validator)
	require.True(t, found)
	a.DistrKeeper.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin("uhpx", 1000)))
	ctx = ctx.WithBlockHeight(repair1261HistoricalHeight)

	response, err := a.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.Len(t, response.Rewards, 1)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	want := repair1261Rewards{Rewards: make([]repair1261Reward, 0), Total: make([]repair1261Coin, 0)}
	for _, reward := range response.Rewards {
		row := repair1261Reward{ValidatorAddress: reward.ValidatorAddress, Coins: make([]repair1261Coin, 0)}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, repair1261Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
		}
		want.Rewards = append(want.Rewards, row)
	}
	for _, coin := range response.Total {
		want.Total = append(want.Total, repair1261Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
	}
	p, err := v640.NewPrecompile(a)
	require.NoError(t, err)
	require.Equal(t, "view", p.Methods[v640.RewardsMethod].StateMutability)
	input, err := p.Pack(v640.RewardsMethod, caller)
	require.NoError(t, err)
	return &repair1261Env{app: a, ctx: ctx, caller: caller, abi: p.GetABI(), input: input, want: want}
}

func (e *repair1261Env) evm(t *testing.T, height int64, tracing bool) (*vm.EVM, *state.DBImpl, sdk.Context) {
	t.Helper()
	ctx, _ := e.ctx.CacheContext()
	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)
	k := e.app.EvmKeeper
	served := k.CustomPrecompiles(ctx)
	address := common.HexToAddress(v640.DistrAddress)
	require.Contains(t, served, address)
	if tracing && height < repair1261MergeHeight {
		p, ok := served[address].(*legacycommon.DynamicGasPrecompile)
		require.True(t, ok, "historical dispatch must select the frozen wrapper")
		require.IsType(t, &v640.PrecompileExecutor{}, p.GetExecutor())
		require.Equal(t, "v6.4.0", k.GetCustomPrecompilesVersions(ctx)[address])
	} else {
		p, ok := served[address].(*pcommon.DynamicGasPrecompile)
		require.True(t, ok, "live dispatch must select the current wrapper")
		require.IsType(t, &distribution.PrecompileExecutor{}, p.GetExecutor())
	}
	db := state.NewDBImpl(ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(ctx)), vm.Config{}, served)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db, ctx
}

func (e *repair1261Env) assertRewards(t *testing.T, output []byte) {
	t.Helper()
	decoded, err := e.abi.Methods[v640.RewardsMethod].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	actual := abi.ConvertType(decoded[0], new(repair1261Rewards)).(*repair1261Rewards)
	require.Equal(t, e.want, *actual)
}

func TestRepairM_1261_HistoricalBytes(t *testing.T) {
	for path, want := range map[string]string{
		"distribution.go": "9ffcc2b557fd8e0a8954d79ff1c377af6cd43f6dd00c4d0d88c21cfa2e0bac76",
		"abi.json":        "f0846d0d841d118ac9524b14881d48eb3599c03456dba43e57f6dc76a7cc6be6",
	} {
		data, err := repair1261HistoricalFiles.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, want, fmt.Sprintf("%x", sha256.Sum256(data)), "historical replay source must remain byte-compatible")
	}
}

func TestRepairM_1261_HistoricalActivationRawCALL(t *testing.T) {
	e := newRepair1261Env(t)
	address := common.HexToAddress(v640.DistrAddress)
	beforeActivation := e.ctx.WithBlockHeight(repair1261HistoricalHeight - 1).WithIsTracing(true)
	require.Equal(t, "v6.3.0", e.app.EvmKeeper.GetCustomPrecompilesVersions(beforeActivation)[address])
	for _, height := range []int64{repair1261HistoricalHeight, repair1261HistoricalHeight + 1, repair1261MergeHeight - 1} {
		for _, value := range []int64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height_%d/value_%d", height, value), func(t *testing.T) {
				evm, db, ctx := e.evm(t, height, true)
				callerBefore, targetBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(address).ToBig()
				output, _, err := evm.Call(e.caller, address, e.input, 1_000_000, uint256.NewInt(uint64(value)))
				require.NoError(t, err)
				require.NoError(t, db.GetPrecompileError())
				e.assertRewards(t, output)
				wantCaller := new(big.Int).Sub(callerBefore, big.NewInt(value))
				wantTarget := new(big.Int).Add(targetBefore, big.NewInt(value))
				require.Equal(t, wantCaller, db.GetBalance(e.caller).ToBig())
				require.Equal(t, wantTarget, db.GetBalance(address).ToBig())
				_, err = db.Finalize()
				require.NoError(t, err)
				reopened := state.NewDBImpl(ctx, e.app.EvmKeeper, true)
				defer reopened.Cleanup()
				require.Equal(t, wantCaller, reopened.GetBalance(e.caller).ToBig())
				require.Equal(t, wantTarget, reopened.GetBalance(address).ToBig())
			})
		}
	}
}

func TestRepairM_1261_LiveRoutingActivation(t *testing.T) {
	e := newRepair1261Env(t)
	address := common.HexToAddress(v640.DistrAddress)
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{repair1261MergeHeight - 1, repair1261MergeHeight, repair1261MergeHeight + 1} {
			for _, value := range []int64{1, 1_000_000_000_000, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("tracing_%t/height_%d/value_%d", tracing, height, value), func(t *testing.T) {
					evm, db, ctx := e.evm(t, height, tracing)
					require.Equal(t, height >= repair1261MergeHeight, pcommon.MergePlanActive(ctx, e.app.EvmKeeper))
					callerBefore, targetBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(address).ToBig()
					output, _, err := evm.Call(e.caller, address, e.input, 1_000_000, uint256.NewInt(uint64(value)))
					if height < repair1261MergeHeight {
						require.NoError(t, err)
						e.assertRewards(t, output)
						require.Equal(t, new(big.Int).Sub(callerBefore, big.NewInt(value)), db.GetBalance(e.caller).ToBig())
						require.Equal(t, new(big.Int).Add(targetBefore, big.NewInt(value)), db.GetBalance(address).ToBig())
						return
					}
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.Empty(t, output)
					require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
					require.Equal(t, callerBefore, db.GetBalance(e.caller).ToBig())
					require.Equal(t, targetBefore, db.GetBalance(address).ToBig())
					_, err = db.Finalize()
					require.NoError(t, err)
					reopened := state.NewDBImpl(ctx, e.app.EvmKeeper, true)
					defer reopened.Cleanup()
					require.Equal(t, callerBefore, reopened.GetBalance(e.caller).ToBig())
					require.Equal(t, targetBefore, reopened.GetBalance(address).ToBig())
				})
			}
		}
	}
}

func TestRepairM_1261_ZeroValueRewards(t *testing.T) {
	e := newRepair1261Env(t)
	address := common.HexToAddress(v640.DistrAddress)
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{repair1261HistoricalHeight, repair1261MergeHeight - 1, repair1261MergeHeight, repair1261MergeHeight + 1} {
			for _, static := range []bool{false, true} {
				t.Run(fmt.Sprintf("tracing_%t/height_%d/static_%t", tracing, height, static), func(t *testing.T) {
					evm, db, _ := e.evm(t, height, tracing)
					callerBefore, targetBefore := db.GetBalance(e.caller).ToBig(), db.GetBalance(address).ToBig()
					var output []byte
					var err error
					if static {
						output, _, err = evm.StaticCall(e.caller, address, e.input, 1_000_000)
					} else {
						output, _, err = evm.Call(e.caller, address, e.input, 1_000_000, uint256.NewInt(0))
					}
					require.NoError(t, err)
					require.NoError(t, db.GetPrecompileError())
					e.assertRewards(t, output)
					require.Equal(t, callerBefore, db.GetBalance(e.caller).ToBig())
					require.Equal(t, targetBefore, db.GetBalance(address).ToBig())
				})
			}
		}
	}
}
