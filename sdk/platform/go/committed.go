package layerx

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/bits"
	"time"
)

type CometBlockID struct {
	Hash         []byte
	PartSetTotal uint32
	PartSetHash  []byte
}

type CommittedHeader struct {
	VersionBlock       uint64
	VersionApp         uint64
	ChainID            string
	Height             int64
	Time               time.Time
	LastBlockID        CometBlockID
	LastCommitHash     []byte
	DataHash           []byte
	ValidatorsHash     []byte
	NextValidatorsHash []byte
	ConsensusHash      []byte
	AppHash            []byte
	LastResultsHash    []byte
	EvidenceHash       []byte
	ProposerAddress    []byte
}

type CometValidator struct {
	PublicKey   [32]byte
	VotingPower int64
}

const (
	CometAbsent uint8 = 1
	CometBlock  uint8 = 2
	CometNil    uint8 = 3
)

type CometCommitSignature struct {
	BlockIDFlag      uint8
	ValidatorAddress []byte
	Timestamp        time.Time
	Signature        []byte
}

type CometCommit struct {
	Height     int64
	Round      int32
	BlockID    CometBlockID
	Signatures []CometCommitSignature
}

type CommittedProof struct {
	Total int64
	Index int64
	Aunts [][32]byte
}

type ExecTxResult struct {
	Code      uint32
	Data      []byte
	GasWanted int64
	GasUsed   int64
}

type ReceiptInclusion struct {
	Transaction      []byte
	TransactionProof CommittedProof
	Result           ExecTxResult
	ResultProof      CommittedProof
	BatchTransaction []byte
	BatchProof       CommittedProof
	NextHeader       CommittedHeader
	NextCommit       CometCommit
	NextValidators   []CometValidator
}

type Committed struct {
	SequencerReceiptVerification
	Height      uint64
	BlockHash   [32]byte
	VotingPower int64
}

func VerifyCommittedReceipt(receiptVerification SequencerReceiptVerification, blockHeader CommittedHeader, receiptInclusion ReceiptInclusion, commit CometCommit, validatorSet []CometValidator) (Committed, error) {
	fail := func(check ReceiptCheck) (Committed, error) { return Committed{}, receiptFailure(check) }
	r := receiptVerification.Receipt
	i := receiptInclusion
	if receiptVerification.Rung != "receipted" || receiptVerification.Level != "receipted" ||
		!bytes.Equal(receiptVerification.CanonicalBytes, encodeSequencerReceipt(r, true)) ||
		receiptVerification.ReceiptDigest != domainDigest([]byte(ReceiptDomain+"\x00"), encodeSequencerReceipt(r, false)) ||
		r.ResultCode < 0 || r.ResultCode > 1 || receiptVerification.Status != uint64(1-r.ResultCode) ||
		len(r.Effects) == 0 || len(r.Effects[0].Body) < 28 ||
		!bytes.Equal(r.BatchID[:24], make([]byte, 24)) {
		return fail("committed-receipt")
	}
	height := binary.BigEndian.Uint64(r.BatchID[24:])
	if height == 0 || blockHeader.Height <= 0 || blockHeader.Height == math.MaxInt64 || uint64(blockHeader.Height) < height ||
		i.NextHeader.Height != blockHeader.Height+1 || i.NextHeader.ChainID != blockHeader.ChainID ||
		!bytes.Equal(blockHeader.NextValidatorsHash, i.NextHeader.ValidatorsHash) {
		return fail("committed-height")
	}
	power, ok := committedVerifyCommit(blockHeader, commit, validatorSet)
	if !ok {
		return fail("committed-commit")
	}
	hash := blockHeader.Hash()
	if !bytes.Equal(i.NextHeader.LastBlockID.Hash, hash[:]) ||
		!committedBlockIDsEqual(i.NextHeader.LastBlockID, commit.BlockID) {
		return fail("committed-next-header")
	}
	if _, ok := committedVerifyCommit(i.NextHeader, i.NextCommit, i.NextValidators); !ok {
		return fail("committed-next-commit")
	}
	if len(i.Transaction) == 0 || len(i.Transaction) > maximumMessageBytes || receiptKeccak256(i.Transaction) != r.ActivityID ||
		i.TransactionProof.Index <= 0 || i.TransactionProof.Index != i.ResultProof.Index ||
		i.TransactionProof.Total != i.ResultProof.Total || i.BatchProof.Total != i.TransactionProof.Total ||
		i.BatchProof.Index != 0 || uint64(i.TransactionProof.Index) != uint64(binary.BigEndian.Uint32(r.Effects[0].Body[24:28])) ||
		!committedVerifyProof(i.Transaction, i.TransactionProof, blockHeader.DataHash) ||
		!committedVerifyProof(i.Result.bytes(), i.ResultProof, i.NextHeader.LastResultsHash) ||
		!committedVerifyProof(i.BatchTransaction, i.BatchProof, blockHeader.DataHash) {
		return fail("committed-inclusion")
	}
	var status int32
	switch i.Result.Code {
	case 0:
		status = 0
	case 45:
		status = 1
	default:
		return fail("committed-result")
	}
	if status != r.ResultCode || i.Result.GasUsed < 0 || i.Result.GasWanted < i.Result.GasUsed ||
		uint64(i.Result.GasUsed) != binary.BigEndian.Uint64(r.Effects[0].Body[:8]) {
		return fail("committed-result")
	}
	const batchDomain = "PAXEER/v1/sequence-batch\x00"
	if len(i.BatchTransaction) != len(batchDomain)+24+128+64 || !bytes.HasPrefix(i.BatchTransaction, []byte(batchDomain)) {
		return fail("committed-batch")
	}
	batch := i.BatchTransaction[len(batchDomain):]
	batchHeight := binary.BigEndian.Uint64(batch[:8])
	from := binary.BigEndian.Uint64(batch[8:16])
	to := binary.BigEndian.Uint64(batch[16:24])
	if batchHeight != uint64(blockHeader.Height) || from == 0 || r.GlobalSequence < from || r.GlobalSequence > to {
		return fail("committed-batch")
	}
	result := receiptVerification
	result.Rung = "committed"
	result.Level = "committed"
	return Committed{SequencerReceiptVerification: result, Height: uint64(blockHeader.Height), BlockHash: hash, VotingPower: power}, nil
}

