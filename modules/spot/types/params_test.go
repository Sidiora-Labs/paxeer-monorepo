package types_test

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var (
	paxPointer = common.HexToAddress("0x00000000000000000000000000000000000a0001")
	sidPointer = common.HexToAddress("0x00000000000000000000000000000000000a0002")
)

func openingParams() types.Params {
	p := types.DefaultParams()
	p.Tokens = types.OpeningTokens(paxPointer, sidPointer)
	return p
}

type paramsCase struct {
	name   string
	mutate func(*types.Params)
	want   error
}

func runParamsCases(t *testing.T, cases []paramsCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := openingParams()
			tc.mutate(&p)
			err := p.Validate()
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestParamsValidate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, uint64(7), p.SwapFeeBps)
	require.Equal(t, uint64(0), p.Tier1FeeBps)
	require.Equal(t, uint64(0), p.Tier2FeeBps)
	require.True(t, p.PriceImpactEnabled)
	require.Equal(t, uint64(100), p.PriceImpactScalarBps)
	require.Equal(t, uint64(4), p.OrderFeeBps)
	require.Equal(t, uint64(0), p.OrderTier1FeeBps)
	require.Equal(t, uint64(0), p.OrderTier2FeeBps)
	require.True(t, p.OrderPriceImpactEnabled)
	require.Equal(t, uint64(100), p.OrderPriceImpactScalarBps)
	require.Equal(t, types.GovernanceEVMAddress(), p.FeeCollector)
	require.False(t, p.SwapPaused)
	require.False(t, p.OrdersPaused)
	require.True(t, p.KeeperOpen)
	require.Empty(t, p.Keepers)
	require.Equal(t, uint64(0), p.FillQualityTwapPeriod)
	require.False(t, p.TwapSanityEnabled)
	require.Equal(t, uint64(300), p.TwapSanityPeriod)
	require.Equal(t, uint64(200), p.MaxTwapDeviationBps)
	require.Equal(t, uint64(86_400), p.TwapMaxPeriod)

	maxUint := uint64(math.MaxUint64)
	runParamsCases(t, []paramsCase{
		{"opening tokens validate", func(*types.Params) {}, nil},
		{"swap stack at 200", func(p *types.Params) { p.SwapFeeBps, p.Tier1FeeBps, p.Tier2FeeBps = 100, 50, 50 }, nil},
		{"swap stack above 200", func(p *types.Params) { p.SwapFeeBps, p.Tier1FeeBps, p.Tier2FeeBps = 101, 50, 50 }, types.ErrParamsFeeTooHigh},
		{"swap base above 200", func(p *types.Params) { p.SwapFeeBps = 201 }, types.ErrParamsFeeTooHigh},
		{"swap tiers wrap", func(p *types.Params) { p.Tier1FeeBps, p.Tier2FeeBps = maxUint, maxUint }, types.ErrParamsFeeTooHigh},
		{"swap tier2 below tier1", func(p *types.Params) { p.Tier1FeeBps, p.Tier2FeeBps = 20, 19 }, types.ErrParamsTier2BelowTier1},
		{"swap scalar at 500", func(p *types.Params) { p.PriceImpactScalarBps = 500 }, nil},
		{"swap scalar above 500", func(p *types.Params) { p.PriceImpactScalarBps = 501 }, types.ErrParamsScalarTooHigh},
		{"order stack at 200", func(p *types.Params) { p.OrderFeeBps, p.OrderTier1FeeBps, p.OrderTier2FeeBps = 100, 50, 50 }, nil},
		{"order stack above 200", func(p *types.Params) { p.OrderFeeBps, p.OrderTier1FeeBps, p.OrderTier2FeeBps = 101, 50, 50 }, types.ErrParamsFeeExceedsMax},
		{"order base above 200", func(p *types.Params) { p.OrderFeeBps = 201 }, types.ErrParamsFeeExceedsMax},
		{"order tiers wrap", func(p *types.Params) { p.OrderTier1FeeBps, p.OrderTier2FeeBps = maxUint, maxUint }, types.ErrParamsFeeExceedsMax},
		{"order tier2 below tier1", func(p *types.Params) { p.OrderTier1FeeBps, p.OrderTier2FeeBps = 20, 19 }, types.ErrParamsTier2BelowTier1},
		{"order scalar at 500", func(p *types.Params) { p.OrderPriceImpactScalarBps = 500 }, nil},
		{"order scalar above 500", func(p *types.Params) { p.OrderPriceImpactScalarBps = 501 }, types.ErrParamsScalarTooHigh},
		{"periods at twapMaxPeriod", func(p *types.Params) { p.FillQualityTwapPeriod, p.TwapSanityPeriod = 86_400, 86_400 }, nil},
		{"fill quality period above max", func(p *types.Params) { p.FillQualityTwapPeriod = 86_401 }, types.ErrParamsInvalidTWAPPeriod},
		{"sanity period above max", func(p *types.Params) { p.TwapSanityPeriod = 86_401 }, types.ErrParamsInvalidTWAPPeriod},
		{"zero fee collector", func(p *types.Params) { p.FeeCollector = common.Address{} }, types.ErrParamsZeroAddress},
		{"listed keeper", func(p *types.Params) { p.Keepers = []common.Address{paxPointer} }, nil},
		{"zero keeper", func(p *types.Params) { p.Keepers = []common.Address{paxPointer, {}} }, types.ErrParamsZeroAddress},
		{"token listed twice", func(p *types.Params) { p.Tokens = append(p.Tokens, p.Tokens[1]) }, types.ErrParamsTokenAlreadyRegistered},
	})
}

