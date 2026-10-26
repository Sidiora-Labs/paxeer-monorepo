package keeper_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRequestFromProgramRecordsTheFeeDenom(t *testing.T) {
	s := newProgramSuite(t)
	for i, asset := range []programAsset{paxAsset, sidAsset, usdcAsset, usdlAsset} {
		t.Run(asset.denom, func(t *testing.T) {
			id := uint64(i*2 + 1)
			key := keeper.ProgramFeeDenomKey(asset.id)
			denom, found := s.k.ProgramFeeDenom(s.ctx, asset.id)
			require.False(t, found)
			require.Empty(t, denom)
			require.Nil(t, s.ctx.KVStore(s.key).Get(key))

			event := webRequest(id)
			err := s.k.RequestFromProgram(s.ctx, newCall(feeLeg(programPrincipal, asset, sdk.ZeroInt())),
				event.topic, event.data)
			require.EqualError(t, err, "LXP_FEE_UNPAYABLE")
			denom, found = s.k.ProgramFeeDenom(s.ctx, asset.id)
			require.False(t, found)
			require.Empty(t, denom)
			require.Nil(t, s.ctx.KVStore(s.key).Get(key))
			s.requireNoRecord(id)
			require.Equal(t, uint64(i*2), s.k.ProgramPendingCount(s.ctx))

			for offset := uint64(0); offset < 2; offset++ {
				call := newCall(feeLeg(programPrincipal, asset, sdk.NewInt(30)),
					feeLeg(programPrincipal, asset, sdk.NewInt(45)))
				require.NoError(t, s.run(call, webRequest(id+offset)))
				denom, found = s.k.ProgramFeeDenom(s.ctx, asset.id)
				require.True(t, found)
				require.Equal(t, asset.denom, denom)
				require.Equal(t, []byte(asset.denom), s.ctx.KVStore(s.key).Get(key))
				record, recorded, err := s.k.GetProgramPendingRecord(s.ctx, programP, id+offset)
				require.NoError(t, err)
				require.True(t, recorded)
				require.Equal(t, asset.id, record.FeeAsset)
				require.Equal(t, sdk.NewInt(75), record.Amount)
				require.Equal(t, id+offset, s.k.ProgramPendingCount(s.ctx))
			}
			require.Equal(t, sdk.NewInt(150), s.balanceOf(keeper.ProgramFeeAccount(asset.id), asset))
			require.Equal(t, programBalance.Sub(sdk.NewInt(150)), s.balanceOf(programPrincipal, asset))
		})
	}
}

func TestRequestFromProgramRefusesADenomChangeForAnAsset(t *testing.T) {
	s := newProgramSuite(t)
	snapshot := func() map[string][]byte {
		store := make(map[string][]byte)
		iterator := s.ctx.KVStore(s.key).Iterator(nil, nil)
		defer func() { require.NoError(t, iterator.Close()) }()
		for ; iterator.Valid(); iterator.Next() {
			store[string(iterator.Key())] = bytes.Clone(iterator.Value())
		}
		return store
	}
	assets := []programAsset{paxAsset, sidAsset, usdcAsset, usdlAsset}
	for i, asset := range assets {
		t.Run(asset.denom, func(t *testing.T) {
			id := uint64(i*2 + 1)
			paid := sdk.NewInt(75)
			require.NoError(t, s.run(newCall(feeLeg(programPrincipal, asset, paid)), webRequest(id)))
			before := snapshot()
			changed := programAsset{asset.id, assets[(i+1)%len(assets)].denom}
			call := newCall(feeLeg(programPrincipal, changed, paid))
			event := webRequest(id + 1)
			principalBalance := s.balanceOf(programPrincipal, changed)
			feeBalance := s.balanceOf(keeper.ProgramFeeAccount(asset.id), changed)

			require.EqualError(t, s.k.RequestFromProgram(s.ctx, call, event.topic, event.data), "LXP_ASSET_MISMATCH")
			require.Equal(t, before, snapshot(), "a refusal must not write to the xweb store")
			require.EqualError(t, s.run(call, event), "LXP_ASSET_MISMATCH")
			require.Equal(t, before, snapshot())
			require.Equal(t, principalBalance, s.balanceOf(programPrincipal, changed))
			require.Equal(t, feeBalance, s.balanceOf(keeper.ProgramFeeAccount(asset.id), changed))
			s.requireNoRecord(id + 1)
			require.Equal(t, id, s.k.ProgramPendingCount(s.ctx))
			denom, found := s.k.ProgramFeeDenom(s.ctx, asset.id)
			require.True(t, found)
			require.Equal(t, asset.denom, denom)

			call.Legs[0].Denom = asset.denom
			require.NoError(t, s.run(call, event), "a refusal must not mark the call as requested")
			require.Equal(t, id+1, s.k.ProgramPendingCount(s.ctx))
		})
	}
}
