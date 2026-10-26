package evmrpc

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type UncleAPI struct {
	tmClient   client.LocalClient
	watermarks *WatermarkManager
}

func NewUncleAPI(tmClient client.LocalClient, watermarks *WatermarkManager) *UncleAPI {
	return &UncleAPI{
		tmClient:   tmClient,
		watermarks: watermarks,
	}
}

func (a *UncleAPI) GetUncleByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (map[string]interface{}, error) {
	return nil, nil
}

func (a *UncleAPI) GetUncleByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (map[string]interface{}, error) {
	return nil, nil
}
