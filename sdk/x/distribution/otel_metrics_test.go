package distribution_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/stretchr/testify/require"
)

func beginBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_distribution", "distribution_begin_blocker_duration")
}

func TestBeginBlockerRecordsDistributionBeginBlockerDuration(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	pubKey := paxapp.CreateTestPubKeys(1)[0]
	consAddr := sdk.ConsAddress(pubKey.Address())
	ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ProposerAddress: consAddr})
	addr := paxapp.AddTestAddrs(app, ctx, 1, sdk.NewInt(1000))[0]
	tstaking := teststaking.NewHelper(t, ctx, app.StakingKeeper)
	tstaking.CreateValidator(sdk.ValAddress(addr), pubKey, sdk.NewInt(100), true)
	app.DistrKeeper.SetPreviousProposerConsAddr(ctx, consAddr)
	fees := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100))
	require.NoError(t, apptesting.FundModuleAccount(app.BankKeeper, ctx, authtypes.FeeCollectorName, fees))
	ctx = ctx.WithBlockHeight(2)
	votes := []abci.VoteInfo{{
		Validator:       abci.Validator{Address: consAddr, Power: 100},
		SignedLastBlock: true,
	}}

	before := beginBlockerDuration(reader)
	distribution.BeginBlocker(ctx, votes, app.DistrKeeper)
	after := beginBlockerDuration(reader)
	require.Equal(t, uint64(1), after-before)
	require.Equal(t, consAddr, app.DistrKeeper.GetPreviousProposerConsAddr(ctx))
	feeCollector := app.AccountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName)
	require.True(t, app.BankKeeper.GetAllBalances(ctx, feeCollector.GetAddress()).IsZero())
	require.False(t, app.DistrKeeper.GetValidatorOutstandingRewards(ctx, sdk.ValAddress(addr)).Rewards.IsZero())
}
