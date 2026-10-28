package layerx

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

type committedFixture struct {
	receipt    SequencerReceiptVerification
	header     CommittedHeader
	inclusion  ReceiptInclusion
	commit     CometCommit
	validators []CometValidator
	keys       []ed25519.PrivateKey
}

func committedFixtureTransactionTree(txs [3][]byte) ([32]byte, [3]CommittedProof) {
	var leaves [3][32]byte
	for n, tx := range txs {
		hash := sha256.Sum256(tx)
		var preimage [33]byte
		copy(preimage[1:], hash[:])
		leaves[n] = sha256.Sum256(preimage[:])
	}
	inner := func(left, right [32]byte) [32]byte {
		var preimage [65]byte
		preimage[0] = 1
		copy(preimage[1:33], left[:])
		copy(preimage[33:], right[:])
		return sha256.Sum256(preimage[:])
	}
	left := inner(leaves[0], leaves[1])
	return inner(left, leaves[2]), [3]CommittedProof{
		{Total: 3, Index: 0, Aunts: [][32]byte{leaves[1], leaves[2]}},
		{Total: 3, Index: 1, Aunts: [][32]byte{leaves[0], leaves[2]}},
		{Total: 3, Index: 2, Aunts: [][32]byte{left}},
	}
}

func committedFixtureProof(leaves [][]byte, index int) CommittedProof {
	proof := CommittedProof{Total: int64(len(leaves)), Index: int64(index)}
	var visit func([][]byte, int)
	visit = func(items [][]byte, position int) {
		if len(items) == 1 {
			return
		}
		split := int(committedSplit(int64(len(items))))
		if position < split {
			visit(items[:split], position)
			proof.Aunts = append(proof.Aunts, committedRoot(items[split:]))
		} else {
			visit(items[split:], position-split)
			proof.Aunts = append(proof.Aunts, committedRoot(items[:split]))
		}
	}
	visit(leaves, index)
	return proof
}

func committedFixtureCommit(header CommittedHeader, keys []ed25519.PrivateKey, signers int) CometCommit {
	hash := header.Hash()
	commit := CometCommit{Height: header.Height, Round: 7,
		BlockID:    CometBlockID{Hash: hash[:], PartSetTotal: 2, PartSetHash: bytes.Repeat([]byte{0x22}, 32)},
		Signatures: make([]CometCommitSignature, len(keys))}
	for n, key := range keys {
		if n >= signers {
			commit.Signatures[n].BlockIDFlag = CometAbsent
			continue
		}
		address := sha256.Sum256(key.Public().(ed25519.PublicKey))
		sig := CometCommitSignature{BlockIDFlag: CometBlock, ValidatorAddress: address[:20], Timestamp: time.Unix(1700000123+int64(n), 987654321-int64(n)).UTC()}
		sig.Signature = ed25519.Sign(key, committedVoteBytes(header.ChainID, commit, sig))
		commit.Signatures[n] = sig
	}
	return commit
}

