package types

import (
	"bytes"
	"encoding/binary"
)

const (
	SequenceBatchTxLength = len(SequenceBatchDomain) + 3*8 + 3*HashLength + KeyIDLength + SignatureLength
)

func EncodeSequenceBatchTx(m MsgSequenceBatch) ([]byte, error) {
	if err := m.ValidateBasic(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, SequenceBatchTxLength)
	b = append(b, SequenceBatchDomain...)
	b = binary.BigEndian.AppendUint64(b, m.Height)
	b = binary.BigEndian.AppendUint64(b, m.FromSeq)
	b = binary.BigEndian.AppendUint64(b, m.ToSeq)
	b = append(b, m.PrevSeqHash...)
	b = append(b, m.LastSeqHash...)
	b = append(b, m.TxRoot...)
	b = append(b, m.KeyId...)
	return append(b, m.Sig...), nil
}

func DecodeSequenceBatchTx(tx []byte) (MsgSequenceBatch, error) {
	if len(tx) != SequenceBatchTxLength || string(tx[:len(SequenceBatchDomain)]) != SequenceBatchDomain {
		return MsgSequenceBatch{}, ErrInvalidSequenceBatch.Wrapf("tx 0 is not a sequence batch: %d bytes", len(tx))
	}
	rest := tx[len(SequenceBatchDomain):]
	next := func(n int) []byte {
		field := bytes.Clone(rest[:n])
		rest = rest[n:]
		return field
	}
	m := MsgSequenceBatch{
		Height:  binary.BigEndian.Uint64(next(8)),
		FromSeq: binary.BigEndian.Uint64(next(8)),
		ToSeq:   binary.BigEndian.Uint64(next(8)),
	}
	m.PrevSeqHash = next(HashLength)
	m.LastSeqHash = next(HashLength)
	m.TxRoot = next(HashLength)
	m.KeyId = next(KeyIDLength)
	m.Sig = next(SignatureLength)
	if err := m.ValidateBasic(); err != nil {
		return MsgSequenceBatch{}, err
	}
	return m, nil
}
