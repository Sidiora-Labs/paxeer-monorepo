package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
)

type lightValidatorVectorV1 struct {
	Address     string `json:"address"`
	PublicKey   string `json:"public_key"`
	VotingPower int64  `json:"voting_power"`
	Protobuf    string `json:"protobuf"`
}

type lightSignatureVectorV1 struct {
	ValidatorIndex   int         `json:"validator_index"`
	BlockIDFlag      BlockIDFlag `json:"block_id_flag"`
	ValidatorAddress string      `json:"validator_address"`
	Timestamp        time.Time   `json:"timestamp"`
	VoteSignBytes    string      `json:"vote_sign_bytes"`
	Signature        string      `json:"signature"`
}

type lightCommitVectorV1 struct {
	Height     int64                    `json:"height"`
	Round      int32                    `json:"round"`
	BlockID    BlockID                  `json:"block_id"`
	Signatures []lightSignatureVectorV1 `json:"signatures"`
}

func TestLightCommitVectorV1(t *testing.T) {
	keys := make(map[string]crypto.PrivKey)
	validators := make([]*Validator, 4)
	for i := range validators {
		key := ed25519.TestSecretKey([]byte{byte(i + 1)})
		pub := key.Public()
		keys[hex.EncodeToString(pub.Address())] = key
		validators[i] = NewValidator(pub, 10)
	}
	vals := NewValidatorSet(validators)
	hash := func(label string) []byte {
		return crypto.Checksum([]byte("light-vector-v1/" + label)).Bytes()
	}
	header := Header{
		Version:            version.Consensus{Block: 11, App: 1},
		ChainID:            "layerx-light-v1",
		Height:             42,
		Time:               time.Unix(1700000000, 123456789).UTC(),
		LastBlockID:        BlockID{Hash: hash("previous-block"), PartSetHeader: PartSetHeader{Total: 2, Hash: hash("previous-parts")}},
		LastCommitHash:     hash("last-commit"),
		DataHash:           hash("data"),
		ValidatorsHash:     vals.Hash(),
		NextValidatorsHash: vals.Hash(),
		ConsensusHash:      hash("consensus"),
		AppHash:            hash("app"),
		LastResultsHash:    hash("last-results"),
		EvidenceHash:       hash("evidence"),
		ProposerAddress:    vals.GetProposer().Address,
	}
	commit := &Commit{
		Height:     header.Height,
		Round:      2,
		BlockID:    BlockID{Hash: header.Hash(), PartSetHeader: PartSetHeader{Total: 3, Hash: hash("block-parts")}},
		Signatures: make([]CommitSig, len(vals.Validators)),
	}
	valVectors := make([]lightValidatorVectorV1, len(vals.Validators))
	sigVectors := make([]lightSignatureVectorV1, len(vals.Validators))
	for i, val := range vals.Validators {
		vote := &Vote{
			Type:             tmproto.PrecommitType,
			Height:           commit.Height,
			Round:            commit.Round,
			BlockID:          commit.BlockID,
			Timestamp:        header.Time.Add(time.Duration(i+1) * time.Millisecond),
			ValidatorAddress: val.Address,
			ValidatorIndex:   int32(i),
		}
		signBytes := VoteSignBytes(header.ChainID, vote.ToProto())
		sig := keys[hex.EncodeToString(val.Address)].Sign(signBytes)
		vote.Signature = utils.Some(sig)
		if err := vote.Verify(header.ChainID, val.PubKey); err != nil {
			t.Fatalf("vote %d: %v", i, err)
		}
		commit.Signatures[i] = vote.CommitSig()
		commitSignBytes, ok := commit.VoteSignBytes(header.ChainID, int32(i))
		if !ok || !bytes.Equal(signBytes, commitSignBytes) {
			t.Fatalf("vote %d sign bytes differ from commit", i)
		}
		valVectors[i] = lightValidatorVectorV1{
			Address:     hex.EncodeToString(val.Address),
			PublicKey:   hex.EncodeToString(val.PubKey.Bytes()),
			VotingPower: val.VotingPower,
			Protobuf:    hex.EncodeToString(val.Bytes()),
		}
		sigVectors[i] = lightSignatureVectorV1{
			ValidatorIndex:   i,
			BlockIDFlag:      BlockIDFlagCommit,
			ValidatorAddress: hex.EncodeToString(val.Address),
			Timestamp:        vote.Timestamp,
			VoteSignBytes:    hex.EncodeToString(signBytes),
			Signature:        hex.EncodeToString(sig.Bytes()),
		}
	}
	lightBlock := LightBlock{SignedHeader: &SignedHeader{Header: &header, Commit: commit}, ValidatorSet: vals}
	if err := lightBlock.ValidateBasic(header.ChainID); err != nil {
		t.Fatal(err)
	}
	verifyErr := VerifyCommitLight(header.ChainID, vals, commit.BlockID, header.Height, commit)
	if verifyErr != nil {
		t.Fatalf("VerifyCommitLight: %v", verifyErr)
	}
	if err := VerifyCommitLight("wrong-chain", vals, commit.BlockID, header.Height, commit); err == nil {
		t.Fatal("commit verified for a different chain")
	}
	insufficient := &Commit{Height: commit.Height, Round: commit.Round, BlockID: commit.BlockID, Signatures: append([]CommitSig(nil), commit.Signatures...)}
	insufficient.Signatures[2] = NewCommitSigAbsent()
	insufficient.Signatures[3] = NewCommitSigAbsent()
	if err := VerifyCommitLight(header.ChainID, vals, commit.BlockID, header.Height, insufficient); err == nil {
		t.Fatal("commit verified with only half the voting power")
	}
	headerBytes, err := header.ToProto().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	vector := struct {
		Version           int                      `json:"version"`
		BytesEncoding     string                   `json:"bytes_encoding"`
		Header            Header                   `json:"header"`
		HeaderProtobuf    string                   `json:"header_protobuf"`
		HeaderHash        string                   `json:"header_hash"`
		Validators        []lightValidatorVectorV1 `json:"validators"`
		ValidatorSetHash  string                   `json:"validator_set_hash"`
		Commit            lightCommitVectorV1      `json:"commit"`
		VerifyCommitLight bool                     `json:"verify_commit_light"`
	}{1, "hex", header, hex.EncodeToString(headerBytes), hex.EncodeToString(header.Hash()), valVectors, hex.EncodeToString(vals.Hash()), lightCommitVectorV1{commit.Height, commit.Round, commit.BlockID, sigVectors}, verifyErr == nil}
	compareLightVectorV1(t, "light_commit_v1.json", vector)
}