func newCommittedFixture(t *testing.T, id string, powers [4]int64, offset int64) committedFixture {
	t.Helper()
	var c sequencerReceiptCase
	for _, candidate := range sequencerReceiptCases(t) {
		if candidate.ID == id {
			c = candidate
			break
		}
	}
	if c.ID == "" {
		t.Fatal("receipt fixture not found")
	}
	raw := sequencerFixtureBytes(t, c.RawTransaction)
	receipt, err := VerifySequencerReceipt(c.ReceiptJSON, raw, sequencerFixtureKeys(t, c))
	if err != nil {
		t.Fatal(err)
	}
	f := committedFixture{receipt: receipt}
	for n, power := range powers {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(n + 1)}, 32))
		f.keys = append(f.keys, key)
		var public [32]byte
		copy(public[:], key.Public().(ed25519.PublicKey))
		f.validators = append(f.validators, CometValidator{PublicKey: public, VotingPower: power})
	}
	r := receipt.Receipt
	height := int64(binary.BigEndian.Uint64(r.BatchID[24:])) + offset
	const domain = "PAXEER/v1/sequence-batch\x00"
	batch := append([]byte(nil), domain...)
	batch = binary.BigEndian.AppendUint64(batch, uint64(height))
	batch = binary.BigEndian.AppendUint64(batch, r.GlobalSequence)
	batch = binary.BigEndian.AppendUint64(batch, r.GlobalSequence)
	batch = append(batch, r.PreviousStateRoot[:]...)
	batch = append(batch, r.ResultingStateRoot[:]...)
	txRoot := sha256.Sum256(r.ActivityID[:])
	batch = append(batch, txRoot[:]...)
	batch = append(batch, r.ContextHash[:]...)
	signed := append([]byte(domain+"committed-sdk-test"), batch[len(domain):len(domain)+24+96]...)
	batchDigest := sha256.Sum256(signed)
	batchKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x5e}, 32))
	batch = append(batch, ed25519.Sign(batchKey, batchDigest[:])...)
	gas := int64(binary.BigEndian.Uint64(r.Effects[0].Body[:8]))
	result := ExecTxResult{GasWanted: gas + 1000, GasUsed: gas}
	if r.ResultCode == 1 {
		result.Code = 45
	}
	response := committedVarint(1, uint64(gas))
	if len(receipt.ReturnData) > 0 {
		response = append(response, committedBytes(3, receipt.ReturnData)...)
	}
	response = append(response, committedBytes(4, []byte("0x"+hex.EncodeToString(r.ActivityID[:])))...)
	message := committedBytes(1, []byte("/paxprotocol.paxchain.evm.MsgEVMTransaction"))
	message = append(message, committedBytes(2, response)...)
	result.Data = committedBytes(1, message)
	// Three leaves exercise both a full subtree and the unbalanced right branch.
	txs := [3][]byte{batch, raw, sequencerFixtureBytes(t, sequencerReceiptCases(t)[1].RawTransaction)}
	results := [][]byte{(ExecTxResult{}).bytes(), result.bytes(), (ExecTxResult{GasWanted: 60000, GasUsed: 46220}).bytes()}
	dataHash, txProofs := committedFixtureTransactionTree(txs)
	resultsHash := committedRoot(results)
	validatorsHash := committedValidatorsHash(f.validators)
	f.header = CommittedHeader{VersionBlock: 11, VersionApp: 2, ChainID: "committed-sdk-test", Height: height,
		Time: time.Unix(1700000123, 987654321).UTC(), DataHash: dataHash[:], ValidatorsHash: validatorsHash[:], NextValidatorsHash: validatorsHash[:]}
	f.commit = committedFixtureCommit(f.header, f.keys, 3)
	next := f.header
	next.Height++
	next.Time = next.Time.Add(time.Second)
	next.LastBlockID = f.commit.BlockID
	next.LastResultsHash = resultsHash[:]
	f.inclusion = ReceiptInclusion{Transaction: raw, TransactionProof: txProofs[1],
		Result: result, ResultProof: committedFixtureProof(results, 1), BatchTransaction: batch, BatchProof: txProofs[0],
		NextHeader: next, NextCommit: committedFixtureCommit(next, f.keys, 3), NextValidators: f.validators}
	return f
}

func (f committedFixture) verify() (Committed, error) {
	return VerifyCommittedReceipt(f.receipt, f.header, f.inclusion, f.commit, f.validators)
}

