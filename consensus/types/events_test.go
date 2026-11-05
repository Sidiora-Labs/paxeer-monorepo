package types

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/jsontypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify that the event data types satisfy their shared interface.
var (
	_ EventData = EventDataBlockSyncStatus{}
	_ EventData = EventDataCompleteProposal{}
	_ EventData = EventDataNewBlock{}
	_ EventData = EventDataNewBlockHeader{}
	_ EventData = EventDataNewEvidence{}
	_ EventData = EventDataNewRound{}
	_ EventData = EventDataRoundState{}
	_ EventData = EventDataStateSyncStatus{}
	_ EventData = EventDataTx{}
	_ EventData = EventDataValidatorSetUpdates{}
	_ EventData = EventDataVote{}
	_ EventData = EventDataString("")
)

func TestQueryTxFor(t *testing.T) {
	tx := Tx("foo")
	assert.Equal(t,
		fmt.Sprintf("tm.event = 'Tx' AND tx.hash = '%X'", tx.Hash()),
		EventQueryTxFor(tx).String(),
	)
}

func TestQueryForEvent(t *testing.T) {
	assert.Equal(t,
		"tm.event = 'NewBlock'",
		QueryForEvent(EventNewBlockValue).String(),
	)
	assert.Equal(t,
		"tm.event = 'NewEvidence'",
		QueryForEvent(EventNewEvidenceValue).String(),
	)
}

func TestTryUnmarshalForEvent(t *testing.T) {
	eventData := EventDataTx{
		TxResultV2: types.TxResultV2{
			Height: 123,
			Index:  2,
			Tx:     []byte("nonzero-result"),
			Result: types.ExecTxResult{
				Code: 37, Data: []byte{0, 255, 1}, GasWanted: 100, GasUsed: 73,
				Events: []types.Event{{Type: "execution", Attributes: []types.EventAttribute{
					{Key: []byte("outcome"), Value: []byte("failed"), Index: true},
				}}},
			},
		},
	}
	garbage := json.RawMessage("stuff")

	bz, err := jsontypes.Marshal(eventData)
	require.NoError(t, err)

	unmarshaled, err := TryUnmarshalEventData(json.RawMessage(bz))
	require.NoError(t, err)
	assert.Equal(t, eventData, unmarshaled)
	legacy := eventData.ToLegacy().(LegacyEventDataTx)
	legacyBytes, err := jsontypes.Marshal(legacy)
	require.NoError(t, err)
	var decoded LegacyEventData
	require.NoError(t, jsontypes.Unmarshal(legacyBytes, &decoded))
	require.Equal(t, legacy, decoded)
	require.Equal(t, eventData.Result.Code, legacy.TxResult.Result.Code)
	require.Equal(t, eventData.Result.Data, legacy.TxResult.Result.Data)
	require.Equal(t, "100", legacy.TxResult.Result.GasWanted)
	require.Equal(t, "73", legacy.TxResult.Result.GasUsed)
	require.Equal(t, eventData.Result.Events, legacy.TxResult.Result.Events)

	_, err = TryUnmarshalEventData(garbage)
	require.Error(t, err)
}
