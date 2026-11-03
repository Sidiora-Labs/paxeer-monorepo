package evidence_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
)

func TestEvidenceEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	sink := oteltest.InstallLegacySink(t)
	telemetry.MeasureSince(time.Now(), "evidence", "legacy_test", "control")
	controlKeys := sink.Keys()
	require.Equal(t, []string{"oteltest.evidence.legacy_test.control"}, controlKeys)
	before := beginBlockerDuration(reader)

	for i, misbehaviour := range [][]abci.Misbehavior{nil, {}, nil} {
		ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: int64(i + 1)})
		evidence.BeginBlocker(ctx, misbehaviour, app.EvidenceKeeper)

		require.Equal(t, before+uint64(i+1), beginBlockerDuration(reader), "begin block %d must record one evidence duration", i+1)
		require.Equal(t, controlKeys, sink.Keys(), "begin block %d must not emit legacy telemetry", i+1)
	}
}
