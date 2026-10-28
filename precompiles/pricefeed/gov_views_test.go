package pricefeed_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func getParams(h *harness) pricefeed.ParamsView {
	h.t.Helper()
	out := h.view(pricefeed.GetParamsMethod)
	require.Len(h.t, out, 1)
	return *abi.ConvertType(out[0], new(pricefeed.ParamsView)).(*pricefeed.ParamsView)
}

func getMarketCount(h *harness) *big.Int {
	h.t.Helper()
	out := h.view(pricefeed.GetMarketCountMethod)
	require.Len(h.t, out, 1)
	require.IsType(h.t, new(big.Int), out[0])
	return out[0].(*big.Int)
}

func TestGovernedParamsVisibleInPrecompileViews(t *testing.T) {
	h := newHarness(t)
	before := h.keeper.GetParams(h.ctx())
	require.Len(t, before.Markets, 18)
	require.Equal(t, big.NewInt(18), getMarketCount(h))
	require.Equal(t, before.MaxPriceAgeMs, getParams(h).MaxPriceAgeMs)

	registry := cdctypes.NewInterfaceRegistry()
	pricefeedtypes.RegisterInterfaces(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	pricefeedtypes.RegisterMsgServer(router, pricefeedkeeper.NewMsgServerImpl(*h.keeper))

	market := pricefeedtypes.Market{
		ID: 18, Symbol: "TEST-USD", ChainflowSymbol: "TEST/USD",
		MinPrice: sdk.NewInt(1), MaxPrice: sdk.NewIntWithDecimal(1000000, 18), Enabled: true,
	}
	msg := &pricefeedtypes.MsgSetParams{
		Authority:                    layerxgovtypes.GovernanceAuthority(),
		Markets:                      append(append([]pricefeedtypes.Market(nil), before.Markets...), market),
		StalenessWindowMs:            before.StalenessWindowMs,
		MaxClockSkewMs:               before.MaxClockSkewMs,
		MinSources:                   before.MinSources,
		MaxDivergenceBps:             before.MaxDivergenceBps,
		MaxDeviationBps:              before.MaxDeviationBps,
		MaxObservationsPerSubmission: before.MaxObservationsPerSubmission,
		MaxSubmissionBytes:           before.MaxSubmissionBytes,
		MarkEmaTauMs:                 before.MarkEmaTauMs,
		MaxMarkBasisBps:              before.MaxMarkBasisBps,
		MaxPriceAgeMs:                30000,
	}
	require.NotEqual(t, before.MaxPriceAgeMs, msg.MaxPriceAgeMs)
	proposal, err := layerxgovtypes.NewLayerXProposal("Update pricefeed parameters", "Append a market and increase the maximum price age", msg)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	require.Equal(t, before, h.keeper.GetParams(h.ctx()))
	require.NoError(t, layerxgov.NewProposalHandler(router)(h.ctx(), proposal))

	stored := h.keeper.GetParams(h.ctx())
	require.Equal(t, msg.Authority, stored.Authority)
	require.Equal(t, msg.Markets, stored.Markets)
	require.Len(t, stored.Markets, 19)
	require.Equal(t, market, stored.Markets[18])
	require.Equal(t, uint64(30000), stored.MaxPriceAgeMs)

	params := getParams(h)
	require.Equal(t, pricefeed.ParamsView{
		StalenessWindowMs:            stored.StalenessWindowMs,
		MaxClockSkewMs:               stored.MaxClockSkewMs,
		MinSources:                   stored.MinSources,
		MaxDivergenceBps:             stored.MaxDivergenceBps,
		MaxDeviationBps:              stored.MaxDeviationBps,
		MaxObservationsPerSubmission: stored.MaxObservationsPerSubmission,
		MaxSubmissionBytes:           stored.MaxSubmissionBytes,
		MarkEmaTauMs:                 stored.MarkEmaTauMs,
		MaxMarkBasisBps:              stored.MaxMarkBasisBps,
		MaxPriceAgeMs:                stored.MaxPriceAgeMs,
	}, params)
	require.Equal(t, msg.MaxPriceAgeMs, params.MaxPriceAgeMs)
	count := getMarketCount(h)
	require.Equal(t, big.NewInt(19), count)
	require.Equal(t, new(big.Int).SetUint64(uint64(len(stored.Markets))), count)
}
