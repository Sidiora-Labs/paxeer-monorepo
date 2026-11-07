package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerKeeperExecutesThroughHost(t *testing.T) {
	f, actor, args := newAssetKeeperFixture(t)
	before := ledgerViewStore(t, f.ctx, f.hostKey)
	ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
	next, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	identitySequence, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)

	result, err := f.keeper.RegisterAsset(f.ctx, actor, args, [32]byte{0x95, 1})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
	require.Equal(t, next, result.Receipt.GlobalSequence)
	require.NotEmpty(t, result.Receipt.Encoded)
	decoded, err := codec.DecodeReceipt(result.Receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, result.Receipt.Encoded, decoded.CanonicalBytes())
	require.Equal(t, int32(result.Receipt.ResultCode), decoded.ResultCode)
	require.Equal(t, next, decoded.GlobalSequence)
	require.Equal(t, result.Receipt.PreviousStateRoot, decoded.PreviousStateRoot)
	require.Equal(t, result.Receipt.ResultingStateRoot, decoded.ResultingStateRoot)
	require.NotEqual(t, result.Receipt.PreviousStateRoot, result.Receipt.ResultingStateRoot)
	require.Equal(t, result.Receipt.Events, result.Events)
	require.Contains(t, result.Events, lxphost.ReceiptEvent{Code: 1, Body: args.AssetID[:]})

	after, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	require.Equal(t, next+1, after)
	afterIdentity, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)
	require.Equal(t, identitySequence+1, afterIdentity)
	require.NotEqual(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Empty(t, ledgerBefore)
	require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
	asset, err := f.host.ReadAsset(f.ctx, args.AssetID)
	require.NoError(t, err)
	require.Equal(t, args.AssetID, asset.AssetID)
	require.Equal(t, actor.Authority.Actor, asset.IssuerDID)
	require.Equal(t, args.SupplyCap, asset.SupplyCap)

	paused := accountActivity(t, f.harness, actor, 0x00010002,
		types.EncodePausePayload(args.AssetID), [32]byte{0x95, 2})
	require.Equal(t, after, paused.GlobalSequence)
	require.Equal(t, result.Receipt.ResultingStateRoot, paused.PreviousStateRoot)
	resumed, err := f.keeper.UnpauseAsset(f.ctx, actor, args.AssetID, [32]byte{0x95, 3})
	require.NoError(t, err)
	require.Equal(t, after+1, resumed.Receipt.GlobalSequence)
	require.Equal(t, paused.ResultingStateRoot, resumed.Receipt.PreviousStateRoot)
	after, err = lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	require.Equal(t, next+3, after)
	require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))

	pausedResult, err := f.keeper.PauseAsset(f.ctx, actor, args.AssetID, [32]byte{0x95, 4})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), pausedResult.Receipt.ResultCode)
	require.Equal(t, after, pausedResult.Receipt.GlobalSequence)
	require.Equal(t, resumed.Receipt.ResultingStateRoot, pausedResult.Receipt.PreviousStateRoot)
	require.NotEqual(t, pausedResult.Receipt.PreviousStateRoot, pausedResult.Receipt.ResultingStateRoot)
	require.Equal(t, pausedResult.Receipt.Events, pausedResult.Events)
	require.Contains(t, pausedResult.Events, lxphost.ReceiptEvent{Code: 2, Body: args.AssetID[:]})
	asset, err = f.host.ReadAsset(f.ctx, args.AssetID)
	require.NoError(t, err)
	require.True(t, asset.Paused)
	finalSequence, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	require.Equal(t, after+1, finalSequence)
	finalIdentity, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)
	require.Equal(t, identitySequence+4, finalIdentity)
	require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
}

func TestLedgerKeeperRefusedActivityLeavesHostUnchanged(t *testing.T) {
	for _, name := range []string{"malformed_payload", "kernel_asset_id_mismatch"} {
		t.Run(name, func(t *testing.T) {
			f, actor, args := newAssetKeeperFixture(t)
			_, err := f.keeper.RegisterAsset(f.ctx, actor, args, [32]byte{0x96, 1})
			require.NoError(t, err)
			original, err := f.host.ReadAsset(f.ctx, args.AssetID)
			require.NoError(t, err)
			if name == "malformed_payload" {
				args.Symbol = ""
			} else {
				args.AssetID[0] ^= 0xff
				payload, err := types.EncodeRegisterPayload(args)
				require.NoError(t, err)
				require.NoError(t, types.ValidateRegisterPayload(payload))
			}
			before := ledgerViewStore(t, f.ctx, f.hostKey)
			ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
			next, err := lxphost.NextSequence(f.ctx, f.hostKey)
			require.NoError(t, err)
			identitySequence, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
			require.NoError(t, err)

			result, err := f.keeper.RegisterAsset(f.ctx, actor, args, [32]byte{0x96, 2})
			if name == "malformed_payload" {
				require.ErrorIs(t, err, types.ErrNonCanonical)
			} else {
				require.ErrorIs(t, err, types.ErrAssetMismatch)
				require.Equal(t, lxphost.Code(-402), result.Receipt.ResultCode)
				require.NotEmpty(t, result.Receipt.Encoded)
				require.Equal(t, next, result.Receipt.GlobalSequence)
				decoded, decodeErr := codec.DecodeReceipt(result.Receipt.Encoded)
				require.NoError(t, decodeErr)
				require.Equal(t, result.Receipt.Encoded, decoded.CanonicalBytes())
				require.Equal(t, int32(-402), decoded.ResultCode)
				require.Equal(t, next, decoded.GlobalSequence)
				_, readErr := f.host.ReadAsset(f.ctx, args.AssetID)
				require.Error(t, readErr)
			}
			after, err := lxphost.NextSequence(f.ctx, f.hostKey)
			require.NoError(t, err)
			afterIdentity, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
			require.NoError(t, err)
			hostAfter := ledgerViewStore(t, f.ctx, f.hostKey)
			if name == "malformed_payload" {
				assert.Equal(t, next, after, "malformed input must leave the shared host sequence unchanged")
				assert.Equal(t, identitySequence, afterIdentity)
				assert.Equal(t, before, hostAfter, "malformed input must leave every host key and value unchanged")
			} else {
				assert.Equal(t, next+1, after, "a receipted kernel refusal must advance the shared host sequence")
				assert.Equal(t, identitySequence+1, afterIdentity)
				actorID, err := lxphost.DIDID(actor.DID)
				require.NoError(t, err)
				for _, key := range []string{"\x02", "\x03" + string(actorID[:])} {
					require.Contains(t, before, key)
					require.Contains(t, hostAfter, key)
					delete(before, key)
					delete(hostAfter, key)
				}
				assert.Equal(t, before, hostAfter, "a kernel refusal must leave every host key and value except the two sequence counters unchanged")
			}
			assert.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
			asset, err := f.host.ReadAsset(f.ctx, original.AssetID)
			require.NoError(t, err)
			assert.Equal(t, original, asset)
		})
	}
}
