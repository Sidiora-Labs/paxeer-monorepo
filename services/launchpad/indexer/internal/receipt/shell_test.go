package receipt

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

func shellReceiptFixture(t *testing.T, withTransaction bool) (types.Transactions, types.Receipts, common.Hash, *big.Int) {
	t.Helper()
	blockHash := common.HexToHash("0x" + strings.Repeat("1", 64))
	height := big.NewInt(7)
	tx := types.NewTx(&types.LegacyTx{Nonce: 1, Gas: 21000, GasPrice: big.NewInt(1), To: ptrAddress(common.HexToAddress("0x" + strings.Repeat("2", 40)))})
	log := &types.Log{Address: common.HexToAddress("0x" + strings.Repeat("3", 40)), BlockHash: blockHash, BlockNumber: height.Uint64(), TxHash: common.HexToHash("0x" + strings.Repeat("4", 64)), TxIndex: 0, Index: 0, Topics: []common.Hash{common.HexToHash("0x" + strings.Repeat("5", 64))}}
	shell := &types.Receipt{Type: ShellReceiptType, Status: 1, BlockHash: blockHash, BlockNumber: new(big.Int).Set(height), TxHash: log.TxHash, TransactionIndex: 0, Logs: []*types.Log{log}}
	shell.Bloom = types.CreateBloom(shell)
	if !withTransaction {
		return nil, types.Receipts{shell}, blockHash, height
	}
	txLog := &types.Log{Address: common.HexToAddress("0x" + strings.Repeat("6", 40)), BlockHash: blockHash, BlockNumber: height.Uint64(), TxHash: tx.Hash(), TxIndex: 0, Index: 0}
	regular := &types.Receipt{Status: 1, BlockHash: blockHash, BlockNumber: new(big.Int).Set(height), TxHash: tx.Hash(), TransactionIndex: 0, Logs: []*types.Log{txLog}}
	regular.Bloom = types.CreateBloom(regular)
	// The node returns the listed receipt first and the set-aside shell second;
	// the log indexes still span both receipts.
	log.Index = 1
	return types.Transactions{tx}, types.Receipts{regular, shell}, blockHash, height
}

func ptrAddress(a common.Address) *common.Address { return &a }

func TestShellReceiptBlockWithoutTransactionsCommits(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	bloom := types.MergeBloom(receipts)
	regular, shells, err := SeparateShellReceipts(txs, receipts, hash, height, bloom, types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err != nil || len(regular) != 0 || len(shells) != 1 {
		t.Fatalf("shell-only block = regular %d shells %d err %v", len(regular), len(shells), err)
	}
}

func TestShellReceiptMixedBlockBindsListedTransactions(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, true)
	regular, shells, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(receipts), types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err != nil || len(regular) != 1 || len(shells) != 1 || regular[0].TxHash != txs[0].Hash() {
		t.Fatalf("mixed block = regular %d shells %d err %v", len(regular), len(shells), err)
	}
}

func TestShellReceiptRejectsForeignRegularReceipt(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, true)
	receipts[0].TxHash = common.HexToHash("0x" + strings.Repeat("7", 64))
	receipts[0].Bloom = types.CreateBloom(receipts[0])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(receipts), types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("foreign regular receipt was admitted")
	}
}

func TestShellReceiptRejectsListedHash(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, true)
	receipts[1].TxHash = txs[0].Hash()
	receipts[1].Logs[0].TxHash = receipts[1].TxHash
	receipts[1].Bloom = types.CreateBloom(receipts[1])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(receipts), types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("shell receipt claiming a listed transaction was admitted")
	}
}

func TestShellReceiptRejectsForeignBlock(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	receipts[0].BlockHash = common.HexToHash("0x" + strings.Repeat("8", 64))
	receipts[0].Bloom = types.CreateBloom(receipts[0])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(receipts), types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("shell receipt with a foreign block was admitted")
	}
}

func TestShellReceiptRejectsDuplicateHash(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	receipts = append(receipts, receipts[0])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(receipts), types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("duplicate shell receipt was admitted")
	}
}

func TestShellReceiptCountLimitIncludesSetAside(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	for i := 1; i < 4097; i++ {
		r := *receipts[0]
		r.TxHash = common.BigToHash(big.NewInt(int64(i)))
		r.Logs = nil
		r.Bloom = types.CreateBloom(&r)
		receipts = append(receipts, &r)
	}
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(receipts), types.DeriveSha(receipts, trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("receipt count over the limit was admitted")
	}
}
