package types_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmstate "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/state"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepairM_1118_InvalidHeaderVector(t *testing.T) {
	header := repairM1118Header(t)
	require.NoError(t, header.ValidateBasic())
	require.Len(t, header.ProposerAddress, crypto.AddressSize)
	for _, digest := range [][]byte{header.LastCommitHash, header.DataHash, header.ValidatorsHash,
		header.NextValidatorsHash, header.ConsensusHash, header.LastResultsHash, header.EvidenceHash} {
		require.Len(t, digest, sha256.Size)
	}
	wire, err := header.ToProto().Marshal()
	require.NoError(t, err)
	var transported tmproto.Header
	require.NoError(t, transported.Unmarshal(wire))
	decoded, err := types.HeaderFromProto(&transported)
	require.NoError(t, err)
	require.Equal(t, header, decoded)
	require.Equal(t, "d408adccc0ffde478eddda84c349482c61962cae22a930cee1e074631e9ebba3", hex.EncodeToString(decoded.Hash()))

	// The historical ASCII digest/address shapes remain refused by the consumer.
	for _, mutate := range []func(*tmproto.Header){
		func(pb *tmproto.Header) { pb.DataHash = []byte(hex.EncodeToString(header.DataHash)) },
		func(pb *tmproto.Header) { pb.ProposerAddress = []byte(hex.EncodeToString(header.ProposerAddress)) },
	} {
		var malformed tmproto.Header
		require.NoError(t, malformed.Unmarshal(wire))
		mutate(&malformed)
		_, err := types.HeaderFromProto(&malformed)
		require.Error(t, err)
	}
}

func TestRepairM_1118_LegacyEventTestGap(t *testing.T) {
	// These existing proofs execute the real application and the RPC Subscribe
	// handler over loopback WebSockets, including Message.LegacyData and ResultEvent.
	t.Run("application_failure", TestRepair_93_FailedTransactionSubscription)
	for _, code := range []uint32{0, 37} {
		t.Run(strconv.FormatUint(uint64(code), 10), func(t *testing.T) {
			producer := abci.ResponseFinalizeBlock{TxResults: []*abci.ExecTxResult{{
				Code: code, Data: []byte{0, 255, 128}, GasWanted: 1000, GasUsed: 731,
				Events: []abci.Event{{Type: "execution", Attributes: []abci.EventAttribute{
					{Key: []byte("outcome"), Value: []byte("committed"), Index: true},
				}}},
			}}}
			wire, err := producer.Marshal()
			require.NoError(t, err)
			var transported abci.ResponseFinalizeBlock
			require.NoError(t, transported.Unmarshal(wire))
			event := types.EventDataTx{TxResultV2: abci.TxResultV2{
				Height: 42, Index: 2, Tx: []byte("repair-m1118/subscription"), Result: *transported.TxResults[0],
			}}
			decoded, _ := repair93SubscriptionRoundTrip(t, event)
			repair93AssertResult(t, event, decoded)
			require.Equal(t, code, decoded.TxResult.Result.Code)
		})
	}
}

func TestRepairM_1118_AttackEvidenceDigest(t *testing.T) {
	evidence := repairM1118Evidence(t)
	// This exact preimage and its SHA-256 were independently computed from the
	// committed header vector, followed by the signed varint for common height 41.
	const preimageHex = "d408adccc0ffde478eddda84c349482c61962cae22a930cee1e074631e9ebba352"
	preimage := append(append([]byte(nil), evidence.ConflictingBlock.Hash().Bytes()...), binary.AppendVarint(nil, evidence.CommonHeight)...)
	require.Equal(t, preimageHex, hex.EncodeToString(preimage))
	require.NotZero(t, preimage[sha256.Size-1])
	sum := sha256.Sum256(preimage)
	require.Equal(t, "14f22cca4417a45610f002f790b0304ec2a64df12025d87c1e332da21c3ad90a", hex.EncodeToString(sum[:]))
	require.Equal(t, sum[:], evidence.Hash())

	other := repairM1118Evidence(t)
	other.ConflictingBlock.Commit.Signatures[3] = types.NewCommitSigAbsent()
	require.NoError(t, other.ValidateBasic())
	require.NoError(t, other.ConflictingBlock.ValidatorSet.VerifyCommitLight(other.ConflictingBlock.ChainID,
		other.ConflictingBlock.Commit.BlockID, other.ConflictingBlock.Height, other.ConflictingBlock.Commit))
	require.Equal(t, evidence.Hash(), other.Hash(), "signature subsets must retain one evidence identity")
	other.CommonHeight++
	require.NoError(t, other.ValidateBasic())
	require.NotEqual(t, evidence.Hash(), other.Hash())
}

