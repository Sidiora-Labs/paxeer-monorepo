package layerxgov_test

import (
	"bytes"
	"testing"

	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func NewProposalHandler(t *testing.T, testApp *app.App) sdkgovtypes.Handler {
	t.Helper()
	router := testApp.GovKeeper.Router()
	require.True(t, router.HasRoute(govtypes.RouterKey))
	require.NotNil(t, testApp.MsgServiceRouter().HandlerByTypeURL(sdk.MsgTypeURL(&paytypes.MsgUpdateParams{})))
	return router.GetRoute(govtypes.RouterKey)
}

func TestLayerXProposalPausesPayLive(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, testApp.Close()) })
	handler := NewProposalHandler(t, testApp)
	k := testApp.LayerXPayKeeper
	require.Equal(t, govtypes.GovernanceAuthority(), k.Authority())
	ctx := testApp.GetContextForDeliverTx(nil)
	unpaused := paytypes.DefaultParams()
	unpaused.Paused = false
	paused := unpaused
	paused.Paused = true

	proposal := func(t *testing.T, msgs ...sdk.Msg) *govtypes.LayerXProposal {
		t.Helper()
		p, err := govtypes.NewLayerXProposal("Pause LayerX pay", "Apply pay parameters through the app governance route", msgs...)
		require.NoError(t, err)
		require.Equal(t, govtypes.RouterKey, p.ProposalRoute())
		return p
	}
	update := func(params paytypes.Params) *paytypes.MsgUpdateParams {
		return &paytypes.MsgUpdateParams{Authority: govtypes.GovernanceAuthority(), Params: params}
	}

	t.Run("commits_pause_when_every_message_succeeds", func(t *testing.T) {
		at := ctx.WithEventManager(sdk.NewEventManager())
		k.SetParams(at, unpaused)
		require.Equal(t, unpaused, k.GetParams(at))
		require.False(t, k.GetParams(at).Paused)
		p := proposal(t, update(unpaused), update(paused))
		require.NoError(t, p.ValidateBasic())

		require.NoError(t, handler(at, p))
		require.True(t, k.GetParams(at).Paused)
		require.Equal(t, paused, k.GetParams(at))
	})

	t.Run("rejects_foreign_authority_without_writes", func(t *testing.T) {
		at := ctx.WithEventManager(sdk.NewEventManager())
		k.SetParams(at, unpaused)
		before := k.GetParams(at)
		require.False(t, before.Paused)
		eventsBefore := at.EventManager().Events()
		foreign := update(paused)
		foreign.Authority = sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
		require.NotEqual(t, k.Authority(), foreign.Authority)
		require.NoError(t, foreign.ValidateBasic())
		p := proposal(t, update(paused), foreign)
		require.ErrorIs(t, p.ValidateBasic(), sdkgovtypes.ErrInvalidProposalContent)

		require.ErrorIs(t, handler(at, p), sdkgovtypes.ErrInvalidProposalContent)
		require.Equal(t, before, k.GetParams(at))
		require.Equal(t, eventsBefore, at.EventManager().Events())
	})

	t.Run("rolls_back_pause_when_a_later_message_fails", func(t *testing.T) {
		at := ctx.WithEventManager(sdk.NewEventManager())
		k.SetParams(at, unpaused)
		before := k.GetParams(at)
		require.False(t, before.Paused)
		eventsBefore := at.EventManager().Events()
		prefix := proposal(t, update(paused))
		require.NoError(t, prefix.ValidateBasic())
		probe, _ := at.CacheContext()
		require.NoError(t, handler(probe, prefix))
		require.Equal(t, paused, k.GetParams(probe))
		require.Equal(t, before, k.GetParams(at))

		gated := unpaused
		gated.EscrowEnabled = true
		p := proposal(t, update(paused), update(gated))
		require.NoError(t, p.ValidateBasic())

		err := handler(at, p)
		require.ErrorIs(t, err, paytypes.ErrKernelGateMismatch)
		require.Contains(t, err.Error(), "escrow")
		require.False(t, k.GetParams(at).Paused)
		require.Equal(t, before, k.GetParams(at))
		require.Equal(t, eventsBefore, at.EventManager().Events())
	})
}