func (f *committedFixture) reanchor(t *testing.T) {
	t.Helper()
	txs := [3][]byte{f.inclusion.BatchTransaction, f.inclusion.Transaction, sequencerFixtureBytes(t, sequencerReceiptCases(t)[1].RawTransaction)}
	results := [][]byte{(ExecTxResult{}).bytes(), f.inclusion.Result.bytes(), (ExecTxResult{GasWanted: 60000, GasUsed: 46220}).bytes()}
	dataHash, txProofs := committedFixtureTransactionTree(txs)
	resultsHash, validatorsHash := committedRoot(results), committedValidatorsHash(f.validators)
	f.header.DataHash = dataHash[:]
	f.header.ValidatorsHash = validatorsHash[:]
	f.header.NextValidatorsHash = validatorsHash[:]
	f.commit = committedFixtureCommit(f.header, f.keys, 3)
	f.inclusion.BatchProof = txProofs[0]
	f.inclusion.TransactionProof = txProofs[1]
	f.inclusion.ResultProof = committedFixtureProof(results, 1)
	f.inclusion.NextHeader.ValidatorsHash = validatorsHash[:]
	f.inclusion.NextHeader.LastBlockID = f.commit.BlockID
	f.inclusion.NextHeader.LastResultsHash = resultsHash[:]
	f.inclusion.NextValidators = f.validators
	f.inclusion.NextCommit = committedFixtureCommit(f.inclusion.NextHeader, f.keys, 3)
}

func requireCommittedRefusal(t *testing.T, f committedFixture, check ReceiptCheck) {
	t.Helper()
	result, err := f.verify()
	var verification *VerificationError
	if !errors.As(err, &verification) || verification.Check != check || !errors.Is(err, ErrVerification) || result.Rung != "" {
		t.Fatalf("expected %s refusal, got rung %q, error %v", check, result.Rung, err)
	}
}

func TestCommittedTransactionProofsMatchConsensusContract(t *testing.T) {
	txs := [3][]byte{[]byte("foo"), []byte("bar"), []byte("baz")}
	root, proofs := committedFixtureTransactionTree(txs)
	if got := hex.EncodeToString(root[:]); got != "c9148115f7280f200540da16817e48993037810e4592366390aa81258546107d" {
		t.Fatalf("transaction root differs from SHA256 transaction/RFC6962 contract: %s", got)
	}
	wantAunts := [3][]string{
		{"ebb48bb18728aa8c4fdee8e155e7ea2560ae1e985149f3519b7d6bbaf02e9202", "446c71e2a36c3fb5cf5b000f25ac0ff52998b5e98e369ec51abeb90b8014e4fc"},
		{"1421850065ddc57d658632cd5027088fe5b9c2a5bff09bd6af4a28966f251bd3", "446c71e2a36c3fb5cf5b000f25ac0ff52998b5e98e369ec51abeb90b8014e4fc"},
		{"a0ae9380d7511576a57864ecc0443aa8d9c81e41eda85a441a3d7023060ddf0e"},
	}
	for n, proof := range proofs {
		if proof.Total != 3 || proof.Index != int64(n) || len(proof.Aunts) != len(wantAunts[n]) {
			t.Fatalf("transaction %d has incorrect proof position or length", n)
		}
		for a, aunt := range proof.Aunts {
			if got := hex.EncodeToString(aunt[:]); got != wantAunts[n][a] {
				t.Fatalf("transaction %d aunt %d differs from consensus hashing contract: %s", n, a, got)
			}
		}
		if !committedVerifyTransactionProof(txs[n], proof, root[:]) {
			t.Fatalf("transaction %d consensus-contract proof was refused", n)
		}
		if committedVerifyProof(txs[n], proof, root[:]) {
			t.Fatalf("transaction %d proof accepted an unhashed transaction leaf", n)
		}
	}
}

