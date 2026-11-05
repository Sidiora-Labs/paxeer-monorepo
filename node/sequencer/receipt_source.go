package sequencer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
)

const defaultReceiptSourceCapacity = 65_536

var _ evmrpc.SequencerReceiptSource = (*ReceiptSource)(nil)

type receiptSequenceRange struct{ first, last uint64 }

type receiptSourceEntry struct {
	height int64
	send   evmrpc.SequencerSend
	exec   *evmtypes.Receipt
}

// ReceiptSource retains accepted sends after their receipt heights commit,
// evicting the oldest committed sends only when capacity is needed.
// The zero value is ready for use and holds at most 65,536 sends.
type ReceiptSource struct {
	mu             sync.RWMutex
	entries        map[common.Hash]receiptSourceEntry
	capacity       int
	committed      int64
	reserved       int
	pending        func(common.Hash) (*evmrpc.SequencerSend, error)
	evicted        []receiptSequenceRange
	pendingOrder   []common.Hash
	committedOrder []common.Hash
}

// NewReceiptSource sets the maximum number of retained sends. Capacity must be positive.
func NewReceiptSource(capacity int) *ReceiptSource {
	if capacity <= 0 {
		panic("sequencer: receipt source capacity must be positive")
	}
	return &ReceiptSource{capacity: capacity}
}

func (s *ReceiptSource) restoreCommitted(ctx context.Context, log *wal.WAL) error {
	tip := log.LastCommitted()
	if tip == nil {
		return nil
	}
	if tip.Height > math.MaxInt64 {
		return fmt.Errorf("%w: committed height exceeds int64", wal.ErrMalformed)
	}
	s.Committed(int64(tip.Height))
	return log.Scan(func(entry wal.Entry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		a, ok := entry.(*wal.Accept)
		if !ok || a.Height > tip.Height {
			return nil
		}
		receipt, err := codec.DecodeReceipt(a.Receipt)
		if err != nil {
			return fmt.Errorf("sequencer: stored receipt at sequence %d: %w", a.Seq, err)
		}
		if receipt.SequencerSignature == nil || receipt.ProtocolVersion != receiptProtocolVersion ||
			receipt.ModuleVersion != ReceiptLayoutVersion || receipt.ModuleID <= KernelReservedModules ||
			!bytes.Equal(receipt.BatchID[:24], make([]byte, 24)) || a.Height == 0 ||
			binary.BigEndian.Uint64(receipt.BatchID[24:]) != a.Height || receipt.GlobalSequence != a.Seq || a.Seq > tip.LastSeq {
			return fmt.Errorf("%w: stored receipt at sequence %d", wal.ErrMalformed, a.Seq)
		}
		if len(receipt.Effects) == 0 || receipt.Effects[0].Kind != effectKindState ||
			receipt.Effects[0].ModuleID != receipt.ModuleID || receipt.Effects[0].Ordinal != 0 || len(receipt.Effects[0].Body) < 109 {
			return inputErr("stored receipt at sequence %d has no execution summary", a.Seq)
		}
		body := receipt.Effects[0].Body
		vmDigest := sha256.Sum256([]byte(a.VMError))
		if uint64(len(a.ReturnData)) != uint64(binary.BigEndian.Uint32(body[100:104])) ||
			sha256.Sum256(a.ReturnData) != receipt.ActivityRoot || !bytes.Equal(vmDigest[:], body[68:100]) ||
			len(a.LogData) != len(receipt.Effects)-1 || uint64(len(a.LogData)) != uint64(binary.BigEndian.Uint32(body[104:108])) {
			return fmt.Errorf("%w: stored execution outputs at sequence %d", wal.ErrReceiptMismatch, a.Seq)
		}
		for i, data := range a.LogData {
			effect := receipt.Effects[i+1]
			digest := sha256.Sum256(data)
			if effect.ModuleID != receipt.ModuleID || int(effect.Ordinal) != i+1 || effect.Kind != effectKindEvent ||
				len(effect.Body) < 32 || !bytes.Equal(digest[:], effect.Body[len(effect.Body)-32:]) {
				return fmt.Errorf("%w: stored log %d at sequence %d", wal.ErrReceiptMismatch, i, a.Seq)
			}
		}
		send := evmrpc.SequencerSend{Nonce: receipt.FromSequence, Receipt: a.Receipt,
			ReturnData: a.ReturnData, LogData: a.LogData, VMError: a.VMError}
		if receipt.Operation != CosmosOperation && send.LogData == nil {
			send.LogData = make([][]byte, 0)
		}
		s.record(common.Hash(receipt.ActivityID), send, true)
		return nil
	})
}