func TestRepairM_1118_InvalidAttackVector(t *testing.T) {
	evidence := repairM1118Evidence(t)
	pb, err := types.EvidenceToProto(evidence)
	require.NoError(t, err)
	wire, err := pb.Marshal()
	require.NoError(t, err)
	var transported tmproto.Evidence
	require.NoError(t, transported.Unmarshal(wire))
	decoded, err := types.EvidenceFromProto(&transported)
	require.NoError(t, err)
	attack, ok := decoded.(*types.LightClientAttackEvidence)
	require.True(t, ok)
	require.NoError(t, attack.ConflictingBlock.ValidateBasic(evidence.ConflictingBlock.ChainID))
	require.NoError(t, attack.ConflictingBlock.ValidatorSet.VerifyCommitLight(attack.ConflictingBlock.ChainID,
		attack.ConflictingBlock.Commit.BlockID, attack.ConflictingBlock.Height, attack.ConflictingBlock.Commit))
	require.Equal(t, evidence.Bytes(), attack.Bytes())
	require.Equal(t, evidence.Hash(), attack.Hash())
	vectorBytes, err := os.ReadFile("testdata/attack_evidence_v2.json")
	require.NoError(t, err)
	var vector struct {
		Version      int    `json:"version"`
		CommonHeight int64  `json:"common_height"`
		HeaderHash   string `json:"header_hash"`
		Preimage     string `json:"preimage"`
		EvidenceHash string `json:"evidence_hash"`
	}
	require.NoError(t, json.Unmarshal(vectorBytes, &vector))
	require.Equal(t, 2, vector.Version)
	require.Equal(t, attack.CommonHeight, vector.CommonHeight)
	require.Equal(t, hex.EncodeToString(attack.ConflictingBlock.Hash().Bytes()), vector.HeaderHash)
	require.Equal(t, vector.HeaderHash+"52", vector.Preimage)
	require.Equal(t, hex.EncodeToString(attack.Hash()), vector.EvidenceHash)

	for _, mutate := range []func(*tmproto.LightClientAttackEvidence){
		func(e *tmproto.LightClientAttackEvidence) {
			e.ConflictingBlock.SignedHeader.Header.LastCommitHash = make([]byte, 64)
		},
		func(e *tmproto.LightClientAttackEvidence) {
			e.ConflictingBlock.SignedHeader.Header.ProposerAddress = make([]byte, 40)
		},
		func(e *tmproto.LightClientAttackEvidence) { e.ConflictingBlock.SignedHeader.Commit.Height++ },
	} {
		require.NoError(t, transported.Unmarshal(wire))
		mutate(transported.GetLightClientAttackEvidence())
		_, err := types.EvidenceFromProto(&transported)
		require.Error(t, err)
	}
}

