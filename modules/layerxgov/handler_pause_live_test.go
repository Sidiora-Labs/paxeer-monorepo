package layerxgov_test

import (
	"testing"
	"time"

	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

const ErrPausedScope uint32 = 735

func CheckPauseScope(t *testing.T, module, method string, paused bool) {
	t.Helper()
	err := govtypes.CheckPauseScope(module, paused, method)
	if paused {
		require.ErrorIs(t, err, govtypes.ErrPausedScope)
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, govtypes.ModuleName, codespace)
		require.Equal(t, ErrPausedScope, code)
		require.Contains(t, err.Error(), module+"."+method)
	} else {
		require.NoError(t, err)
	}
	require.NoError(t, govtypes.CheckPauseScope(module, paused, "revokeGrant"))
}

func TestLayerXProposalPausesLedgerAndAuthLive(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, testApp.Close()) })
	ctx := testApp.GetContextForDeliverTx(nil).WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	require.NotNil(t, testApp.LayerXLedgerKeeper)
	require.NotNil(t, testApp.LayerXAuthKeeper)

	ledgerBefore := testApp.LayerXLedgerKeeper.GetParams(ctx)
	authBefore := testApp.LayerXAuthKeeper.GetParams(ctx)
	require.False(t, ledgerBefore.Paused)
	require.False(t, authBefore.Paused)
	CheckPauseScope(t, ledgertypes.ModuleName, "send", ledgerBefore.Paused)
	CheckPauseScope(t, authtypes.ModuleName, "grantSession", authBefore.Paused)

	ledgerPaused := ledgerBefore
	ledgerPaused.Paused = true
	authPaused := authBefore
	authPaused.Paused = true
	ledgerUpdate := &ledgertypes.MsgUpdateParams{
		Authority: govtypes.GovernanceAuthority(), Params: ledgerPaused,
	}
	authUpdate := &authtypes.MsgUpdateParams{
		Authority: govtypes.GovernanceAuthority(), Params: authPaused,
	}
	for _, msg := range []sdk.Msg{ledgerUpdate, authUpdate} {
		require.NotNil(t, testApp.MsgServiceRouter().HandlerByTypeURL(sdk.MsgTypeURL(msg)))
	}
	proposal, err := govtypes.NewLayerXProposal(
		"Pause ledger and authorization", "Pause new ledger and authorization activity while preserving grant revocation",
		ledgerUpdate, authUpdate,
	)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	require.Len(t, proposal.Messages, 2)
	require.Equal(t, ledgerBefore, testApp.LayerXLedgerKeeper.GetParams(ctx))
	require.Equal(t, authBefore, testApp.LayerXAuthKeeper.GetParams(ctx))

	handler := testApp.GovKeeper.Router().GetRoute(proposal.ProposalRoute())
	require.NotNil(t, handler)
	require.NoError(t, handler(ctx, proposal))

	readCtx, _ := ctx.CacheContext()
	ledgerAfter := testApp.LayerXLedgerKeeper.GetParams(readCtx)
	authAfter := testApp.LayerXAuthKeeper.GetParams(readCtx)
	require.Equal(t, ledgerPaused, ledgerAfter)
	require.Equal(t, authPaused, authAfter)
	require.True(t, ledgerAfter.Paused)
	require.True(t, authAfter.Paused)
	t.Run(ledgertypes.ModuleName, func(t *testing.T) {
		CheckPauseScope(t, ledgertypes.ModuleName, "send", ledgerAfter.Paused)
	})
	t.Run(authtypes.ModuleName, func(t *testing.T) {
		CheckPauseScope(t, authtypes.ModuleName, "grantSession", authAfter.Paused)
	})
}