// Record stores an owned copy of an accepted, signed send. Invalid input or
// capacity exhausted before commit panics: this void API cannot report a lost
// accepted receipt. Retained committed sends are evicted oldest first as needed.
// First publications are retained even if commit advanced before publication.
// Evicted sequence positions cannot be resurrected by duplicate publication.
func (s *ReceiptSource) Record(hash common.Hash, send evmrpc.SequencerSend) {
	s.record(hash, send, false)
}

func (s *ReceiptSource) record(hash common.Hash, send evmrpc.SequencerSend, replay bool) {
	send = cloneReceiptSourceSend(send)
	receipt, err := codec.DecodeReceipt(send.Receipt)
	if err != nil {
		panic(fmt.Sprintf("sequencer: receipt source: %v", err))
	}
	height := binary.BigEndian.Uint64(receipt.BatchID[24:])
	if receipt.SequencerSignature == nil || receipt.ProtocolVersion != receiptProtocolVersion ||
		receipt.ModuleVersion != ReceiptLayoutVersion || receipt.ModuleID <= KernelReservedModules ||
		!bytes.Equal(receipt.BatchID[:24], make([]byte, 24)) || height == 0 || height > math.MaxInt64 ||
		common.Hash(receipt.ActivityID) != hash || receipt.FromSequence != send.Nonce {
		panic("sequencer: receipt source needs a signed sequencer send with matching hash and nonce")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !replay && s.wasEvicted(receipt.GlobalSequence) {
		return
	}
	capacity := s.capacity
	if capacity == 0 {
		capacity = defaultReceiptSourceCapacity
	}
	entry, exists := s.entries[hash]
	if !exists && len(s.entries) >= capacity {
		switch {
		case len(s.committedOrder) > 0:
			old, err := codec.DecodeReceipt(s.entries[s.committedOrder[0]].send.Receipt)
			if err != nil {
				panic(err)
			}
			s.rememberEvicted(old.GlobalSequence)
			delete(s.entries, s.committedOrder[0])
			s.committedOrder = s.committedOrder[1:]
		case replay && len(s.pendingOrder) > 0:
			old, err := codec.DecodeReceipt(s.entries[s.pendingOrder[0]].send.Receipt)
			if err != nil {
				panic(err)
			}
			s.rememberEvicted(old.GlobalSequence)
			delete(s.entries, s.pendingOrder[0])
			s.pendingOrder = s.pendingOrder[1:]
		default:
			panic("sequencer: receipt source capacity exhausted before commit")
		}
	}
	if exists && entry.height <= s.committed {
		for i, committedHash := range s.committedOrder {
			if committedHash == hash {
				s.committedOrder = append(s.committedOrder[:i], s.committedOrder[i+1:]...)
				break
			}
		}
	}
	if !exists || entry.height <= s.committed {
		if int64(height) <= s.committed {
			i := sort.Search(len(s.committedOrder), func(i int) bool {
				return s.entries[s.committedOrder[i]].height > int64(height)
			})
			s.committedOrder = append(s.committedOrder, common.Hash{})
			copy(s.committedOrder[i+1:], s.committedOrder[i:])
			s.committedOrder[i] = hash
		} else {
			s.pendingOrder = append(s.pendingOrder, hash)
		}
	}
	if s.entries == nil {
		s.entries = make(map[common.Hash]receiptSourceEntry)
	}
	s.entries[hash] = receiptSourceEntry{height: int64(height), send: send}
}

// SequencerSend returns an owned copy of the recorded send, or nil for an unknown hash.
// Each lookup reads the current record; modifying a returned send cannot change
// what the RPC consumer decodes on its next lookup.
func (s *ReceiptSource) SequencerSend(hash common.Hash) (*evmrpc.SequencerSend, error) {
	s.mu.RLock()
	entry, ok := s.entries[hash]
	pending := s.pending
	s.mu.RUnlock()
	if !ok {
		if pending != nil {
			return pending(hash)
		}
		return nil, nil
	}
	send := cloneReceiptSourceSend(entry.send)
	return &send, nil
}

// Committed retains sends at or below height for lookup until capacity eviction,
// ordered by height and then insertion order. The commit watermark never decreases.
func (s *ReceiptSource) Committed(height int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if height <= s.committed {
		return
	}
	s.committed = height
	var newlyCommitted []common.Hash
	pending := s.pendingOrder[:0]
	for _, hash := range s.pendingOrder {
		if s.entries[hash].height <= height {
			newlyCommitted = append(newlyCommitted, hash)
		} else {
			pending = append(pending, hash)
		}
	}
	s.pendingOrder = pending
	sort.SliceStable(newlyCommitted, func(i, j int) bool {
		return s.entries[newlyCommitted[i]].height < s.entries[newlyCommitted[j]].height
	})
	s.committedOrder = append(s.committedOrder, newlyCommitted...)
}

// Eviction ranges remember exact positions, so a late first publication is
// never mistaken for a duplicate merely because a later sequence was evicted.
// Normal ordered publication coalesces into a single range.
func (s *ReceiptSource) wasEvicted(seq uint64) bool {
	i := sort.Search(len(s.evicted), func(i int) bool { return s.evicted[i].last >= seq })
	return i < len(s.evicted) && s.evicted[i].first <= seq
}

func (s *ReceiptSource) rememberEvicted(seq uint64) {
	i := sort.Search(len(s.evicted), func(i int) bool { return s.evicted[i].last >= seq || s.evicted[i].last == seq-1 })
	if i == len(s.evicted) {
		s.evicted = append(s.evicted, receiptSequenceRange{seq, seq})
		return
	}
	if s.evicted[i].first > seq && s.evicted[i].first-seq > 1 {
		s.evicted = append(s.evicted, receiptSequenceRange{})
		copy(s.evicted[i+1:], s.evicted[i:])
		s.evicted[i] = receiptSequenceRange{seq, seq}
	} else {
		s.evicted[i].first = min(s.evicted[i].first, seq)
		s.evicted[i].last = max(s.evicted[i].last, seq)
	}
	if i+1 < len(s.evicted) && s.evicted[i+1].first-s.evicted[i].last == 1 {
		s.evicted[i].last = s.evicted[i+1].last
		s.evicted = append(s.evicted[:i+1], s.evicted[i+2:]...)
	}
}

func cloneReceiptSourceSend(send evmrpc.SequencerSend) evmrpc.SequencerSend {
	send.Receipt = bytes.Clone(send.Receipt)
	send.ReturnData = bytes.Clone(send.ReturnData)
	if send.LogData != nil {
		logs := make([][]byte, len(send.LogData))
		for i, data := range send.LogData {
			logs[i] = bytes.Clone(data)
		}
		send.LogData = logs
	}
	return send
}

var ErrReceiptCapacity = errors.New("sequencer: receipt source capacity exhausted before execution")

// Reserve accounts for every in-flight send before any execution can change state.
func (s *ReceiptSource) Reserve() (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	capacity := s.capacity
	if capacity == 0 {
		capacity = defaultReceiptSourceCapacity
	}
	if len(s.pendingOrder)+s.reserved >= capacity {
		return nil, ErrReceiptCapacity
	}
	s.reserved++
	var once sync.Once
	return func() { once.Do(func() { s.mu.Lock(); s.reserved--; s.mu.Unlock() }) }, nil
}
