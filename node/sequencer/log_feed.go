package sequencer

import (
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
)

type LogFeedPublisher struct {
	feed *evmrpc.ReceiptLogFeed
}

// NewLogFeedPublisher publishes execution logs into the same receipt feed that
// the RPC server uses for pending log queries, preserving its ordering checks.
func NewLogFeedPublisher(feed *evmrpc.ReceiptLogFeed) *LogFeedPublisher {
	if feed == nil {
		panic("sequencer: log feed publisher needs a receipt log feed")
	}
	return &LogFeedPublisher{feed: feed}
}

func (p *LogFeedPublisher) Receipted(seq uint64, height int64, txHash common.Hash, rcpt *evmtypes.Receipt) error {
	return p.feed.Receipted(seq, height, txHash, rcpt)
}

func (p *LogFeedPublisher) Committed(height int64) error {
	return p.feed.Committed(height)
}
