package chainfixture

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const shellOnlyCase = "shell only"

// ShellBlock applies the shell receipt wire form to recorded block and log
// material. The listed receipts retain their original signed transactions.
type ShellBlock struct {
	Fixture      *Fixture
	Header       *types.Header
	Hash         common.Hash
	AnchorHash   common.Hash
	Transactions types.Transactions
	Receipts     types.Receipts
	Reject       bool
}

func ShellCases() []string {
	return []string{shellOnlyCase, "mixed", "missing listed bloom", "extra header bloom", "shell own bloom"}
}

func NewShellCase(t testing.TB, name string) *ShellBlock {
	t.Helper()
	b := NewShellBlock(t, name != shellOnlyCase)
	switch name {
	case shellOnlyCase, "mixed":
	case "missing listed bloom":
		b.Header.Bloom = types.Bloom{}
		b.Reject = true
	case "extra header bloom":
		b.Header.Bloom.Add([]byte("unreceipted log"))
		b.Reject = true
	case "shell own bloom":
		b.Receipts[len(b.Receipts)-1].Bloom = types.Bloom{}
		b.Reject = true
	default:
		t.Fatalf("unknown shell receipt case %q", name)
	}
	return b
}

func NewShellBlock(t testing.TB, withTransaction bool) *ShellBlock {
	t.Helper()
	f := New(t)
	var header types.Header
	var block struct {
		Hash         common.Hash
		Transactions []*types.Transaction
	}
	raw := f.entries[`eth_getBlockByNumber ["0x2063ec4",true]`].Result
	if err := json.Unmarshal(raw, &header); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	encodedHash, err := json.Marshal(block.Hash)
	if err != nil {
		t.Fatal(err)
	}
	key := Key("eth_getBlockByHash", []json.RawMessage{encodedHash, json.RawMessage("true")})
	f.entries[key] = recorded{Key: key, Result: raw}
	encodedNumber := hexutil.EncodeBig(header.Number)
	encodedNumberJSON, err := json.Marshal(encodedNumber)
	if err != nil {
		t.Fatal(err)
	}
	numberKey := Key("eth_getBlockByNumber", []json.RawMessage{encodedNumberJSON, json.RawMessage("false")})
	f.entries[numberKey] = recorded{Key: numberKey, Result: raw}
	var anchor struct {
		Hash         common.Hash
		Transactions []json.RawMessage
	}
	if err := json.Unmarshal(f.entries[`eth_getBlockByNumber ["0x2063ec3",false]`].Result, &anchor); err != nil {
		t.Fatal(err)
	}
	if anchor.Hash == (common.Hash{}) || anchor.Transactions == nil || len(anchor.Transactions) != 0 {
		t.Fatal("recorded empty anchor required")
	}
	regular := make(types.Receipts, 0, 1)
	selector, err := json.Marshal(map[string]any{"blockHash": block.Hash, "requireCanonical": true})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(f.entries[Key("eth_getBlockReceipts", []json.RawMessage{selector})].Result, &regular); err != nil {
		t.Fatal(err)
	}
	if len(block.Transactions) != 1 || len(regular) != 1 || len(regular[0].Logs) == 0 {
		t.Fatal("recorded listed receipt required")
	}
	// A different recorded execution supplies the shell's log material.
	var shell *types.Receipt
	for key, entry := range f.entries {
		if key == `eth_getBlockReceipts [{"blockHash":"0x1cd6b450aa87745be16a8760a928a2e7ddd1097e872bdcd99b31ff66f77c2945","requireCanonical":true}]` {
			var receipts types.Receipts
			if err := json.Unmarshal(entry.Result, &receipts); err != nil {
				t.Fatal(err)
			}
			if len(receipts) != 1 || len(receipts[0].Logs) == 0 {
				t.Fatal("recorded shell log material required")
			}
			shell = receipts[0]
		}
	}
	if shell == nil {
		t.Fatal("recorded shell log material unavailable")
	}
	if !withTransaction {
		block.Transactions = nil
		regular = nil
		header.Bloom = types.Bloom{}
		header.TxHash = types.EmptyTxsHash
	}
	shell.Type = ^uint8(0)
	shell.BlockHash = block.Hash
	shell.BlockNumber = new(big.Int).Set(header.Number)
	shell.TxHash = crypto.Keccak256Hash(block.Hash[:], []byte("shell"))
	shell.TransactionIndex = uint(len(block.Transactions))
	logIndex := uint(0)
	for _, r := range regular {
		logIndex += uint(len(r.Logs))
	}
	for _, l := range shell.Logs {
		l.BlockHash = block.Hash
		l.BlockNumber = header.Number.Uint64()
		l.TxHash = shell.TxHash
		l.TxIndex = shell.TransactionIndex
		l.Index = logIndex
		logIndex++
	}
	return &ShellBlock{Fixture: f, Header: &header, Hash: block.Hash, AnchorHash: anchor.Hash, Transactions: block.Transactions, Receipts: append(regular, shell)}
}

// Serve preserves the recorded responses except for this block's shell form.
func (b *ShellBlock) Serve(t testing.TB) string {
	t.Helper()
	for key, entry := range b.Fixture.entries {
		var block map[string]json.RawMessage
		if json.Unmarshal(entry.Result, &block) == nil && string(block["hash"]) == `"`+b.Hash.Hex()+`"` {
			block["logsBloom"], _ = json.Marshal(b.Header.Bloom)
			block["receiptsRoot"], _ = json.Marshal(b.Header.ReceiptHash)
			block["transactionsRoot"], _ = json.Marshal(b.Header.TxHash)
			var txs []json.RawMessage
			if err := json.Unmarshal(block["transactions"], &txs); err != nil {
				t.Fatal(err)
			}
			if len(b.Transactions) == 0 {
				txs = []json.RawMessage{}
			}
			block["transactions"], _ = json.Marshal(txs)
			entry.Result, _ = json.Marshal(block)
			b.Fixture.entries[key] = entry
		}
	}
	wire := make([]map[string]json.RawMessage, len(b.Receipts))
	for i, r := range b.Receipts {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &wire[i]); err != nil {
			t.Fatal(err)
		}
		if r.Type == ^uint8(0) {
			wire[i]["type"] = json.RawMessage(`"0xffffffff"`)
		}
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	for _, selector := range []any{b.Hash, map[string]any{"blockHash": b.Hash, "requireCanonical": true}} {
		encoded, err := json.Marshal(selector)
		if err != nil {
			t.Fatal(err)
		}
		key := Key("eth_getBlockReceipts", []json.RawMessage{encoded})
		b.Fixture.entries[key] = recorded{Key: key, Result: raw}
	}
	return b.Fixture.Serve(t)
}
