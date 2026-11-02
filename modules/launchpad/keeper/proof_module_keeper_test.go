// Proves: keeper.Keeper.InitGenesis/ExportGenesis/PriceHistory and keeper.SurfaceRuntime -> launchpad.AppModule JSON, registered Query service and EVM 0x1017 -> node.App wiring and the launchpadtest/precompile consumers.
//
// Repairs price_history.go: Keeper.PriceHistory must refuse corrupt persisted
// market JSON and invalid snapshot values with ErrInvalidMarket, not panic or
// publish an invalid query response. Repairs genesis.go: Keeper.ExportGenesis
// must validate its assembled result before publishing it to AppModule.
// The stateless runtime is executed afresh for every read and forwards the real
// caller, including a refused stranger and a successful guardian mutation.
// Module state crosses a closed and reopened genesis file into fresh keepers;
// EVM code belongs to the EVM module and remains in the application's EVM store.
// sdk/types/int.go: Int.MarshalJSON normalizes a nil Int to zero, so a
// missing snapshot is injected into the serialized producer output instead.
// simulation/decoder.go: NewDecodeStore currently panics on malformed JSON;
// this proof preserves that explicit refusal without changing that other path.
package keeper_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	moduleimpl "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/launchpadtest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	launchpadprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// proofModuleClient uses the same AppModule.RegisterServices entry point as
// node.App and its real protobuf-marshalling ABCI query router.
func proofModuleClient(t *testing.T, k *keeper.Keeper, ctx sdk.Context) types.QueryClient {
	t.Helper()
	app := testkeeper.EVMTestApp
	queries := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	messages := baseapp.NewMsgServiceRouter()
	messages.SetInterfaceRegistry(app.InterfaceRegistry())
	moduleimpl.NewAppModule(k).RegisterServices(module.NewConfigurator(app.AppCodec(), messages, queries))
	return types.NewQueryClient(queries)
}

func proofModuleABI(t *testing.T, role string) abi.ABI {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../../precompiles/launchpad/abi", role+".json"))
	require.NoError(t, err)
	parsed, err := abi.JSON(bytes.NewReader(raw))
	require.NoError(t, err)
	return parsed
}

// proofModuleEVM constructs the same EVM crossing as newDomain, newNFTEnv
// and newFeeTestEnv, with the real precompile and the imported module keeper.
func proofModuleEVM(t *testing.T, k *keeper.Keeper, ctx sdk.Context) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	app := testkeeper.EVMTestApp
	pre, err := launchpadprecompile.NewPrecompileWithKeeper(k, app.GetPrecompileKeepers().EVMK())
	require.NoError(t, err)
	contracts := map[common.Address]vm.PrecompiledContract{}
	for address, contract := range app.EvmKeeper.CustomPrecompiles(ctx) {
		contracts[address] = contract
	}
	contracts[common.HexToAddress(types.LaunchpadAddress)] = pre
	db := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	cfg := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	return vm.NewEVM(*block, db, cfg, vm.Config{}, contracts), db
}

func proofModuleView(t *testing.T, evm *vm.EVM, caller, target common.Address, contract abi.ABI, name string, args ...interface{}) interface{} {
	t.Helper()
	input, err := contract.Pack(name, args...)
	require.NoError(t, err)
	raw, _, err := evm.StaticCall(caller, target, input, 30_000_000)
	require.NoError(t, err, "%s: %x", name, raw)
	values, err := contract.Unpack(name, raw)
	require.NoError(t, err)
	require.Len(t, values, 1)
	return values[0]
}

