package ics23

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type repairM1359Vector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Root  string `json:"root"`
	Proof string `json:"proof"`
}

func repairM1359Hex(t *testing.T, value string) []byte {
	t.Helper()
	bz, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return bz
}

func repairM1359Load(t *testing.T, tree, name string) (*CommitmentProof, CommitmentRoot, []byte, []byte) {
	t.Helper()
	bz, err := os.ReadFile(filepath.Join("testdata", "repair_m1359", tree, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var vector repairM1359Vector
	if err := json.Unmarshal(bz, &vector); err != nil {
		t.Fatal(err)
	}
	proof := new(CommitmentProof)
	if err := proof.Unmarshal(repairM1359Hex(t, vector.Proof)); err != nil {
		t.Fatal(err)
	}
	return proof, repairM1359Hex(t, vector.Root), repairM1359Hex(t, vector.Key), repairM1359Hex(t, vector.Value)
}

func repairM1359Transport(t *testing.T, proof *CommitmentProof) *CommitmentProof {
	t.Helper()
	bz, err := proof.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(CommitmentProof)
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func repairM1359Reject(t *testing.T, proof *CommitmentProof) {
	t.Helper()
	if got, err := Decompress(proof); err == nil || got != nil {
		t.Fatalf("Decompress = %v, %v; want nil, error", got, err)
	}
	if got, err := Compress(proof); err == nil || got != nil {
		t.Fatalf("Compress = %v, %v; want nil, error", got, err)
	}
	if got, err := CombineProofs([]*CommitmentProof{proof}); err == nil || got != nil {
		t.Fatalf("CombineProofs = %v, %v; want nil, error", got, err)
	}
	if got, err := proof.Calculate(); err == nil || got != nil {
		t.Fatalf("Calculate = %x, %v; want nil, error", got, err)
	}
	if VerifyMembership(TendermintSpec, nil, proof, []byte("key"), []byte("value")) {
		t.Fatal("malformed membership accepted")
	}
	if VerifyNonMembership(TendermintSpec, nil, proof, []byte("key")) {
		t.Fatal("malformed non-membership accepted")
	}
	if BatchVerifyMembership(TendermintSpec, nil, proof, nil) {
		t.Fatal("malformed empty membership batch accepted")
	}
	if BatchVerifyNonMembership(TendermintSpec, nil, proof, nil) {
		t.Fatal("malformed empty non-membership batch accepted")
	}
}

func TestRepairM_1359_VerificationCoverage(t *testing.T) {
	for tree, spec := range map[string]*ProofSpec{"tendermint": TendermintSpec, "iavl": IavlSpec} {
		for _, name := range []string{"exist_left", "exist_middle", "exist_right", "nonexist_left", "nonexist_middle", "nonexist_right"} {
			t.Run(tree+"/"+name, func(t *testing.T) {
				proof, root, key, value := repairM1359Load(t, tree, name)
				check := func(proof *CommitmentProof) {
					t.Helper()
					calculated, err := proof.Calculate()
					if err != nil || !bytes.Equal(calculated, root) {
						t.Fatalf("Calculate = %x, %v; want %x", calculated, err, root)
					}
					badRoot := append(CommitmentRoot(nil), root...)
					badRoot[0] ^= 1
					if value != nil && len(value) > 0 {
						if !VerifyMembership(spec, root, proof, key, value) {
							t.Fatal("producer membership rejected")
						}
						if !BatchVerifyMembership(spec, root, proof, map[string][]byte{string(key): value}) {
							t.Fatal("producer batch membership rejected")
						}
						if VerifyMembership(spec, badRoot, proof, key, value) || VerifyMembership(spec, root, proof, key, []byte("wrong")) || VerifyMembership(spec, root, proof, []byte("wrong"), value) {
							t.Fatal("tampered membership accepted")
						}
					} else {
						if !VerifyNonMembership(spec, root, proof, key) {
							t.Fatal("producer non-membership rejected")
						}
						if !BatchVerifyNonMembership(spec, root, proof, [][]byte{key}) {
							t.Fatal("producer batch non-membership rejected")
						}
						if VerifyNonMembership(spec, badRoot, proof, key) {
							t.Fatal("tampered root accepted")
						}
						non := getNonExistProofForKey(proof, key)
						if IsCompressed(proof) {
							expanded, err := Decompress(proof)
							if err != nil {
								t.Fatal(err)
							}
							non = getNonExistProofForKey(expanded, key)
						}
						neighbor := non.Left
						if neighbor == nil {
							neighbor = non.Right
						}
						if VerifyNonMembership(spec, root, proof, neighbor.Key) {
							t.Fatal("existing neighbor accepted as absent")
						}
					}
				}
				check(proof)
				// Repeated proofs share real inner operations in the compression registry.
				compressed, err := CombineProofs([]*CommitmentProof{proof, proof})
				if err != nil {
					t.Fatal(err)
				}
				if !IsCompressed(compressed) || len(compressed.GetCompressed().Entries) != 2 || len(compressed.GetCompressed().LookupInners) == 0 {
					t.Fatal("combination did not produce compressed entries and lookup table")
				}
				if got, err := Compress(compressed); err != nil || got != compressed {
					t.Fatalf("compression not idempotent: %v", err)
				}
				decoded := repairM1359Transport(t, compressed)
				check(decoded)
				expanded, err := Decompress(decoded)
				if err != nil {
					t.Fatal(err)
				}
				if got, err := Decompress(expanded); err != nil || got != expanded {
					t.Fatalf("decompression not idempotent: %v", err)
				}
				check(expanded)
				recompressed, err := Compress(expanded)
				if err != nil {
					t.Fatal(err)
				}
				first, err := decoded.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				second, err := recompressed.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(first, second) {
					t.Fatal("compression round trip changed wire proof")
				}
			})
		}
	}
}

func TestRepairM_1359_ProtobufRetirement(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"retired", "github.com/cosmos/cosmos-sdk/ics23/go v0.8.0", "not this directory"} {
		if !strings.Contains(string(readme), text) {
			t.Fatalf("replacement boundary missing %q", text)
		}
	}
	if strings.Contains(string(makefile), "../proofs.proto") || strings.Contains(string(makefile), "go install") {
		t.Fatal("retired target still references missing schema or installs a generator")
	}
	if !strings.Contains(string(makefile), "protoc install-proto-dep:") || !strings.Contains(string(makefile), "@exit 1") {
		t.Fatal("generation targets must explicitly refuse retired generation")
	}
	// The published producer's transport remains consumable by the retained codec.
	proof, root, key, value := repairM1359Load(t, "tendermint", "exist_middle")
	wire, err := proof.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := repairM1359Transport(t, proof)
	reencoded, err := decoded.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, reencoded) || !VerifyMembership(TendermintSpec, root, decoded, key, value) {
		t.Fatal("historical protobuf wire compatibility lost")
	}
}

func TestRepairM_1359_CompressEmptyVariant(t *testing.T) {
	var nilExist *BatchEntry_Exist
	var nilNonexist *BatchEntry_Nonexist
	cases := []*BatchEntry{nil, {}, {Proof: nilExist}, {Proof: nilNonexist}, {Proof: &BatchEntry_Exist{}}, {Proof: &BatchEntry_Nonexist{}}, {Proof: &BatchEntry_Nonexist{Nonexist: &NonExistenceProof{}}}}
	for i, entry := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			proof := &CommitmentProof{Proof: &CommitmentProof_Batch{Batch: &BatchProof{Entries: []*BatchEntry{entry}}}}
			repairM1359Reject(t, proof)
		})
	}
	decoded := repairM1359Transport(t, &CommitmentProof{Proof: &CommitmentProof_Batch{Batch: &BatchProof{Entries: []*BatchEntry{{}}}}})
	if len(decoded.GetBatch().Entries) != 1 || decoded.GetBatch().Entries[0].GetProof() != nil {
		t.Fatal("codec did not preserve unset entry oneof")
	}
	repairM1359Reject(t, decoded)
}

