package ics23

import "fmt"

// IsCompressed returns true if the proof was compressed.
func IsCompressed(proof *CommitmentProof) bool {
	p, ok := proof.GetProof().(*CommitmentProof_Compressed)
	return ok && p != nil && p.Compressed != nil
}

// Compress converts a batch proof to a compressed batch proof. Other valid proof
// variants are returned unchanged. Malformed input returns an error.
func Compress(proof *CommitmentProof) (*CommitmentProof, error) {
	if err := validateProof(proof); err != nil {
		return nil, err
	}
	batch := proof.GetBatch()
	if batch == nil {
		return proof, nil
	}
	comp, err := compress(batch)
	if err != nil {
		return nil, err
	}
	return &CommitmentProof{Proof: &CommitmentProof_Compressed{Compressed: comp}}, nil
}

// Decompress converts a compressed batch proof to a batch proof. Other valid
// variants are returned unchanged. Every decoded index and entry is checked.
func Decompress(proof *CommitmentProof) (*CommitmentProof, error) {
	if p, ok := proof.GetProof().(*CommitmentProof_Compressed); ok {
		if p == nil || p.Compressed == nil {
			return nil, fmt.Errorf("missing compressed proof")
		}
		batch, err := decompress(p.Compressed)
		if err != nil {
			return nil, err
		}
		proof = &CommitmentProof{Proof: &CommitmentProof_Batch{Batch: batch}}
	}
	if err := validateProof(proof); err != nil {
		return nil, err
	}
	return proof, nil
}

func compress(batch *BatchProof) (*CompressedBatchProof, error) {
	var entries []*CompressedBatchEntry
	var lookup []*InnerOp
	registry := make(map[string]int32)
	for i, entry := range batch.Entries {
		compressed, err := compressEntry(entry, &lookup, registry)
		if err != nil {
			return nil, fmt.Errorf("batch entry %d: %w", i, err)
		}
		entries = append(entries, compressed)
	}
	return &CompressedBatchProof{Entries: entries, LookupInners: lookup}, nil
}

func compressEntry(entry *BatchEntry, lookup *[]*InnerOp, registry map[string]int32) (*CompressedBatchEntry, error) {
	if err := validateBatchEntry(entry); err != nil {
		return nil, err
	}
	if exist := entry.GetExist(); exist != nil {
		compressed, err := compressExist(exist, lookup, registry)
		if err != nil {
			return nil, err
		}
		return &CompressedBatchEntry{Proof: &CompressedBatchEntry_Exist{Exist: compressed}}, nil
	}
	non := entry.GetNonexist()
	left, err := compressExist(non.Left, lookup, registry)
	if err != nil {
		return nil, err
	}
	right, err := compressExist(non.Right, lookup, registry)
	if err != nil {
		return nil, err
	}
	return &CompressedBatchEntry{Proof: &CompressedBatchEntry_Nonexist{Nonexist: &CompressedNonExistenceProof{
		Key: non.Key, Left: left, Right: right,
	}}}, nil
}

func compressExist(exist *ExistenceProof, lookup *[]*InnerOp, registry map[string]int32) (*CompressedExistenceProof, error) {
	if exist == nil {
		return nil, nil // An absent neighbor is valid in a non-existence proof.
	}
	if err := validateExistence(exist); err != nil {
		return nil, err
	}
	res := &CompressedExistenceProof{Key: exist.Key, Value: exist.Value, Leaf: exist.Leaf, Path: make([]int32, len(exist.Path))}
	for i, step := range exist.Path {
		index, err := compressStep(step, lookup, registry)
		if err != nil {
			return nil, err
		}
		res.Path[i] = index
	}
	return res, nil
}

func compressStep(step *InnerOp, lookup *[]*InnerOp, registry map[string]int32) (int32, error) {
	if step == nil {
		return 0, fmt.Errorf("nil inner operation")
	}
	bz, err := step.Marshal()
	if err != nil {
		return 0, fmt.Errorf("marshal inner operation: %w", err)
	}
	sig := string(bz)
	if num, ok := registry[sig]; ok {
		return num, nil
	}
	if int64(len(*lookup)) > int64(1<<31-1) {
		return 0, fmt.Errorf("too many inner operations")
	}
	num := int32(len(*lookup))
	*lookup = append(*lookup, step)
	registry[sig] = num
	return num, nil
}

func decompress(comp *CompressedBatchProof) (*BatchProof, error) {
	if comp == nil || len(comp.Entries) == 0 {
		return nil, fmt.Errorf("compressed proof has no entries")
	}
	for i, op := range comp.LookupInners {
		if op == nil {
			return nil, fmt.Errorf("nil lookup operation %d", i)
		}
	}
	entries := make([]*BatchEntry, len(comp.Entries))
	for i, entry := range comp.Entries {
		var err error
		entries[i], err = decompressEntry(entry, comp.LookupInners)
		if err != nil {
			return nil, fmt.Errorf("compressed entry %d: %w", i, err)
		}
	}
	return &BatchProof{Entries: entries}, nil
}

// TendermintSpec constrains the format from proofs-tendermint (crypto/merkle SimpleProof)
var TendermintSpec = &ProofSpec{
	LeafSpec: &LeafOp{
		Prefix:       []byte{0},
		PrehashKey:   HashOp_NO_HASH,
		Hash:         HashOp_SHA256,
		PrehashValue: HashOp_SHA256,
		Length:       LengthOp_VAR_PROTO,
	},
	InnerSpec: &InnerSpec{
		ChildOrder:      []int32{0, 1},
		MinPrefixLength: 1,
		MaxPrefixLength: 1,
		ChildSize:       32, // (no length byte)
		Hash:            HashOp_SHA256,
	},
}

func decompressExist(exist *CompressedExistenceProof, lookup []*InnerOp) (*ExistenceProof, error) {
	if exist == nil {
		return nil, nil // An absent neighbor is valid in a non-existence proof.
	}
	res := &ExistenceProof{Key: exist.Key, Value: exist.Value, Leaf: exist.Leaf, Path: make([]*InnerOp, len(exist.Path))}
	for i, index := range exist.Path {
		if index < 0 || int64(index) >= int64(len(lookup)) {
			return nil, fmt.Errorf("inner index %d out of range for lookup length %d", index, len(lookup))
		}
		if lookup[index] == nil {
			return nil, fmt.Errorf("nil lookup operation %d", index)
		}
		res.Path[i] = lookup[index]
	}
	if err := validateExistence(res); err != nil {
		return nil, err
	}
	return res, nil
}