func TestCommittedRungRefusesWrongProofPreimages(t *testing.T) {
	for _, mutation := range []string{"unhashed transactions", "hashed results"} {
		t.Run(mutation, func(t *testing.T) {
			f := newCommittedFixture(t, "V1-pax-transfer", [4]int64{1, 1, 1, 1}, 0)
			switch mutation {
			case "unhashed transactions":
				txs := [][]byte{f.inclusion.BatchTransaction, f.inclusion.Transaction, sequencerFixtureBytes(t, sequencerReceiptCases(t)[1].RawTransaction)}
				root := committedRoot(txs)
				f.header.DataHash = root[:]
				f.inclusion.BatchProof = committedFixtureProof(txs, 0)
				f.inclusion.TransactionProof = committedFixtureProof(txs, 1)
			case "hashed results":
				results := [3][]byte{(ExecTxResult{}).bytes(), f.inclusion.Result.bytes(), (ExecTxResult{GasWanted: 60000, GasUsed: 46220}).bytes()}
				root, proofs := committedFixtureTransactionTree(results)
				f.inclusion.NextHeader.LastResultsHash = root[:]
				f.inclusion.ResultProof = proofs[1]
			}
			f.commit = committedFixtureCommit(f.header, f.keys, 3)
			f.inclusion.NextHeader.LastBlockID = f.commit.BlockID
			f.inclusion.NextCommit = committedFixtureCommit(f.inclusion.NextHeader, f.keys, 3)
			requireCommittedRefusal(t, f, "committed-inclusion")
		})
	}
}

func TestCommittedRungAcceptsTwoThirdsCommit(t *testing.T) {
	for _, id := range []string{"V1-pax-transfer", "V2-layerxledger-revert"} {
		for _, offset := range []int64{0, 1} {
			f := newCommittedFixture(t, id, [4]int64{1, 1, 1, 1}, offset)
			result, err := f.verify()
			if err != nil {
				t.Fatalf("%s at offset %d: %v", id, offset, err)
			}
			if result.Rung != "committed" || result.Level != "committed" || result.Height != uint64(f.header.Height) || result.VotingPower != 3 || result.BlockHash != f.header.Hash() || result.ReceiptDigest != f.receipt.ReceiptDigest || result.Status != f.receipt.Status {
				t.Fatal("committed result did not preserve the receipt or identify the landing block")
			}
			if f.receipt.Rung != "receipted" {
				t.Fatal("input was mutated")
			}
		}
	}
	t.Run("consensus reference encodings", func(t *testing.T) {
		f := newCommittedFixture(t, "V1-pax-transfer", [4]int64{40, 30, 20, 10}, 0)
		setHash := committedValidatorsHash(f.validators)
		if hex.EncodeToString(setHash[:]) != "0321b92eefbfb6129dcbfb8c9066c149bb1091c312f388e2142ef4bb4ba86fb0" {
			t.Fatal("validator hash differs from recorded consensus Go vector")
		}
		h := CommittedHeader{VersionBlock: 11, VersionApp: 2, ChainID: "paxeer-committed-π", Height: 1234567890123,
			Time: time.Unix(1700000123, 987654321).UTC(), LastBlockID: CometBlockID{Hash: bytes.Repeat([]byte{0x11}, 32), PartSetTotal: 257, PartSetHash: bytes.Repeat([]byte{0x22}, 32)},
			LastCommitHash: bytes.Repeat([]byte{0x33}, 32), DataHash: bytes.Repeat([]byte{0x44}, 32), ValidatorsHash: setHash[:],
			NextValidatorsHash: bytes.Repeat([]byte{0x55}, 32), ConsensusHash: bytes.Repeat([]byte{0x66}, 32), AppHash: []byte{0, 1, 2, 255, 128},
			LastResultsHash: bytes.Repeat([]byte{0x77}, 32), EvidenceHash: bytes.Repeat([]byte{0x88}, 32), ProposerAddress: bytesOfCommittedHex(t, "34750f98bd59fcfc946da45aaabe933be154a4b5")}
		hash := h.Hash()
		if hex.EncodeToString(hash[:]) != "6ea46ec595bac2dc17d1a567a4bff602c059522020a3b5afafa4e8607ea4db99" {
			t.Fatal("header hash differs from recorded consensus Go vector")
		}
		commit := CometCommit{Height: 1234567890123, Round: 7, BlockID: CometBlockID{Hash: bytes.Repeat([]byte{0x11}, 32), PartSetTotal: 2, PartSetHash: bytes.Repeat([]byte{0x22}, 32)}}
		vote := CometCommitSignature{BlockIDFlag: CometBlock, Timestamp: time.Unix(1700000123, 987654321).UTC()}
		signature := ed25519.Sign(f.keys[0], committedVoteBytes("paxeer-commit-reference", commit, vote))
		if hex.EncodeToString(signature) != "5b0029c887146fb4be46a5fabcacaf811b9159c73d50a2f326e85edf91c57c4839af327a00ef0afb74114f08cd1645efaff7216d2c244dfa026b9eab90540d0e" {
			t.Fatal("precommit sign bytes differ from consensus reference")
		}
		if got := hex.EncodeToString((ExecTxResult{Code: 45, Data: []byte{1, 2}, GasWanted: 300, GasUsed: 210}).bytes()); got != "082d1202010228ac0230d201" {
			t.Fatalf("incorrect deterministic result encoding: %s", got)
		}
	})
}

