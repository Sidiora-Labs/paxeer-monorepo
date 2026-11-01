package sequencer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
)

const defaultReceiptSourceCapacity = 65_536

var _ evmrpc.SequencerReceiptSource = (*ReceiptSource)(nil)

type receiptSourceEntry struct {
	height int64
	send   evmrpc.SequencerSend
}

// ReceiptSource retains accepted sends after their receipt heights commit,
// evicting the oldest committed sends only when capacity is needed.
// The zero value is ready for use and holds at most 65,536 sends.
type ReceiptSource struct {
	mu             sync.RWMutex
	entries        map[common.Hash]receiptSourceEntry
	capacity       int
	committed      int64
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

// Record stores an owned copy of an accepted, signed send. Invalid input or
// capacity exhausted before commit panics: this void API cannot report a lost
// accepted receipt. Retained committed sends are evicted oldest first as needed.
// Sends recorded at already committed heights are not retained.
func (s *ReceiptSource) Record(hash common.Hash, send evmrpc.SequencerSend) {
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
	if int64(height) <= s.committed {
		return
	}
	capacity := s.capacity
	if capacity == 0 {
		capacity = defaultReceiptSourceCapacity
	}
	entry, exists := s.entries[hash]
	if !exists && len(s.entries) >= capacity {
		if len(s.committedOrder) == 0 {
			panic("sequencer: receipt source capacity exhausted before commit")
		}
		delete(s.entries, s.committedOrder[0])
		s.committedOrder = s.committedOrder[1:]
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
		s.pendingOrder = append(s.pendingOrder, hash)
	}
	if s.entries == nil {
		s.entries = make(map[common.Hash]receiptSourceEntry)
	}
	s.entries[hash] = receiptSourceEntry{height: int64(height), send: send}
}

// SequencerSend returns an owned copy of the recorded send, or nil for an unknown hash.
func (s *ReceiptSource) SequencerSend(hash common.Hash) (*evmrpc.SequencerSend, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[hash]
	if !ok {
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
