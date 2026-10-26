package sequencer

import (
	"context"
	"fmt"
	"math"

	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type admissionKey struct{}

type admissionUsage struct {
	gas   uint64
	bytes int64
}

func (s *sequential) admit(tx sdk.Tx, txBytes []byte) error {
	cp := s.host.GetConsensusParams(s.open.ctx)
	if cp == nil || cp.Block == nil {
		return ErrNoBlockParams
	}
	var wanted uint64
	if msg := s.host.GetEVMMsg(tx); msg != nil && !msg.IsAssociateTx() {
		etx, _ := msg.AsTransaction()
		if etx == nil {
			return ErrUndecodableTx
		}
		wanted = etx.Gas()
	} else if feeTx, ok := tx.(sdk.FeeTx); ok {
		wanted = feeTx.GetGas()
	}
	used, _ := s.open.ctx.Context().Value(admissionKey{}).(admissionUsage)
	if wanted > math.MaxInt64 || used.gas > math.MaxUint64-wanted {
		return fmt.Errorf("%w: gas wanted overflows", ErrBlockFull)
	}
	nextGas := used.gas + wanted
	if nextGas > uint64(cp.Block.MaxGas) { //nolint:gosec // -1 is unlimited, as in checkTotalBlockGas
		return fmt.Errorf("%w: gas wanted %d exceeds Block.MaxGas %d", ErrBlockFull, nextGas, cp.Block.MaxGas)
	}
	if nextGas > uint64(cp.Block.MaxGasWanted) { //nolint:gosec // same reading as checkTotalBlockGas
		return fmt.Errorf("%w: gas wanted %d exceeds Block.MaxGasWanted %d", ErrBlockFull, nextGas, cp.Block.MaxGasWanted)
	}
	tx0, err := app.EncodeSequenceBatch(seqtypes.MsgSequenceBatch{
		Height:      1,
		FromSeq:     1,
		ToSeq:       1,
		PrevSeqHash: make([]byte, seqtypes.HashLength),
		LastSeqHash: make([]byte, seqtypes.HashLength),
		TxRoot:      make([]byte, seqtypes.HashLength),
		KeyId:       make([]byte, seqtypes.KeyIDLength),
		Sig:         make([]byte, seqtypes.SignatureLength),
	})
	if err != nil {
		return err
	}
	reserve := int64(len(tx0))
	maxBytes := cp.Block.MaxBytes
	if maxBytes < reserve || used.bytes > maxBytes-reserve || int64(len(txBytes)) > maxBytes-reserve-used.bytes {
		return fmt.Errorf("%w: bytes %d plus transaction %d and tx 0 %d exceed Block.MaxBytes %d",
			ErrBlockFull, used.bytes, len(txBytes), reserve, maxBytes)
	}
	s.open.ctx = s.open.ctx.WithContext(context.WithValue(s.open.ctx.Context(), admissionKey{}, admissionUsage{
		gas: nextGas, bytes: used.bytes + int64(len(txBytes)),
	}))
	return nil
}