type resultsProofVectorV1 struct {
	Total    int64    `json:"total"`
	Index    int64    `json:"index"`
	LeafHash string   `json:"leaf_hash"`
	Aunts    []string `json:"aunts"`
}

type resultVectorV1 struct {
	Code      uint32               `json:"code"`
	Data      string               `json:"data"`
	GasWanted int64                `json:"gas_wanted"`
	GasUsed   int64                `json:"gas_used"`
	Protobuf  string               `json:"protobuf"`
	Proof     resultsProofVectorV1 `json:"proof"`
}

func TestResultsHashVectorV1(t *testing.T) {
	responses := []*abci.ResponseDeliverTx{
		{Code: 0, Data: []byte{0, 1, 255}, GasWanted: 100, GasUsed: 73, Log: "volatile log", Info: "volatile info", Events: []abci.Event{{Type: "volatile"}}, Codespace: "volatile", EvmTxInfo: &abci.EvmTxInfo{}},
		{Code: 7, Data: []byte("denied"), GasWanted: 200, GasUsed: 150, Log: "another log"},
		{},
	}
	results := NewResults(responses)
	root := results.Hash()
	if len(results) != 3 || len(root) != crypto.HashSize {
		t.Fatal("expected three results and a SHA-256 root")
	}
	vectors := make([]resultVectorV1, len(results))
	for i, result := range results {
		if result.Code != responses[i].Code || !bytes.Equal(result.Data, responses[i].Data) || result.GasWanted != responses[i].GasWanted || result.GasUsed != responses[i].GasUsed {
			t.Fatalf("result %d lost deterministic fields", i)
		}
		if result.Log != "" || result.Info != "" || len(result.Events) != 0 || result.Codespace != "" || result.EvmTxInfo != nil {
			t.Fatalf("result %d retained non-deterministic fields", i)
		}
		encoded, err := result.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		proof := results.ProveResult(i)
		if proof.Total != 3 || proof.Index != int64(i) {
			t.Fatalf("result %d has incorrect proof position", i)
		}
		if err := proof.Verify(root, encoded); err != nil {
			t.Fatalf("result %d proof: %v", i, err)
		}
		if err := proof.Verify(root, append(append([]byte(nil), encoded...), 0)); err == nil {
			t.Fatalf("result %d proof accepted changed bytes", i)
		}
		aunts := make([]string, len(proof.Aunts))
		for j, aunt := range proof.Aunts {
			aunts[j] = hex.EncodeToString(aunt)
		}
		vectors[i] = resultVectorV1{result.Code, hex.EncodeToString(result.Data), result.GasWanted, result.GasUsed, hex.EncodeToString(encoded), resultsProofVectorV1{proof.Total, proof.Index, hex.EncodeToString(proof.LeafHash), aunts}}
	}
	vector := struct {
		Version         int              `json:"version"`
		BytesEncoding   string           `json:"bytes_encoding"`
		Results         []resultVectorV1 `json:"results"`
		ABCIResultsHash string           `json:"abci_results_hash"`
	}{1, "hex", vectors, hex.EncodeToString(root)}
	compareLightVectorV1(t, "results_hash_v1.json", vector)
}

func compareLightVectorV1(t *testing.T, name string, vector any) {
	t.Helper()
	computed, err := json.MarshalIndent(vector, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	computed = append(computed, '\n')
	path := filepath.Join("testdata", name)
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, computed, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(computed, committed) {
		t.Fatalf("%s differs from computed vector\ncomputed:\n%s\ncommitted:\n%s", path, computed, committed)
	}
}
