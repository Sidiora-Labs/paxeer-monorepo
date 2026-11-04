/*
*
This implements the client side functions as specified in
https://github.com/cosmos/ics/tree/master/spec/ics-023-vector-commitments

In particular:

	// Assumes ExistenceProof
	type verifyMembership = (root: CommitmentRoot, proof: CommitmentProof, key: Key, value: Value) => boolean

	// Assumes NonExistenceProof
	type verifyNonMembership = (root: CommitmentRoot, proof: CommitmentProof, key: Key) => boolean

	// Assumes BatchProof - required ExistenceProofs may be a subset of all items proven
	type batchVerifyMembership = (root: CommitmentRoot, proof: CommitmentProof, items: Map<Key, Value>) => boolean

	// Assumes BatchProof - required NonExistenceProofs may be a subset of all items proven
	type batchVerifyNonMembership = (root: CommitmentRoot, proof: CommitmentProof, keys: Set<Key>) => boolean

We make an adjustment to accept a Spec to ensure the provided proof is in the format of the expected merkle store.
This can avoid an range of attacks on fake preimages, as we need to be careful on how to map key, value -> leaf
and determine neighbors
*/
package ics23

import (
	"bytes"
	"fmt"
)

// CommitmentRoot is a byte slice that represents the merkle root of a tree that can be used to validate proofs
type CommitmentRoot []byte

// VerifyMembership returns true iff
// proof is (contains) an ExistenceProof for the given key and value AND
// calculating the root for the ExistenceProof matches the provided CommitmentRoot
func VerifyMembership(spec *ProofSpec, root CommitmentRoot, proof *CommitmentProof, key []byte, value []byte) bool {
	// decompress it before running code (no-op if not compressed)
	if validateSpec(spec) != nil {
		return false
	}
	var err error
	proof, err = Decompress(proof)
	if err != nil {
		return false
	}
	ep := getExistProofForKey(proof, key)
	if ep == nil {
		return false
	}
	err = ep.Verify(spec, root, key, value)
	return err == nil
}

// VerifyNonMembership returns true iff
// proof is (contains) a NonExistenceProof
// both left and right sub-proofs are valid existence proofs (see above) or nil
// left and right proofs are neighbors (or left/right most if one is nil)
// provided key is between the keys of the two proofs
func VerifyNonMembership(spec *ProofSpec, root CommitmentRoot, proof *CommitmentProof, key []byte) bool {
	// decompress it before running code (no-op if not compressed)
	if validateSpec(spec) != nil {
		return false
	}
	var err error
	proof, err = Decompress(proof)
	if err != nil {
		return false
	}
	np := getNonExistProofForKey(proof, key)
	if np == nil {
		return false
	}
	err = np.Verify(spec, root, key)
	return err == nil
}

// BatchVerifyMembership will ensure all items are also proven by the CommitmentProof (which should be a BatchProof,
// unless there is one item, when a ExistenceProof may work)
func BatchVerifyMembership(spec *ProofSpec, root CommitmentRoot, proof *CommitmentProof, items map[string][]byte) bool {
	// decompress it before running code (no-op if not compressed) - once for batch
	if validateSpec(spec) != nil {
		return false
	}
	var err error
	proof, err = Decompress(proof)
	if err != nil {
		return false
	}
	for k, v := range items {
		valid := VerifyMembership(spec, root, proof, []byte(k), v)
		if !valid {
			return false
		}
	}
	return true
}

// BatchVerifyNonMembership will ensure all items are also proven to not be in the Commitment by the CommitmentProof
// (which should be a BatchProof, unless there is one item, when a NonExistenceProof may work)
func BatchVerifyNonMembership(spec *ProofSpec, root CommitmentRoot, proof *CommitmentProof, keys [][]byte) bool {
	// decompress it before running code (no-op if not compressed) - once for batch
	if validateSpec(spec) != nil {
		return false
	}
	var err error
	proof, err = Decompress(proof)
	if err != nil {
		return false
	}
	for _, k := range keys {
		valid := VerifyNonMembership(spec, root, proof, k)
		if !valid {
			return false
		}
	}
	return true
}

// CombineProofs takes a number of commitment proofs (simple or batch) and
// converts them into a batch and compresses them.
//
// This is designed for proof generation libraries to create efficient batches
func CombineProofs(proofs []*CommitmentProof) (*CommitmentProof, error) {
	var entries []*BatchEntry

	for _, proof := range proofs {
		var err error
		proof, err = Decompress(proof)
		if err != nil {
			return nil, err
		}
		if ex := proof.GetExist(); ex != nil {
			entry := &BatchEntry{
				Proof: &BatchEntry_Exist{
					Exist: ex,
				},
			}
			entries = append(entries, entry)
		} else if non := proof.GetNonexist(); non != nil {
			entry := &BatchEntry{
				Proof: &BatchEntry_Nonexist{
					Nonexist: non,
				},
			}
			entries = append(entries, entry)
		} else if batch := proof.GetBatch(); batch != nil {
			entries = append(entries, batch.Entries...)
		} else {
			return nil, fmt.Errorf("proof neither exist or nonexist: %#v", proof.GetProof())
		}
	}

	batch := &CommitmentProof{
		Proof: &CommitmentProof_Batch{
			Batch: &BatchProof{
				Entries: entries,
			},
		},
	}

	return Compress(batch)
}

