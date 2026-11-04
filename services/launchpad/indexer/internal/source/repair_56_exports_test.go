package source

import (
	"context"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Test exports keep the explorer's request plan private in the production API.
func Repair56ExplorerFilter(options CommittedPaxscanOptions, o FetchOptions, discoveries []AddressEpoch) ([]string, func(ethtypes.Log) bool, error) {
	filter, err := newExplorerEpochFilter(options, o, discoveries)
	return append([]string(nil), filter.addresses...), filter.active, err
}

func Repair56HistoricalPage(ctx context.Context, options CommittedPaxscanOptions, o FetchOptions, discoveries []AddressEpoch, explorer bool) (FetchResult, error) {
	history := &HistoricalSource{RPC: options.RPC, Explorer: &CommittedPaxscanSource{options: options}}
	return history.fetchPage(ctx, o, explorer, discoveries)
}

var Repair56QualifyWithDiscoveries = qualifyWithDiscoveries