func TestRepairM_1359_CompressedIndexBounds(t *testing.T) {
	for _, nonexist := range []bool{false, true} {
		for _, index := range []int32{-1, -2147483648, 0, 2147483647} {
			for _, empty := range []bool{false, true} {
				t.Run(fmt.Sprintf("nonexist=%v/index=%d/empty=%v", nonexist, index, empty), func(t *testing.T) {
					name := "exist_middle"
					if nonexist {
						name = "nonexist_middle"
					}
					proof, root, key, value := repairM1359Load(t, "tendermint", name)
					compressed, err := CombineProofs([]*CommitmentProof{proof})
					if err != nil {
						t.Fatal(err)
					}
					batch := compressed.GetCompressed()
					badIndex := index
					if index == 0 && !empty {
						badIndex = int32(len(batch.LookupInners))
					}
					if empty {
						batch.LookupInners = nil
					}
					entry := batch.Entries[0]
					var existence *CompressedExistenceProof
					if nonexist {
						existence = entry.GetNonexist().Left
					} else {
						existence = entry.GetExist()
					}
					existence.Path[0] = badIndex
					decoded := repairM1359Transport(t, compressed)
					var path []int32
					if nonexist {
						path = decoded.GetCompressed().Entries[0].GetNonexist().Left.Path
					} else {
						path = decoded.GetCompressed().Entries[0].GetExist().Path
					}
					if path[0] != badIndex {
						t.Fatal("wire codec changed malformed index")
					}
					repairM1359Reject(t, decoded)
					if VerifyMembership(TendermintSpec, root, decoded, key, value) || VerifyNonMembership(TendermintSpec, root, decoded, key) {
						t.Fatal("invalid decoded index accepted against real root")
					}
				})
			}
		}
	}
	proof, _, _, _ := repairM1359Load(t, "tendermint", "exist_middle")
	compressed, err := CombineProofs([]*CommitmentProof{proof})
	if err != nil {
		t.Fatal(err)
	}
	compressed.GetCompressed().LookupInners[0] = nil
	repairM1359Reject(t, compressed)
}