func TestProofModuleKeeperEndToEnd(t *testing.T) {
	e := newEnv(t)
	app := testkeeper.EVMTestApp
	require.Same(t, app.LaunchpadKeeper, app.GetPrecompileKeepers().LaunchpadK())
	// This is the keeper wired by node.App into both the module and precompile.
	require.Same(t, app.LaunchpadKeeper, e.k)
	require.NoError(t, e.k.InstallRoleSurfaces(e.ctx, &app.EvmKeeper))
	creator := e.account(5_000_000)
	market := e.create(creator, types.FeeStrategyClaim)
	trader := e.account(100_000_000)
	want := []types.PricePoint{}
	for n := 0; n < 2; n++ {
		e.ctx = e.ctx.WithBlockHeight(int64(10 * (n + 1)))
		e.at(mature + time.Duration(n)*time.Minute)
		e.swap(trader, market.Denom, true, 2_000_000)
		price, err := e.market(market.Denom).Price()
		require.NoError(t, err)
		want = append(want, types.PricePoint{Height: uint64(e.ctx.BlockHeight()), Price: sdk.NewIntFromBigInt(price)})
	}
	pool := common.HexToAddress(market.PoolAddress)
	owner := e.evmAddress(creator)
	am := moduleimpl.NewAppModule(e.k)
	basic := moduleimpl.NewAppModuleBasic()
	require.Equal(t, types.ModuleName, am.Name())
	raw := am.ExportGenesis(e.ctx, app.AppCodec())
	require.NoError(t, basic.ValidateGenesis(app.AppCodec(), nil, raw))
	require.JSONEq(t, string(raw), string(<-am.ExportGenesisStream(e.ctx, app.AppCodec())))

	// Publish the actual module export durably, close it, and drop the bytes.
	path := filepath.Join(t.TempDir(), "launchpad-genesis.json")
	file, err := os.Create(path)
	require.NoError(t, err)
	n, err := file.Write(raw)
	require.NoError(t, err)
	require.Equal(t, len(raw), n)
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())
	raw = nil

	for restart := 0; restart < 2; restart++ {
		file, err := os.Open(path)
		require.NoError(t, err)
		imported, err := io.ReadAll(file)
		require.NoError(t, err)
		require.NoError(t, file.Close())
		fresh, ctx := launchpadtest.NewKeeper(app, e.ctx)
		require.Zero(t, fresh.GetMarketCount(ctx))
		consumer := moduleimpl.NewAppModule(fresh)
		require.Empty(t, consumer.InitGenesis(ctx, app.AppCodec(), imported))
		require.JSONEq(t, string(imported), string(consumer.ExportGenesis(ctx, app.AppCodec())))
		client := proofModuleClient(t, fresh, ctx)
		evm, db := proofModuleEVM(t, fresh, ctx)
		poolABI := proofModuleABI(t, types.RolePool)
		nftABI := proofModuleABI(t, types.RoleFeeRights)
		require.Equal(t, keeper.SurfaceRuntime, db.GetCode(pool))
		require.Equal(t, keeper.SurfaceRuntime, db.GetCode(fresh.Generation2().SidioraNFT))
		for read := 0; read < 2; read++ {
			points, err := fresh.PriceHistory(ctx, market.Denom, 0, ^uint64(0))
			require.NoError(t, err)
			require.Equal(t, want, points)
			response, err := client.PriceHistory(context.Background(), &types.QueryPriceHistoryRequest{Denom: market.Denom, ToHeight: ^uint64(0)})
			require.NoError(t, err)
			require.Equal(t, want, response.Points)
			snapshots := proofModuleView(t, evm, owner, pool, poolABI, "getPriceSnapshots").([types.SnapshotSlots]*big.Int)
			for i, point := range response.Points {
				require.Equal(t, point.Price.BigInt(), snapshots[i])
			}
			require.Equal(t, owner, proofModuleView(t, evm, owner, fresh.Generation2().SidioraNFT, nftABI, "ownerOf", new(big.Int).SetUint64(market.FeeRightID)))
			// Poison only the returned slice: the next call must re-read keeper state.
			response.Points[0].Height = 999
			points[0].Height = 998
		}
		response, err := client.PriceHistory(context.Background(), &types.QueryPriceHistoryRequest{Denom: "unknown-proof-market"})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrUnknownMarket)
		response, err = client.PriceHistory(context.Background(), &types.QueryPriceHistoryRequest{Denom: market.Denom, FromHeight: 2, ToHeight: 1})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrInvalidRange)

		pause, err := poolABI.Pack("pause")
		require.NoError(t, err)
		stranger := address("module proof stranger")
		refused, _, err := evm.Call(stranger, pool, pause, 30_000_000, uint256.NewInt(0))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, crypto.Keccak256([]byte("Unauthorized()"))[:4], refused)
		require.Equal(t, false, proofModuleView(t, evm, owner, pool, poolABI, "paused"))
		_, _, err = evm.Call(owner, pool, pause, 30_000_000, uint256.NewInt(0))
		require.NoError(t, err)
		for read := 0; read < 2; read++ {
			require.Equal(t, true, proofModuleView(t, evm, owner, pool, poolABI, "paused"))
		}
		stored, found := fresh.GetMarket(db.Ctx(), market.Denom)
		require.True(t, found)
		require.True(t, stored.Paused, "SurfaceRuntime must forward the guardian, not its own address")

		// The simulation consumer receives the real producer's stored bytes.
		decoders := sdk.StoreDecoderRegistry{}
		consumer.RegisterStoreDecoder(decoders)
		key := types.MarketKey(market.Denom)
		pair := kv.Pair{Key: key, Value: ctx.KVStore(sdk.NewKVStoreKey(types.StoreKey)).Get(key)}
		decoded := decoders[types.StoreKey](pair, pair)
		require.Contains(t, decoded, market.Denom)
		require.Contains(t, decoded, "Market:")
		pair.Value = pair.Value[:len(pair.Value)-1]
		require.Panics(t, func() { decoders[types.StoreKey](pair, pair) })

		// Refuse both malformed JSON and a well-formed, invalid producer export
		// before writing any state in the receiving keeper.
		untouched := consumer.ExportGenesis(ctx, app.AppCodec())
		invalid := imported[:len(imported)-1]
		require.Error(t, basic.ValidateGenesis(app.AppCodec(), nil, invalid))
		require.Panics(t, func() { consumer.InitGenesis(ctx, app.AppCodec(), invalid) })
		var broken types.GenesisState
		require.NoError(t, json.Unmarshal(imported, &broken))
		broken.Markets[0].SnapshotCount = types.SnapshotSlots + 1
		invalid, err = json.Marshal(broken)
		require.NoError(t, err)
		require.ErrorIs(t, basic.ValidateGenesis(app.AppCodec(), nil, invalid), types.ErrInvalidMarket)
		require.Panics(t, func() { consumer.InitGenesis(ctx, app.AppCodec(), invalid) })
		require.JSONEq(t, string(untouched), string(consumer.ExportGenesis(ctx, app.AppCodec())))
	}
}

