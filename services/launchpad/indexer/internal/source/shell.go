package source

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

// separateShellReceipts applies the node's mixed receipt form to sources whose
// block wire format carries hashes rather than decoded transactions. Listed
// receipts are returned in listed-transaction order; shell logs are validated
// and then omitted from source records because no approved EVM role can emit
// them.
func separateShellReceipts(receipts ethtypes.Receipts, txHashes []common.Hash, blockHash common.Hash, height uint64, logsBloom, receiptsRoot string, reportedIdentity bool) (ethtypes.Receipts, error) {
	if len(txHashes) > 4096 || len(receipts) > 4096 || blockHash == (common.Hash{}) {
		return nil, sourceError(IntegrityError, "receipts", nil)
	}
	byHash := make(map[common.Hash]struct{}, len(txHashes))
	for _, hash := range txHashes {
		if hash == (common.Hash{}) {
			return nil, sourceError(IncompleteError, "receipts", nil)
		}
		if _, exists := byHash[hash]; exists {
			return nil, sourceError(IntegrityError, "receipts", nil)
		}
		byHash[hash] = struct{}{}
	}
	regularByHash := make(map[common.Hash]*ethtypes.Receipt, len(txHashes))
	shellHashes := map[common.Hash]struct{}{}
	logIndex := uint(0)
	totalBytes := 0
	blockNumber := new(big.Int).SetUint64(height)
	for _, r := range receipts {
		if r == nil || r.BlockHash != blockHash || r.BlockNumber == nil || r.BlockNumber.Cmp(blockNumber) != 0 || r.Status > 1 || len(r.Logs) > 4096 {
			return nil, sourceError(IntegrityError, "receipts", nil)
		}
		if receipt.IsShellReceipt(r) {
			if r.TxHash == (common.Hash{}) {
				return nil, sourceError(IntegrityError, "receipts", nil)
			}
			if _, listed := byHash[r.TxHash]; listed {
				return nil, sourceError(IntegrityError, "receipts", nil)
			}
			if _, duplicate := shellHashes[r.TxHash]; duplicate {
				return nil, sourceError(IntegrityError, "receipts", nil)
			}
			shellHashes[r.TxHash] = struct{}{}
		} else {
			if _, listed := byHash[r.TxHash]; !listed || regularByHash[r.TxHash] != nil {
				return nil, sourceError(IntegrityError, "receipts", nil)
			}
			regularByHash[r.TxHash] = r
		}
		for _, l := range r.Logs {
			if l == nil || l.Removed || l.BlockHash != blockHash || l.BlockNumber != height || l.TxHash != r.TxHash || l.TxIndex != r.TransactionIndex || l.Index != logIndex || logIndex > uint(^uint32(0)) || len(l.Topics) > 4 || len(l.Data) > 64<<10 {
				return nil, sourceError(IntegrityError, "receipts", nil)
			}
			totalBytes += 256 + len(l.Data) + 32*len(l.Topics)
			if totalBytes > 32<<20 {
				return nil, sourceError(OversizeError, "receipts", nil)
			}
			logIndex++
		}
		if r.Bloom != ethtypes.CreateBloom(r) {
			return nil, sourceError(IntegrityError, "receipts", nil)
		}
	}
	if len(regularByHash) != len(txHashes) {
		return nil, sourceError(IntegrityError, "receipts", nil)
	}
	regular := make(ethtypes.Receipts, len(txHashes))
	for i, hash := range txHashes {
		r := regularByHash[hash]
		if r == nil || r.TransactionIndex != uint(i) {
			return nil, sourceError(IntegrityError, "receipts", nil)
		}
		regular[i] = r
	}
	if reportedIdentity {
		bloomBytes, err := hex.DecodeString(strings.TrimPrefix(logsBloom, "0x"))
		if err != nil || len(bloomBytes) != ethtypes.BloomByteLength || ethtypes.MergeBloom(receipts) != ethtypes.BytesToBloom(bloomBytes) {
			return nil, sourceError(IntegrityError, "receipts", nil)
		}
	}
	if !reportedIdentity {
		if !validHash(receiptsRoot) || ethtypes.DeriveSha(receipts, trie.NewStackTrie(nil)) != common.HexToHash(receiptsRoot) {
			return nil, sourceError(IntegrityError, "receipts", nil)
		}
	}
	return regular, nil
}