func TestRepairM_1359_PublicVerifierInputs(t *testing.T) {
	var exist *CommitmentProof_Exist
	var nonexist *CommitmentProof_Nonexist
	var batch *CommitmentProof_Batch
	var compressed *CommitmentProof_Compressed
	cases := []*CommitmentProof{nil, {}, {Proof: exist}, {Proof: nonexist}, {Proof: batch}, {Proof: compressed}, {Proof: &CommitmentProof_Exist{}}, {Proof: &CommitmentProof_Nonexist{}}, {Proof: &CommitmentProof_Batch{}}, {Proof: &CommitmentProof_Compressed{}}, {Proof: &CommitmentProof_Batch{Batch: &BatchProof{}}}, {Proof: &CommitmentProof_Compressed{Compressed: &CompressedBatchProof{}}}}
	for i, proof := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) { repairM1359Reject(t, proof) })
	}
	for _, proof := range []*CommitmentProof{{}, {Proof: &CommitmentProof_Exist{}}, {Proof: &CommitmentProof_Nonexist{}}, {Proof: &CommitmentProof_Batch{}}, {Proof: &CommitmentProof_Compressed{}}} {
		repairM1359Reject(t, repairM1359Transport(t, proof))
	}
	proof, root, key, value := repairM1359Load(t, "tendermint", "exist_middle")
	badSpecs := []*ProofSpec{nil, {}, {LeafSpec: TendermintSpec.LeafSpec}, {InnerSpec: TendermintSpec.InnerSpec}}
	for _, change := range []func(*ProofSpec){
		func(p *ProofSpec) { p.InnerSpec.ChildSize = 0 },
		func(p *ProofSpec) { p.InnerSpec.ChildSize = -1 },
		func(p *ProofSpec) { p.InnerSpec.ChildOrder = nil },
		func(p *ProofSpec) { p.InnerSpec.ChildOrder = []int32{0, 0} },
		func(p *ProofSpec) { p.InnerSpec.ChildOrder = []int32{0, 2} },
		func(p *ProofSpec) { p.InnerSpec.ChildOrder = []int32{-1, 1} },
		func(p *ProofSpec) { p.InnerSpec.MinPrefixLength = -1 },
		func(p *ProofSpec) { p.InnerSpec.MaxPrefixLength = 0 },
		func(p *ProofSpec) { p.MinDepth = -1 },
		func(p *ProofSpec) { p.MinDepth = 2; p.MaxDepth = 1 },
	} {
		p := *TendermintSpec
		inner := *p.InnerSpec
		p.InnerSpec = &inner
		change(&p)
		badSpecs = append(badSpecs, &p)
	}
	non, nonRoot, nonKey, _ := repairM1359Load(t, "tendermint", "nonexist_middle")
	for i, spec := range badSpecs {
		t.Run(fmt.Sprintf("spec/%d", i), func(t *testing.T) {
			if VerifyMembership(spec, root, proof, key, value) || BatchVerifyMembership(spec, root, proof, nil) || VerifyNonMembership(spec, nonRoot, non, nonKey) || BatchVerifyNonMembership(spec, nonRoot, non, nil) {
				t.Fatal("invalid specification accepted")
			}
			if err := proof.GetExist().Verify(spec, root, key, value); err == nil {
				t.Fatal("direct existence verification accepted invalid spec")
			}
			if err := non.GetNonexist().Verify(spec, nonRoot, nonKey); err == nil {
				t.Fatal("direct non-existence verification accepted invalid spec")
			}
		})
	}
	proof.GetExist().Path[0] = nil
	repairM1359Reject(t, proof)
	// A valid entry must not hide a malformed sibling entry.
	valid, _, _, _ := repairM1359Load(t, "tendermint", "exist_middle")
	mixed := &CommitmentProof{Proof: &CommitmentProof_Batch{Batch: &BatchProof{Entries: []*BatchEntry{{Proof: &BatchEntry_Exist{Exist: valid.GetExist()}}, {}}}}}
	repairM1359Reject(t, repairM1359Transport(t, mixed))
	if VerifyMembership(TendermintSpec, root, mixed, key, value) {
		t.Fatal("valid entry concealed malformed batch entry")
	}
}

