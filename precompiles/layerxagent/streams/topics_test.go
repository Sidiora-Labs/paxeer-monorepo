package streams

import (
	"strings"
	"testing"

	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	payprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestStreamsEventTopicsMatchContract(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	for _, tc := range []struct {
		name   string
		prefix string
	}{
		{"StreamOpened", "0xc87f2fb1"},
		{"StreamSettled", "0x0ad296c7"},
		{"StreamClosed", "0xbd03ee89"},
		{"StreamRateUpdated", "0xd3372d2a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event, ok := parsed.Events[tc.name]
			require.True(t, ok, "0x0906 ABI is missing %s", tc.name)
			require.False(t, event.Anonymous, "%s must emit topic0", tc.name)
			require.True(t, strings.HasPrefix(event.ID.Hex(), tc.prefix),
				"%s topic0 %s must begin with contract prefix %s", tc.name, event.ID.Hex(), tc.prefix)
		})
	}

	t.Run("StreamRateUpdatedMatchesPay", func(t *testing.T) {
		pay, err := payprecompile.NewPrecompile(&paykeeper.Keeper{})
		require.NoError(t, err)
		agentEvent, ok := parsed.Events["StreamRateUpdated"]
		require.True(t, ok, "0x0906 ABI is missing StreamRateUpdated")
		payEvent, ok := pay.GetABI().Events["StreamRateUpdated"]
		require.True(t, ok, "0x101B ABI is missing StreamRateUpdated")
		require.False(t, payEvent.Anonymous, "0x101B StreamRateUpdated must emit topic0")
		require.Equal(t, crypto.Keccak256Hash([]byte("StreamRateUpdated(bytes32,uint128,uint128,uint128)")), payEvent.ID,
			"0x101B StreamRateUpdated topic0 must match its canonical signature")
		require.True(t, strings.HasPrefix(payEvent.ID.Hex(), "0x6006a657"),
			"0x101B StreamRateUpdated topic0 %s must begin with contract prefix 0x6006a657", payEvent.ID.Hex())
		require.Equal(t, crypto.Keccak256Hash([]byte("StreamRateUpdated(uint256,uint256,uint256)")), agentEvent.ID,
			"0x0906 StreamRateUpdated topic0 must match its canonical signature")
		require.True(t, strings.HasPrefix(agentEvent.ID.Hex(), "0xd3372d2a"),
			"0x0906 StreamRateUpdated topic0 %s must begin with contract prefix 0xd3372d2a", agentEvent.ID.Hex())
		require.NotEqual(t, agentEvent.ID, payEvent.ID,
			"0x0906 %s and 0x101B EmitStreamRateUpdated %s must have distinct topic0 values", agentEvent.Sig, payEvent.Sig)
	})
}
