package evmrpc_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmcrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	ics23 "github.com/confio/ics23/go"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type namesStorageProofOp struct {
	Type string        `json:"type"`
	Key  hexutil.Bytes `json:"key"`
	Data hexutil.Bytes `json:"data"`
}

type namesStorageProofVector struct {
	Version  int                   `json:"version"`
	Height   int64                 `json:"height"`
	AppHash  hexutil.Bytes         `json:"app_hash"`
	Store    string                `json:"store"`
	Address  common.Address        `json:"address"`
	Slot     common.Hash           `json:"slot"`
	Key      hexutil.Bytes         `json:"key"`
	Value    hexutil.Bytes         `json:"value"`
	Node     common.Hash           `json:"node"`
	Name     string                `json:"name"`
	ProofOps []namesStorageProofOp `json:"proof_ops"`
}

func TestNamesContenthashProofVerifiesAgainstTheAppHash(t *testing.T) {
	vectorDir := filepath.Join("..", "tests", "vectors", "names")
	layoutBytes, err := os.ReadFile(filepath.Join(vectorDir, "resolver_layout_v1.json"))
	require.NoError(t, err)
	var layout struct {
		Version            int           `json:"version"`
		ContenthashTag     hexutil.Bytes `json:"contenthash_tag"`
		ManifestDigestSlot uint64        `json:"manifest_digest_slot"`
		Contracts          map[string]struct {
			CreationCode hexutil.Bytes `json:"creation_code"`
		} `json:"contracts"`
	}
	require.NoError(t, json.Unmarshal(layoutBytes, &layout))
	require.Equal(t, 1, layout.Version)
	require.Len(t, layout.ContenthashTag, 4)

	application := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	ctx := application.GetContextForDeliverTx(nil)
	deployer := common.BytesToAddress(crypto.Keccak256([]byte("names storage proof deployer"))[12:])
	deploy := func(name string, args []byte) common.Address {
		contract, ok := layout.Contracts[name]
		require.True(t, ok, "layout must contain %s", name)
		require.NotEmpty(t, contract.CreationCode)
		nonce := application.EvmKeeper.GetNonce(ctx, deployer)
		input := append(append([]byte(nil), contract.CreationCode...), args...)
		_, err := application.EvmKeeper.CallEVM(ctx, deployer, nil, nil, input)
		require.NoError(t, err, "deploy %s", name)
		address := crypto.CreateAddress(deployer, nonce)
		require.NotEmpty(t, application.EvmKeeper.GetCode(ctx, address), name)
		return address
	}
	registry := deploy("PaxRegistry", nil)
	addressType, err := abi.NewType("address", "", nil)
	require.NoError(t, err)
	constructorArgs, err := (abi.Arguments{{Type: addressType}, {Type: addressType}, {Type: addressType}}).
		Pack(registry, deployer, deployer)
	require.NoError(t, err)
	resolver := deploy("PaxPublicResolver", constructorArgs)
	require.NotEqual(t, registry, resolver)

	registryABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"setSubnodeOwner","inputs":[{"type":"bytes32","name":"node"},{"type":"bytes32","name":"label"},{"type":"address","name":"owner"}],"outputs":[{"type":"bytes32"}]}]`))
	require.NoError(t, err)
	var node common.Hash
	for _, label := range []string{"pax", "example"} {
		labelHash := crypto.Keccak256Hash([]byte(label))
		input, err := registryABI.Pack("setSubnodeOwner", node, labelHash, deployer)
		require.NoError(t, err)
		ret, err := application.EvmKeeper.CallEVM(ctx, deployer, &registry, nil, input)
		require.NoError(t, err, "create %s subnode", label)
		node = crypto.Keccak256Hash(node[:], labelHash[:])
		require.Equal(t, node.Bytes(), ret)
	}
	resolverABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"setContenthash","inputs":[{"type":"bytes32","name":"node"},{"type":"bytes","name":"hash"}],"outputs":[]}]`))
	require.NoError(t, err)
	digest := common.HexToHash("0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	contenthash := append(append([]byte(nil), layout.ContenthashTag...), digest[:]...)
	input, err := resolverABI.Pack("setContenthash", node, contenthash)
	require.NoError(t, err)
	_, err = application.EvmKeeper.CallEVM(ctx, deployer, &resolver, nil, input)
	require.NoError(t, err)

	for height := int64(1); height <= MockHeight8; height++ {
		_, err := application.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: application.ChainID, Height: height},
		})
		require.NoError(t, err)
		application.SetDeliverStateToCommit()
		_, err = application.Commit(context.Background())
		require.NoError(t, err)
	}
	commit := application.LastCommitID()
	require.Equal(t, int64(MockHeight8), commit.Version)
	require.Len(t, commit.Hash, 32)
	if receiptStore := application.EvmKeeper.ReceiptStore(); receiptStore != nil {
		require.NoError(t, receiptStore.SetLatestVersion(commit.Version))
		require.NoError(t, receiptStore.SetEarliestVersion(1))
	}

	bytes32Type, err := abi.NewType("bytes32", "", nil)
	require.NoError(t, err)
	uint256Type, err := abi.NewType("uint256", "", nil)
	require.NoError(t, err)
	slotInput, err := (abi.Arguments{{Type: bytes32Type}, {Type: uint256Type}}).
		Pack(node, new(big.Int).SetUint64(layout.ManifestDigestSlot))
	require.NoError(t, err)
	slot := crypto.Keccak256Hash(slotInput)
	key := append(evmtypes.StateKey(resolver), slot[:]...)
	keypath := merkle.KeyPath{}.
		AppendKey([]byte(evmtypes.StoreKey), merkle.KeyEncodingHex).
		AppendKey(key, merkle.KeyEncodingHex).String()
	ctxProvider := func(height int64) sdk.Context {
		if height <= 0 {
			height = commit.Version
		}
		return application.GetCheckCtx().WithBlockHeight(height)
	}
	client := &MockClient{latestOverride: commit.Version}
	watermarks := evmrpc.NewWatermarkManager(client, ctxProvider, nil, application.EvmKeeper.ReceiptStore())
	stateAPI := evmrpc.NewStateAPI(client, &application.EvmKeeper, ctxProvider, evmrpc.ConnectionTypeHTTP, watermarks, application.CommitMultiStore().(storetypes.Queryable))
	proof, err := stateAPI.GetProof(t.Context(), resolver, []string{slot.Hex()}, gethrpc.BlockNumberOrHashWithNumber(gethrpc.LatestBlockNumber))
	require.NoError(t, err)
	require.Equal(t, resolver, proof.Address)
	require.Len(t, proof.HexValues, 1)
	require.Equal(t, hex.EncodeToString(digest[:]), proof.HexValues[0])
	require.Len(t, proof.StorageProof, 1)
	require.NotNil(t, proof.StorageProof[0])

	rootQuery := application.CommitMultiStore().(storetypes.Queryable).Query(t.Context(), abci.RequestQuery{
		Path: "/" + evmtypes.StoreKey + "/key", Data: key, Height: commit.Version, Prove: true,
	})
	require.Zero(t, rootQuery.Code, rootQuery.Log)
	require.Equal(t, commit.Version, rootQuery.Height)
	require.Equal(t, key, rootQuery.Key)
	require.Equal(t, digest.Bytes(), rootQuery.Value)
	require.NotNil(t, rootQuery.ProofOps)
	require.Len(t, rootQuery.ProofOps.Ops, 2)
	require.Len(t, proof.StorageProof[0].Ops, 2)
	require.Equal(t, rootQuery.ProofOps, proof.StorageProof[0])
	require.Equal(t, storetypes.ProofOpIAVLCommitment, proof.StorageProof[0].Ops[0].Type)
	require.Equal(t, storetypes.ProofOpSimpleMerkleCommitment, proof.StorageProof[0].Ops[1].Type)
	runtime := rootmulti.DefaultProofRuntime()
	require.NoError(t, runtime.VerifyValue(rootQuery.ProofOps, commit.Hash, keypath, digest[:]))
	require.NoError(t, runtime.VerifyValue(proof.StorageProof[0], commit.Hash, keypath, digest[:]))
	flipped := append([]byte(nil), digest[:]...)
	flipped[0] ^= 1
	require.Error(t, runtime.VerifyValue(proof.StorageProof[0], commit.Hash, keypath, flipped))

	var absentNode common.Hash
	for _, label := range []string{"pax", "absent"} {
		labelHash := crypto.Keccak256Hash([]byte(label))
		absentNode = crypto.Keccak256Hash(absentNode[:], labelHash[:])
	}
	absentSlotInput, err := (abi.Arguments{{Type: bytes32Type}, {Type: uint256Type}}).
		Pack(absentNode, new(big.Int).SetUint64(layout.ManifestDigestSlot))
	require.NoError(t, err)
	absentSlot := crypto.Keccak256Hash(absentSlotInput)
	absentKey := append(evmtypes.StateKey(resolver), absentSlot[:]...)
	absentKeypath := merkle.KeyPath{}.
		AppendKey([]byte(evmtypes.StoreKey), merkle.KeyEncodingHex).
		AppendKey(absentKey, merkle.KeyEncodingHex).String()
	absence, err := stateAPI.GetProof(t.Context(), resolver, []string{absentSlot.Hex()}, gethrpc.BlockNumberOrHashWithNumber(gethrpc.BlockNumber(commit.Version)))
	require.NoError(t, err)
	require.Equal(t, resolver, absence.Address)
	require.Equal(t, []string{""}, absence.HexValues)
	require.Len(t, absence.StorageProof, 1)
	require.NotNil(t, absence.StorageProof[0])
	require.Len(t, absence.StorageProof[0].Ops, 2)
	require.Equal(t, storetypes.ProofOpIAVLCommitment, absence.StorageProof[0].Ops[0].Type)
	require.Equal(t, storetypes.ProofOpSimpleMerkleCommitment, absence.StorageProof[0].Ops[1].Type)
	var absenceCommitment ics23.CommitmentProof
	require.NoError(t, absenceCommitment.Unmarshal(absence.StorageProof[0].Ops[0].Data))
	require.NotNil(t, absenceCommitment.GetNonexist())
	require.Equal(t, absentKey, absenceCommitment.GetNonexist().Key)
	require.NoError(t, runtime.VerifyAbsence(absence.StorageProof[0], commit.Hash, absentKeypath))
	wrongHash := append([]byte(nil), commit.Hash...)
	wrongHash[0] ^= 1
	require.Error(t, runtime.VerifyAbsence(absence.StorageProof[0], wrongHash, absentKeypath))
	require.Error(t, runtime.VerifyAbsence(proof.StorageProof[0], commit.Hash, keypath))

	absencePath := filepath.Join(vectorDir, "storage_absence_v1.json")
	if os.Getenv("NAMES_WRITE_VECTOR") == "1" {
		vector := namesStorageProofVector{
			Version: 1, Height: commit.Version, AppHash: commit.Hash, Store: evmtypes.StoreKey,
			Address: resolver, Slot: absentSlot, Key: absentKey, Value: []byte{}, Node: absentNode, Name: "absent.pax",
		}
		for _, op := range absence.StorageProof[0].Ops {
			vector.ProofOps = append(vector.ProofOps, namesStorageProofOp{Type: op.Type, Key: op.Key, Data: op.Data})
		}
		encoded, err := json.MarshalIndent(vector, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(absencePath, append(encoded, '\n'), 0o644))
	}
	absenceBytes, err := os.ReadFile(absencePath)
	require.NoError(t, err)
	var absenceVector namesStorageProofVector
	require.NoError(t, json.Unmarshal(absenceBytes, &absenceVector))
	require.Equal(t, 1, absenceVector.Version)
	require.Positive(t, absenceVector.Height)
	require.Len(t, absenceVector.AppHash, 32)
	require.Equal(t, evmtypes.StoreKey, absenceVector.Store)
	require.Equal(t, resolver, absenceVector.Address)
	require.Equal(t, absentSlot, absenceVector.Slot)
	require.Equal(t, absentKey, []byte(absenceVector.Key))
	require.Empty(t, absenceVector.Value)
	require.Equal(t, absentNode, absenceVector.Node)
	require.Equal(t, "absent.pax", absenceVector.Name)
	require.Len(t, absenceVector.ProofOps, 2)
	require.Equal(t, storetypes.ProofOpIAVLCommitment, absenceVector.ProofOps[0].Type)
	require.Equal(t, absentKey, []byte(absenceVector.ProofOps[0].Key))
	require.Equal(t, storetypes.ProofOpSimpleMerkleCommitment, absenceVector.ProofOps[1].Type)
	require.Equal(t, []byte(evmtypes.StoreKey), []byte(absenceVector.ProofOps[1].Key))
	storedAbsenceOps := &tmcrypto.ProofOps{}
	for i, op := range absenceVector.ProofOps {
		var commitmentProof ics23.CommitmentProof
		require.NoError(t, commitmentProof.Unmarshal(op.Data))
		if i == 0 {
			require.NotNil(t, commitmentProof.GetNonexist())
		} else {
			require.NotNil(t, commitmentProof.GetExist())
		}
		storedAbsenceOps.Ops = append(storedAbsenceOps.Ops, tmcrypto.ProofOp{Type: op.Type, Key: op.Key, Data: op.Data})
	}
	require.NoError(t, runtime.VerifyAbsence(storedAbsenceOps, absenceVector.AppHash, absentKeypath))

	vectorPath := filepath.Join(vectorDir, "storage_proof_v1.json")
	vectorBytes, err := os.ReadFile(vectorPath)
	require.NoError(t, err)
	var vector namesStorageProofVector
	require.NoError(t, json.Unmarshal(vectorBytes, &vector))
	require.Equal(t, 1, vector.Version)
	require.Positive(t, vector.Height)
	require.Len(t, vector.AppHash, 32)
	require.Equal(t, evmtypes.StoreKey, vector.Store)
	require.Equal(t, resolver, vector.Address)
	require.Equal(t, slot, vector.Slot)
	require.Equal(t, key, []byte(vector.Key))
	require.Equal(t, digest.Bytes(), []byte(vector.Value))
	require.Equal(t, node, vector.Node)
	require.Equal(t, "example.pax", vector.Name)
	require.Len(t, vector.ProofOps, 2)
	require.Equal(t, storetypes.ProofOpIAVLCommitment, vector.ProofOps[0].Type)
	require.Equal(t, key, []byte(vector.ProofOps[0].Key))
	require.Equal(t, storetypes.ProofOpSimpleMerkleCommitment, vector.ProofOps[1].Type)
	require.Equal(t, []byte(evmtypes.StoreKey), []byte(vector.ProofOps[1].Key))
	storedOps := &tmcrypto.ProofOps{}
	for _, op := range vector.ProofOps {
		var commitmentProof ics23.CommitmentProof
		require.NoError(t, commitmentProof.Unmarshal(op.Data))
		require.NotNil(t, commitmentProof.GetExist())
		storedOps.Ops = append(storedOps.Ops, tmcrypto.ProofOp{Type: op.Type, Key: op.Key, Data: op.Data})
	}
	require.NoError(t, runtime.VerifyValue(storedOps, vector.AppHash, keypath, vector.Value))
	require.Error(t, runtime.VerifyValue(storedOps, vector.AppHash, keypath, flipped))
}
