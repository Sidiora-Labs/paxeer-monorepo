package chain

import (
	"errors"
	"math/big"
	"strconv"

	contractv2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// RPCReceipt carries RPC receipt evidence for the shared public projection.
// Logs preserve event positions in the RPC-to-public-receipt projection.
type RPCReceipt struct {
	TransactionHash   common.Hash     `json:"transactionHash"`
	TransactionIndex  hexutil.Uint64  `json:"transactionIndex"`
	BlockHash         common.Hash     `json:"blockHash"`
	BlockNumber       *hexutil.Big    `json:"blockNumber"`
	From              common.Address  `json:"from"`
	To                *common.Address `json:"to"`
	Status            *hexutil.Uint64 `json:"status"`
	GasUsed           *hexutil.Big    `json:"gasUsed"`
	EffectiveGasPrice *hexutil.Big    `json:"effectiveGasPrice"`
	Logs              []*types.Log    `json:"logs"`
}

// TxStatus uses the same projection as Client.ReceiptState. gasPrice is the
// effective price resolved by the caller; it must not be an invented default.
func (r *RPCReceipt) TxStatus(gasPrice *hexutil.Big) (contractv2.TxStatusData, error) {
	if r == nil || r.Status == nil || r.GasUsed == nil || !(*big.Int)(r.GasUsed).IsUint64() {
		return contractv2.TxStatusData{}, errors.New("malformed receipt")
	}
	return projectCommittedReceipt(&types.Receipt{
		TxHash: r.TransactionHash, TransactionIndex: uint(r.TransactionIndex),
		BlockHash: r.BlockHash, BlockNumber: (*big.Int)(r.BlockNumber),
		Status: uint64(*r.Status), GasUsed: (*big.Int)(r.GasUsed).Uint64(),
		EffectiveGasPrice: (*big.Int)(gasPrice), Logs: r.Logs,
	}, r.From, r.To)
}

func projectCommittedReceipt(receipt *types.Receipt, from common.Address, to *common.Address) (contractv2.TxStatusData, error) {
	if receipt == nil || receipt.TxHash == (common.Hash{}) || receipt.BlockHash == (common.Hash{}) || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.Status > 1 || receipt.EffectiveGasPrice == nil || receipt.EffectiveGasPrice.Sign() < 0 || receipt.EffectiveGasPrice.BitLen() > 256 {
		return contractv2.TxStatusData{}, errors.New("malformed receipt")
	}
	// The frozen v2 Receipt requires logIndex. For log-free transactions its
	// legacy encoding is zero, with identity supplied by the transaction index
	// and hash. This is not evidence of an event at log zero. Event-bearing
	// receipts retain their actual first block-global log index.
	logIndex := uint(0)
	for i, log := range receipt.Logs {
		if log == nil || log.Removed || log.TxHash != receipt.TxHash || log.BlockHash != receipt.BlockHash || log.BlockNumber != receipt.BlockNumber.Uint64() || log.TxIndex != receipt.TransactionIndex || (i > 0 && log.Index <= receipt.Logs[i-1].Index) {
			return contractv2.TxStatusData{}, errors.New("receipt log identity mismatch")
		}
		if i == 0 {
			logIndex = log.Index
		}
	}
	position := contractv2.ChainPosition{
		ChainId: "125", Height: contractv2.UInt256(receipt.BlockNumber.String()),
		BlockHash: receipt.BlockHash.Hex(), TransactionHash: receipt.TxHash.Hex(),
		TransactionIndex: contractv2.UInt256(strconv.FormatUint(uint64(receipt.TransactionIndex), 10)),
		LogIndex:         contractv2.UInt256(strconv.FormatUint(uint64(logIndex), 10)),
	}
	var destination *string
	if to != nil {
		address := to.Hex()
		destination = &address
	}
	status := "included_success"
	if receipt.Status == types.ReceiptStatusFailed {
		status = "included_reverted"
	}
	return contractv2.TxStatusData{Status: status, Receipt: &contractv2.Receipt{
		TransactionHash: receipt.TxHash.Hex(), BlockHash: receipt.BlockHash.Hex(),
		Height: position.Height, Position: position, From: from.Hex(), To: destination,
		Success:              receipt.Status == types.ReceiptStatusSuccessful,
		GasUsedRaw:           contractv2.UInt256(strconv.FormatUint(receipt.GasUsed, 10)),
		EffectiveGasPriceRaw: contractv2.UInt256(receipt.EffectiveGasPrice.String()),
	}}, nil
}
