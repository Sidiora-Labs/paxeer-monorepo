package types

import (
	"bytes"
	"encoding/binary"
)

const (
	// domain is the wire prefix, shared with the batch signing domain.
	domain = SequenceBatchDomain
	// SequenceBatchTxLength includes the domain, three u64be positions, three
	// hashes, the receipt key identifier and its ed25519 signature.
	SequenceBatchTxLength = len(domain) + 3*8 + 3*HashLength + KeyIDLength + SignatureLength
)

// EncodeSequenceBatchTx produces tx 0 in canonical field order. Authentication
// and binding to the block's activities are checked separately by consumers.
func EncodeSequenceBatchTx(m MsgSequenceBatch) ([]byte, error) {
	if err := m.ValidateBasic(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, SequenceBatchTxLength)
	b = append(b, domain...)
	b = binary.BigEndian.AppendUint64(b, m.Height)
	b = binary.BigEndian.AppendUint64(b, m.FromSeq)
	b = binary.BigEndian.AppendUint64(b, m.ToSeq)
	b = append(b, m.PrevSeqHash...)
	b = append(b, m.LastSeqHash...)
	b = append(b, m.TxRoot...)
	b = append(b, m.KeyId...)
	return append(b, m.Sig...), nil
}

// DecodeSequenceBatchTx checks the complete wire envelope and field shape.
// The returned fields own their bytes so a reused transport buffer is safe.
func DecodeSequenceBatchTx(tx []byte) (MsgSequenceBatch, error) {
	if len(tx) != SequenceBatchTxLength || string(tx[:len(domain)]) != domain {
		return MsgSequenceBatch{}, ErrInvalidSequenceBatch.Wrapf("tx 0 is not a sequence batch: %d bytes", len(tx))
	}
	rest := tx[len(domain):]
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
