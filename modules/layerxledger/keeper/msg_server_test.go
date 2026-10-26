package keeper

import (
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

func newLedgerMsgKeeper(t *testing.T) (*Keeper, sdk.Context) {
	t.Helper()
	db := tmdb.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	stores := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	stores.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmtypes.Header{}, false)
	cdc := codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	authority := authtypes.NewModuleAddress("ledger-admin").String()
	return NewKeeper(cdc, key, nil, nil, nil, authority), ctx
}

func TestLedgerUpdateParamsAuthority(t *testing.T) {
	k, ctx := newLedgerMsgKeeper(t)
	server := NewMsgServerImpl(k)
	goCtx := sdk.WrapSDKContext(ctx)
	params := types.Params{Paused: true}

	for _, authority := range []string{"", govtypes.GovernanceAuthority(), authtypes.NewModuleAddress("other-admin").String()} {
		response, err := server.UpdateParams(goCtx, &types.MsgUpdateParams{Authority: authority, Params: params})
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Nil(t, response)
		require.Equal(t, types.DefaultParams(), k.GetParams(ctx))
		require.False(t, k.store(ctx).Has(types.ParamsKey), "unauthorized updates must not write params")
	}

	for _, paused := range []bool{true, false} {
		params := types.Params{Paused: paused}
		response, err := server.UpdateParams(goCtx, &types.MsgUpdateParams{Authority: k.authority, Params: params})
		require.NoError(t, err)
		require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
		require.Equal(t, params, k.GetParams(ctx))
		require.Equal(t, k.cdc.MustMarshal(&params), k.store(ctx).Get(types.ParamsKey))
	}

	response, err := server.UpdateParams(goCtx, nil)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	require.Nil(t, response)
	require.Equal(t, types.Params{Paused: false}, k.GetParams(ctx))
}

func TestLedgerUpdateParamsPauses(t *testing.T) {
	k, ctx := newLedgerMsgKeeper(t)
	server := NewMsgServerImpl(k)
	goCtx := sdk.WrapSDKContext(ctx)
	params := types.Params{Paused: true}
	response, err := server.UpdateParams(goCtx, &types.MsgUpdateParams{Authority: k.authority, Params: params})
	require.NoError(t, err)
	require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
	require.Equal(t, params, k.GetParams(ctx))

	guard, ok := interface{}(k).(interface {
		checkPause(sdk.Context, string) error
	})
	require.True(t, ok, "Keeper.checkPause from task 2.3.3 must be available to enforce stored pause params")
	require.ErrorIs(t, guard.checkPause(ctx, "mint"), govtypes.ErrPausedScope)

	response, err = server.UpdateParams(goCtx, &types.MsgUpdateParams{Authority: k.authority, Params: types.Params{Paused: false}})
	require.NoError(t, err)
	require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
	require.False(t, k.GetParams(ctx).Paused)
	require.NoError(t, guard.checkPause(ctx, "mint"))
}