func bytesOfCommittedHex(t *testing.T, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCommittedRungRefusesInsufficientPower(t *testing.T) {
	for _, tc := range []struct {
		name     string
		powers   [4]int64
		signers  int
		accepted bool
	}{
		{"half", [4]int64{1, 1, 1, 1}, 2, false},
		{"exactly two thirds", [4]int64{1, 1, 2, 2}, 3, false},
		{"three signatures below threshold", [4]int64{1, 1, 1, 9}, 3, false},
		{"weighted quorum", [4]int64{4, 3, 2, 1}, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommittedFixture(t, "V1-pax-transfer", tc.powers, 0)
			f.commit = committedFixtureCommit(f.header, f.keys, tc.signers)
			if tc.accepted {
				if _, err := f.verify(); err != nil {
					t.Fatal(err)
				}
			} else {
				requireCommittedRefusal(t, f, "committed-commit")
			}
		})
	}
	for _, mutation := range []string{"signature", "duplicate vote", "validator power", "wrong chain", "unsigned successor", "nil vote", "absent vote", "zero power", "duplicate key"} {
		t.Run(mutation, func(t *testing.T) {
			f := newCommittedFixture(t, "V1-pax-transfer", [4]int64{1, 1, 1, 1}, 0)
			check := ReceiptCheck("committed-commit")
			switch mutation {
			case "signature":
				f.commit.Signatures[0].Signature[0] ^= 1
			case "duplicate vote":
				f.commit.Signatures[1] = f.commit.Signatures[0]
			case "validator power":
				f.validators[3].VotingPower++
			case "wrong chain":
				f.header.ChainID = "another-chain"
				f.inclusion.NextHeader.ChainID = f.header.ChainID
			case "unsigned successor":
				f.inclusion.NextCommit = committedFixtureCommit(f.inclusion.NextHeader, f.keys, 2)
				check = "committed-next-commit"
			case "nil vote":
				f.commit.Signatures[2].BlockIDFlag = CometNil
				f.commit.Signatures[2].Signature = ed25519.Sign(f.keys[2], committedVoteBytes(f.header.ChainID, f.commit, f.commit.Signatures[2]))
			case "absent vote":
				f.commit.Signatures[3].Signature = make([]byte, 64)
			case "zero power":
				f.validators[0].VotingPower = 0
				f.reanchor(t)
			case "duplicate key":
				f.validators[1].PublicKey = f.validators[0].PublicKey
				f.reanchor(t)
			}
			requireCommittedRefusal(t, f, check)
		})
	}
}