func TestRepairM_1118_TxHashLength(t *testing.T) {
	block := repairM1118Evidence(t).ConflictingBlock
	tx := types.Tx("repair-m1118/transaction")
	key := tx.Hash()
	proposal := types.NewProposal(block.Height, 0, -1, block.Commit.BlockID, block.Time,
		[]types.TxHash{key}, *block.Header, block.Commit, nil, block.ProposerAddress)
	signer := ed25519.TestSecretKey([]byte("repair-m1118/proposer"))
	proposal.Signature = signer.Sign(types.ProposalSignBytes(block.ChainID, proposal.ToProto()))
	require.NoError(t, proposal.ValidateBasic())
	pb := proposal.ToProto()
	wire, err := pb.Marshal()
	require.NoError(t, err)
	var transported tmproto.Proposal
	require.NoError(t, transported.Unmarshal(wire))
	decoded, err := types.ProposalFromProto(&transported)
	require.NoError(t, err)
	require.Equal(t, proposal.TxHashes, decoded.TxHashes)
	require.Equal(t, 0, types.Txs{tx}.IndexByHash(decoded.TxHashes[0]))
	require.NoError(t, signer.Public().Verify(types.ProposalSignBytes(block.ChainID, decoded.ToProto()), decoded.Signature))

	for _, size := range []int{0, 31, 33} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			var malformed tmproto.Proposal
			require.NoError(t, malformed.Unmarshal(wire))
			malformed.TxKeys[0].TxKey = make([]byte, size)
			copy(malformed.TxKeys[0].TxKey, key[:])
			encoded, err := malformed.Marshal()
			require.NoError(t, err)
			var remote tmproto.Proposal
			require.NoError(t, remote.Unmarshal(encoded))
			_, err = types.ProposalFromProto(&remote)
			require.Error(t, err)
			_, err = types.TxHashFromProto(remote.TxKeys[0])
			require.Error(t, err)
		})
	}
	_, err = types.TxHashFromProto(nil)
	require.Error(t, err)
}

func TestRepairM_1118_NodeKeyAtomicSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node_key.json")
	key, err := types.LoadOrGenNodeKey(path)
	require.NoError(t, err)
	originalID := key.ID()
	previousBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	previousInfo, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), previousInfo.Mode().Perm())

	// An interruption while writing the staging file must leave the published
	// identity readable. The incomplete file is a crash-state fixture.
	staging, err := os.CreateTemp(dir, ".node-key-*")
	require.NoError(t, err)
	_, err = staging.Write([]byte(`{"priv_key":`))
	require.NoError(t, err)
	require.NoError(t, staging.Sync())
	require.NoError(t, staging.Close())
	key = types.NodeKey{}
	reopened, err := types.LoadOrGenNodeKey(path)
	require.NoError(t, err)
	require.Equal(t, types.NodeIDFromPubKey(reopened.PubKey()), reopened.ID())
	actualBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, previousBytes, actualBytes)
	var original types.NodeKey
	require.NoError(t, json.Unmarshal(previousBytes, &original))
	require.Equal(t, originalID, original.ID())
	require.Equal(t, original.ID(), reopened.ID())

	// A second link pins the old complete inode; atomic publication must not
	// overwrite it. This also catches direct truncation of an existing key.
	snapshot := filepath.Join(dir, "previous_key.json")
	require.NoError(t, os.Link(path, snapshot))
	require.NoError(t, os.Chmod(path, 0644))
	replacement := types.GenNodeKey()
	require.NoError(t, replacement.SaveAs(path))
	current, err := types.LoadOrGenNodeKey(path)
	require.NoError(t, err)
	require.Equal(t, replacement.ID(), current.ID())
	previous, err := types.LoadNodeKey(snapshot)
	require.NoError(t, err)
	require.Equal(t, original.ID(), previous.ID())
	currentInfo, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, os.SameFile(previousInfo, currentInfo))
	require.Equal(t, os.FileMode(0600), currentInfo.Mode().Perm())

	// A real rename refusal must return an error and clean its staging file.
	blocked := filepath.Join(t.TempDir(), "node_key.json")
	require.NoError(t, os.Mkdir(blocked, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(blocked, "keep"), previousBytes, 0600))
	require.Error(t, current.SaveAs(blocked))
	entries, err := os.ReadDir(filepath.Dir(blocked))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	stillPrevious, err := types.LoadNodeKey(filepath.Join(blocked, "keep"))
	require.NoError(t, err)
	require.Equal(t, original.ID(), stillPrevious.ID())
}

