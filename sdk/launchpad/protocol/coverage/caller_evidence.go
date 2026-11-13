package coverage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
)

// NativeLaunchpadEmitter is reserved to the native launchpad producer. A native
// Swap has a logical pool caller, but does not execute an emitSwap EVM frame.
var NativeLaunchpadEmitter = common.HexToAddress("0x1017")

type CallFrame struct {
	Type  string         `json:"type"`
	From  common.Address `json:"from"`
	To    common.Address `json:"to"`
	Input hexutil.Bytes  `json:"input"`
	Value *hexutil.Big   `json:"value,omitempty"`
	Error string         `json:"error,omitempty"`
	Calls []CallFrame    `json:"calls,omitempty"`
	Logs  []CallFrameLog `json:"logs,omitempty"`
}

type CallFrameLog struct {
	Address  common.Address `json:"address"`
	Topics   []common.Hash  `json:"topics"`
	Data     hexutil.Bytes  `json:"data"`
	Position *hexutil.Uint  `json:"position"`
}

type ExecutionTraceSource interface {
	TraceTransactionInBlock(context.Context, common.Hash, common.Hash) (*CallFrame, error)
}

type TraceRPC interface {
	CallContext(context.Context, any, string, ...any) error
}

type RPCTraceSource struct{ RPC TraceRPC }

func (s *RPCTraceSource) TraceTransactionInBlock(ctx context.Context, block, tx common.Hash) (*CallFrame, error) {
	if s == nil || s.RPC == nil || block == (common.Hash{}) || tx == (common.Hash{}) {
		return nil, fmt.Errorf("block-bound execution trace required")
	}
	var traces []struct {
		TransactionHash common.Hash `json:"txHash"`
		Result          *CallFrame  `json:"result"`
		Error           string      `json:"error"`
	}
	// Trace the committed block, never the pending transaction or latest state.
	err := s.RPC.CallContext(ctx, &traces, "debug_traceBlockByHash", block, map[string]any{
		"tracer": "callTracer", "tracerConfig": map[string]any{"withLog": true},
	})
	if err != nil {
		return nil, fmt.Errorf("block execution trace: %w", err)
	}
	var frame *CallFrame
	for _, trace := range traces {
		if trace.TransactionHash != tx {
			continue
		}
		if frame != nil || trace.Error != "" || trace.Result == nil {
			return nil, fmt.Errorf("ambiguous or failed block transaction trace")
		}
		frame = trace.Result
	}
	if frame == nil {
		return nil, fmt.Errorf("transaction absent from block trace")
	}
	return frame, nil
}

type tracedLog struct {
	log      CallFrameLog
	caller   common.Address
	selector [4]byte
}

