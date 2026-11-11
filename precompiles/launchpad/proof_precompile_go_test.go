// Proves: Keeper.Pause and InstallSurface -> SurfaceRuntime and Precompile.RunAndCalculateGas -> GetCustomPrecompilesE and PoolKeeper views.
//
// The producer installs the real bytecode and pauses a pool through an EVM CALL.
// A committed LevelDB store is closed and reopened before consumer calls, without
// replaying genesis or the producer. Duplicate pause is refused with Paused().
// Repairs surface.go: SurfaceRuntime aliased the keeper's bytecode and
// InstallSurface trusted that mutable exported slice, so a Go consumer could
// corrupt every subsequent installation. Installations now use the keeper's
// canonical runtime and the exported snapshot owns its bytes.
// The restart assertion decodes paused() as a bool; getPoolInfo is a reserve
// tuple and does not contain the pause flag (pool.go, poolViews).
package launchpad_test

import (
	"math/big"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ibctransferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	keeperv610 "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/legacy/v610/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// proofGoKeepers supplies concrete launchpad and EVM keepers to the same
// registration function used by the application. The remaining constructors
// receive the real application keepers; no precompile or transport is replaced.
type proofGoKeepers struct {
	versionedLaunchpadKeepers
	evm *evmkeeper.Keeper
}

func (k proofGoKeepers) EVMK() utils.EVMKeeper { return k.evm }

type proofGoComponent struct {
	*surfaceHarness
	cms       storetypes.CommitMultiStore
	ctx       sdk.Context
	evmKeeper *evmkeeper.Keeper
	keepers   proofGoKeepers
}

func openProofGoComponent(t *testing.T, dir string) *proofGoComponent {
	t.Helper()
	db, err := dbm.NewGoLevelDB("precompile-go", dir)
	require.NoError(t, err, "precompiles/launchpad: open real producer store")
	cms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	evmKey := sdk.NewKVStoreKey("proof_evm")
	bankKey := sdk.NewKVStoreKey("proof_bank")
	authKey := sdk.NewKVStoreKey("proof_auth")
	transient := sdk.NewTransientStoreKey("proof_transient")
	for _, key := range []*sdk.KVStoreKey{key, evmKey, bankKey, authKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: genesisTime, ChainID: "precompile-go"}, false)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), evmKey, transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, authKey, subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil})
	bank := bankkeeper.NewBaseKeeper(cdc, bankKey, accounts, subspace("bank"), nil)
	ek := evmkeeper.NewKeeper(evmKey, transient, subspace("evm"), nil, bank, &accounts, nil,
		ibctransferkeeper.Keeper{}, nil, nil, &testkeeper.EVMTestApp.UpgradeKeeper)
	k := launchpadkeeper.NewKeeper(key, bank, nil, ek)
	legacy := keeperv610.NewKeeper(key, bank, nil, ek)
	keepers := proofGoKeepers{newVersionedLaunchpadKeepers(testkeeper.EVMTestApp.GetPrecompileKeepers(), k, legacy), ek}
	s := &surfaceHarness{t: t, keeper: k, creator: named("proof guardian"), alice: named("proof reader"), bob: named("proof intruder")}
	c := &proofGoComponent{surfaceHarness: s, cms: cms, ctx: ctx, evmKeeper: ek, keepers: keepers}
	if cms.LastCommitID().Version == 0 {
		ek.SetParams(ctx, evmtypes.DefaultParams())
		bank.SetParams(ctx, banktypes.DefaultParams())
		s.pools[0], s.tokens[0] = named("proof pool"), named("proof token")
		market := migratedMarket(1, types.Generation2, s.pools[0], s.tokens[0], sdk.AccAddress(s.creator.Bytes()), k.Generation2())
		gs := types.DefaultGenesis()
		gs.Markets = []types.Market{market}
		gs.Surfaces = []types.Surface{{Address: market.PoolAddress, Role: types.RolePool}}
		gs.PoolMetadata = []types.PoolMetadata{{Pool: market.PoolAddress, Creator: s.creator.Hex(),
			Token: market.TokenAddress, Optical: market.Optical, NFTID: market.FeeRightID,
			CreatedAt: market.CreationTime, CreatedBlock: ctx.BlockHeight()}}
		gs.FeeRights = types.FeeRightCollection{NextTokenID: 2, PoolIndexMigrationCursor: 2, PoolIndexReady: true}
		k.InitGenesis(ctx, *gs)
		s.markets[0] = market
	}
	return c
}

