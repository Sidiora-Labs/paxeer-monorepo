package sequencer

import (
	"errors"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

var (
	ErrNoOpenLayer       = errors.New("sequencer: no open layer")
	ErrSealedPending     = errors.New("sequencer: the sealed layer is not committed yet")
	ErrNoSealedLayer     = errors.New("sequencer: no sealed layer")
	ErrHeightMismatch    = errors.New("sequencer: height does not follow the layer below")
	ErrUpgradeHeight     = errors.New("sequencer: an upgrade plan executes at this height")
	ErrNoBlockParams     = errors.New("sequencer: consensus block params are not set")
	ErrNoBlockSpace      = errors.New("sequencer: block max bytes leave no room for transactions")
	ErrBlockFull         = errors.New("sequencer: transaction does not fit the block limits")
	ErrUndecodableTx     = errors.New("sequencer: transaction does not decode")
	ErrMaliciousGasless  = errors.New("sequencer: gasless check panicked")
	ErrBeginBlockPanic   = errors.New("sequencer: begin block panicked")
	ErrDeliverTxPanicked = errors.New("sequencer: deliver tx panicked")
)

// SequenceBatchFields are the MsgSequenceBatch fields of tx 0 derived from the seqHash chain.
type SequenceBatchFields struct {
	Height      uint64
	FromSeq     uint64
	ToSeq       uint64
	PrevSeqHash [32]byte
	LastSeqHash [32]byte
	TxRoot      [32]byte
}

// SealedLayer is L_N: a block whose end-of-block ran in ProcessBlock order.
type SealedLayer struct {
	Height   int64
	Txs      [][]byte
	Results  []*abci.ExecTxResult
	Events   []abci.Event
	EndBlock abci.ResponseEndBlock
	Batch    SequenceBatchFields
	ctx      sdk.Context
	ms       sdk.CacheMultiStore
}

// Context returns the sealed block's context over L_N.
func (s *SealedLayer) Context() sdk.Context {
	return s.ctx
}