func TestParamsValidateBounds(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, sdk.NewIntWithDecimal(10, 18).String(), p.MinOrderNotional.String())
	require.Equal(t, uint64(100), p.MaxActiveOrdersPerUser)
	require.Equal(t, uint64(200), p.MaxExpiriesPerBlock)
	require.Equal(t, uint64(500), p.ActivationScanPerBlock)
	require.Equal(t, uint64(500), p.FillScanPerBlock)
	require.Equal(t, uint64(200), p.MaxFillsPerBlock)
	require.Equal(t, uint64(256), p.MaxFillBackoffBlocks)
	require.Empty(t, p.Tokens)

	require.Equal(t, []types.TokenParams{
		{Address: common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479"), Kind: types.TokenKindERC20, IsStablecoin: true, Decimals: 6, MaxUtilizationBps: 9_000},
		{Address: paxPointer, Kind: types.TokenKindNative, Decimals: 6, MarketID: 11, MaxUtilizationBps: 9_000},
		{Address: sidPointer, Kind: types.TokenKindNative, Decimals: 6, MarketID: 12, MaxUtilizationBps: 9_000},
	}, types.OpeningTokens(paxPointer, sidPointer))
	require.Equal(t, 10_000, types.DefaultMaxUtilizationBps)

	runParamsCases(t, []paramsCase{
		{"opening tokens validate", func(*types.Params) {}, nil},
		{"zero maxExpiriesPerBlock", func(p *types.Params) { p.MaxExpiriesPerBlock = 0 }, types.ErrParamsInvalid},
		{"zero activationScanPerBlock", func(p *types.Params) { p.ActivationScanPerBlock = 0 }, types.ErrParamsInvalid},
		{"zero fillScanPerBlock", func(p *types.Params) { p.FillScanPerBlock = 0 }, types.ErrParamsInvalid},
		{"zero maxFillsPerBlock", func(p *types.Params) { p.MaxFillsPerBlock = 0 }, types.ErrParamsInvalid},
		{"zero maxActiveOrdersPerUser", func(p *types.Params) { p.MaxActiveOrdersPerUser = 0 }, types.ErrParamsInvalid},
		{"zero maxFillBackoffBlocks", func(p *types.Params) { p.MaxFillBackoffBlocks = 0 }, types.ErrParamsInvalid},
		{"zero minOrderNotional", func(p *types.Params) { p.MinOrderNotional = sdk.ZeroInt() }, nil},
		{"nil minOrderNotional", func(p *types.Params) { p.MinOrderNotional = sdk.Int{} }, types.ErrParamsInvalid},
		{"negative minOrderNotional", func(p *types.Params) { p.MinOrderNotional = sdk.NewInt(-1) }, types.ErrParamsInvalid},
		{"PAX cap at 10000", func(p *types.Params) { p.Tokens[1].MaxUtilizationBps = 10_000 }, nil},
		{"PAX cap above 10000", func(p *types.Params) { p.Tokens[1].MaxUtilizationBps = 10_001 }, types.ErrParamsInvalid},
		{"zero cap", func(p *types.Params) { p.Tokens[0].MaxUtilizationBps = 0 }, types.ErrParamsInvalid},
		{"token without a kind", func(p *types.Params) { p.Tokens[2].Kind = types.TokenKindUnspecified }, types.ErrParamsInvalid},
		{"unknown kind", func(p *types.Params) { p.Tokens[2].Kind = types.TokenKindNative + 1 }, types.ErrParamsInvalid},
		{"decimals at 36", func(p *types.Params) { p.Tokens[0].Decimals = 36 }, nil},
		{"decimals above 36", func(p *types.Params) { p.Tokens[0].Decimals = 37 }, types.ErrParamsUnsupportedToken},
		{"zero token address", func(p *types.Params) { p.Tokens[1].Address = common.Address{} }, types.ErrParamsZeroAddress},
	})
}

