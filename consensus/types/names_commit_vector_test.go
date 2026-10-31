package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/libs/protoio"
	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
)

type namesValidatorVectorV1 struct {
	PubKey      crypto.PubKey `json:"pub_key"`
	VotingPower int64         `json:"voting_power"`
}

type namesValidatorsVectorV1 struct {
	Validators []namesValidatorVectorV1 `json:"validators"`
	TotalPower int64                    `json:"total_power"`
}

type namesBlockIDVectorV1 struct {
	Hash          tmbytes.HexBytes `json:"hash"`
	PartSetHeader PartSetHeader    `json:"part_set_header"`
}

type namesCommitVectorV1 struct {
	ChainID    string               `json:"chain_id"`
	Height     int64                `json:"height"`
	Round      int32                `json:"round"`
	Header     Header               `json:"header"`
	HeaderHash tmbytes.HexBytes     `json:"header_hash"`
	BlockID    namesBlockIDVectorV1 `json:"block_id"`
	Signatures []CommitSig          `json:"signatures"`
	SignBytes  []string             `json:"sign_bytes"`
}

func TestNamesCommitVectorV1(t *testing.T) {
	const chainID = "paxeer-names-vector"
	const height int64 = 7
	const round int32 = 0
	now := time.Date(2025, time.January, 2, 3, 4, 5, 123456789, time.UTC)
	appHash := sha256.Sum256([]byte("names-vector-app-hash"))
	vectorDir := filepath.Join("..", "..", "tests", "vectors", "names")
	validatorsPath := filepath.Join(vectorDir, "validators_v1.json")
	commitPath := filepath.Join(vectorDir, "commit_v1.json")

	if os.Getenv("NAMES_WRITE_VECTOR") == "1" {
		vals, privVals := RandValidatorSet(4, 10)
		block := MakeBlock(height, nil, &Commit{}, nil)
		block.ChainID = chainID
		block.Time = now
		block.AppHash = appHash[:]
		block.ValidatorsHash = vals.Hash()
		block.NextValidatorsHash = vals.Hash()
		block.ProposerAddress = vals.GetProposer().Address
		require.NoError(t, block.Header.ValidateBasic())
		parts, err := block.MakePartSet(BlockPartSizeBytes)
		require.NoError(t, err)
		blockID := BlockID{Hash: block.Header.Hash(), PartSetHeader: parts.Header()}
		voteSet := NewVoteSet(chainID, height, round, tmproto.PrecommitType, vals)
		commit, err := MakeCommit(t.Context(), blockID, height, round, voteSet, privVals, now)
		require.NoError(t, err)
		require.NoError(t, vals.VerifyCommit(chainID, blockID, height, commit))

		validatorsVector := namesValidatorsVectorV1{TotalPower: vals.TotalVotingPower()}
		for _, val := range vals.Validators {
			validatorsVector.Validators = append(validatorsVector.Validators, namesValidatorVectorV1{
				PubKey: val.PubKey, VotingPower: val.VotingPower,
			})
		}
		commitVector := namesCommitVectorV1{
			ChainID: chainID, Height: height, Round: round,
			Header: block.Header, HeaderHash: block.Header.Hash(),
			BlockID:    namesBlockIDVectorV1{Hash: blockID.Hash, PartSetHeader: blockID.PartSetHeader},
			Signatures: commit.Signatures,
		}
		for i := range commit.Signatures {
			vote, ok := commit.GetVote(int32(i))
			require.True(t, ok)
			commitVector.SignBytes = append(commitVector.SignBytes, hex.EncodeToString(VoteSignBytes(chainID, vote.ToProto())))
		}
		require.NoError(t, os.MkdirAll(vectorDir, 0755))
		for _, file := range []struct {
			path  string
			value any
		}{{validatorsPath, validatorsVector}, {commitPath, commitVector}} {
			data, err := json.MarshalIndent(file.value, "", "  ")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(file.path, append(data, '\n'), 0644))
		}
	}

	var validatorsVector namesValidatorsVectorV1
	var vector namesCommitVectorV1
	for _, file := range []struct {
		path  string
		value any
	}{{validatorsPath, &validatorsVector}, {commitPath, &vector}} {
		data, err := os.ReadFile(file.path)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, file.value))
	}

	require.Len(t, validatorsVector.Validators, 4)
	validators := make([]*Validator, len(validatorsVector.Validators))
	for i, val := range validatorsVector.Validators {
		require.EqualValues(t, 10, val.VotingPower)
		validators[i] = NewValidator(val.PubKey, val.VotingPower)
	}
	vals := NewValidatorSet(validators)
	require.NoError(t, vals.ValidateBasic())
	require.EqualValues(t, 40, validatorsVector.TotalPower)
	require.Equal(t, validatorsVector.TotalPower, vals.TotalVotingPower())
	require.Equal(t, chainID, vector.ChainID)
	require.Equal(t, height, vector.Height)
	require.Equal(t, round, vector.Round)
	require.Equal(t, vector.ChainID, vector.Header.ChainID)
	require.Equal(t, vector.Height, vector.Header.Height)
	require.True(t, now.Equal(vector.Header.Time))
	require.Equal(t, appHash[:], []byte(vector.Header.AppHash))
	require.Equal(t, vals.Hash(), []byte(vector.Header.ValidatorsHash))
	require.Equal(t, vals.Hash(), []byte(vector.Header.NextValidatorsHash))
	require.NoError(t, vector.Header.ValidateBasic())
	headerHash := vector.Header.Hash()
	require.Len(t, headerHash, sha256.Size)
	require.Equal(t, vector.HeaderHash, headerHash)
	require.Equal(t, headerHash, vector.BlockID.Hash)

	block := MakeBlock(vector.Height, nil, &Commit{}, nil)
	block.Header = vector.Header
	parts, err := block.MakePartSet(BlockPartSizeBytes)
	require.NoError(t, err)
	require.Equal(t, parts.Header(), vector.BlockID.PartSetHeader)
	blockID := BlockID{Hash: vector.BlockID.Hash, PartSetHeader: vector.BlockID.PartSetHeader}
	require.NoError(t, blockID.ValidateBasic())
	commit := &Commit{
		Height: vector.Height, Round: vector.Round, BlockID: blockID,
		Signatures: vector.Signatures,
	}
	require.NoError(t, commit.ValidateBasic())
	require.Len(t, commit.Signatures, len(validators))
	require.Len(t, vector.SignBytes, len(commit.Signatures))
	for i, sig := range commit.Signatures {
		require.Equal(t, BlockIDFlagCommit, sig.BlockIDFlag)
		require.Equal(t, vals.Validators[i].Address, sig.ValidatorAddress)
		require.True(t, now.Equal(sig.Timestamp))
		require.True(t, sig.Signature.IsPresent())
		vote, ok := commit.GetVote(int32(i))
		require.True(t, ok)
		precommit := vote.ToProto()
		canonical := CanonicalizeVote(vector.ChainID, precommit)
		canonicalBytes, err := protoio.MarshalDelimited(&canonical)
		require.NoError(t, err)
		signBytes := VoteSignBytes(vector.ChainID, precommit)
		require.Equal(t, canonicalBytes, signBytes)
		require.Equal(t, vector.SignBytes[i], hex.EncodeToString(signBytes))
		require.NoError(t, vote.Verify(vector.ChainID, vals.Validators[i].PubKey))
	}
	require.NoError(t, vals.VerifyCommit(vector.ChainID, blockID, vector.Height, commit))

	changedHeader := vector.Header
	changedHeader.AppHash = bytes.Clone(vector.Header.AppHash)
	changedHeader.AppHash[0] ^= 1
	changedBlockID := BlockID{Hash: changedHeader.Hash(), PartSetHeader: blockID.PartSetHeader}
	require.NotEqual(t, headerHash, changedBlockID.Hash)
	require.Error(t, vals.VerifyCommit(vector.ChainID, changedBlockID, vector.Height, commit))
	changedCommit := &Commit{
		Height: commit.Height, Round: commit.Round, BlockID: changedBlockID,
		Signatures: commit.Signatures,
	}
	require.Error(t, vals.VerifyCommit(vector.ChainID, changedBlockID, vector.Height, changedCommit))
}
