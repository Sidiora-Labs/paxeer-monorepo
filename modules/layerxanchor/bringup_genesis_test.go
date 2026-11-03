package layerxanchor_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/stretchr/testify/require"
)

func TestAnchorGenesisValidationAcceptsAbsentExportState(t *testing.T) {
	encoding := app.MakeEncodingConfig()
	consumer, ok := app.ModuleBasics[types.ModuleName]
	require.True(t, ok)
	for name, state := range map[string]json.RawMessage{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, consumer.ValidateGenesis(encoding.Marshaler, encoding.TxConfig, state))
		})
	}
	state := consumer.DefaultGenesis(encoding.Marshaler)
	require.NotEmpty(t, state)
	require.NoError(t, consumer.ValidateGenesis(encoding.Marshaler, encoding.TxConfig, state))
}

func TestAnchorGenesisValidationRefusesMalformedSuppliedState(t *testing.T) {
	encoding := app.MakeEncodingConfig()
	consumer, ok := app.ModuleBasics[types.ModuleName]
	require.True(t, ok)
	state := consumer.DefaultGenesis(encoding.Marshaler)
	for name, malformed := range map[string]json.RawMessage{
		"truncated":  state[:len(state)-1],
		"whitespace": []byte(" "),
		"wrong type": []byte("[]"),
	} {
		t.Run(name, func(t *testing.T) {
			err := consumer.ValidateGenesis(encoding.Marshaler, encoding.TxConfig, malformed)
			require.ErrorContains(t, err, "failed to unmarshal layerxanchor genesis state")
		})
	}
	require.Error(t, consumer.ValidateGenesis(encoding.Marshaler, encoding.TxConfig, json.RawMessage("{}")))
}