func committedVarint(field uint64, value uint64) []byte {
	if value == 0 {
		return nil
	}
	b := binary.AppendUvarint(nil, field<<3)
	return binary.AppendUvarint(b, value)
}

func committedBytes(field uint64, value []byte) []byte {
	b := binary.AppendUvarint(nil, field<<3|2)
	b = binary.AppendUvarint(b, uint64(len(value)))
	return append(b, value...)
}

func committedWrapped(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return committedBytes(1, value)
}

func committedTimestamp(t time.Time) []byte {
	b := committedVarint(1, uint64(t.Unix()))
	return append(b, committedVarint(2, uint64(t.Nanosecond()))...)
}

func (id CometBlockID) bytes() []byte {
	parts := committedVarint(1, uint64(id.PartSetTotal))
	if len(id.PartSetHash) != 0 {
		parts = append(parts, committedBytes(2, id.PartSetHash)...)
	}
	var b []byte
	if len(id.Hash) != 0 {
		b = committedBytes(1, id.Hash)
	}
	return append(b, committedBytes(2, parts)...)
}

func committedBlockIDsEqual(a, b CometBlockID) bool {
	return a.PartSetTotal == b.PartSetTotal && bytes.Equal(a.Hash, b.Hash) && bytes.Equal(a.PartSetHash, b.PartSetHash)
}

func (h CommittedHeader) Hash() [32]byte {
	version := committedVarint(1, h.VersionBlock)
	version = append(version, committedVarint(2, h.VersionApp)...)
	return committedRoot([][]byte{
		version, committedWrapped([]byte(h.ChainID)), committedVarint(1, uint64(h.Height)),
		committedTimestamp(h.Time), h.LastBlockID.bytes(), committedWrapped(h.LastCommitHash),
		committedWrapped(h.DataHash), committedWrapped(h.ValidatorsHash), committedWrapped(h.NextValidatorsHash),
		committedWrapped(h.ConsensusHash), committedWrapped(h.AppHash), committedWrapped(h.LastResultsHash),
		committedWrapped(h.EvidenceHash), committedWrapped(h.ProposerAddress),
	})
}

func committedValidatorsHash(set []CometValidator) [32]byte {
	leaves := make([][]byte, len(set))
	for n, v := range set {
		leaves[n] = committedBytes(1, committedBytes(1, v.PublicKey[:]))
		leaves[n] = append(leaves[n], committedVarint(2, uint64(v.VotingPower))...)
	}
	return committedRoot(leaves)
}

func committedLeaf(b []byte) [32]byte { return sha256.Sum256(append([]byte{0}, b...)) }

func committedInner(left, right [32]byte) [32]byte {
	b := make([]byte, 1, 65)
	b[0] = 1
	b = append(b, left[:]...)
	return sha256.Sum256(append(b, right[:]...))
}

func committedSplit(total int64) int64 { return int64(1) << (bits.Len64(uint64(total-1)) - 1) }

