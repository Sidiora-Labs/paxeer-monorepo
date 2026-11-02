package evmrpc

import (
	"context"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type UncleAPI struct {
	tmClient       client.LocalClient
	watermarks     *WatermarkManager
	connectionType ConnectionType
}

func NewUncleAPI(tmClient client.LocalClient, watermarks *WatermarkManager, connectionType ConnectionType) *UncleAPI {
	return &UncleAPI{
		tmClient:       tmClient,
		watermarks:     watermarks,
		connectionType: connectionType,
	}
}

func (a *UncleAPI) GetUncleByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (result map[string]interface{}, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getUncleByBlockNumberAndIndex", a.connectionType, startTime, returnErr, recover())
	}()
	return nil, nil
}

func (a *UncleAPI) GetUncleByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (result map[string]interface{}, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getUncleByBlockHashAndIndex", a.connectionType, startTime, returnErr, recover())
	}()
	return nil, nil
}
