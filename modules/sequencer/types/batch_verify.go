package types

import "bytes"

func VerifyBatchTxs(m MsgSequenceBatch, activityIDs [][32]byte) error {
	if m.FromSeq == 0 {
		return ErrInvalidSequenceBatch.Wrapf("fromSeq is zero")
	}
	var count uint64
	if !m.IsEmpty() {
		if m.ToSeq < m.FromSeq {
			return ErrInvalidSequenceBatch.Wrapf("invalid sequence range %d..%d", m.FromSeq, m.ToSeq)
		}
		count = m.ToSeq - m.FromSeq + 1
	}
	if uint64(len(activityIDs)) != count {
		return ErrInvalidSequenceBatch.Wrapf("activity id count is %d, want %d", len(activityIDs), count)
	}
	root := TxRoot(activityIDs)
	if !bytes.Equal(root[:], m.TxRoot) {
		return ErrInvalidSequenceBatch.Wrapf("txRoot differs from activity id root")
	}
	if len(m.PrevSeqHash) != HashLength {
		return ErrInvalidSequenceBatch.Wrapf("prevSeqHash is %d bytes, want %d", len(m.PrevSeqHash), HashLength)
	}
	var last [32]byte
	copy(last[:], m.PrevSeqHash)
	for i, id := range activityIDs {
		last = SeqHash(last, m.FromSeq+uint64(i), id)
	}
	if !bytes.Equal(last[:], m.LastSeqHash) {
		return ErrInvalidSequenceBatch.Wrapf("lastSeqHash differs from activity id chain")
	}
	return nil
}
