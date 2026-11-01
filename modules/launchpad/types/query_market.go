package types

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

func marketInfoInt(value Int) Int {
	amount, ok := sdk.NewIntFromString(value.String())
	if !ok {
		panic("launchpad: invalid market integer")
	}
	return amount
}

func MarketInfoOf(m Market) MarketInfo {
	prices := make([]Int, len(m.PriceSnapshots))
	for i, price := range m.PriceSnapshots {
		prices[i] = marketInfoInt(price)
	}
	heights := make([]uint64, len(m.SnapshotHeights))
	copy(heights, m.SnapshotHeights)
	return MarketInfo{
		Denom:                     m.Denom,
		Index:                     m.Index,
		Name:                      m.Name,
		Symbol:                    m.Symbol,
		Pointer:                   m.Pointer,
		Creator:                   m.Creator,
		Guardian:                  m.Guardian,
		FeeRightsHolder:           m.FeeRightsHolder,
		FeeStrategy:               uint32(m.FeeStrategy),
		Paused:                    m.Paused,
		TotalSupply:               marketInfoInt(m.TotalSupply),
		VirtualQuoteReserve:       marketInfoInt(m.VirtualQuoteReserve),
		RealQuoteBalance:          marketInfoInt(m.RealQuoteBalance),
		TokenReserve:              marketInfoInt(m.TokenReserve),
		CreationTime:              m.CreationTime,
		CumulativeVolume:          marketInfoInt(m.CumulativeVolume),
		PriceSnapshots:            prices,
		SnapshotHeights:           heights,
		SnapshotIndex:             m.SnapshotIndex,
		SnapshotCount:             m.SnapshotCount,
		AccumulatedQuoteFees:      marketInfoInt(m.AccumulatedQuoteFees),
		AccumulatedTokenFees:      marketInfoInt(m.AccumulatedTokenFees),
		AccumulatedFees:           marketInfoInt(m.AccumulatedFees),
		AirdropEpoch:              m.AirdropEpoch,
		AirdropBalance:            marketInfoInt(m.AirdropBalance),
		Generation:                uint32(m.Generation),
		PoolAddress:               m.PoolAddress,
		TokenAddress:              m.TokenAddress,
		Optical:                   m.Optical,
		Factory:                   m.Factory,
		FeeAccumulator:            m.FeeAccumulator,
		ProtocolConfig:            m.ProtocolConfig,
		EventEmitter:              m.EventEmitter,
		PendingGuardian:           m.PendingGuardian,
		LastSnapshotTimestamp:     m.LastSnapshotTimestamp,
		CumulativeBuyQuoteVolume:  marketInfoInt(m.CumulativeBuyQuoteVolume),
		CumulativeSellQuoteVolume: marketInfoInt(m.CumulativeSellQuoteVolume),
		FeeRightCollection:        m.FeeRightCollection,
		FeeRightID:                m.FeeRightID,
	}
}
