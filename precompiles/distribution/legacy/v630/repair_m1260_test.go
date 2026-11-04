package v630_test

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ibctransferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	legacycommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v630"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	v630 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution/legacy/v630"
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
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

//go:embed distribution.go abi.json
var repairM1260Frozen embed.FS

const (
	repairM1260HistoricalHeight int64 = 10
	repairM1260MergeHeight      int64 = 20
)

// Decoder fields follow ABI tuple order, including nested tuples.
type repairM1260Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repairM1260Reward struct {
	Coins            []repairM1260Coin
	ValidatorAddress string
}

type repairM1260Rewards struct {
	Rewards []repairM1260Reward
	Total   []repairM1260Coin
}

type repairM1260Env struct {
	ctx      sdk.Context
	keeper   *evmkeeper.Keeper
	caller   common.Address
	address  common.Address
	abi      abi.ABI
	versions utils.VersionedPrecompiles
	expected repairM1260Rewards
}

// Only the EVM and distribution dependencies are requested by GetVersioned.
// Both are production keepers; unrequested interfaces remain unbound.
type repairM1260Keepers struct {
	utils.Keepers
	evm          *evmkeeper.Keeper
	distribution distrkeeper.Keeper
}

func (k repairM1260Keepers) EVMK() utils.EVMKeeper { return k.evm }
func (k repairM1260Keepers) DistributionK() utils.DistributionKeeper {
	return k.distribution
}

func newRepairM1260Env(t *testing.T, height int64, tracing bool) *repairM1260Env {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys("auth", "bank", "staking", "distribution", "evm", "params", "upgrade")
	transient := sdk.NewTransientStoreKey("transient")
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}, false).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys["params"], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys["auth"], subspace("auth"), authtypes.ProtoBaseAccount, map[string][]string{
		authtypes.FeeCollectorName:     nil,
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	})
	accounts.SetParams(ctx, authtypes.DefaultParams())
	bank := bankkeeper.NewBaseKeeper(cdc, keys["bank"], accounts, subspace("bank"), nil)
	bank.InitGenesis(ctx, banktypes.DefaultGenesisState())
	staking := stakingkeeper.NewKeeper(cdc, keys["staking"], accounts, bank, subspace("staking"))
	distr := distrkeeper.NewKeeper(cdc, keys["distribution"], subspace("distribution"), accounts, bank, &staking, authtypes.FeeCollectorName, nil)
	staking.SetHooks(distr.Hooks())
	distr.InitGenesis(ctx, *distrtypes.DefaultGenesisState())
	upgrades := upgradekeeper.NewKeeper(nil, keys["upgrade"], cdc, "", nil)
	k := evmkeeper.NewKeeper(keys["evm"], transient, subspace("evm"), nil, bank, &accounts, &staking, ibctransferkeeper.Keeper{}, nil, nil, &upgrades)
	k.SetParams(ctx, evmtypes.DefaultParams())
	k.SetAddressMapping(ctx, accounts.GetModuleAddress(authtypes.FeeCollectorName), evmkeeper.GetCoinbaseAddress())
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = k.GetBaseDenom(ctx)
	staking.SetParams(ctx, stakingParams)
	if _, registered := sdk.GetDenomUnit(stakingParams.BondDenom); !registered {
		require.NoError(t, sdk.RegisterDenom(stakingParams.BondDenom, sdk.NewDecWithPrec(1, 6)))
	}
	require.Equal(t, stakingParams.BondDenom, sdk.MustGetBaseDenom())
	require.Zero(t, upgrades.GetDoneHeight(ctx, "v6.3.0"))
	require.Zero(t, upgrades.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	upgrades.SetDone(ctx.WithBlockHeight(repairM1260HistoricalHeight), "v6.3.0")
	upgrades.SetDone(ctx.WithBlockHeight(repairM1260MergeHeight), pcommon.MergeUpgrade)

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[30], key.Key[31] = 12, 60
	validator := sdk.ValAddress(key.PubKey().Address())
	delegator := sdk.AccAddress(validator)
	selfBond := sdk.NewCoins(sdk.NewInt64Coin(stakingParams.BondDenom, 100))
	require.NoError(t, bank.MintCoins(ctx, evmtypes.ModuleName, selfBond))
	require.NoError(t, bank.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, selfBond))
	commission := stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.NewDecWithPrec(2, 1), sdk.NewDecWithPrec(1, 2))
	create, err := stakingtypes.NewMsgCreateValidator(validator, key.PubKey(), selfBond[0], stakingtypes.Description{}, commission, sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(staking).CreateValidator(sdk.WrapSDKContext(ctx), create)
	require.NoError(t, err)
	val, found := staking.GetValidator(ctx, validator)
	require.True(t, found)
	distr.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin("uhpx", 1000)))

	caller := common.HexToAddress("0x1260000000000000000000000000000000000001")
	k.SetAddressMapping(ctx, delegator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(k.GetBaseDenom(ctx), 100_000_000))
	require.NoError(t, bank.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, bank.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	ctx = ctx.WithBlockHeight(height).WithIsTracing(tracing)
	if height < repairM1260MergeHeight {
		ctx = ctx.WithClosestUpgradeName("v6.3.0")
	} else {
		ctx = ctx.WithClosestUpgradeName(pcommon.MergeUpgrade)
	}
	versions := distribution.GetVersioned(pcommon.MergeUpgrade, repairM1260Keepers{distribution: distr, evm: k})
	address := common.HexToAddress(v630.DistrAddress)
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	historical, ok := versions["v6.3.0"].(*legacycommon.DynamicGasPrecompile)
	require.True(t, ok)
	_, ok = historical.GetExecutor().(*v630.PrecompileExecutor)
	require.True(t, ok)
	current, ok := versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
	require.True(t, ok)
	_, ok = current.GetExecutor().(*distribution.PrecompileExecutor)
	require.True(t, ok)
	require.Equal(t, historical.GetABI(), current.GetABI())
	queryABI := historical.GetABI()
	require.Equal(t, "view", queryABI.Methods[v630.RewardsMethod].StateMutability)

	response, err := distr.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	expected := repairM1260Rewards{Rewards: make([]repairM1260Reward, 0), Total: make([]repairM1260Coin, 0)}
	for _, reward := range response.Rewards {
		row := repairM1260Reward{Coins: make([]repairM1260Coin, 0), ValidatorAddress: reward.ValidatorAddress}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, repairM1260Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
		}
		expected.Rewards = append(expected.Rewards, row)
	}
	for _, coin := range response.Total {
		expected.Total = append(expected.Total, repairM1260Coin{Amount: coin.Amount.BigInt(), Decimals: big.NewInt(sdk.Precision), Denom: coin.Denom})
	}
	return &repairM1260Env{ctx: ctx, keeper: k, caller: caller, address: address, abi: queryABI, versions: versions, expected: expected}
}