func TestRepairM_1118_NodePublicKeyRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node_key.json")
	pubPath := filepath.Join(dir, "node_pubkey.txt")
	// The first setup durably publishes the private key, but a directory at the
	// companion path causes the real rename to fail even when tests run as root.
	require.NoError(t, os.Mkdir(pubPath, 0700))
	_, err := types.LoadOrGenNodeKey(path)
	require.Error(t, err)
	persisted, err := types.LoadNodeKey(path)
	require.NoError(t, err)
	wantID := persisted.ID()
	wantPublic := "node:" + persisted.PubKey().String()
	privateBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	persisted = types.NodeKey{}
	require.NoError(t, os.Remove(pubPath))
	for _, condition := range []string{"missing", "stale", "symlink"} {
		t.Run(condition, func(t *testing.T) {
			if condition == "stale" {
				require.NoError(t, os.WriteFile(pubPath, []byte("wrong peer"), 0644))
			}
			var target string
			if condition == "symlink" {
				target = filepath.Join(dir, "unrelated")
				require.NoError(t, os.WriteFile(target, []byte("untouched"), 0600))
				require.NoError(t, os.Remove(pubPath))
				require.NoError(t, os.Symlink(target, pubPath))
			}
			recovered, err := types.LoadOrGenNodeKey(path)
			require.NoError(t, err)
			require.Equal(t, wantID, recovered.ID())
			pub, err := os.ReadFile(pubPath)
			require.NoError(t, err)
			require.Equal(t, wantPublic, string(pub))
			info, err := os.Lstat(pubPath)
			require.NoError(t, err)
			require.True(t, info.Mode().IsRegular())
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			gotPrivate, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, privateBytes, gotPrivate)
			if target != "" {
				got, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, "untouched", string(got))
			}
		})
	}
	// A corrupt established identity is refused, never silently regenerated.
	require.NoError(t, os.WriteFile(path, []byte("{"), 0600))
	_, err = types.LoadOrGenNodeKey(path)
	require.Error(t, err)
	corrupt, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "{", string(corrupt))
}

func TestRepairM_1118_VoteExtensionEnable(t *testing.T) {
	for _, tc := range []struct {
		name              string
		old, next, height int64
		valid             bool
	}{
		{"zero_to_future", 0, 11, 10, true},
		{"zero_to_current", 0, 10, 10, false},
		{"zero_to_past", 0, 9, 10, false},
		{"zero_to_negative", 0, -1, 10, false},
		{"active_to_changed", 5, 11, 10, false},
		{"activation_boundary", 10, 11, 10, false},
		{"active_to_disabled", 5, 0, 10, false},
		{"active_unchanged", 5, 5, 10, true},
		{"disabled_unchanged", 0, 0, 10, true},
		{"scheduled_changed", 20, 30, 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultConsensusParams()
			params.ABCI.VoteExtensionsEnableHeight = tc.old
			block := repairM1118Evidence(t).ConflictingBlock
			genesis, err := state.MakeGenesisState(&types.GenesisDoc{
				GenesisTime: block.Time, ChainID: block.ChainID, InitialHeight: 1,
				ConsensusParams: params,
				Validators:      []types.GenesisValidator{{PubKey: block.ValidatorSet.Validators[0].PubKey, Power: 10}},
			})
			require.NoError(t, err)
			update := &tmproto.ConsensusParams{Abci: &tmproto.ABCIParams{VoteExtensionsEnableHeight: tc.next}}
			wire, err := update.Marshal()
			require.NoError(t, err)
			var transported tmproto.ConsensusParams
			require.NoError(t, transported.Unmarshal(wire))
			validationErr := params.ValidateUpdate(&transported, tc.height)
			header := *block.Header
			header.Height = tc.height
			updated, err := genesis.Update(block.Commit.BlockID, &header, nil, &transported, nil)
			if !tc.valid {
				require.Error(t, validationErr)
				require.Error(t, err)
				require.Equal(t, tc.old, updated.ConsensusParams.ABCI.VoteExtensionsEnableHeight)
			} else {
				require.NoError(t, validationErr)
				require.NoError(t, err)
				require.Equal(t, tc.next, updated.ConsensusParams.ABCI.VoteExtensionsEnableHeight)
				require.Equal(t, tc.height+1, updated.LastHeightConsensusParamsChanged)
				encoded, err := updated.Bytes()
				require.NoError(t, err)
				var received tmstate.State
				require.NoError(t, received.Unmarshal(encoded))
				recovered, err := state.FromProto(&received)
				require.NoError(t, err)
				require.Equal(t, tc.next, recovered.ConsensusParams.ABCI.VoteExtensionsEnableHeight)
				require.Equal(t, tc.height+1, recovered.LastHeightConsensusParamsChanged)
			}
			require.Equal(t, tc.old, genesis.ConsensusParams.ABCI.VoteExtensionsEnableHeight)
		})
	}
}