func TestParamsValidateBoundsBudgets(t *testing.T) {
	require.Equal(t, 1000, types.MaxActiveOrdersPerUserLimit)
	require.Equal(t, 2000, types.MaxExpiriesPerBlockLimit)
	require.Equal(t, 5000, types.MaxScanPerBlockLimit)
	require.Equal(t, 2000, types.MaxFillsPerBlockLimit)
	require.Equal(t, 2560, types.MaxFillBackoffBlocksLimit)

	for _, tc := range []struct {
		name  string
		limit uint64
		field func(*types.Params) *uint64
	}{
		{"maxActiveOrdersPerUser", types.MaxActiveOrdersPerUserLimit, func(p *types.Params) *uint64 { return &p.MaxActiveOrdersPerUser }},
		{"maxExpiriesPerBlock", types.MaxExpiriesPerBlockLimit, func(p *types.Params) *uint64 { return &p.MaxExpiriesPerBlock }},
		{"activationScanPerBlock", types.MaxScanPerBlockLimit, func(p *types.Params) *uint64 { return &p.ActivationScanPerBlock }},
		{"fillScanPerBlock", types.MaxScanPerBlockLimit, func(p *types.Params) *uint64 { return &p.FillScanPerBlock }},
		{"maxFillsPerBlock", types.MaxFillsPerBlockLimit, func(p *types.Params) *uint64 { return &p.MaxFillsPerBlock }},
		{"maxFillBackoffBlocks", types.MaxFillBackoffBlocksLimit, func(p *types.Params) *uint64 { return &p.MaxFillBackoffBlocks }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, boundary := range []struct {
				name  string
				value uint64
				valid bool
			}{
				{"zero", 0, false},
				{"minimum", 1, true},
				{"limit", tc.limit, true},
				{"above limit", tc.limit + 1, false},
				{"max uint64", math.MaxUint64, false},
			} {
				t.Run(boundary.name, func(t *testing.T) {
					p := types.DefaultParams()
					*tc.field(&p) = boundary.value
					err := p.Validate()
					if boundary.valid {
						require.NoError(t, err)
						return
					}
					require.ErrorIs(t, err, types.ErrParamsInvalid)
					require.Contains(t, err.Error(), tc.name)
				})
			}
		})
	}
}

func TestParamsValidateRequiresMarketID(t *testing.T) {
	for _, tc := range []struct {
		name   string
		index  int
		market uint64
		valid  bool
	}{
		{"stablecoin without market", 0, 0, true},
		{"stablecoin with market", 0, 1, true},
		{"PAX without market", 1, 0, false},
		{"PAX with market", 1, types.PAXMarketID, true},
		{"SID without market", 2, 0, false},
		{"SID with market", 2, types.SIDMarketID, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := openingParams()
			p.Tokens[tc.index].MarketID = tc.market
			err := p.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, types.ErrParamsInvalid)
			require.Contains(t, err.Error(), "marketID")
			require.Contains(t, err.Error(), p.Tokens[tc.index].Address.Hex())
		})
	}
}

func TestParamsValidateBoundsTokenCount(t *testing.T) {
	require.Equal(t, 64, types.MaxRegisteredTokens)
	p := types.DefaultParams()
	for i := 0; i < types.MaxRegisteredTokens+1; i++ {
		p.Tokens = append(p.Tokens, types.TokenParams{
			Address:           common.BytesToAddress([]byte{byte(i + 1)}),
			Kind:              types.TokenKindERC20,
			Decimals:          types.OpeningTokenDecimals,
			MarketID:          types.PAXMarketID,
			MaxUtilizationBps: types.DefaultMaxUtilizationBps,
		})
	}
	overLimit := p.Tokens
	p.Tokens = overLimit[:types.MaxRegisteredTokens]
	require.NoError(t, p.Validate())
	p.Tokens = overLimit
	err := p.Validate()
	require.ErrorIs(t, err, types.ErrParamsInvalid)
	require.Contains(t, err.Error(), "tokens count")

	p.Tokens[0].Address = common.Address{}
	err = p.Validate()
	require.ErrorIs(t, err, types.ErrParamsInvalid)
	require.Contains(t, err.Error(), "tokens count")
}
