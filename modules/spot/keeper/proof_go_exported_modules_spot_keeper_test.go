// Proves: keeper.Keeper.InitGenesis/ActiveLimitOrderID/ActiveStopLimitOrderIDs/ActiveStopLossOrderIDs/GetActiveLimitOrderCount/GetActiveStopLimitOrderCount/GetActiveStopLossOrderCount/ExecuteResting -> exported Go methods and SDK stores -> spot.AppModule, node.App's served spot.Precompile ABI rails and spot.EndBlocker.
//
// Real order placement produces the genesis imported by AppModule. The spot
// database is committed, closed and reopened before the application's existing
// keeper and precompiles consume it. Bank/EVM dependencies remain hosted by the
// in-process application; no transport, keeper or consumer is replaced.
// The proof repairs modules/spot/keeper/orders.go:activeAt accepting zero IDs
// and inconsistent forward/reverse positions, which previously escaped through
// both orders.go and stoporders.go ABI views as successful uint256 results.
package keeper_test

import (
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	spotprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestProofGoExportedModulesSpotKeeper(t *testing.T) {
	f := newStopFixture(t)
	f.tokenCall("approve(address,uint256)", common.HexToAddress(types.PECOROrdersAddress), big.NewInt(1_000_000_000_000))
	var limits, stopLimits, stopLosses []uint64
	for i := 0; i < 2; i++ {
		expiry := uint64(0)
		if i == 0 {
			expiry = uint64(f.ctx.BlockTime().Unix()) + 1
		}
		id, _, err := f.k.PlaceLimit(f.ctx, f.user, f.stable, f.base, sdk.NewInt(100_000_000), stopTestPrice(60), expiry, types.OrderTypeLimitSell)
		require.NoError(t, err)
		limits = append(limits, id)
		id, err = f.k.PlaceStopLimit(f.ctx, f.user, f.stable, f.base, sdk.NewInt(100_000_000), stopTestPrice(55), stopTestPrice(60), 0, types.OrderTypeStopLimitBuy)
		require.NoError(t, err)
		stopLimits = append(stopLimits, id)
		id, err = f.k.PlaceStopLoss(f.ctx, f.user, f.base, f.stable, sdk.NewInt(100_000_000), stopTestPrice(40), 0)
		require.NoError(t, err)
		stopLosses = append(stopLosses, id)
	}
	source := f.k.ExportGenesis(f.ctx)
	require.NoError(t, source.Validate())
	encoded, err := json.Marshal(source)
	require.NoError(t, err)
	app := testkeeper.EVMTestApp
	k := app.SpotKeeper
	require.NotNil(t, k, "node/app.go must wire SpotKeeper")
	key := app.GetKey(types.StoreKey)
	require.NotNil(t, key)
	baseCtx := f.ctx
	user := f.user
	price := append([]byte(nil), f.ctx.KVStore(f.feedKey).Get(pricefeedtypes.AggregateKey(11))...)
	require.NotEmpty(t, price)
	// Keep the application's actual bank, auth and EVM stores. Only its spot
	// store is mounted on a disk-backed SDK root store for the restart proof.
	parents := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keys := make(map[string]storetypes.StoreKey)
	for _, appKey := range baseCtx.MultiStore().StoreKeys() {
		if appKey.Name() != types.StoreKey {
			parents[appKey] = baseCtx.MultiStore().GetKVStore(appKey)
			keys[appKey.Name()] = appKey
		}
	}
	metadata := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, metadata.Close()) })
	directory := t.TempDir()
	root, diskKey, closeStore := proofSpotOpenStore(t, directory)
	parents[key], keys[key.Name()] = root.GetKVStore(diskKey), key
	multi := cachemulti.NewStore(metadata, parents, keys, nil, nil, nil, 0)
	ctx := baseCtx.WithMultiStore(multi).WithEventManager(sdk.NewEventManager())
	module := spot.NewAppModule(k)
	require.NoError(t, module.ValidateGenesis(nil, nil, encoded))
	require.Empty(t, module.InitGenesis(ctx, nil, encoded))
	require.Equal(t, source, k.ExportGenesis(ctx))
	// The application's pricefeed keeper uses its own registered key.
	ctx.KVStore(app.GetKey(pricefeedtypes.StoreKey)).Set(pricefeedtypes.AggregateKey(11), price)
	beforeRestart := proofSpotSnapshot(t, ctx.KVStore(key))
	multi.Write()
	root.Commit(true)
	closeStore()
	delete(parents, key)
	ctx, f = sdk.Context{}, nil
	multi = cachemulti.Store{}

	root, diskKey, closeStore = proofSpotOpenStore(t, directory)
	parents[key] = root.GetKVStore(diskKey)
	multi = cachemulti.NewStore(metadata, parents, keys, nil, nil, nil, 0)
	ctx = baseCtx.WithMultiStore(multi).WithEventManager(sdk.NewEventManager()).WithEvmEventManager(sdk.NewEVMEventManager())
	require.Equal(t, beforeRestart, proofSpotSnapshot(t, ctx.KVStore(key)))
	for repeat := 0; repeat < 2; repeat++ {
		require.JSONEq(t, string(encoded), string(module.ExportGenesis(ctx, nil)))
	}

	contracts := app.EvmKeeper.CustomPrecompiles(ctx)
	call := func(callCtx sdk.Context, address common.Address, method string, args ...interface{}) ([]byte, error) {
		t.Helper()
		contract, ok := contracts[address].(*spotprecompile.Precompile)
		require.True(t, ok, "node/app.go must serve the real spot ABI rail at %s", address)
		input, err := contract.ABI.Pack(method, args...)
		require.NoError(t, err)
		var output []byte
		var callErr error
		require.NoError(t, app.EvmKeeper.RunWithOneOffEVMInstance(callCtx, func(evm *vm.EVM) error {
			var left uint64
			output, left, callErr = contract.RunAndCalculateGas(evm, user, address, input, 1_000_000, new(big.Int), nil, true, false)
			require.Greater(t, left, uint64(0))
			require.Less(t, left, uint64(1_000_000))
			return nil
		}, func(string, string) {}))
		return output, callErr
	}
	read := func(callCtx sdk.Context, address common.Address, method string, want uint64, args ...interface{}) {
		t.Helper()
		output, err := call(callCtx, address, method, args...)
		require.NoError(t, err, method)
		contract := contracts[address].(*spotprecompile.Precompile)
		decoded, err := contract.ABI.Methods[method].Outputs.Unpack(output)
		require.NoError(t, err)
		require.Equal(t, []interface{}{new(big.Int).SetUint64(want)}, decoded, method)
	}
	refused := func(callCtx sdk.Context, address common.Address, method, reason string, args ...interface{}) {
		t.Helper()
		output, err := call(callCtx, address, method, args...)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		decoded, err := abi.UnpackRevert(output)
		require.NoError(t, err)
		require.Contains(t, decoded, reason)
	}
	lists := []struct {
		name, countMethod, idMethod string
		address                     common.Address
		prefix                      []byte
		ids                         []uint64
		count                       func(*keeper.Keeper, sdk.Context) (uint64, error)
		at                          func(*keeper.Keeper, sdk.Context, uint64) (uint64, error)
	}{
		{"limit", "getActiveLimitOrderCount", "activeLimitOrderIds", common.HexToAddress(types.PECOROrdersAddress), types.ActiveLimitPrefix, limits, (*keeper.Keeper).GetActiveLimitOrderCount, (*keeper.Keeper).ActiveLimitOrderID},
		{"stop limit", "getActiveStopLimitOrderCount", "activeStopLimitOrderIds", common.HexToAddress(types.PECORStopOrdersAddress), types.ActiveStopLimitPrefix, stopLimits, (*keeper.Keeper).GetActiveStopLimitOrderCount, (*keeper.Keeper).ActiveStopLimitOrderIDs},
		{"stop loss", "getActiveStopLossOrderCount", "activeStopLossOrderIds", common.HexToAddress(types.PECORStopOrdersAddress), types.ActiveStopLossPrefix, stopLosses, (*keeper.Keeper).GetActiveStopLossOrderCount, (*keeper.Keeper).ActiveStopLossOrderIDs},
	}
	for _, list := range lists {
		t.Run(list.name, func(t *testing.T) {
			for repeat := 0; repeat < 2; repeat++ {
				count, err := list.count(k, ctx)
				require.NoError(t, err)
				require.Equal(t, uint64(2), count)
				read(ctx, list.address, list.countMethod, 2)
				for pos, want := range list.ids {
					id, err := list.at(k, ctx, uint64(pos))
					require.NoError(t, err)
					require.Equal(t, want, id)
					read(ctx, list.address, list.idMethod, want, big.NewInt(int64(pos)))
				}
			}
			refused(ctx, list.address, list.idMethod, "out of bounds", big.NewInt(2))
			refused(ctx, list.address, list.idMethod, "out of bounds", new(big.Int).Lsh(big.NewInt(1), 64))
			for _, damage := range []string{"short position", "zero id", "wrong reverse", "missing reverse", "short count"} {
				t.Run(damage, func(t *testing.T) {
					bad, _ := ctx.CacheContext()
					store := bad.KVStore(key)
					positionKey := types.ActivePositionKey(list.prefix, 0)
					reverseKey := stopTestKey(list.prefix, list.ids[0])
					method, args, reason := list.idMethod, []interface{}{big.NewInt(0)}, "corrupt active index"
					switch damage {
					case "short position":
						store.Set(positionKey, []byte{1})
					case "zero id":
						store.Set(positionKey, make([]byte, 8))
						reason = "zero order id"
					case "wrong reverse":
						store.Set(reverseKey, binary.BigEndian.AppendUint64(nil, 1))
					case "missing reverse":
						store.Delete(reverseKey)
					case "short count":
						store.Set(types.ActiveLengthKey(list.prefix), []byte{1})
						method, args, reason = list.countMethod, nil, "invalid integer"
					}
					before := proofSpotSnapshot(t, store)
					for repeat := 0; repeat < 2; repeat++ {
						refused(bad, list.address, method, reason, args...)
					}
					require.Equal(t, before, proofSpotSnapshot(t, store))
					read(ctx, list.address, list.idMethod, list.ids[0], big.NewInt(0))
				})
			}
		})
	}

	// ExecuteResting produces real expiry/refund state; EndBlocker consumes
	// its counters as events, and the same ABI rail observes the changed list.
	ctx = ctx.WithBlockTime(time.Unix(1002, 0).UTC())
	trial, _ := ctx.CacheContext()
	work, err := k.ExecuteResting(trial)
	require.NoError(t, err)
	require.Equal(t, uint64(1), work.Samples)
	require.Equal(t, uint64(1), work.Expired)
	require.Zero(t, work.Activated)
	require.Zero(t, work.Filled)
	read(ctx, lists[0].address, lists[0].countMethod, 2)
	for repeat := 0; repeat < 2; repeat++ {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		require.NoError(t, spot.EndBlocker(ctx, k))
		expired := "1"
		if repeat == 1 {
			expired = "0"
		}
		require.Contains(t, ctx.EventManager().Events(), sdk.NewEvent(types.EventTypeResting,
			sdk.NewAttribute(types.AttributeKeySamples, "1"),
			sdk.NewAttribute(types.AttributeKeyExpired, expired),
			sdk.NewAttribute(types.AttributeKeyActivated, "0"),
			sdk.NewAttribute(types.AttributeKeyFilled, "0")))
		read(ctx, lists[0].address, lists[0].countMethod, 1)
		read(ctx, lists[0].address, lists[0].idMethod, limits[1], big.NewInt(0))
		order, err := k.GetLimitOrder(ctx, limits[0])
		require.NoError(t, err)
		require.Equal(t, types.OrderStatusExpired, order.Status)
	}
	bad, _ := ctx.CacheContext()
	bad = bad.WithEventManager(sdk.NewEventManager())
	bad.KVStore(key).Set(append(append([]byte(nil), types.CursorPrefix...), byte(1)), []byte{1})
	before := proofSpotSnapshot(t, bad.KVStore(key))
	require.EqualError(t, spot.EndBlocker(bad, k), "spot: invalid fill cursor")
	require.Equal(t, sdk.Events{sdk.NewEvent(types.EventTypeRestingFailed, sdk.NewAttribute(types.AttributeKeyError, "spot: invalid fill cursor"))}, bad.EventManager().Events())
	require.Equal(t, before, proofSpotSnapshot(t, bad.KVStore(key)))

	invalid := *source
	invalid.NextOrderID = 0
	invalidJSON, err := json.Marshal(invalid)
	require.NoError(t, err)
	before = proofSpotSnapshot(t, ctx.KVStore(key))
	require.Panics(t, func() { module.InitGenesis(ctx, nil, invalidJSON) })
	require.Panics(t, func() { module.InitGenesis(ctx, nil, json.RawMessage(`{`)) })
	require.Equal(t, before, proofSpotSnapshot(t, ctx.KVStore(key)))
	multi.Write()
	root.Commit(true)
	closeStore()
}