func repairM1118Header(t *testing.T) types.Header {
	t.Helper()
	data, err := os.ReadFile("testdata/light_commit_v1.json")
	require.NoError(t, err)
	var vector struct {
		HeaderProtobuf string `json:"header_protobuf"`
	}
	require.NoError(t, json.Unmarshal(data, &vector))
	wire, err := hex.DecodeString(vector.HeaderProtobuf)
	require.NoError(t, err)
	var pb tmproto.Header
	require.NoError(t, pb.Unmarshal(wire))
	header, err := types.HeaderFromProto(&pb)
	require.NoError(t, err)
	reencoded, err := header.ToProto().Marshal()
	require.NoError(t, err)
	require.Equal(t, wire, reencoded)
	return header
}

func repairM1118Evidence(t *testing.T) *types.LightClientAttackEvidence {
	t.Helper()
	header := repairM1118Header(t)
	keys := make(map[string]crypto.PrivKey)
	validators := make([]*types.Validator, 4)
	for i := range validators {
		key := ed25519.TestSecretKey([]byte{byte(i + 1)})
		keys[string(key.Public().Address())] = key
		validators[i] = types.NewValidator(key.Public(), 10)
	}
	vals := types.NewValidatorSet(validators)
	require.Equal(t, []byte(header.ValidatorsHash), vals.Hash())
	commit := &types.Commit{Height: header.Height, Round: 2,
		BlockID: types.BlockID{Hash: header.Hash(), PartSetHeader: types.PartSetHeader{
			Total: 3, Hash: crypto.Checksum([]byte("light-vector-v1/block-parts")).Bytes(),
		}},
	}
	for i, val := range vals.Validators {
		vote := &types.Vote{Type: tmproto.PrecommitType, Height: commit.Height, Round: commit.Round,
			BlockID: commit.BlockID, Timestamp: header.Time.Add(time.Duration(i+1) * time.Millisecond),
			ValidatorAddress: val.Address, ValidatorIndex: int32(i)}
		vote.Signature = utils.Some(keys[string(val.Address)].Sign(types.VoteSignBytes(header.ChainID, vote.ToProto())))
		require.NoError(t, vote.Verify(header.ChainID, val.PubKey))
		commit.Signatures = append(commit.Signatures, vote.CommitSig())
	}
	block := &types.LightBlock{SignedHeader: &types.SignedHeader{Header: &header, Commit: commit}, ValidatorSet: vals}
	require.NoError(t, block.ValidateBasic(header.ChainID))
	require.NoError(t, vals.VerifyCommitLight(header.ChainID, commit.BlockID, header.Height, commit))
	evidence := &types.LightClientAttackEvidence{ConflictingBlock: block, CommonHeight: 41,
		ByzantineValidators: vals.Validators[:2], TotalVotingPower: vals.TotalVotingPower(), Timestamp: header.Time}
	require.NoError(t, evidence.ValidateBasic())
	return evidence
}
