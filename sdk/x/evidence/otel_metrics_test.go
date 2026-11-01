package evidence_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
)

func beginBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_evidence", "evidence_begin_blocker_duration")
}

func TestBeginBlockerRecordsEvidenceBeginBlockerDuration(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	before := beginBlockerDuration(reader)
	previous := before

	for i, misbehaviour := range [][]abci.Misbehavior{nil, {}, nil} {
		ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: int64(i + 1)})
		evidence.BeginBlocker(ctx, misbehaviour, app.EvidenceKeeper)

		after := beginBlockerDuration(reader)
		require.Equal(t, previous+1, after, "begin block %d must record one evidence duration", i+1)
		previous = after
	}

	require.Equal(t, before+3, beginBlockerDuration(reader))
}
