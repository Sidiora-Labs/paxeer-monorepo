package query

import (
	"context"
	"fmt"

	pagination "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/tx-scanner/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const txPageSize = 100

// GetTxsEvent query the detailed transaction data, same as `paxd q txs --events`
func GetTxsEvent(blockHeight int64) (*txtypes.GetTxsEventResponse, error) {
	return GetTxsEventContext(context.Background(), blockHeight)
}

// GetTxsEventContext retrieves the complete indexed set or returns an error.
func GetTxsEventContext(ctx context.Context, blockHeight int64) (*txtypes.GetTxsEventResponse, error) {
	request := &txtypes.GetTxsEventRequest{
		Events:     []string{fmt.Sprintf("tx.height=%d", blockHeight)},
		Pagination: &pagination.PageRequest{Limit: txPageSize, CountTotal: true},
		OrderBy:    txtypes.OrderBy_ORDER_BY_ASC,
	}
	result := &txtypes.GetTxsEventResponse{Pagination: &pagination.PageResponse{}}
	seen := make(map[string]struct{})
	for {
		pageCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		page, err := client.GetTxClient().GetTxsEvent(pageCtx, request)
		cancel()
		if err != nil {
			return nil, err
		}
		if page == nil || page.Pagination == nil || len(page.Txs) != len(page.TxResponses) {
			return nil, status.Error(codes.DataLoss, "incomplete transaction page")
		}
		if request.Pagination.Offset == 0 {
			result.Pagination.Total = page.Pagination.Total
		} else if page.Pagination.Total != result.Pagination.Total {
			return nil, status.Error(codes.DataLoss, "indexed transaction total changed during pagination")
		}
		remaining := result.Pagination.Total - uint64(len(result.Txs))
		expected := min(uint64(txPageSize), remaining)
		if uint64(len(page.Txs)) != expected {
			return nil, status.Error(codes.DataLoss, "transaction page does not match indexed total")
		}
		for i, response := range page.TxResponses {
			if page.Txs[i] == nil || response == nil || response.TxHash == "" {
				return nil, status.Error(codes.DataLoss, "transaction page contains an incomplete entry")
			}
			if _, exists := seen[response.TxHash]; exists {
				return nil, status.Error(codes.DataLoss, "transaction pagination repeated a hash")
			}
			seen[response.TxHash] = struct{}{}
		}
		result.Txs = append(result.Txs, page.Txs...)
		result.TxResponses = append(result.TxResponses, page.TxResponses...)
		if uint64(len(result.Txs)) == result.Pagination.Total {
			return result, nil
		}
		request.Pagination.Offset += txPageSize
	}
}

// GetTxByHash query the transaction by TX hash, same as `paxd q tx --hash`
func GetTxByHash(txHash string) (*txtypes.GetTxResponse, error) {
	return GetTxByHashContext(context.Background(), txHash)
}

// GetTxByHashContext queries a transaction within the scan's context.
func GetTxByHashContext(ctx context.Context, txHash string) (*txtypes.GetTxResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request := &txtypes.GetTxRequest{
		Hash: txHash,
	}
	return client.GetTxClient().GetTx(ctx, request)
}
