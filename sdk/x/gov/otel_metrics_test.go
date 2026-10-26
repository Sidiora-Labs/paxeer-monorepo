package gov_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/stretchr/testify/require"
)

func endBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_gov", "gov_end_blocker_duration")
}

func TestEndBlockerRecordsGovEndBlockerDuration(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, Time: time.Now()})
	before := endBlockerDuration(reader)

	gov.EndBlocker(ctx, app.GovKeeper)
	require.Equal(t, uint64(1), endBlockerDuration(reader)-before, "EndBlocker must record exactly one duration")

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(time.Second))
	beforeNextBlock := endBlockerDuration(reader)
	gov.EndBlocker(ctx, app.GovKeeper)
	require.Equal(t, uint64(1), endBlockerDuration(reader)-beforeNextBlock, "the next block must record exactly one duration")
	require.Equal(t, uint64(2), endBlockerDuration(reader)-before)
}
