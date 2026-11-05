package perps_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func viewGoldenAccountCall(e *positionsEnv, method string, args ...interface{}) []byte {
	e.t.Helper()
	input, err := e.perpsABI.Pack(method, args...)
	require.NoError(e.t, err)
	output, err := e.precompile.Run(e.evm, positionsTraderA, positionsTraderA, input, big.NewInt(0), true, false, nil)
	require.NoError(e.t, err, "%s reverted with %x", method, output)
	return output
}

func viewGetAccount(e *positionsEnv) []byte {
	e.t.Helper()
	return viewGoldenAccountCall(e, "getAccount", positionsTraderA)
}

func viewGetPosition(e *positionsEnv, positionID *big.Int) []byte {
	e.t.Helper()
	return viewGoldenAccountCall(e, "getPosition", positionID)
}

func TestViewGoldenAccountAndPosition(t *testing.T) {
	e := newPositionsEnv(t, 150_000_000)
	e.mustCall(positionsTraderA, "deposit", e.usdl, big.NewInt(50_000_000))
	positionID := e.open()
	require.Equal(t, big.NewInt(2), positionID)
	e.setMark(positionsE18(59_000))

	data, err := os.ReadFile("testdata/views_account_golden.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(data, &golden))

	cases := []struct {
		method string
		call   func() []byte
	}{
		{method: "getAccount", call: func() []byte { return viewGetAccount(e) }},
		{method: "getPosition", call: func() []byte { return viewGetPosition(e, positionID) }},
		{method: "getPositionDetails", call: func() []byte {
			return viewGoldenAccountCall(e, "getPositionDetails", positionID)
		}},
	}
	require.Len(t, golden, len(cases))
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			encoded, ok := golden[tc.method]
			require.True(t, ok, "missing golden for %s", tc.method)
			expected, err := hexutil.Decode(encoded)
			require.NoError(t, err)
			require.NotEmpty(t, expected)
			require.Equal(t, expected, tc.call(), "%s ABI output", tc.method)
		})
	}
}
