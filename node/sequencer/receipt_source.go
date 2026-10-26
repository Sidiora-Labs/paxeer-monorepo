package sequencer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
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

// ReceiptSource retains accepted sends until their receipt heights commit.
// The zero value is ready for use and holds at most 65,536 sends.
type ReceiptSource struct {
	mu        sync.RWMutex
	entries   map[common.Hash]receiptSourceEntry
	capacity  int
	committed int64
}

// NewReceiptSource sets the maximum number of retained sends. Capacity must be positive.
func NewReceiptSource(capacity int) *ReceiptSource {
	if capacity <= 0 {
		panic("sequencer: receipt source capacity must be positive")
	}
	return &ReceiptSource{capacity: capacity}
}

// Record stores an owned copy of an accepted, signed send. Invalid input or
// exhausted capacity panics: this void API cannot report a lost accepted receipt.
// Sends at already committed heights are not retained.
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
	if _, exists := s.entries[hash]; !exists && len(s.entries) >= capacity {
		panic("sequencer: receipt source capacity exhausted before commit")
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

// Committed drops all sends at or below height. The commit watermark never decreases.
func (s *ReceiptSource) Committed(height int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if height <= s.committed {
		return
	}
	s.committed = height
	for hash, entry := range s.entries {
		if entry.height <= height {
			delete(s.entries, hash)
		}
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
