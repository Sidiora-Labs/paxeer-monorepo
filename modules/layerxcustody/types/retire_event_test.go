package types_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestEventRetiredReleaseRoundTripsAsATypedEvent(t *testing.T) {
	want := &types.EventRetiredRelease{
		AssetId:         hash32("retired custody asset"),
		Recipient:       types.GovernanceAuthority(),
		Amount:          "123456789012345678901234567890",
		Denom:           "usid",
		CancelledClaims: 9007199254740993,
	}

	event, err := sdk.TypedEventToEvent(want)
	require.NoError(t, err)
	require.Equal(t, "paxprotocol.paxchain.layerxcustody.EventRetiredRelease", event.Type)
	require.Len(t, event.Attributes, 5)

	parsed, err := sdk.ParseTypedEvent(abci.Event(event))
	require.NoError(t, err)
	got, ok := parsed.(*types.EventRetiredRelease)
	require.True(t, ok, "parsed event has type %T", parsed)
	require.Equal(t, want.AssetId, got.AssetId)
	require.Equal(t, want.Recipient, got.Recipient)
	require.Equal(t, want.Amount, got.Amount)
	require.Equal(t, want.Denom, got.Denom)
	require.Equal(t, want.CancelledClaims, got.CancelledClaims)
}
