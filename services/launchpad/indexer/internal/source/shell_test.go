package source

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

func TestShellReceiptSourceSetsAsideForeignReceipt(t *testing.T) {
	blockHash := common.HexToHash("0x" + strings.Repeat("1", 64))
	txHash := common.HexToHash("0x" + strings.Repeat("2", 64))
	shellHash := common.HexToHash("0x" + strings.Repeat("3", 64))
	height := uint64(7)
	regular := &ethtypes.Receipt{Status: 1, BlockHash: blockHash, BlockNumber: new(big.Int).SetUint64(height), TxHash: txHash, TransactionIndex: 0}
	regular.Bloom = ethtypes.CreateBloom(regular)
	shell := &ethtypes.Receipt{Type: receipt.ShellReceiptType, Status: 1, BlockHash: blockHash, BlockNumber: new(big.Int).SetUint64(height), TxHash: shellHash, TransactionIndex: 1}
	shell.Bloom = ethtypes.CreateBloom(shell)
	receipts := ethtypes.Receipts{regular, shell}
	got, err := separateShellReceipts(receipts, []common.Hash{txHash}, blockHash, height, strings.Repeat("0", ethtypes.BloomByteLength*2), ethtypes.DeriveSha(receipts, trie.NewStackTrie(nil)).Hex(), false)
	if err != nil || len(got) != 1 || got[0].TxHash != txHash {
		t.Fatalf("source receipt separation = %+v, %v", got, err)
	}
}