func TestProofModuleKeeperCorruptProducerRefused(t *testing.T) {
	e := newSurfacedEnv(t)
	market := e.create(e.account(5_000_000), types.FeeStrategyClaim)
	e.at(mature)
	e.swap(e.account(100_000_000), market.Denom, true, 2_000_000)
	key := types.MarketKey(market.Denom)
	store := e.ctx.KVStore(e.key)
	original := append([]byte(nil), store.Get(key)...)
	client := proofModuleClient(t, e.k, e.ctx)
	var bad types.Market
	require.NoError(t, json.Unmarshal(original, &bad))
	bad.PriceSnapshots[0] = sdk.NewInt(-1)
	negative, err := json.Marshal(bad)
	require.NoError(t, err)
	// Mutate the real producer's wire value: sdk.Int{} would encode as "0".
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(original, &fields))
	var snapshots []json.RawMessage
	require.NoError(t, json.Unmarshal(fields["price_snapshots"], &snapshots))
	snapshots[0] = json.RawMessage("null")
	fields["price_snapshots"], err = json.Marshal(snapshots)
	require.NoError(t, err)
	missing, err := json.Marshal(fields)
	require.NoError(t, err)
	for _, corrupt := range [][]byte{original[:len(original)-1], negative, missing} {
		store.Set(key, corrupt)
		for read := 0; read < 2; read++ {
			response, err := client.PriceHistory(context.Background(), &types.QueryPriceHistoryRequest{Denom: market.Denom, ToHeight: ^uint64(0)})
			require.Nil(t, response)
			require.ErrorIs(t, err, types.ErrInvalidMarket)
		}
		require.Panics(t, func() { moduleimpl.NewAppModule(e.k).ExportGenesis(e.ctx, testkeeper.EVMTestApp.AppCodec()) })
	}
	store.Set(key, original)
	response, err := client.PriceHistory(context.Background(), &types.QueryPriceHistoryRequest{Denom: market.Denom, ToHeight: ^uint64(0)})
	require.NoError(t, err)
	require.Len(t, response.Points, 1)
	require.NoError(t, e.k.ExportGenesis(e.ctx).Validate())
}
