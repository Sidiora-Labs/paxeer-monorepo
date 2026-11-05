package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func sendAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt("paxcosmos_x_bank_keeper", "last_send_amount")
}

func TestMsgSendRecordsLastSendAmount(t *testing.T) {
	reader := oteltest.Install()
	suite := new(IntegrationTestSuite)
	suite.SetT(t)
	suite.SetupTest()
	_, bankKeeper := suite.initKeepersWithmAccPerms(make(map[string]bool))
	ctx := suite.ctx
	from := sdk.AccAddress("send_from___________")
	to := sdk.AccAddress("send_to_____________")
	amount := sdk.NewInt64Coin("uhpx", 7)
	coins := sdk.NewCoins(amount)
	require.NoError(t, bankKeeper.MintCoins(ctx, authtypes.Minter, coins))
	require.NoError(t, bankKeeper.SendCoinsFromModuleToAccount(ctx, authtypes.Minter, from, coins))
	require.Equal(t, amount, bankKeeper.GetBalance(ctx, from, "uhpx"))
	require.True(t, bankKeeper.GetBalance(ctx, to, "uhpx").IsZero())

	before, _ := sendAmount(reader)
	response, err := keeper.NewMsgServerImpl(bankKeeper).Send(sdk.WrapSDKContext(ctx), &types.MsgSend{
		FromAddress: from.String(),
		ToAddress:   to.String(),
		Amount:      coins,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	after, found := sendAmount(reader)
	require.True(t, found, "MsgServer.Send must record last_send_amount")
	require.Equal(t, int64(7), after)
	require.Equal(t, int64(7), after-before)
	require.True(t, bankKeeper.GetBalance(ctx, from, "uhpx").IsZero())
	require.Equal(t, amount, bankKeeper.GetBalance(ctx, to, "uhpx"))
}
