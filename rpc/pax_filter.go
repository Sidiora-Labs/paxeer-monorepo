package evmrpc

import (
	"context"

	"github.com/ethereum/go-ethereum/eth/filters"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
)

type PaxFilterAPI struct {
	f *FilterAPI
}

func NewPaxFilterAPI(f *FilterAPI) *PaxFilterAPI {
	return &PaxFilterAPI{f: f}
}

func (a *PaxFilterAPI) NewFilter(ctx context.Context, crit filters.FilterCriteria) (id ethrpc.ID, err error) {
	return a.f.NewFilter(ctx, crit)
}

func (a *PaxFilterAPI) NewBlockFilter(ctx context.Context) (id ethrpc.ID, err error) {
	return a.f.NewBlockFilter(ctx)
}

func (a *PaxFilterAPI) GetFilterChanges(ctx context.Context, filterID ethrpc.ID) (res interface{}, err error) {
	return a.f.GetFilterChanges(ctx, filterID)
}

func (a *PaxFilterAPI) GetFilterLogs(ctx context.Context, filterID ethrpc.ID) (res []*FilterLog, err error) {
	return a.f.GetFilterLogs(ctx, filterID)
}

func (a *PaxFilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error) {
	return a.f.GetLogs(ctx, crit)
}

func (a *PaxFilterAPI) UninstallFilter(ctx context.Context, filterID ethrpc.ID) (res bool) {
	return a.f.UninstallFilter(ctx, filterID)
}