func (e *repairM1260Env) call(t *testing.T, value uint64, static, rejected bool) {
	t.Helper()
	input, err := e.abi.Pack(v630.RewardsMethod, e.caller)
	require.NoError(t, err)
	require.Equal(t, common.FromHex("0700037d"), input[:4])
	db := state.NewDBImpl(e.ctx, e.keeper, false)
	t.Cleanup(db.Cleanup)
	block, err := e.keeper.GetVMBlockContext(db.Ctx(), e.keeper.GetGasPool())
	require.NoError(t, err)
	routes := e.keeper.CustomPrecompiles(e.ctx)
	require.Contains(t, routes, e.address)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(e.keeper.ChainID(e.ctx)), vm.Config{}, routes)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	callerBefore := db.GetBalance(e.caller).ToBig()
	precompileBefore := db.GetBalance(e.address).ToBig()
	var output []byte
	if static {
		require.Zero(t, value)
		output, _, err = evm.StaticCall(e.caller, e.address, input, 1_000_000)
	} else {
		output, _, err = evm.Call(e.caller, e.address, input, 1_000_000, uint256.NewInt(value))
	}
	if rejected {
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Empty(t, output)
		require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
	} else {
		require.NoError(t, err)
		require.NoError(t, db.GetPrecompileError())
		decoded, decodeErr := e.abi.Methods[v630.RewardsMethod].Outputs.Unpack(output)
		require.NoError(t, decodeErr)
		require.Len(t, decoded, 1)
		actual := abi.ConvertType(decoded[0], new(repairM1260Rewards)).(*repairM1260Rewards)
		require.Equal(t, e.expected, *actual)
	}
	paid := new(big.Int).SetUint64(value)
	if rejected {
		paid.SetUint64(0)
	}
	callerAfter := new(big.Int).Sub(callerBefore, paid)
	precompileAfter := new(big.Int).Add(precompileBefore, paid)
	require.Zero(t, callerAfter.Cmp(db.GetBalance(e.caller).ToBig()), "caller balance must equal the exact expected value")
	require.Zero(t, precompileAfter.Cmp(db.GetBalance(e.address).ToBig()), "precompile balance must equal the exact expected value")
	_, err = db.Finalize()
	require.NoError(t, err)
	reopened := state.NewDBImpl(e.ctx, e.keeper, true)
	t.Cleanup(reopened.Cleanup)
	require.Zero(t, callerAfter.Cmp(reopened.GetBalance(e.caller).ToBig()), "reopened caller balance must equal the exact expected value")
	require.Zero(t, precompileAfter.Cmp(reopened.GetBalance(e.address).ToBig()), "reopened precompile balance must equal the exact expected value")
}

