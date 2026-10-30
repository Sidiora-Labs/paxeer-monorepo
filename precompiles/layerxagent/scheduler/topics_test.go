package scheduler

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestSchedulerEventTopicsMatchContract(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)
	outcomes, err := abi.JSON(strings.NewReader(jobOutcomeABI))
	require.NoError(t, err)
	for name, event := range outcomes.Events {
		parsed.Events[name] = event
	}

	for _, tc := range []struct {
		name      string
		signature string
		prefix    string
	}{
		{"JobScheduled", "JobScheduled(uint256,address,address,uint64,uint64,uint256)", "0xf946d531"},
		{"JobCancelled", "JobCancelled(uint256,address,uint256)", "0xa80c76c4"},
		{"JobRescheduled", "JobRescheduled(uint256,uint64)", "0xb060fcde"},
		{"JobExecuted", "JobExecuted(uint256,address,uint64,uint256)", "0x2052025f"},
		{"JobFailed", "JobFailed(uint256,address,string)", "0x0bf83276"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event, exists := parsed.Events[tc.name]
			require.True(t, exists, "%s is missing from the 0x0905 abiJSON", tc.name)
			require.True(t, strings.HasPrefix(event.ID.Hex(), tc.prefix),
				"%s topic0 %s must begin with contract prefix %s", tc.name, event.ID.Hex(), tc.prefix)
			require.Equal(t, tc.signature, event.Sig)
			require.Equal(t, crypto.Keccak256Hash([]byte(tc.signature)), event.ID)
		})
	}
}
