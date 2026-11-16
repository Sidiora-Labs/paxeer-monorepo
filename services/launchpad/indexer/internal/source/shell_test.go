package source

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	got, err := separateShellReceipts(receipts, []common.Hash{txHash}, blockHash, height, strings.Repeat("0", ethtypes.BloomByteLength*2), ethtypes.DeriveSha(ethtypes.Receipts{regular}, trie.NewStackTrie(nil)).Hex(), false)
	if err != nil || len(got) != 1 || got[0].TxHash != txHash {
		t.Fatalf("source receipt separation = %+v, %v", got, err)
	}
}

func TestShellReceiptSourceHeaderCoverage(t *testing.T) {
	for _, reported := range []bool{false, true} {
		for _, name := range append(chainfixture.ShellCases(), "mixed receipt root", "wrong receipt root") {
			t.Run(fmt.Sprintf("%s/reported=%t", name, reported), func(t *testing.T) {
				baseName := name
				if name == "mixed receipt root" || name == "wrong receipt root" {
					baseName = "mixed"
				}
				block := chainfixture.NewShellCase(t, baseName)
				hashes := make([]common.Hash, len(block.Transactions))
				for i, tx := range block.Transactions {
					hashes[i] = tx.Hash()
				}
				listed := block.Receipts[:len(hashes)]
				root := ethtypes.DeriveSha(listed, trie.NewStackTrie(nil))
				if name == "mixed receipt root" {
					root = ethtypes.DeriveSha(block.Receipts, trie.NewStackTrie(nil))
					block.Reject = !reported
				}
				if name == "wrong receipt root" {
					root = common.HexToHash("0xbad")
					block.Reject = !reported
				}
				regular, err := separateShellReceipts(block.Receipts, hashes, block.Hash, block.Header.Number.Uint64(), hexutil.Encode(block.Header.Bloom[:]), root.Hex(), reported)
				if block.Reject {
					var se *SourceError
					if !errors.As(err, &se) || se.Kind != IntegrityError {
						t.Fatalf("receipt coverage: %v", err)
					}
				} else if err != nil || len(regular) != len(hashes) {
					t.Fatalf("source listed receipts=%d: %v", len(regular), err)
				} else {
					for i, r := range regular {
						if r != listed[i] {
							t.Fatal("listed receipt changed")
						}
					}
				}
			})
		}
	}
}

func TestShellReceiptCommittedSourceReceiptCoverage(t *testing.T) {
	for _, name := range chainfixture.ShellCases() {
		t.Run(name, func(t *testing.T) {
			block := chainfixture.NewShellCase(t, name)
			ctx := context.Background()
			endpoint := block.Serve(t)
			manifest := strings.Repeat("a", 64)
			address := block.Receipts[0].Logs[0].Address
			height := block.Header.Number.Uint64()
			s, err := NewCommittedSource(ctx, CommittedOptions{
				Endpoints:     map[string]string{"primary": endpoint},
				Anchor:        &CommittedAnchor{Height: height - 1, Hash: block.AnchorHash},
				BlockIdentity: valueReported, ManifestDigest: manifest,
				Epochs: []AddressEpoch{{Address: address.Hex(), Role: "EventEmitter", ABIRevision: "recorded", ManifestDigest: manifest, ActivationHeight: height}},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := s.Close(); err != nil {
					t.Error(err)
				}
			}()
			e := s.endpoints[0]
			wire, record, err := s.block(ctx, e, fmtHeight(height), false)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.receiptLogs(ctx, e, wire, record, height, []common.Address{address}, false)
			if block.Reject {
				var se *SourceError
				if !errors.As(err, &se) || se.Kind != IntegrityError {
					t.Fatalf("invalid receipt coverage admitted: %v", err)
				}
				return
			}
			want := 0
			for _, r := range block.Receipts[:len(block.Transactions)] {
				for _, l := range r.Logs {
					if l.Address == address {
						want++
					}
				}
			}
			if err != nil || len(got) != want {
				t.Fatalf("source logs=%d want=%d: %v", len(got), want, err)
			}
			for _, l := range got {
				if len(block.Transactions) == 0 || l.TxHash != block.Transactions[0].Hash() {
					t.Fatal("shell log entered source records")
				}
			}
		})
	}
}