func TestRepairM_1359_RootLengthError(t *testing.T) {
	for _, childSize := range []int32{32, 33, 48, 63} {
		t.Run(fmt.Sprint(childSize), func(t *testing.T) {
			spec := *TendermintSpec
			inner := *spec.InnerSpec
			spec.InnerSpec = &inner
			spec.InnerSpec.Hash = HashOp_SHA512
			spec.InnerSpec.ChildSize = childSize
			proof := repairM1359Transport(t, &CommitmentProof{Proof: &CommitmentProof_Exist{Exist: &ExistenceProof{Key: []byte("key"), Value: []byte("value"), Leaf: spec.LeafSpec, Path: []*InnerOp{{Hash: HashOp_SHA512, Prefix: []byte{1}, Suffix: make([]byte, childSize)}}}}})
			exist := proof.GetExist()
			if err := exist.CheckAgainstSpec(&spec); err != nil {
				t.Fatalf("size mismatch must reach calculation: %v", err)
			}
			root, err := exist.calculate(&spec)
			if err == nil || root != nil || !strings.Contains(err.Error(), "inner hash length 64 exceeds child size") {
				t.Fatalf("size mismatch = %x, %v", root, err)
			}
			unconstrained, err := exist.Calculate()
			if err != nil || len(unconstrained) != 64 {
				t.Fatalf("unconstrained calculation = %x, %v", unconstrained, err)
			}
			if err := exist.Verify(&spec, nil, exist.Key, exist.Value); err == nil {
				t.Fatal("nil root masked size violation")
			}
			if VerifyMembership(&spec, nil, proof, exist.Key, exist.Value) || VerifyMembership(&spec, unconstrained, proof, exist.Key, exist.Value) {
				t.Fatal("size violation accepted by verifier")
			}
			spec.InnerSpec.ChildSize = 64
			exist.Path[0].Suffix = make([]byte, 64)
			consistent, err := exist.calculate(&spec)
			if err != nil || len(consistent) != 64 {
				t.Fatalf("consistent size rejected: %v", err)
			}
			if !VerifyMembership(&spec, consistent, proof, exist.Key, exist.Value) {
				t.Fatal("consistent SHA512 proof rejected")
			}
		})
	}
}

func TestRepairM_1359_DecompressEmptyVariant(t *testing.T) {
	var exist *CompressedBatchEntry_Exist
	var nonexist *CompressedBatchEntry_Nonexist
	for i, entry := range []*CompressedBatchEntry{nil, {}, {Proof: exist}, {Proof: nonexist}, {Proof: &CompressedBatchEntry_Exist{}}, {Proof: &CompressedBatchEntry_Nonexist{}}, {Proof: &CompressedBatchEntry_Nonexist{Nonexist: &CompressedNonExistenceProof{}}}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			proof := &CommitmentProof{Proof: &CommitmentProof_Compressed{Compressed: &CompressedBatchProof{Entries: []*CompressedBatchEntry{entry}}}}
			repairM1359Reject(t, proof)
		})
	}
	for i, entry := range []*CompressedBatchEntry{{}, {Proof: &CompressedBatchEntry_Exist{}}, {Proof: &CompressedBatchEntry_Nonexist{}}} {
		t.Run(fmt.Sprintf("decoded/%d", i), func(t *testing.T) {
			proof := repairM1359Transport(t, &CommitmentProof{Proof: &CommitmentProof_Compressed{Compressed: &CompressedBatchProof{Entries: []*CompressedBatchEntry{entry}}}})
			if len(proof.GetCompressed().Entries) != 1 {
				t.Fatal("empty message was not decoded as a compressed entry")
			}
			repairM1359Reject(t, proof)
		})
	}
}

