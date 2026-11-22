package receipt

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

// ShellReceiptType is the value go-ethereum retains after decoding the node's
// wire type 0xffffffff into Receipt.Type (uint8). Shell receipts represent
// chain-generated entries and have no listed EVM transaction.
const ShellReceiptType uint8 = ^uint8(0)

func IsShellReceipt(r *types.Receipt) bool {
	return r != nil && r.Type == ShellReceiptType
}

// SeparateShellReceipts authenticates a block receipt list and returns the
// receipts that belong to listed transactions in transaction order. Shell
// receipts are checked against the same block and log bounds, then set aside.
// The node numbers listed receipts compactly after shell entries are removed;
// log indexes remain one contiguous sequence across every receipt in the
// response, including the set-aside receipts.
func SeparateShellReceipts(transactions types.Transactions, receipts types.Receipts, blockHash common.Hash, blockNumber *big.Int, expectedBloom types.Bloom, expectedReceiptRoot common.Hash, reportedIdentity bool) (types.Receipts, types.Receipts, error) {
	if blockHash == (common.Hash{}) || blockNumber == nil || !blockNumber.IsUint64() {
		return nil, nil, ErrQualification
	}
	byHash := make(map[common.Hash]*types.Transaction, len(transactions))
	for _, tx := range transactions {
		if tx == nil || tx.Hash() == (common.Hash{}) {
			return nil, nil, ErrQualification
		}
		if _, exists := byHash[tx.Hash()]; exists {
			return nil, nil, ErrQualification
		}
		byHash[tx.Hash()] = tx
	}
	regularByHash := make(map[common.Hash]*types.Receipt, len(transactions))
	shells := make(types.Receipts, 0)
	logIndex := uint(0)
	totalBytes := 0
	for _, r := range receipts {
		if r == nil || r.BlockNumber == nil || r.BlockNumber.Cmp(blockNumber) != 0 || r.BlockHash != blockHash || r.Status > 1 {
			return nil, nil, ErrQualification
		}
		if IsShellReceipt(r) {
			if r.TxHash == (common.Hash{}) || byHash[r.TxHash] != nil || regularByHash[r.TxHash] != nil {
				return nil, nil, ErrQualification
			}
			for _, prior := range shells {
				if prior.TxHash == r.TxHash {
					return nil, nil, ErrQualification
				}
			}
			shells = append(shells, r)
		} else {
			tx := byHash[r.TxHash]
			if tx == nil || r.Type != tx.Type() || regularByHash[r.TxHash] != nil {
				return nil, nil, ErrQualification
			}
			regularByHash[r.TxHash] = r
		}
		for _, l := range r.Logs {
			if l == nil || l.Removed || l.BlockHash != blockHash || l.BlockNumber != blockNumber.Uint64() || l.TxHash != r.TxHash || l.TxIndex != r.TransactionIndex || l.Index != logIndex || logIndex > uint(^uint32(0)) || len(l.Topics) > 4 {
				return nil, nil, ErrQualification
			}
			totalBytes += 256 + len(l.Data) + 32*len(l.Topics)
			if totalBytes > maxBundleBytes {
				return nil, nil, ErrQualification
			}
			logIndex++
		}
		if r.Bloom != types.CreateBloom(r) {
			return nil, nil, ErrQualification
		}
	}
	if len(regularByHash) != len(transactions) {
		return nil, nil, ErrQualification
	}
	regular := make(types.Receipts, len(transactions))
	for i, tx := range transactions {
		r := regularByHash[tx.Hash()]
		if r == nil || r.TransactionIndex != uint(i) {
			return nil, nil, ErrQualification
		}
		regular[i] = r
	}
	// The eth header bloom covers listed receipts only; shell logs are absent.
	// Ethereum identity commits the trie of those same listed receipts. Reported
	// identity carries the Comet block-results root, not an Ethereum receipt trie.
	if types.MergeBloom(regular) != expectedBloom || !reportedIdentity && types.DeriveSha(regular, trie.NewStackTrie(nil)) != expectedReceiptRoot {
		return nil, nil, ErrQualification
	}
	return regular, shells, nil
}
