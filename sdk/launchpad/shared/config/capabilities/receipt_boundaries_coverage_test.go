package capabilities

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
)

type creationReceiptBoundaryReader struct {
	*PinnedRPCReader
	mode        string
	balanceCall int
}

func (r *creationReceiptBoundaryReader) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	receipt, err := r.PinnedRPCReader.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, err
	}
	copied := *receipt
	switch r.mode {
	case "receipt unavailable":
		return nil, context.DeadlineExceeded
	case "receipt status":
		copied.Status = 0
	case "receipt transaction":
		copied.TxHash = common.Hash{}
	case "receipt block number":
		copied.BlockNumber = nil
	case "nil log":
		copied.Logs = []*types.Log{nil}
	case "missing transfer":
		copied.Logs = nil
	case "removed transfer", "transfer amount", "transfer topic", "transfer position":
		copied.Logs = append([]*types.Log(nil), receipt.Logs...)
		for index, log := range copied.Logs {
			if log.Address != r.usdl(ctx) {
				continue
			}
			candidate := *log
			switch r.mode {
			case "removed transfer":
				candidate.Removed = true
			case "transfer amount":
				candidate.Data = make([]byte, 32)
			case "transfer topic":
				candidate.Topics = append([]common.Hash(nil), log.Topics...)
				candidate.Topics[1] = common.Hash{}
			case "transfer position":
				candidate.TxIndex = 1
			}
			copied.Logs[index] = &candidate
		}
	}
	return &copied, nil
}
func (r *creationReceiptBoundaryReader) usdl(ctx context.Context) common.Address {
	return common.HexToAddress(deployment.Current().Roles["USDL"].Address)
}
func (r *creationReceiptBoundaryReader) ReportedBlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	if r.mode == "block unavailable" {
		return nil, context.DeadlineExceeded
	}
	block, err := r.PinnedRPCReader.ReportedBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if r.mode == "block transactions" {
		return types.NewBlockWithHeader(block.Header()), nil
	}
	return block, nil
}
func (r *creationReceiptBoundaryReader) HeaderByReportedHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	if r.mode == "parent unavailable" {
		return nil, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.HeaderByReportedHash(ctx, hash)
}
func (r *creationReceiptBoundaryReader) ReportedHeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, common.Hash, error) {
	if r.mode == "canonical unavailable" {
		return nil, common.Hash{}, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.ReportedHeaderByNumber(ctx, n)
}
func (r *creationReceiptBoundaryReader) CallContract(ctx context.Context, msg ethereum.CallMsg, n *big.Int) ([]byte, error) {
	if len(msg.Data) >= 4 && string(msg.Data[:4]) == string(crypto.Keccak256([]byte("balanceOf(address)"))[:4]) {
		r.balanceCall++
		if r.mode == "balance "+big.NewInt(int64(r.balanceCall)).String() {
			return nil, context.DeadlineExceeded
		}
	}
	if r.mode == "treasury read" && len(msg.Data) >= 4 && string(msg.Data[:4]) == string(crypto.Keccak256([]byte("treasury()"))[:4]) {
		return nil, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.CallContract(ctx, msg, n)
}

func TestCreationFeeReconciliationRejectsIncompleteEvidence(t *testing.T) {
	withSurfaceTreasuryCreationFees(t, func(ctx context.Context, engine *PreflightEngine, observer *creationFeeReadObserver, hash common.Hash, identity deployment.Role) {
		for _, mode := range []string{"receipt unavailable", "receipt status", "receipt transaction", "receipt block number", "block unavailable", "block transactions", "parent unavailable", "canonical unavailable", "nil log", "missing transfer", "removed transfer", "transfer amount", "transfer topic", "transfer position", "treasury read", "balance 1", "balance 2", "balance 3", "balance 4"} {
			t.Run(mode, func(t *testing.T) {
				reader := &creationReceiptBoundaryReader{PinnedRPCReader: observer.PinnedRPCReader, mode: mode}
				candidate := *engine
				candidate.reader = reader
				result, err := candidate.ReconcileCreationFee(ctx, hash, identity)
				if err == nil || result.TransactionHash != "" || !strings.Contains(err.Error(), "creation_fee_reconciliation_unverified") {
					t.Fatalf("incomplete fee evidence admitted: %+v %v", result, err)
				}
				if strings.HasPrefix(mode, "balance ") && reader.balanceCall == 0 {
					t.Fatal("fee evidence did not reach balance boundary")
				}
			})
		}
		for _, mode := range []string{"address", "runtime", "kind", "implementation"} {
			t.Run("treasury "+mode, func(t *testing.T) {
				changed := identity
				switch mode {
				case "address":
					changed.Address = common.Address{}.Hex()
				case "runtime":
					changed.RuntimeHash = common.HexToHash("0x01").Hex()
				case "kind":
					changed.Kind = "unknown"
				case "implementation":
					changed.ImplementationAddress = identity.Address
				}
				if _, err := engine.ReconcileCreationFee(ctx, hash, changed); err == nil {
					t.Fatal("unverified treasury admitted")
				}
			})
		}
	})
}
