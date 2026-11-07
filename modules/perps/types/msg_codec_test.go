package types_test

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
)

func TestParamsCustomTypeRoundTrip(t *testing.T) {
	want := types.DefaultParams()
	raw, err := want.Marshal()
	require.NoError(t, err)
	require.Equal(t, len(raw), want.Size())

	var got types.Params
	require.NoError(t, got.Unmarshal(raw))
	require.Equal(t, want, got)

	buffer := make([]byte, want.Size())
	n, err := want.MarshalTo(buffer)
	require.NoError(t, err)
	require.Equal(t, len(raw), n)
	require.Equal(t, raw, buffer)
	n, err = want.MarshalTo(buffer[:len(buffer)-1])
	require.ErrorIs(t, err, io.ErrShortBuffer)
	require.Zero(t, n)

	jsonRaw, err := json.Marshal(want)
	require.NoError(t, err)
	require.Equal(t, raw, jsonRaw)
	var fromJSON types.Params
	require.NoError(t, json.Unmarshal(jsonRaw, &fromJSON))
	require.Equal(t, want, fromJSON)
	require.Contains(t, string(jsonRaw), `"collateral_token"`)
	require.Contains(t, string(jsonRaw), `"fee_tiers"`)

	for _, invalid := range [][]byte{
		[]byte(`{"unknown":true}`),
		append(append([]byte(nil), raw...), []byte(` {}`)...),
		[]byte(`{"paused":`),
	} {
		require.Error(t, got.Unmarshal(invalid))
		require.Equal(t, want, got)
	}

	msg := &types.MsgUpdateParams{Authority: "governance", Params: want}
	wire, err := proto.Marshal(msg)
	require.NoError(t, err)
	require.Equal(t, len(wire), msg.Size())
	var decoded types.MsgUpdateParams
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	require.Equal(t, msg, &decoded)
}

func TestMarketCustomTypeRoundTrip(t *testing.T) {
	want := types.Market{
		ID:                   0,
		Symbol:               "BTC",
		Name:                 "Bitcoin",
		OracleSymbol:         "BTC/USD",
		SzDecimals:           5,
		PxDecimals:           1,
		MaxLeverage:          types.VenueMaxLeverage,
		TierClass:            types.TierClassA,
		ImpactNotionalUsd:    usd(20_000),
		OiCapUsd:             usd(100_000_000),
		FundingCapPpmPerHour: 40_000,
		ListedAt:             1_790_000_000,
		Enabled:              true,
	}
	raw, err := want.Marshal()
	require.NoError(t, err)
	require.Equal(t, len(raw), want.Size())

	var got types.Market
	require.NoError(t, got.Unmarshal(raw))
	require.Equal(t, want, got)

	buffer := make([]byte, want.Size())
	n, err := want.MarshalTo(buffer)
	require.NoError(t, err)
	require.Equal(t, len(raw), n)
	require.Equal(t, raw, buffer)
	n, err = want.MarshalTo(buffer[:len(buffer)-1])
	require.ErrorIs(t, err, io.ErrShortBuffer)
	require.Zero(t, n)

	jsonRaw, err := json.Marshal(want)
	require.NoError(t, err)
	require.Equal(t, raw, jsonRaw)
	var fromJSON types.Market
	require.NoError(t, json.Unmarshal(jsonRaw, &fromJSON))
	require.Equal(t, want, fromJSON)
	require.Contains(t, string(jsonRaw), `"id":0`)
	require.Contains(t, string(jsonRaw), `"sz_decimals":5`)
	require.Contains(t, string(jsonRaw), `"px_decimals":1`)

	for _, invalid := range [][]byte{
		[]byte(`{"unknown":true}`),
		append(append([]byte(nil), raw...), []byte(` {}`)...),
		[]byte(`{"enabled":`),
	} {
		require.Error(t, got.Unmarshal(invalid))
		require.Equal(t, want, got)
	}

	msg := &types.MsgSetMarket{Authority: "governance", Market: want}
	wire, err := proto.Marshal(msg)
	require.NoError(t, err)
	require.Equal(t, len(wire), msg.Size())
	var decoded types.MsgSetMarket
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	require.Equal(t, msg, &decoded)
}
