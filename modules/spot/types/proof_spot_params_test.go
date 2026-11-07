// Proves: types.DefaultParams/types.OpeningTokens -> JSON types.GenesisState -> types.GenesisState.Validate/types.Params.Validate.
// The consumer here uses the same json.Unmarshal and GenesisState.Validate
// boundary as modules/spot/module.go AppModuleBasic.ValidateGenesis. This
// parameter surface has no persistent state: every observation decodes fresh
// producer bytes, twice, without retaining a previous decoded value. Malformed
// JSON is refused at decoding; out-of-contract values are refused at validation.
package types_test

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestProofSpotParams(t *testing.T) {
	t.Run("activation_scan_per_block", activation_scan_per_block)
	t.Run("fill_scan_per_block", fill_scan_per_block)
	t.Run("max_active_orders_per_user", max_active_orders_per_user)
	t.Run("max_expiries_per_block", max_expiries_per_block)
	t.Run("max_fill_backoff_blocks", max_fill_backoff_blocks)
	t.Run("max_fills_per_block", max_fills_per_block)
	t.Run("tokens", tokens)
}

// These named cases follow the genesis interface names and exercise the Go
// fields consumers compile against, rather than looking them up by reflection.
func activation_scan_per_block(t *testing.T) {
	require.Equal(t, 5000, types.MaxScanPerBlockLimit)
	proofSpotBudget(t, "activation_scan_per_block", "activationScanPerBlock", 500, types.MaxScanPerBlockLimit,
		func(p *types.Params) *uint64 { return &p.ActivationScanPerBlock })
}

func fill_scan_per_block(t *testing.T) {
	require.Equal(t, 5000, types.MaxScanPerBlockLimit)
	proofSpotBudget(t, "fill_scan_per_block", "fillScanPerBlock", 500, types.MaxScanPerBlockLimit,
		func(p *types.Params) *uint64 { return &p.FillScanPerBlock })
}

func max_active_orders_per_user(t *testing.T) {
	require.Equal(t, 1000, types.MaxActiveOrdersPerUserLimit)
	proofSpotBudget(t, "max_active_orders_per_user", "maxActiveOrdersPerUser", 100, types.MaxActiveOrdersPerUserLimit,
		func(p *types.Params) *uint64 { return &p.MaxActiveOrdersPerUser })
}

func max_expiries_per_block(t *testing.T) {
	require.Equal(t, 2000, types.MaxExpiriesPerBlockLimit)
	proofSpotBudget(t, "max_expiries_per_block", "maxExpiriesPerBlock", 200, types.MaxExpiriesPerBlockLimit,
		func(p *types.Params) *uint64 { return &p.MaxExpiriesPerBlock })
}

func max_fill_backoff_blocks(t *testing.T) {
	require.Equal(t, 2560, types.MaxFillBackoffBlocksLimit)
	proofSpotBudget(t, "max_fill_backoff_blocks", "maxFillBackoffBlocks", 256, types.MaxFillBackoffBlocksLimit,
		func(p *types.Params) *uint64 { return &p.MaxFillBackoffBlocks })
}

func max_fills_per_block(t *testing.T) {
	require.Equal(t, 2000, types.MaxFillsPerBlockLimit)
	proofSpotBudget(t, "max_fills_per_block", "maxFillsPerBlock", 200, types.MaxFillsPerBlockLimit,
		func(p *types.Params) *uint64 { return &p.MaxFillsPerBlock })
}

func proofSpotBudget(t *testing.T, wireName, errorName string, defaultValue, limit uint64, field func(*types.Params) *uint64) {
	t.Helper()
	gs := types.DefaultGenesis()
	require.Equal(t, defaultValue, *field(&gs.Params))
	proofSpotGenesis(t, gs, nil, "")
	for _, tc := range []struct {
		name  string
		value uint64
		valid bool
	}{
		{"minimum", 1, true},
		{"maximum", limit, true},
		{"zero", 0, false},
		{"over_maximum", limit + 1, false},
		{"uint64_maximum", math.MaxUint64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := types.DefaultGenesis()
			*field(&producer.Params) = tc.value
			var want error
			if !tc.valid {
				want = types.ErrParamsInvalid
			}
			consumer := proofSpotGenesis(t, producer, want, errorName)
			require.Equal(t, tc.value, *field(&consumer.Params))

			// Pin the external key independently of the Go field tags: a tag
			// rename must not silently change the genesis contract.
			raw, err := json.Marshal(producer)
			require.NoError(t, err)
			var envelope struct {
				Params map[string]json.RawMessage `json:"params"`
			}
			require.NoError(t, json.Unmarshal(raw, &envelope))
			require.Contains(t, envelope.Params, wireName)
			var observed uint64
			require.NoError(t, json.Unmarshal(envelope.Params[wireName], &observed))
			require.Equal(t, tc.value, observed)
		})
	}

	t.Run("malformed_at_decode", func(t *testing.T) {
		raw, err := json.Marshal(gs)
		require.NoError(t, err)
		var envelope map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &envelope))
		var params map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(envelope["params"], &params))
		require.Contains(t, params, wireName)
		params[wireName] = json.RawMessage(`"not a uint64"`)
		envelope["params"], err = json.Marshal(params)
		require.NoError(t, err)
		malformed, err := json.Marshal(envelope)
		require.NoError(t, err)
		for observation := 0; observation < 2; observation++ {
			var consumer types.GenesisState
			var typeError *json.UnmarshalTypeError
			require.ErrorAs(t, json.Unmarshal(malformed, &consumer), &typeError)
			require.Contains(t, typeError.Field, wireName)
		}
	})
}

