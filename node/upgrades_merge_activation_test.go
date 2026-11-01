package app_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"math"
	"testing"
	"time"

	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func mergePrecompileGate() []common.Address {
	return []common.Address{
		common.HexToAddress("0x101A"),
		common.HexToAddress("0x101B"),
		common.HexToAddress("0x101C"),
		common.HexToAddress("0x101D"),
		common.HexToAddress("0x0901"),
		common.HexToAddress("0x0902"),
		common.HexToAddress("0x0903"),
		common.HexToAddress("0x0904"),
		common.HexToAddress("0x0905"),
		common.HexToAddress("0x0906"),
		common.HexToAddress("0x0907"),
		common.HexToAddress("0x0908"),
	}
}

func TestMergePlanActivatesGatedPrecompiles(t *testing.T) {
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	const height = int64(42)
	ctx = ctx.WithBlockHeight(height)
	require.True(t, a.UpgradeKeeper.HasHandler(app.MergeUpgrade))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.ActivationUpgrade)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(2), app.V6120Upgrade)

	store := ctx.KVStore(a.GetKey(keys.SequencerStoreKey))
	iter := store.Iterator(nil, nil)
	var stale [][]byte
	for ; iter.Valid(); iter.Next() {
		stale = append(stale, bytes.Clone(iter.Key()))
	}
	require.NoError(t, iter.Close())
	for _, key := range stale {
		store.Delete(key)
	}
	ctx.KVStore(a.GetKey(paramstypes.StoreKey)).Delete(append([]byte(seqtypes.ModuleName+"/"), seqkeeper.KeyReceiptKeys...))
	versions := ctx.KVStore(a.GetKey(upgradetypes.StoreKey))
	for _, name := range []string{
		seqtypes.ModuleName, ledgertypes.ModuleName, paytypes.ModuleName,
		authtypes.ModuleName, agenttypes.ModuleName, "layerxprograms",
	} {
		versions.Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
		require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), name)
	}
	a.RefreshPrecompiles(ctx)
	for _, tracing := range []bool{false, true} {
		before := ctx.WithIsTracing(tracing)
		served := a.EvmKeeper.CustomPrecompiles(before)
		servedVersions := a.EvmKeeper.GetCustomPrecompilesVersions(before)
		for _, address := range mergePrecompileGate() {
			require.NotContains(t, served, address, "before the merge: %s, tracing=%t", address, tracing)
			require.NotContains(t, servedVersions, address, "before the merge: %s, tracing=%t", address, tracing)
		}
	}

	primary := bytes.Repeat([]byte{0x51}, 32)
	standby := bytes.Repeat([]byte{0x52}, 32)
	publicKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	info, err := json.Marshal(map[string]any{
		"leaders": [][]byte{primary, standby},
		"receipt_keys": seqkeeper.ReceiptKeys{{
			KeyId:       bytes.Repeat([]byte{0x44}, seqtypes.KeyIDLength),
			PublicKey:   publicKey,
			FirstHeight: uint64(height),
			LastHeight:  math.MaxUint64,
		}},
	})
	require.NoError(t, err)
	plan := upgradetypes.Plan{Name: app.MergeUpgrade, Height: height, Info: string(info)}
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	require.NotPanics(t, func() { a.UpgradeKeeper.ApplyUpgrade(ctx, plan) })
	require.Equal(t, height, a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	a.RefreshPrecompiles(ctx)

	for _, tracing := range []bool{false, true} {
		for _, blockHeight := range []int64{height - 1, height, height + 1} {
			at := ctx.WithBlockHeight(blockHeight).WithIsTracing(tracing)
			served := a.EvmKeeper.CustomPrecompiles(at)
			servedVersions := a.EvmKeeper.GetCustomPrecompilesVersions(at)
			for _, address := range mergePrecompileGate() {
				if blockHeight < height {
					require.NotContains(t, served, address, "below merge height: %s, tracing=%t", address, tracing)
					require.NotContains(t, servedVersions, address, "below merge height: %s, tracing=%t", address, tracing)
				} else {
					require.Contains(t, served, address, "after the merge: %s, height=%d, tracing=%t", address, blockHeight, tracing)
					require.NotNil(t, served[address], "served precompile at %s", address)
					require.Equal(t, app.MergeUpgrade, servedVersions[address], "served version at %s, height=%d, tracing=%t", address, blockHeight, tracing)
				}
			}
		}
	}
}
