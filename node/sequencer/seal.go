package sequencer

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

var (
	// ErrSealPanicked reports a panic in the end-of-block sequence; the open layer is discarded.
	ErrSealPanicked = errors.New("sequencer: seal panicked")
	// ErrMalformedBatch reports a sealed batch whose transactions, results and seq range disagree.
	ErrMalformedBatch = errors.New("sequencer: sealed batch is malformed")
	// ErrBatchFieldsMismatch reports a MsgSequenceBatch whose fields differ from the sealed batch.
	ErrBatchFieldsMismatch = errors.New("sequencer: sequence batch fields differ from the sealed batch")
)

func endOfBlock(a *app.App, ctx sdk.Context, b *Batch, results []*abci.ExecTxResult, evmMsgs []*evmtypes.MsgEVMTransaction) (events []abci.Event, endBlock abci.ResponseEndBlock, err error) {
	defer func() {
		if r := recover(); r != nil {
			events = nil
			endBlock = abci.ResponseEndBlock{}
			err = fmt.Errorf("%w: %v", ErrSealPanicked, r)
		}
	}()
	blockEvents, endBlock, err := a.CompleteBlock(ctx, b.Height, results, evmMsgs)
	if err != nil {
		return nil, abci.ResponseEndBlock{}, err
	}
	events = append(events, b.BeginBlockEvents...)
	events = append(events, blockEvents...)
	return events, endBlock, nil
}

// BatchFields derives the MsgSequenceBatch fields of tx 0 from b: its height, seq range and
// seqHash chain, and txRoot as sha256 over the activity ids of b.Txs in order (keccak256 of the
// signed EVM transaction, else sha256 of the tx bytes). An empty batch gives toSeq = fromSeq - 1.
func BatchFields(b *Batch) SequenceBatchFields {
	root := sha256.New()
	for i, tx := range b.Txs {
		id := sha256.Sum256(tx)
		if i < len(b.EVMMsgs) && b.EVMMsgs[i] != nil {
			if etx, _ := b.EVMMsgs[i].AsTransaction(); etx != nil {
				id = etx.Hash()
			}
		}
		root.Write(id[:])
	}
	var txRoot [32]byte
	copy(txRoot[:], root.Sum(nil))
	return SequenceBatchFields{
		Height:      uint64(b.Height), //nolint:gosec // SealBatch refuses heights below 1
		FromSeq:     b.FromSeq,
		ToSeq:       b.FromSeq + uint64(len(b.Txs)) - 1,
		PrevSeqHash: b.PrevSeqHash,
		LastSeqHash: b.LastSeqHash,
		TxRoot:      txRoot,
	}
}

// SealBatch applies msg as tx 0 of b with DeliverSequenceBatch, runs the end-of-block steps over
// b in ProcessBlock order and returns L_N with the encoded tx 0. It refuses a msg whose fields
// differ from BatchFields(b). The steps run on a branch of b.Store that is written only when
// every step succeeds, so on any error b.Store is unchanged. The stored historical header
// remains provisional until FinalizeBlocker supplies the decided consensus header.
func SealBatch(a *app.App, b *Batch, msg seqtypes.MsgSequenceBatch) (*SealedLayer, []byte, error) {
	if a == nil {
		return nil, nil, errors.New("sequencer: nil app")
	}
	if b == nil {
		return nil, nil, fmt.Errorf("%w: nil batch", ErrMalformedBatch)
	}
	if b.Height < 1 {
		return nil, nil, fmt.Errorf("%w: height %d", ErrHeightMismatch, b.Height)
	}
	if b.Store == nil {
		return nil, nil, fmt.Errorf("%w: no open layer store", ErrMalformedBatch)
	}
	if len(b.Results) != len(b.Txs) || len(b.EVMMsgs) != len(b.Txs) {
		return nil, nil, fmt.Errorf("%w: %d txs, %d results, %d EVM messages", ErrMalformedBatch, len(b.Txs), len(b.Results), len(b.EVMMsgs))
	}
	fields := BatchFields(b)
	if fields.ToSeq != b.LastSeq {
		return nil, nil, fmt.Errorf("%w: %d txs from seq %d end at %d, last seq %d", ErrMalformedBatch, len(b.Txs), b.FromSeq, fields.ToSeq, b.LastSeq)
	}
	if err := matchBatchFields(fields, msg); err != nil {
		return nil, nil, err
	}
	tx0, err := app.EncodeSequenceBatch(msg)
	if err != nil {
		return nil, nil, err
	}

	branch := b.Store.CacheMultiStore()
	ctx := b.Ctx.WithMultiStore(branch)
	if err := deliverBatch(a, ctx.WithTxIndex(0), msg); err != nil {
		return nil, nil, err
	}
	results := append([]*abci.ExecTxResult{{}}, b.Results...)
	evmMsgs := append([]*evmtypes.MsgEVMTransaction{nil}, b.EVMMsgs...)
	events, endBlock, err := endOfBlock(a, ctx, b, results, evmMsgs)
	if err != nil {
		return nil, nil, err
	}
	branch.Write()

	return &SealedLayer{
		Height:   b.Height,
		Txs:      append([][]byte{tx0}, b.Txs...),
		Results:  results,
		Events:   events,
		EndBlock: endBlock,
		Batch:    fields,
		ctx:      b.Ctx,
		ms:       b.Store,
	}, tx0, nil
}

func matchBatchFields(want SequenceBatchFields, msg seqtypes.MsgSequenceBatch) error {
	for _, field := range []struct {
		name      string
		got, want uint64
	}{
		{"height", msg.Height, want.Height},
		{"fromSeq", msg.FromSeq, want.FromSeq},
		{"toSeq", msg.ToSeq, want.ToSeq},
	} {
		if field.got != field.want {
			return fmt.Errorf("%w: %s %d, sealed %d", ErrBatchFieldsMismatch, field.name, field.got, field.want)
		}
	}
	for _, field := range []struct {
		name string
		got  []byte
		want [32]byte
	}{
		{"prevSeqHash", msg.PrevSeqHash, want.PrevSeqHash},
		{"lastSeqHash", msg.LastSeqHash, want.LastSeqHash},
		{"txRoot", msg.TxRoot, want.TxRoot},
	} {
		if !bytes.Equal(field.got, field.want[:]) {
			return fmt.Errorf("%w: %s %X, sealed %X", ErrBatchFieldsMismatch, field.name, field.got, field.want)
		}
	}
	return nil
}

func deliverBatch(a *app.App, ctx sdk.Context, msg seqtypes.MsgSequenceBatch) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: deliver sequence batch: %v", ErrSealPanicked, r)
		}
	}()
	return a.SequencerKeeper.DeliverSequenceBatch(ctx, msg)
}