func TestRepairM_1260_HistoricalActivationRawCALL(t *testing.T) {
	for _, height := range []int64{repairM1260HistoricalHeight, repairM1260HistoricalHeight + 1, repairM1260MergeHeight - 1} {
		for _, value := range []uint64{1, 1_000_000_000_000, 1_000_000_000_001} {
			t.Run(fmt.Sprintf("height_%d/value_%d", height, value), func(t *testing.T) {
				e := newRepairM1260Env(t, height, true)
				require.Equal(t, "v6.3.0", e.keeper.GetCustomPrecompilesVersions(e.ctx)[e.address])
				require.Same(t, e.versions["v6.3.0"], e.keeper.CustomPrecompiles(e.ctx)[e.address])
				require.False(t, pcommon.MergePlanActive(e.ctx, e.keeper))
				e.call(t, value, false, false)
			})
		}
	}
}

func TestRepairM_1260_LiveRoutingActivation(t *testing.T) {
	for _, height := range []int64{repairM1260MergeHeight - 1, repairM1260MergeHeight, repairM1260MergeHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, value := range []uint64{1, 1_000_000_000_000, 1_000_000_000_001} {
				t.Run(fmt.Sprintf("height_%d/tracing_%t/value_%d", height, tracing, value), func(t *testing.T) {
					e := newRepairM1260Env(t, height, tracing)
					active := height >= repairM1260MergeHeight
					require.Equal(t, active, pcommon.MergePlanActive(e.ctx, e.keeper))
					version := pcommon.MergeUpgrade
					if tracing && !active {
						version = "v6.3.0"
					}
					require.Same(t, e.versions[version], e.keeper.CustomPrecompiles(e.ctx)[e.address])
					e.call(t, value, false, active)
				})
			}
		}
	}
}

func TestRepairM_1260_ZeroValueRewards(t *testing.T) {
	for _, height := range []int64{repairM1260HistoricalHeight, repairM1260MergeHeight - 1, repairM1260MergeHeight, repairM1260MergeHeight + 1} {
		for _, tracing := range []bool{false, true} {
			for _, static := range []bool{false, true} {
				t.Run(fmt.Sprintf("height_%d/tracing_%t/static_%t", height, tracing, static), func(t *testing.T) {
					e := newRepairM1260Env(t, height, tracing)
					e.call(t, 0, static, false)
				})
			}
		}
	}
}

func TestRepairM_1260_FrozenReplayBytes(t *testing.T) {
	for file, expected := range map[string]string{
		"distribution.go": "0f3f7a1ce6478f113fdb9f8000724a5a8e4db9f79812fa61c4a1cc9f5938b48d",
		"abi.json":        "f0846d0d841d118ac9524b14881d48eb3599c03456dba43e57f6dc76a7cc6be6",
	} {
		content, err := repairM1260Frozen.ReadFile(file)
		require.NoError(t, err)
		require.Equal(t, expected, fmt.Sprintf("%x", sha256.Sum256(content)), "historical replay bytes: %s", file)
	}
}