func committedRoot(leaves [][]byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256(nil)
	}
	if len(leaves) == 1 {
		return committedLeaf(leaves[0])
	}
	split := committedSplit(int64(len(leaves)))
	return committedInner(committedRoot(leaves[:split]), committedRoot(leaves[split:]))
}

func committedVerifyProof(leaf []byte, proof CommittedProof, root []byte) bool {
	if len(root) != 32 || proof.Total <= 0 || proof.Index < 0 || proof.Index >= proof.Total || len(proof.Aunts) > 63 {
		return false
	}
	var reconstruct func(int64, int64, [][32]byte) ([32]byte, bool)
	reconstruct = func(index, total int64, aunts [][32]byte) ([32]byte, bool) {
		if total == 1 {
			return committedLeaf(leaf), len(aunts) == 0
		}
		if len(aunts) == 0 {
			return [32]byte{}, false
		}
		split := committedSplit(total)
		aunt := aunts[len(aunts)-1]
		if index < split {
			child, ok := reconstruct(index, split, aunts[:len(aunts)-1])
			return committedInner(child, aunt), ok
		}
		child, ok := reconstruct(index-split, total-split, aunts[:len(aunts)-1])
		return committedInner(aunt, child), ok
	}
	hash, ok := reconstruct(proof.Index, proof.Total, proof.Aunts)
	return ok && bytes.Equal(hash[:], root)
}

func (r ExecTxResult) bytes() []byte {
	b := committedVarint(1, uint64(r.Code))
	if len(r.Data) != 0 {
		b = append(b, committedBytes(2, r.Data)...)
	}
	b = append(b, committedVarint(5, uint64(r.GasWanted))...)
	return append(b, committedVarint(6, uint64(r.GasUsed))...)
}

func committedVoteBytes(chainID string, commit CometCommit, sig CometCommitSignature) []byte {
	b := []byte{8, 2, 0x11}
	b = binary.LittleEndian.AppendUint64(b, uint64(commit.Height))
	if commit.Round != 0 {
		b = append(b, 0x19)
		b = binary.LittleEndian.AppendUint64(b, uint64(commit.Round))
	}
	if sig.BlockIDFlag == CometBlock {
		b = append(b, committedBytes(4, commit.BlockID.bytes())...)
	}
	b = append(b, committedBytes(5, committedTimestamp(sig.Timestamp))...)
	b = append(b, committedBytes(6, []byte(chainID))...)
	return append(binary.AppendUvarint(nil, uint64(len(b))), b...)
}

func committedValidTime(t time.Time) bool {
	return t.Unix() >= -62135596800 && t.Unix() <= 253402300799
}

func committedVerifyCommit(header CommittedHeader, commit CometCommit, set []CometValidator) (int64, bool) {
	if header.Height <= 0 || header.ChainID == "" || len(header.ChainID) > 50 || !committedValidTime(header.Time) ||
		commit.Height != header.Height || commit.Round < 0 || len(set) == 0 || len(commit.Signatures) != len(set) ||
		len(commit.BlockID.Hash) != 32 || commit.BlockID.PartSetTotal == 0 || len(commit.BlockID.PartSetHash) != 32 {
		return 0, false
	}
	setHash := committedValidatorsHash(set)
	headerHash := header.Hash()
	if !bytes.Equal(header.ValidatorsHash, setHash[:]) || !bytes.Equal(commit.BlockID.Hash, headerHash[:]) {
		return 0, false
	}
	seen := make(map[[32]byte]bool, len(set))
	var total, signed int64
	for _, v := range set {
		if v.VotingPower <= 0 || v.VotingPower > math.MaxInt64/8-total || seen[v.PublicKey] {
			return 0, false
		}
		seen[v.PublicKey] = true
		total += v.VotingPower
	}
	for n, sig := range commit.Signatures {
		if sig.BlockIDFlag == CometAbsent {
			if len(sig.Signature) != 0 || len(sig.ValidatorAddress) != 0 || !sig.Timestamp.IsZero() {
				return 0, false
			}
			continue
		}
		if sig.BlockIDFlag != CometBlock && sig.BlockIDFlag != CometNil || !committedValidTime(sig.Timestamp) || len(sig.Signature) != ed25519.SignatureSize {
			return 0, false
		}
		v := set[n]
		address := sha256.Sum256(v.PublicKey[:])
		if !bytes.Equal(sig.ValidatorAddress, address[:20]) || !ed25519.Verify(v.PublicKey[:], committedVoteBytes(header.ChainID, commit, sig), sig.Signature) {
			return 0, false
		}
		if sig.BlockIDFlag == CometBlock {
			signed += v.VotingPower
		}
	}
	return signed, signed*3 > total*2
}
