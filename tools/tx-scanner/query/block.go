package query

import (
	"context"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/tx-scanner/client"
)

const requestTimeout = 5 * time.Second

// GetLatestBlock query the latest block data
func GetLatestBlock() (*tmservice.GetLatestBlockResponse, error) {
	return GetLatestBlockContext(context.Background())
}

// GetLatestBlockContext queries the latest block within the scan's context.
func GetLatestBlockContext(ctx context.Context) (*tmservice.GetLatestBlockResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request := &tmservice.GetLatestBlockRequest{}
	return client.GetTmServiceClient().GetLatestBlock(ctx, request)
}

// GetBlockByHeight query the block data at height
func GetBlockByHeight(height int64) (*tmservice.GetBlockByHeightResponse, error) {
	return GetBlockByHeightContext(context.Background(), height)
}

// GetBlockByHeightContext queries a block within the scan's context.
func GetBlockByHeightContext(ctx context.Context, height int64) (*tmservice.GetBlockByHeightResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request := &tmservice.GetBlockByHeightRequest{
		Height: height,
	}
	return client.GetTmServiceClient().GetBlockByHeight(ctx, request)
}
