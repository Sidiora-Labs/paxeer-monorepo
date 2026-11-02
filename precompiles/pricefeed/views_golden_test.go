package pricefeed_test

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const (
	getPrice   = pricefeed.GetPriceMethod
	getFeedKey = pricefeed.GetFeedKeyMethod
)

func TestViewGolden1020(t *testing.T) {
	h := newHarness(t)
	operator, _ := h.validator("views golden 1020", stakingtypes.Bonded, false)
	ctx := h.ctx().WithBlockHeight(300).WithBlockTime(time.UnixMilli(1_700_000_005_000).UTC())
	staking := testkeeper.EVMTestApp.StakingKeeper
	validator, found := staking.GetValidator(ctx, operator)
	require.True(t, found)
	staking.SetValidatorByPowerIndex(ctx, validator)
	require.Len(t, staking.GetBondedValidatorsByPower(ctx), 1)

	pubKey, err := hex.DecodeString("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
	require.NoError(t, err)
	feeder, previous, err := h.keeper.RegisterFeedKey(ctx, operator, pubKey)
	require.NoError(t, err)
	require.Nil(t, previous)
	accepted, refused, err := h.keeper.SubmitPrices(ctx, types.MsgSubmitPrices{
		Feeder: feeder.String(),
		Observations: []types.Observation{{
			MarketId: 0, Sequence: 7,
			IndexPrice:        sdk.NewIntWithDecimal(65000, 18).String(),
			MarkPrice:         sdk.NewIntWithDecimal(65010, 18).String(),
			SourceTimestampMs: 1_700_000_000_000,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, uint32(1), accepted)
	require.Zero(t, refused)
	h.keeper.Aggregate(ctx)

	goldenBytes, err := os.ReadFile("testdata/views_golden.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(goldenBytes, &golden))
	cases := []struct {
		name string
		args []interface{}
	}{
		{getPrice, []interface{}{big.NewInt(0)}},
		{pricefeed.GetPriceBySymbolMethod, []interface{}{"BTC"}},
		{pricefeed.GetPricesMethod, []interface{}{[]*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(0)}}},
		{pricefeed.GetMarketMethod, []interface{}{big.NewInt(0)}},
		{pricefeed.MarketIDOfMethod, []interface{}{"BTC"}},
		{pricefeed.GetObservationMethod, []interface{}{big.NewInt(0), common.BytesToAddress(operator)}},
		{getFeedKey, []interface{}{common.BytesToAddress(operator)}},
		{pricefeed.GetParamsMethod, nil},
	}
	require.Len(t, golden, len(cases))
	require.Equal(t, pricefeedAddress, h.precompile.Address())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, ok := golden[tc.name]
			require.True(t, ok, "missing golden output for %s", tc.name)
			want, err := hex.DecodeString(encoded)
			require.NoError(t, err)
			require.NotEmpty(t, want)
			method := h.method(tc.name)
			input, err := method.Inputs.Pack(tc.args...)
			require.NoError(t, err)
			args, err := method.Inputs.Unpack(input)
			require.NoError(t, err)
			got, err := h.precompile.GetExecutor().Execute(ctx, &method, h.caller, h.caller,
				args, big.NewInt(0), true, h.evm, nil)
			require.NoError(t, err)
			require.Equal(t, want, got, "%s ABI output differs from views_golden.json", tc.name)
		})
	}
}
