package evmrpc

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
)

func Repair185CursorForTest(api *FilterAPI, id rpc.ID) int64 {
	api.filtersMu.RLock()
	defer api.filtersMu.RUnlock()
	return api.filters[id].lastToHeight
}

func NormalizeRepair185LogsForTest(api *FilterAPI, candidates []*types.Log, crit filters.FilterCriteria) ([]*types.Log, error) {
	logs, err := api.logFetcher.normalizeRangeQueryLogs(context.Background(), candidates, crit)
	if err != nil {
		return nil, err
	}
	if err := api.logFetcher.checkLogLimit(logs); err != nil {
		return nil, err
	}
	return logs, nil
}