func callerFromTrace(root *CallFrame, tx *types.Transaction, receipt *types.Receipt, id Identity) (common.Address, [4]byte, error) {
	fail := func() (common.Address, [4]byte, error) {
		return common.Address{}, [4]byte{}, fmt.Errorf("execution frame or receipt log association differs")
	}
	sender, err := types.Sender(types.LatestSignerForChainID(big.NewInt(125)), tx)
	if err != nil || root == nil || tx.To() == nil || root.Type != "CALL" || root.Error != "" || root.From != sender || root.To != *tx.To() || !bytes.Equal(root.Input, tx.Data()) || root.Value == nil || (*big.Int)(root.Value).Cmp(tx.Value()) != 0 {
		return fail()
	}
	var logs []tracedLog
	frames, payload := 0, 0
	var walk func(*CallFrame, common.Address, common.Address, bool, int) error
	walk = func(f *CallFrame, address, caller common.Address, static bool, depth int) error {
		frames++
		payload += len(f.Input)
		if frames > 65536 || depth > 1024 || payload > MaxMeasurementInput {
			return fmt.Errorf("execution trace bound exceeded")
		}
		if f.Error != "" {
			// Reverted descendants cannot supply successful receipt evidence.
			var hasLogs func(*CallFrame) bool
			hasLogs = func(frame *CallFrame) bool {
				if len(frame.Logs) != 0 {
					return true
				}
				for i := range frame.Calls {
					if hasLogs(&frame.Calls[i]) {
						return true
					}
				}
				return false
			}
			if hasLogs(f) {
				return fmt.Errorf("reverted execution has logs")
			}
			return nil
		}
		var selector [4]byte
		if len(f.Input) >= 4 {
			copy(selector[:], f.Input[:4])
		}
		next := 0
		for i := 0; i <= len(f.Calls); i++ {
			for next < len(f.Logs) {
				l := f.Logs[next]
				if l.Position == nil || uint64(*l.Position) > uint64(len(f.Calls)) || (next > 0 && *l.Position < *f.Logs[next-1].Position) {
					return fmt.Errorf("trace log position absent or unordered")
				}
				if int(*l.Position) != i {
					break
				}
				payload += len(l.Data) + 32*len(l.Topics)
				if static || l.Address != address || len(l.Topics) > 4 || len(logs) >= MaxReceiptLogs || payload > MaxMeasurementInput {
					return fmt.Errorf("trace log context differs")
				}
				logs = append(logs, tracedLog{l, caller, selector})
				next++
			}
			if i == len(f.Calls) {
				break
			}
			child := &f.Calls[i]
			if child.From != address {
				return fmt.Errorf("trace caller ancestry differs")
			}
			childAddress, childCaller, childStatic := child.To, address, static
			switch child.Type {
			case "CALL", "CREATE", "CREATE2":
			case "STATICCALL":
				childStatic = true
			case "DELEGATECALL":
				childAddress, childCaller = address, caller
			case "CALLCODE":
				childAddress = address
			case "SELFDESTRUCT":
				if len(child.Logs) != 0 || len(child.Calls) != 0 {
					return fmt.Errorf("invalid selfdestruct frame")
				}
				continue
			default:
				return fmt.Errorf("unsupported execution frame")
			}
			if err := walk(child, childAddress, childCaller, childStatic, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, root.To, sender, false, 0); err != nil {
		return common.Address{}, [4]byte{}, err
	}
	if len(logs) != len(receipt.Logs) {
		return fail()
	}
	var result *tracedLog
	for i, l := range receipt.Logs {
		if l == nil || l.Removed || l.BlockHash != id.BlockHash || l.TxHash != id.TransactionHash || l.BlockNumber != receipt.BlockNumber.Uint64() || l.TxIndex != receipt.TransactionIndex || (i > 0 && l.Index != receipt.Logs[i-1].Index+1) || !sameRawLog(logs[i].log, l) {
			return fail()
		}
		if l.Index == id.LogIndex && l.Address == id.Emitter {
			result = &logs[i]
		}
	}
	if result == nil || result.caller == (common.Address{}) || result.selector == ([4]byte{}) {
		return fail()
	}
	return result.caller, result.selector, nil
}

func sameRawLog(a CallFrameLog, b *types.Log) bool {
	if b == nil || a.Address != b.Address || !bytes.Equal(a.Data, b.Data) || len(a.Topics) != len(b.Topics) {
		return false
	}
	for i := range a.Topics {
		if a.Topics[i] != b.Topics[i] {
			return false
		}
	}
	return true
}

func (s *RPCSource) AuthenticateReceipt(ctx context.Context, receipt *types.Receipt) error {
	if s == nil || s.RPC == nil || receipt == nil {
		return fmt.Errorf("authenticated receipt required")
	}
	observed, err := s.RPC.TransactionReceipt(ctx, receipt.TxHash)
	if err != nil {
		return err
	}
	// Include RPC coordinates as well as consensus-encoded payloads: the caller
	// evidence must authorize precisely the bytes consumed by Correlate.
	a, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	b, err := json.Marshal(observed)
	if err != nil || observed == nil || !bytes.Equal(a, b) {
		return fmt.Errorf("consumer receipt differs from authenticated RPC receipt")
	}
	return nil
}

type NativeReceiptSource interface {
	VerifyNativeReceipt(context.Context, *types.Header, *types.Transaction, *types.Receipt) error
}

type NativeBlockRPC interface {
	BlockByHash(context.Context, common.Hash) (*types.Block, error)
	BlockReceipts(context.Context, rpc.BlockNumberOrHash) ([]*types.Receipt, error)
}

type RPCNativeReceiptSource struct{ RPC NativeBlockRPC }

func (s *RPCNativeReceiptSource) VerifyNativeReceipt(ctx context.Context, header *types.Header, tx *types.Transaction, receipt *types.Receipt) error {
	if s == nil || s.RPC == nil || header == nil || header.Number == nil || !header.Number.IsUint64() || tx == nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil || receipt.BlockNumber.Cmp(header.Number) != 0 || receipt.TxHash != tx.Hash() || receipt.BlockHash != header.Hash() {
		return fmt.Errorf("native committed receipt required")
	}
	block, err := s.RPC.BlockByHash(ctx, receipt.BlockHash)
	if err != nil {
		return err
	}
	if block == nil || block.Hash() != header.Hash() || len(block.Transactions()) > MaxReceiptLogs || uint64(receipt.TransactionIndex) >= uint64(len(block.Transactions())) || block.Transactions()[receipt.TransactionIndex].Hash() != tx.Hash() || consensusListRoot(block.Transactions()) != header.TxHash {
		return fmt.Errorf("native transaction inclusion differs")
	}
	receipts, err := s.RPC.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(receipt.BlockHash, false))
	if err != nil {
		return err
	}
	if len(receipts) != len(block.Transactions()) {
		return fmt.Errorf("native block receipts incomplete")
	}
	var logIndex uint
	for i, r := range receipts {
		if r == nil || r.TxHash != block.Transactions()[i].Hash() || r.BlockHash != receipt.BlockHash || r.BlockNumber == nil || r.BlockNumber.Cmp(header.Number) != 0 || r.TransactionIndex != uint(i) || len(r.Logs) > MaxReceiptLogs {
			return fmt.Errorf("native receipt coordinates differ")
		}
		for _, l := range r.Logs {
			if l == nil || l.Removed || l.BlockHash != receipt.BlockHash || l.TxHash != r.TxHash || l.BlockNumber != header.Number.Uint64() || l.TxIndex != uint(i) || l.Index != logIndex {
				return fmt.Errorf("native log coordinates differ")
			}
			logIndex++
		}
	}
	if consensusListRoot(types.Receipts(receipts)) != header.ReceiptHash {
		return fmt.Errorf("native receipt root differs")
	}
	expected, err := json.Marshal(receipts[receipt.TransactionIndex])
	if err != nil {
		return err
	}
	actual, err := json.Marshal(receipt)
	if err != nil || !bytes.Equal(expected, actual) {
		return fmt.Errorf("native consumer receipt differs")
	}
	return nil
}

func (s *RPCSource) nativeCaller(ctx context.Context, header *types.Header, tx *types.Transaction, receipt *types.Receipt, log types.Log) (common.Address, [4]byte, error) {
	fail := func(err error) (common.Address, [4]byte, error) { return common.Address{}, [4]byte{}, err }
	if s.NativeReceipts == nil {
		return fail(fmt.Errorf("native verified receipt evidence required"))
	}
	if err := s.NativeReceipts.VerifyNativeReceipt(ctx, header, tx, receipt); err != nil {
		return fail(err)
	}
	decoded := s.Registry.Decode(125, log)
	selector, ok := TypedSelector("Swap")
	if !ok || decoded.Status != statusDecoded || decoded.EmitterRole != roleEventEmitter || decoded.Signature != "Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)" {
		return fail(fmt.Errorf("unsupported native caller evidence"))
	}
	timestamp, timeOK := decoded.Fields["timestamp"].(*big.Int)
	number, numberOK := decoded.Fields["blockNumber"].(*big.Int)
	if !timeOK || !numberOK || timestamp == nil || number == nil || !timestamp.IsUint64() || timestamp.Uint64() != header.Time || number.Cmp(header.Number) != 0 {
		return fail(fmt.Errorf("native event block binding differs"))
	}
	registration, err := s.PoolAt(ctx, log.BlockHash, hashField(decoded.Fields["poolId"]), common.Address{})
	if err != nil {
		return fail(err)
	}
	return registration.Pool, selector, nil
}

// consensusListRoot builds the canonical hexary receipt/transaction trie using
// the existing RLP and Keccak primitives, without importing a node database.
func consensusListRoot(list types.DerivableList) common.Hash {
	type entry struct{ key, value []byte }
	entries := make([]entry, list.Len())
	var buffer bytes.Buffer
	for i := range entries {
		key := rlp.AppendUint64(nil, uint64(i))
		nibbles := make([]byte, 0, 2*len(key))
		for _, b := range key {
			nibbles = append(nibbles, b>>4, b&15)
		}
		buffer.Reset()
		list.EncodeIndex(i, &buffer)
		entries[i] = entry{nibbles, bytes.Clone(buffer.Bytes())}
	}
	if len(entries) == 0 {
		return types.EmptyRootHash
	}
	compact := func(path []byte, leaf bool) []byte {
		flag := byte(0)
		if leaf {
			flag = 2
		}
		odd := len(path) % 2
		result := make([]byte, 1+len(path)/2)
		result[0] = (flag + byte(odd)) << 4
		start := 0
		if odd == 1 {
			result[0] |= path[0]
			start = 1
		}
		for i := start; i < len(path); i += 2 {
			result[1+(i-start)/2] = path[i]<<4 | path[i+1]
		}
		return result
	}
	encode := func(value any) []byte {
		raw, err := rlp.EncodeToBytes(value)
		if err != nil {
			panic(err)
		} // Only byte strings and RLP lists enter here.
		return raw
	}
	reference := func(raw []byte) any {
		if len(raw) < 32 {
			return rlp.RawValue(raw)
		}
		return crypto.Keccak256(raw)
	}
	var node func([]entry, int) []byte
	node = func(items []entry, depth int) []byte {
		if len(items) == 1 {
			return encode([]any{compact(items[0].key[depth:], true), items[0].value})
		}
		end := depth
		for end < len(items[0].key) {
			match := true
			for _, item := range items[1:] {
				if end >= len(item.key) || item.key[end] != items[0].key[end] {
					match = false
					break
				}
			}
			if !match {
				break
			}
			end++
		}
		if end > depth {
			return encode([]any{compact(items[0].key[depth:end], false), reference(node(items, end))})
		}
		branch := make([]any, 17)
		for i := range branch {
			branch[i] = []byte{}
		}
		for nibble := 0; nibble < 16; nibble++ {
			var children []entry
			for _, item := range items {
				if len(item.key) > depth && int(item.key[depth]) == nibble {
					children = append(children, item)
				}
			}
			if len(children) != 0 {
				branch[nibble] = reference(node(children, depth+1))
			}
		}
		for _, item := range items {
			if len(item.key) == depth {
				branch[16] = item.value
			}
		}
		return encode(branch)
	}
	return crypto.Keccak256Hash(node(entries, 0))
}