func TestRepairM_1359_NeighborPathPanics(t *testing.T) {
	proof, root, key, _ := repairM1359Load(t, "tendermint", "nonexist_middle")
	non := proof.GetNonexist()
	left, right := non.Left.Path, non.Right.Path
	if !IsLeftNeighbor(TendermintSpec.InnerSpec, left, right) || !VerifyNonMembership(TendermintSpec, root, proof, key) {
		t.Fatal("recorded neighbor proof rejected")
	}
	common := &InnerOp{Hash: HashOp_SHA256, Prefix: []byte{1}, Suffix: make([]byte, 32)}
	cases := []struct {
		name        string
		left, right []*InnerOp
	}{
		{"empty", nil, nil}, {"empty-left", nil, right}, {"empty-right", left, nil},
		{"nil-left", []*InnerOp{nil}, right}, {"nil-right", left, []*InnerOp{nil}},
		{"identical", left, left}, {"exhausted-left", []*InnerOp{common}, []*InnerOp{common, common}},
		{"exhausted-right", []*InnerOp{common, common}, []*InnerOp{common}},
		{"bad-padding-left", []*InnerOp{{Hash: HashOp_SHA256}}, right},
		{"bad-padding-right", left, []*InnerOp{{Hash: HashOp_SHA256}}},
		{"nil-below-split", append([]*InnerOp{nil}, left...), right},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if IsLeftNeighbor(TendermintSpec.InnerSpec, c.left, c.right) {
				t.Fatal("invalid neighbor paths accepted")
			}
		})
	}
	for _, spec := range []*InnerSpec{nil, {}, {ChildSize: 32, ChildOrder: []int32{0, 0}}, {ChildSize: 32, ChildOrder: []int32{0, 2}}} {
		if IsLeftNeighbor(spec, left, right) || IsLeftMost(spec, left) || IsRightMost(spec, right) {
			t.Fatal("invalid inner specification accepted")
		}
	}
	if IsLeftMost(TendermintSpec.InnerSpec, []*InnerOp{nil}) || IsRightMost(TendermintSpec.InnerSpec, []*InnerOp{nil}) {
		t.Fatal("nil boundary path accepted")
	}
	// Empty paths are valid only for a single-leaf boundary, never two neighbors.
	single := &ExistenceProof{Key: []byte("b"), Value: []byte("v"), Leaf: TendermintSpec.LeafSpec}
	singleRoot, err := single.Calculate()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		key         string
		left, right *ExistenceProof
	}{{"a", nil, single}, {"c", single, nil}} {
		p := repairM1359Transport(t, &CommitmentProof{Proof: &CommitmentProof_Nonexist{Nonexist: &NonExistenceProof{Key: []byte(c.key), Left: c.left, Right: c.right}}})
		if !VerifyNonMembership(TendermintSpec, singleRoot, p, []byte(c.key)) {
			t.Fatal("single-leaf boundary rejected")
		}
	}
	for _, mutate := range []func(*NonExistenceProof){
		func(p *NonExistenceProof) { p.Left.Path = nil; p.Right.Path = nil },
		func(p *NonExistenceProof) { p.Right.Path = p.Left.Path },
		func(p *NonExistenceProof) { p.Left.Path[0] = &InnerOp{Hash: HashOp_SHA256} },
	} {
		p, _, _, _ := repairM1359Load(t, "tendermint", "nonexist_middle")
		mutate(p.GetNonexist())
		decoded := repairM1359Transport(t, p)
		if VerifyNonMembership(TendermintSpec, root, decoded, key) {
			t.Fatal("malformed decoded non-membership accepted")
		}
	}
	// Permuted sibling slots once allowed placeholder checks to slice out of bounds.
	permuted := &InnerSpec{ChildOrder: []int32{0, 2, 1}, ChildSize: 32, MinPrefixLength: 1, MaxPrefixLength: 1, EmptyChild: make([]byte, 32)}
	if IsRightMost(permuted, []*InnerOp{{Prefix: []byte{1}, Suffix: make([]byte, 64)}}) {
		t.Fatal("non-contiguous placeholder siblings accepted")
	}
}
