package receipt

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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
	log.TxIndex = 1
	shell.TransactionIndex = 1
	return types.Transactions{tx}, types.Receipts{regular, shell}, blockHash, height
}

func ptrAddress(a common.Address) *common.Address { return &a }

func TestShellReceiptBlockWithoutTransactionsCommits(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	bloom := types.MergeBloom(listedShellFixtureReceipts(receipts))
	regular, shells, err := SeparateShellReceipts(txs, receipts, hash, height, bloom, types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err != nil || len(regular) != 0 || len(shells) != 1 {
		t.Fatalf("shell-only block = regular %d shells %d err %v", len(regular), len(shells), err)
	}
}

func TestShellReceiptMixedBlockBindsListedTransactions(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, true)
	regular, shells, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(listedShellFixtureReceipts(receipts)), types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err != nil || len(regular) != 1 || len(shells) != 1 || regular[0].TxHash != txs[0].Hash() {
		t.Fatalf("mixed block = regular %d shells %d err %v", len(regular), len(shells), err)
	}
}

func TestShellReceiptRejectsForeignRegularReceipt(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, true)
	receipts[0].TxHash = common.HexToHash("0x" + strings.Repeat("7", 64))
	receipts[0].Bloom = types.CreateBloom(receipts[0])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(listedShellFixtureReceipts(receipts)), types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("foreign regular receipt was admitted")
	}
}

func TestShellReceiptRejectsListedHash(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, true)
	receipts[1].TxHash = txs[0].Hash()
	receipts[1].Logs[0].TxHash = receipts[1].TxHash
	receipts[1].Bloom = types.CreateBloom(receipts[1])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(listedShellFixtureReceipts(receipts)), types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("shell receipt claiming a listed transaction was admitted")
	}
}

func TestShellReceiptRejectsForeignBlock(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	receipts[0].BlockHash = common.HexToHash("0x" + strings.Repeat("8", 64))
	receipts[0].Bloom = types.CreateBloom(receipts[0])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(listedShellFixtureReceipts(receipts)), types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("shell receipt with a foreign block was admitted")
	}
}

func TestShellReceiptRejectsDuplicateHash(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	receipts = append(receipts, receipts[0])
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(listedShellFixtureReceipts(receipts)), types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err == nil {
		t.Fatal("duplicate shell receipt was admitted")
	}
}

func TestShellReceiptCountIncludesSetAside(t *testing.T) {
	txs, receipts, hash, height := shellReceiptFixture(t, false)
	for i := 1; i < 4097; i++ {
		r := *receipts[0]
		r.TxHash = common.BigToHash(big.NewInt(int64(i)))
		r.Logs = nil
		r.Bloom = types.CreateBloom(&r)
		receipts = append(receipts, &r)
	}
	_, _, err := SeparateShellReceipts(txs, receipts, hash, height, types.MergeBloom(listedShellFixtureReceipts(receipts)), types.DeriveSha(listedShellFixtureReceipts(receipts), trie.NewStackTrie(nil)), false)
	if err != nil {
		t.Fatalf("shell receipt content: %v", err)
	}
}

func listedShellFixtureReceipts(receipts types.Receipts) types.Receipts {
	var listed types.Receipts
	for _, r := range receipts {
		if !IsShellReceipt(r) {
			listed = append(listed, r)
		}
	}
	return listed
}

func TestShellReceiptHeaderCoverage(t *testing.T) {
	for _, reported := range []bool{false, true} {
		for _, name := range []string{"shell only", "mixed", "missing listed bloom", "extra header bloom", "shell header bloom", "shell own bloom", "listed own bloom", "mixed receipt root", "wrong receipt root"} {
			t.Run(fmt.Sprintf("%s/reported=%t", name, reported), func(t *testing.T) {
				txs, receipts, hash, height := shellReceiptFixture(t, name != "shell only")
				listed := listedShellFixtureReceipts(receipts)
				bloom := types.MergeBloom(listed)
				root := types.DeriveSha(listed, trie.NewStackTrie(nil))
				wantErr := name != "shell only" && name != "mixed"
				switch name {
				case "missing listed bloom":
					bloom = types.Bloom{}
				case "extra header bloom":
					bloom.Add([]byte("unreceipted log"))
				case "shell header bloom":
					bloom = types.MergeBloom(receipts)
				case "shell own bloom":
					receipts[len(receipts)-1].Bloom = types.Bloom{}
				case "listed own bloom":
					receipts[0].Bloom = types.Bloom{}
				case "mixed receipt root":
					root = types.DeriveSha(receipts, trie.NewStackTrie(nil))
					wantErr = !reported
				case "wrong receipt root":
					root = common.HexToHash("0xbad")
					wantErr = !reported
				}
				regular, shells, err := SeparateShellReceipts(txs, receipts, hash, height, bloom, root, reported)
				if wantErr {
					if !errors.Is(err, ErrQualification) {
						t.Fatalf("receipt coverage: %v", err)
					}
				} else if err != nil || len(regular) != len(txs) || len(shells) != 1 {
					t.Fatalf("receipt coverage regular=%d shells=%d: %v", len(regular), len(shells), err)
				} else if len(regular) > 0 && (regular[0] != receipts[0] || regular[0].TransactionIndex != 0 || regular[0].Logs[0].Index != 0) {
					t.Fatal("listed receipt coordinates changed")
				}
			})
		}
	}
}

func TestShellReceiptAuthorityHeaderCoverage(t *testing.T) {
	for _, name := range chainfixture.ShellCases() {
		t.Run(name, func(t *testing.T) {
			block := chainfixture.NewShellCase(t, name)
			if name == "mixed" {
				// Keep this authority case focused on receipt membership: the
				// listed receipt is valid with no logs, while the shell receipt
				// carries the set-aside log material.
				block.Receipts[0].Logs = []*types.Log{}
				block.Receipts[0].Bloom = types.CreateBloom(block.Receipts[0])
				block.Header.Bloom = types.MergeBloom(block.Receipts[:1])
				for i, log := range block.Receipts[1].Logs {
					log.Index = uint(i)
				}
			}
			ctx := context.Background()
			client, err := ethclient.DialContext(ctx, block.Serve(t))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			authority := &RPCQualification{client: client, options: RPCOptions{BlockIdentity: BlockIdentityReported, Timeout: 15 * time.Second}}
			height := block.Header.Number.Uint64()
			pinned, regular, skipped, err := authority.pinnedBlockAtHeight(ctx, height, nil)
			if block.Reject {
				if !errors.Is(err, ErrQualification) {
					t.Fatalf("invalid receipt coverage admitted: %v", err)
				}
				return
			}
			if err != nil || skipped || pinned.Hash != block.Hash || len(regular) != len(block.Transactions) {
				t.Fatalf("authority receipts=%d skipped=%t: %v", len(regular), skipped, err)
			}
			for i, r := range regular {
				if !receiptEqual(r, block.Receipts[i]) {
					t.Fatal("listed receipt changed during qualification")
				}
				bundle := Bundle{Header: pinned.Header, BlockHash: pinned.Hash, Transaction: pinned.Transactions[i], Receipt: r}
				if err := authority.ValidateReceipt(ctx, bundle); err != nil {
					t.Fatalf("listed receipt qualification: %v", err)
				}
			}
			_, regular, err = authority.pinnedReceipts(ctx, pinned)
			if err != nil || len(regular) != len(block.Transactions) {
				t.Fatalf("pinned receipt coverage: %v", err)
			}
		})
	}
}