func getExistProofForKey(proof *CommitmentProof, key []byte) *ExistenceProof {
	if validateProof(proof) != nil {
		return nil
	}
	switch p := proof.Proof.(type) {
	case *CommitmentProof_Exist:
		ep := p.Exist
		if bytes.Equal(ep.Key, key) {
			return ep
		}
	case *CommitmentProof_Batch:
		for _, sub := range p.Batch.Entries {
			if ep := sub.GetExist(); ep != nil && bytes.Equal(ep.Key, key) {
				return ep
			}
		}
	}
	return nil
}

func getNonExistProofForKey(proof *CommitmentProof, key []byte) *NonExistenceProof {
	if validateProof(proof) != nil {
		return nil
	}
	switch p := proof.Proof.(type) {
	case *CommitmentProof_Nonexist:
		np := p.Nonexist
		if isLeft(np.Left, key) && isRight(np.Right, key) {
			return np
		}
	case *CommitmentProof_Batch:
		for _, sub := range p.Batch.Entries {
			if np := sub.GetNonexist(); np != nil && isLeft(np.Left, key) && isRight(np.Right, key) {
				return np
			}
		}
	}
	return nil
}

func isLeft(left *ExistenceProof, key []byte) bool {
	return left == nil || bytes.Compare(left.Key, key) < 0
}

func isRight(right *ExistenceProof, key []byte) bool {
	return right == nil || bytes.Compare(right.Key, key) > 0
}

// validateProof checks all entries before any lookup can accept a partial batch.
func validateProof(proof *CommitmentProof) error {
	switch p := proof.GetProof().(type) {
	case *CommitmentProof_Exist:
		if p != nil {
			return validateExistence(p.Exist)
		}
	case *CommitmentProof_Nonexist:
		if p != nil {
			return validateNonExistence(p.Nonexist)
		}
	case *CommitmentProof_Batch:
		if p == nil || p.Batch == nil || len(p.Batch.Entries) == 0 {
			return fmt.Errorf("batch proof has no entries")
		}
		for i, entry := range p.Batch.Entries {
			if err := validateBatchEntry(entry); err != nil {
				return fmt.Errorf("batch entry %d: %w", i, err)
			}
		}
		return nil
	case *CommitmentProof_Compressed:
		if p != nil {
			_, err := decompress(p.Compressed)
			return err
		}
	}
	return fmt.Errorf("missing or unrecognized proof variant")
}

func validateBatchEntry(entry *BatchEntry) error {
	switch p := entry.GetProof().(type) {
	case *BatchEntry_Exist:
		if p != nil {
			return validateExistence(p.Exist)
		}
	case *BatchEntry_Nonexist:
		if p != nil {
			return validateNonExistence(p.Nonexist)
		}
	}
	return fmt.Errorf("missing batch entry variant")
}

func validateExistence(proof *ExistenceProof) error {
	if proof == nil || proof.Leaf == nil {
		return fmt.Errorf("existence proof needs a leaf operation")
	}
	for i, op := range proof.Path {
		if op == nil {
			return fmt.Errorf("nil inner operation %d", i)
		}
	}
	return nil
}

func validateNonExistence(proof *NonExistenceProof) error {
	if proof == nil || (proof.Left == nil && proof.Right == nil) {
		return fmt.Errorf("non-existence proof needs a neighbor")
	}
	for _, neighbor := range []*ExistenceProof{proof.Left, proof.Right} {
		if neighbor != nil {
			if err := validateExistence(neighbor); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSpec(spec *ProofSpec) error {
	if spec == nil || spec.LeafSpec == nil {
		return fmt.Errorf("proof specification needs a leaf specification")
	}
	if spec.MinDepth < 0 || spec.MaxDepth < 0 || (spec.MaxDepth > 0 && spec.MinDepth > spec.MaxDepth) {
		return fmt.Errorf("invalid proof depth bounds")
	}
	return validateInnerSpec(spec.InnerSpec)
}

func validateInnerSpec(spec *InnerSpec) error {
	if spec == nil || spec.ChildSize <= 0 || len(spec.ChildOrder) == 0 || spec.MinPrefixLength < 0 || spec.MaxPrefixLength < spec.MinPrefixLength {
		return fmt.Errorf("invalid inner specification")
	}
	seen := make([]bool, len(spec.ChildOrder))
	for _, branch := range spec.ChildOrder {
		if branch < 0 || int64(branch) >= int64(len(seen)) || seen[branch] {
			return fmt.Errorf("child order must be a permutation")
		}
		seen[branch] = true
	}
	return nil
}