func TestCommittedRungRefusesTamperedInclusion(t *testing.T) {
	for _, mutation := range []string{"result sibling", "transaction sibling", "batch sibling", "result data", "result gas", "transaction", "batch", "index", "total", "extra aunt", "missing aunt", "results header", "landing before receipt", "wrong successor"} {
		t.Run(mutation, func(t *testing.T) {
			f := newCommittedFixture(t, "V1-pax-transfer", [4]int64{1, 1, 1, 1}, 0)
			check := ReceiptCheck("committed-inclusion")
			switch mutation {
			case "result sibling":
				f.inclusion.ResultProof.Aunts[0][0] ^= 1
			case "transaction sibling":
				f.inclusion.TransactionProof.Aunts[0][0] ^= 1
			case "batch sibling":
				f.inclusion.BatchProof.Aunts[0][0] ^= 1
			case "result data":
				f.inclusion.Result.Data[0] ^= 1
			case "result gas":
				f.inclusion.Result.GasUsed++
			case "transaction":
				f.inclusion.Transaction[0] ^= 1
			case "batch":
				f.inclusion.BatchTransaction[0] ^= 1
			case "index":
				f.inclusion.ResultProof.Index = 0
			case "total":
				f.inclusion.ResultProof.Total++
			case "extra aunt":
				f.inclusion.ResultProof.Aunts = append(f.inclusion.ResultProof.Aunts, [32]byte{})
			case "missing aunt":
				f.inclusion.ResultProof.Aunts = f.inclusion.ResultProof.Aunts[:1]
			case "results header":
				f.inclusion.NextHeader.LastResultsHash = bytes.Repeat([]byte{0xff}, 32)
				check = "committed-next-commit"
			case "landing before receipt":
				f = newCommittedFixture(t, "V1-pax-transfer", [4]int64{1, 1, 1, 1}, -1)
				check = "committed-height"
			case "wrong successor":
				f.inclusion.NextHeader.Height++
				check = "committed-height"
			}
			requireCommittedRefusal(t, f, check)
		})
	}
	for _, mutation := range []string{"unsupported result code", "wrong result status", "wrong gas", "different transaction", "prehashed transaction", "prehashed batch", "malformed batch", "batch height", "sequence outside batch", "unverified receipt"} {
		t.Run(mutation, func(t *testing.T) {
			f := newCommittedFixture(t, "V1-pax-transfer", [4]int64{1, 1, 1, 1}, 0)
			check := ReceiptCheck("committed-result")
			switch mutation {
			case "unsupported result code":
				f.inclusion.Result.Code = 1
			case "wrong result status":
				f.inclusion.Result.Code = 45
			case "wrong gas":
				f.inclusion.Result.GasUsed++
			case "different transaction":
				f.inclusion.Transaction[0] ^= 1
				check = "committed-inclusion"
			case "prehashed transaction":
				hash := sha256.Sum256(f.inclusion.Transaction)
				f.inclusion.Transaction = hash[:]
				check = "committed-inclusion"
			case "prehashed batch":
				hash := sha256.Sum256(f.inclusion.BatchTransaction)
				f.inclusion.BatchTransaction = hash[:]
				check = "committed-batch"
			case "malformed batch":
				f.inclusion.BatchTransaction[0] ^= 1
				check = "committed-batch"
			case "batch height":
				binary.BigEndian.PutUint64(f.inclusion.BatchTransaction[len("PAXEER/v1/sequence-batch\x00"):], uint64(f.header.Height+1))
				check = "committed-batch"
			case "sequence outside batch":
				binary.BigEndian.PutUint64(f.inclusion.BatchTransaction[len("PAXEER/v1/sequence-batch\x00")+8:], f.receipt.Receipt.GlobalSequence+1)
				check = "committed-batch"
			case "unverified receipt":
				f.receipt = SequencerReceiptVerification{}
				check = "committed-receipt"
			}
			f.reanchor(t)
			requireCommittedRefusal(t, f, check)
		})
	}
	// Check the short branch of a non-power-of-two tree, not just receipt index 1.
	leaves := [][]byte{[]byte("one"), []byte("two"), []byte("three")}
	root := committedRoot(leaves)
	proof := committedFixtureProof(leaves, 2)
	if !committedVerifyProof(leaves[2], proof, root[:]) {
		t.Fatal("unbalanced proof was refused")
	}
	proof.Aunts[0][31] ^= 1
	if committedVerifyProof(leaves[2], proof, root[:]) {
		t.Fatal("tampered unbalanced proof was accepted")
	}
}
