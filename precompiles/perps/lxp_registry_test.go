package perps

import (
	"fmt"
	"strings"
	"testing"

	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestLXPReasonNamesEveryPerpsError(t *testing.T) {
	registered := perpstypes.RegisteredErrors()
	require.Len(t, registered, 44)
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	require.Len(t, parsed.Errors, len(registered))
	seen := make(map[string]bool, len(registered))

	for _, registeredErr := range registered {
		require.NotNil(t, registeredErr)
		code := registeredErr.Error()
		require.True(t, strings.HasPrefix(code, "LXP_PERPS_"), code)
		require.False(t, seen[code], "duplicate registered error %s", code)
		seen[code] = true

		t.Run(code, func(t *testing.T) {
			name := "Perps"
			for _, word := range strings.Split(strings.TrimPrefix(code, "LXP_PERPS_"), "_") {
				require.NotEmpty(t, word)
				name += strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			}
			custom, ok := parsed.Errors[name]
			require.True(t, ok, "missing ABI custom error %s", name)
			require.Empty(t, custom.Inputs, "custom error must be parameterless")
			wrapped := fmt.Errorf("perps execution failed: %w", registeredErr)

			for _, test := range []struct {
				name string
				err  error
				text string
			}{
				{name: "direct", err: registeredErr, text: code},
				{name: "wrapped", err: wrapped, text: code + ": " + wrapped.Error()},
			} {
				t.Run(test.name, func(t *testing.T) {
					reason, err := abi.UnpackRevert(pcommon.LXPReason(test.err))
					require.NoError(t, err)
					require.True(t, strings.HasPrefix(reason, "LXP_PERPS_"), reason)
					require.NotContains(t, reason, "LXP_UNKNOWN")
					require.Equal(t, test.text, reason)

					reverted := revertOf(test.err)
					require.Len(t, reverted, 4)
					require.Equal(t, custom.ID[:4], reverted)
				})
			}
		})
	}
}