func (c *proofGoComponent) start(t *testing.T) {
	t.Helper()
	custom, err := precompiles.GetCustomPrecompilesE("v6.12.0", c.keepers)
	require.NoError(t, err)
	versions := launchpad.GetVersioned("v6.12.0", c.keepers)
	require.IsType(t, &launchpad.Precompile{}, versions["v6.12.0"])
	require.NotEqual(t, versions["v6.10"], versions["v6.12.0"])
	c.contract = custom[precompile]["v6.12.0"]
	require.Equal(t, versions["v6.12.0"], c.contract)
	c.stateDB = state.NewDBImpl(c.ctx, c.evmKeeper, false)
	config := evmtypes.DefaultChainConfig().EthereumConfig(big.NewInt(125))
	hooks := &tracing.Hooks{OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
		c.frames = append(c.frames, callFrame{depth: depth, op: vm.OpCode(typ), from: from, to: to,
			input: common.CopyBytes(input), gas: gas, value: value})
	}}
	c.evm = vm.NewEVM(vm.BlockContext{CanTransfer: core.CanTransfer, Transfer: core.Transfer,
		BlockNumber: big.NewInt(1), Time: uint64(genesisTime.Unix()), GasLimit: surfaceGas},
		c.stateDB, config, vm.Config{Tracer: hooks}, map[common.Address]vm.PrecompiledContract{precompile: c.contract})
}

func (c *proofGoComponent) close(t *testing.T, commit bool) {
	t.Helper()
	if commit {
		_, err := c.stateDB.Finalize()
		require.NoError(t, err)
		require.Positive(t, c.cms.Commit(true).Version)
	}
	c.stateDB.Cleanup()
	require.NoError(t, c.cms.Close())
	c.evm, c.stateDB, c.contract, c.keeper = nil, nil, nil, nil
}