func tokens(t *testing.T) {
	require.Equal(t, 64, types.MaxRegisteredTokens)
	gs := types.DefaultGenesis()
	require.Empty(t, gs.Params.Tokens)
	proofSpotGenesis(t, gs, nil, "")
	pax := common.HexToAddress("0x00000000000000000000000000000000000a0001")
	sid := common.HexToAddress("0x00000000000000000000000000000000000a0002")
	gs.Params.Tokens = types.OpeningTokens(pax, sid)
	expected := []types.TokenParams{
		{Address: common.HexToAddress(types.USDLAddress), Kind: types.TokenKindERC20, IsStablecoin: true,
			Decimals: 6, MaxUtilizationBps: 9000},
		{Address: pax, Kind: types.TokenKindNative, Decimals: 6, MarketID: 11, MaxUtilizationBps: 9000},
		{Address: sid, Kind: types.TokenKindNative, Decimals: 6, MarketID: 12, MaxUtilizationBps: 9000},
	}
	require.Equal(t, expected, proofSpotGenesis(t, gs, nil, "").Params.Tokens)
	raw, err := json.Marshal(gs)
	require.NoError(t, err)
	var envelope struct {
		Params map[string]json.RawMessage `json:"params"`
	}
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.Contains(t, envelope.Params, "tokens")
	var observed []types.TokenParams
	require.NoError(t, json.Unmarshal(envelope.Params["tokens"], &observed))
	require.Equal(t, expected, observed)
	gs.Params.Tokens[1].MarketID = 0
	proofSpotGenesis(t, gs, types.ErrParamsInvalid, "marketID")
	gs.Params.Tokens = types.OpeningTokens(pax, sid)
	gs.Params.Tokens = append(gs.Params.Tokens, gs.Params.Tokens[0])
	proofSpotGenesis(t, gs, types.ErrParamsTokenAlreadyRegistered, "")
	gs.Params.Tokens = types.OpeningTokens(pax, sid)
	for len(gs.Params.Tokens) < types.MaxRegisteredTokens {
		gs.Params.Tokens = append(gs.Params.Tokens, types.TokenParams{
			Address: common.BytesToAddress([]byte{byte(len(gs.Params.Tokens) + 1)}),
			Kind:    types.TokenKindERC20, Decimals: 6, MarketID: 11, MaxUtilizationBps: 10_000,
		})
	}
	consumer := proofSpotGenesis(t, gs, nil, "")
	require.Len(t, consumer.Params.Tokens, 64)
	gs.Params.Tokens = append(gs.Params.Tokens, types.TokenParams{
		Address: common.BytesToAddress([]byte{65}), Kind: types.TokenKindNative,
		Decimals: 6, MarketID: 12, MaxUtilizationBps: 9000,
	})
	proofSpotGenesis(t, gs, types.ErrParamsInvalid, "tokens count")

	t.Run("malformed_address_at_decode", func(t *testing.T) {
		gs.Params.Tokens = types.OpeningTokens(pax, sid)
		raw, err := json.Marshal(gs)
		require.NoError(t, err)
		address, err := json.Marshal(pax)
		require.NoError(t, err)
		require.Equal(t, 1, bytes.Count(raw, address))
		malformed := bytes.Replace(raw, address, []byte(`"0x123"`), 1)
		for observation := 0; observation < 2; observation++ {
			var decoded types.GenesisState
			require.ErrorContains(t, json.Unmarshal(malformed, &decoded), "hex string")
		}
	})
}

// proofSpotGenesis is the consumer: it decodes the real producer's complete
// genesis document and invokes the startup validator, with no substitute codec.
func proofSpotGenesis(t *testing.T, producer *types.GenesisState, want error, detail string) types.GenesisState {
	t.Helper()
	raw, err := json.Marshal(producer)
	require.NoError(t, err)
	var result types.GenesisState
	for observation := 0; observation < 2; observation++ {
		var consumer types.GenesisState
		require.NoError(t, json.Unmarshal(raw, &consumer))
		require.Equal(t, producer.Params, consumer.Params)
		err := consumer.Validate()
		if want == nil {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, want)
			if detail != "" {
				require.Contains(t, err.Error(), detail)
			}
		}
		if observation == 0 {
			// Poison the first decoded value. The next observation must obtain
			// its values from the bytes again, including a fresh token slice.
			consumer.Params.ActivationScanPerBlock = 0
			if len(consumer.Params.Tokens) != 0 {
				consumer.Params.Tokens[0].MarketID = math.MaxUint64
			}
		} else {
			result = consumer
		}
	}
	return result
}
