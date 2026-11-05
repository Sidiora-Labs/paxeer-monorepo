package perps_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func (env *positionsEnv) viewGetInsuranceBalance() []byte {
	env.t.Helper()
	return env.mustCall(positionsTraderA, "getInsuranceBalance", env.usdl)
}

func (env *positionsEnv) viewGetADLRank(positionID *big.Int) []byte {
	env.t.Helper()
	return env.mustCall(positionsTraderA, "getADLRank", positionID)
}

func TestViewGoldenInsuranceAndAdl(t *testing.T) {
	env := newPositionsEnv(t, 100_000_000)
	params, err := env.k.GetParams(env.ctx())
	require.NoError(t, err)
	params.InsuranceSharePpm = 500_000
	require.NoError(t, env.k.UpdateParams(env.ctx(), env.k.Authority(), params))

	positionID := env.open()
	long, exists := env.position(positionsTraderA)
	require.True(t, exists)
	short, exists := env.position(positionsMakerB)
	require.True(t, exists)
	require.Equal(t, long.ID.ToBig(), positionID)
	require.NotEqual(t, long.ID, short.ID)
	require.True(t, long.IsLong)
	require.False(t, short.IsLong)
	require.Equal(t, positionsWord(t, "16500000000000000"), long.SizeBase)
	require.Equal(t, long.SizeBase, short.SizeBase)
	require.Equal(t, *uint256.MustFromBig(positionsE18(60_000)), long.EntryPrice)
	require.Equal(t, long.EntryPrice, short.EntryPrice)
	require.False(t, long.RankScore.IsZero())

	_, contributions := env.perpsLogs("InsuranceContribution")
	require.Len(t, contributions, 2)
	insurance, err := env.k.GetAccount(env.ctx(), env.k.InsuranceAccountAddress())
	require.NoError(t, err)
	require.Equal(t, *uint256.NewInt(297_000), insurance.Collateral)

	env.setMark(positionsE18(61_000))
	outputs := map[string][]byte{
		"getInsuranceBalance": env.viewGetInsuranceBalance(),
		"getADLRank":          env.viewGetADLRank(positionID),
		"getOpenInterest":     env.mustCall(positionsTraderA, "getOpenInterest", big.NewInt(0)),
	}

	data, err := os.ReadFile("testdata/views_insurance_golden.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(data, &golden))
	require.Len(t, golden, len(outputs))
	for _, method := range []string{"getInsuranceBalance", "getADLRank", "getOpenInterest"} {
		t.Run(method, func(t *testing.T) {
			expectedHex, exists := golden[method]
			require.True(t, exists, "missing golden output for %s", method)
			expected, err := hexutil.Decode(expectedHex)
			require.NoError(t, err)
			require.Equal(t, expected, outputs[method], "%s ABI bytes", method)
		})
	}
}