func TestProofPrecompileGo(t *testing.T) {
	dir := t.TempDir()
	c := openProofGoComponent(t, dir)
	pool := c.pools[0]
	denom := c.markets[0].Denom
	surface := launchpadkeeper.Surface{Role: types.RolePool, Generation: types.Generation2, Market: denom}

	// The public byte slice must not corrupt either installation path.
	snapshot := common.CopyBytes(launchpad.SurfaceRuntime)
	func() {
		defer copy(launchpad.SurfaceRuntime, snapshot)
		launchpad.SurfaceRuntime[0] ^= 0xff
		require.NotEqual(t, launchpad.SurfaceRuntime, launchpadkeeper.SurfaceRuntime)
		require.NoError(t, launchpad.InstallSurface(c.ctx, c.evmKeeper, c.keeper, pool, surface))
		require.NoError(t, launchpad.InstallRoleSurfaces(c.ctx, c.evmKeeper, c.keeper))
		require.Equal(t, snapshot, c.evmKeeper.GetCode(c.ctx, pool))
	}()
	c.start(t)
	assertSurfaceRuntime(t, c.stateDB, pool)
	// This is the same real bytecode crossing used by surface_test.go.
	input := c.encode("SidioraPool", "pause")
	ret, err := c.call(c.creator, pool, input)
	require.NoError(t, err)
	require.Empty(t, ret)
	require.Len(t, c.frames, 2)
	require.Equal(t, pool, c.frames[1].from)
	require.Equal(t, precompile, c.frames[1].to)
	require.Equal(t, append(common.CopyBytes(input), c.creator.Bytes()...), c.frames[1].input)
	require.True(t, c.keeper.ExportGenesis(c.surfaceHarness.ctx()).Markets[0].Paused)
	logs := c.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, pool, logs[0].Address)
	poolABI := parseABIFile(t, "abi/SidioraPool.json")
	require.Equal(t, []common.Hash{poolABI.Events["PauseToggled"].ID}, logs[0].Topics)
	toggled, err := poolABI.Events["PauseToggled"].Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{true}, toggled)
	c.close(t, true)
	c = nil

	reopened := openProofGoComponent(t, dir)
	reopened.start(t)
	defer reopened.close(t, false)
	require.Equal(t, int64(1), reopened.cms.LastCommitID().Version)
	assertSurfaceRuntime(t, reopened.stateDB, pool)
	record, found, err := reopened.keeper.GetSurface(reopened.surfaceHarness.ctx(), pool)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, surface, record)
	var consumer launchpad.PoolKeeper = reopened.keeper
	market, found := consumer.GetMarket(reopened.surfaceHarness.ctx(), denom)
	require.True(t, found)
	require.True(t, market.Paused)
	ret, err = reopened.call(reopened.alice, pool, reopened.encode("SidioraPool", "paused"))
	require.NoError(t, err)
	require.Equal(t, []interface{}{true}, reopened.decode("SidioraPool", "paused", ret))
	ret, err = reopened.call(reopened.alice, pool, reopened.encode("SidioraPool", "getPoolInfo"))
	require.NoError(t, err)
	decoded := reopened.decode("SidioraPool", "getPoolInfo", ret)
	require.Len(t, decoded, 1)
	info := abi.ConvertType(decoded[0], new(launchpad.PoolInfo)).(*launchpad.PoolInfo)
	require.Equal(t, common.HexToAddress(market.TokenAddress), info.TokenAddress)
	require.Zero(t, market.VirtualQuoteReserve.BigInt().Cmp(info.VirtualUsdlReserve))
	require.Zero(t, market.RealQuoteBalance.BigInt().Cmp(info.RealUsdlBalance))
	require.Zero(t, market.TokenReserve.BigInt().Cmp(info.TokenReserve))
	ret, err = reopened.call(reopened.alice, surfaceAt(types.RolePoolRegistry), reopened.encode("PoolRegistry", "getPoolCount"))
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(1)}, reopened.decode("PoolRegistry", "getPoolCount", ret))
	direct := parseABIFile(t, "abi.json")
	require.Len(t, direct.Methods, 126)
	require.Len(t, direct.Events, 16)
	require.Len(t, (directABIConsumer{direct}).errors(), 69)
	data, err := direct.Pack("getPoolCount")
	require.NoError(t, err)
	ret, err = reopened.call(reopened.alice, precompile, data)
	require.NoError(t, err)
	observed, err := direct.Unpack("getPoolCount", ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(1)}, observed)
	ret, err = reopened.call(reopened.creator, pool, input)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	refusal := (directABIConsumer{direct}).errors()["Paused"]
	require.Equal(t, refusal.ID[:4], ret)
	ret, err = reopened.call(reopened.bob, pool, reopened.encode("SidioraPool", "unpause"))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	refusal = (directABIConsumer{direct}).errors()["Unauthorized"]
	require.Equal(t, refusal.ID[:4], ret)
	market, found = consumer.GetMarket(reopened.surfaceHarness.ctx(), denom)
	require.True(t, found)
	require.True(t, market.Paused)
	before := reopened.exported()
	require.NoError(t, launchpad.InstallSurface(reopened.surfaceHarness.ctx(), reopened.evmKeeper, reopened.keeper, pool, surface))
	require.NoError(t, launchpad.InstallRoleSurfaces(reopened.surfaceHarness.ctx(), reopened.evmKeeper, reopened.keeper))
	require.JSONEq(t, before, reopened.exported())
	assertSurfaceRuntime(t, reopened.stateDB, pool)
}
