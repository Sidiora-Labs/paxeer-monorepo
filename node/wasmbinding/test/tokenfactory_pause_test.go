package wasmbinding

import (
	"encoding/json"
	"fmt"
	"testing"

	tokenfactorykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	wasmvmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWasmTokenFactoryDenomPausedQuery(t *testing.T) {
	testWrapper, customQuerier := SetupWasmbindingTest(t)
	keeper := testWrapper.App.TokenFactoryKeeper
	denom, err := keeper.CreateDenom(testWrapper.Ctx, app.TestUser, "pause")
	require.NoError(t, err)
	otherDenom, err := keeper.CreateDenom(testWrapper.Ctx, app.TestUser, "other")
	require.NoError(t, err)
	msgServer := tokenfactorykeeper.NewMsgServerImpl(keeper)

	queryPaused := func(denom string, paused bool) {
		t.Helper()
		rawQuery := json.RawMessage(fmt.Sprintf(`{"route":"tokenfactory","query_data":{"denom_paused":{"denom":%q}}}`, denom))
		res, err := customQuerier(testWrapper.Ctx, rawQuery)
		require.NoError(t, err)
		var response tokenfactorytypes.QueryDenomPausedResponse
		require.NoError(t, json.Unmarshal(res, &response))
		require.Equal(t, paused, response.Paused)
	}

	queryPaused(denom, false)
	for _, paused := range []bool{true, false} {
		_, err := msgServer.SetDenomPaused(sdk.WrapSDKContext(testWrapper.Ctx), tokenfactorytypes.NewMsgSetDenomPaused(app.TestUser, denom, paused))
		require.NoError(t, err)
		queryPaused(denom, paused)
		queryPaused(otherDenom, false)
	}

	res, err := customQuerier(testWrapper.Ctx, json.RawMessage(`{"route":"tokenfactory","query_data":{"denom_paused":{"denom":""}}}`))
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, res)
}

func TestWasmEncodeSetDenomPaused(t *testing.T) {
	sender, err := sdk.AccAddressFromBech32(TEST_CREATOR)
	require.NoError(t, err)
	denom := "factory/" + sender.String() + "/pause"

	for _, tc := range []struct {
		name   string
		paused bool
	}{
		{name: "pause", paused: true},
		{name: "unpause", paused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rawMsg := json.RawMessage(fmt.Sprintf(`{"set_denom_paused":{"denom":%q,"paused":%t,"sender":%q}}`, denom, tc.paused, TEST_TARGET_CONTRACT))
			msgs, err := wasmbinding.CustomEncoder(sender, rawMsg, wasmvmtypes.MessageInfo{}, wasmtypes.CodeInfo{})
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			msg, ok := msgs[0].(*tokenfactorytypes.MsgSetDenomPaused)
			require.True(t, ok)
			require.Equal(t, tokenfactorytypes.NewMsgSetDenomPaused(sender.String(), denom, tc.paused), msg)
			require.NoError(t, msg.ValidateBasic())
		})
	}

	msgs, err := wasmbinding.CustomEncoder(sender, json.RawMessage(`{"set_denom_paused":{"denom":"factory/invalid/pause","paused":"true"}}`), wasmvmtypes.MessageInfo{}, wasmtypes.CodeInfo{})
	require.ErrorIs(t, err, tokenfactorytypes.ErrEncodeTokenFactorySetDenomPaused)
	require.Empty(t, msgs)
}
