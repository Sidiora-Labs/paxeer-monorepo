package layerxprograms_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func retirementMerklePath(nodes [][32]byte, index int) ([32]byte, [][32]byte) {
	nodes = append([][32]byte(nil), nodes...)
	var siblings [][32]byte
	for len(nodes) > 1 {
		sibling := index ^ 1
		if sibling >= len(nodes) {
			sibling = index
		}
		siblings = append(siblings, nodes[sibling])
		var next [][32]byte
		for i := 0; i < len(nodes); i += 2 {
			right := i + 1
			if right == len(nodes) {
				right = i
			}
			next = append(next, codec.StateNodeHash(nodes[i], nodes[right]))
		}
		nodes, index = next, index/2
	}
	return nodes[0], siblings
}

func retirementManifestProof(t *testing.T, program [32]byte, manifest, value []byte) ([]byte, [32]byte) {
	t.Helper()
	manifestDigest, valueDigest := sha256.Sum256(manifest), sha256.Sum256(value)
	key := append(append([]byte("progstor"), program[:]...), 1)
	head := binary.BigEndian.AppendUint16(nil, 1)
	head = binary.BigEndian.AppendUint32(head, 1)
	head = append(head, manifestDigest[:]...)
	blobLeaf := func(digest [32]byte, body []byte) [32]byte {
		key := make([]byte, 129)
		key[0] = 0xff
		copy(key[97:], digest[:])
		return codec.StateLeafHash(key, body)
	}
	blobA, blobB := blobLeaf(manifestDigest, manifest), blobLeaf(valueDigest, value)
	if bytes.Compare(manifestDigest[:], valueDigest[:]) > 0 {
		blobA, blobB = blobB, blobA
	}
	subtree, pathA := retirementMerklePath([][32]byte{codec.StateLeafHash(key, head), blobA, blobB}, 0)
	empty, err := codec.DomainHash(codec.DomainStateLeaf, nil)
	require.NoError(t, err)
	registration := []byte{1, 10}
	registration = append(registration, make([]byte, 16)...)
	for ordinal := uint32(1); ordinal <= 11; ordinal++ {
		if ordinal == 8 {
			continue
		}
		registration = binary.BigEndian.AppendUint32(registration, 0x00090000|ordinal)
	}
	universal, _ := retirementMerklePath([][32]byte{
		codec.StateLeafHash([]byte{3, 0, 9, 0, 0, 0, 5}, registration),
		codec.StateLeafHash([]byte("sequence"), binary.BigEndian.AppendUint64(nil, 2)),
	}, 0)
	var modules [][32]byte
	for module := uint16(0); module <= 9; module++ {
		root := empty
		if module == 0 {
			root = universal
		} else if module == 9 {
			root = subtree
		}
		modules = append(modules, codec.StateLeafHash(binary.BigEndian.AppendUint16(nil, module), root[:]))
	}
	root, pathB := retirementMerklePath(modules, 9)
	proof := binary.BigEndian.AppendUint16(nil, 2)
	proof = binary.BigEndian.AppendUint16(proof, 9)
	proof = binary.BigEndian.AppendUint32(proof, uint32(len(key)))
	proof = append(proof, key...)
	proof = binary.BigEndian.AppendUint32(proof, uint32(len(head)))
	proof = append(proof, head...)
	proof = binary.BigEndian.AppendUint32(proof, 0)
	proof = binary.BigEndian.AppendUint32(proof, 3)
	proof = append(proof, byte(len(pathA)))
	for _, sibling := range pathA {
		proof = append(proof, sibling[:]...)
	}
	proof = binary.BigEndian.AppendUint32(proof, 10)
	proof = append(proof, byte(len(pathB)))
	for _, sibling := range pathB {
		proof = append(proof, sibling[:]...)
	}
	witness, err := codec.DecodeStateWitness(proof)
	require.NoError(t, err)
	require.NoError(t, witness.Verify(root))
	return proof, root
}

func retirementRequest(t *testing.T, h *harness) ([]byte, []byte, [32]byte) {
	t.Helper()
	ctx := h.stateDB.Ctx()
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	controller := authority.Principal
	program := [32]byte{0x71}
	value := []byte("retired program state")
	valueDigest := sha256.Sum256(value)
	manifest := binary.BigEndian.AppendUint16(nil, 1)
	manifest = binary.BigEndian.AppendUint32(manifest, 1)
	manifest = binary.BigEndian.AppendUint16(manifest, 14)
	manifest = append(manifest, []byte("paxai/state/v1")...)
	manifest = append(manifest, valueDigest[:]...)
	manifest = binary.BigEndian.AppendUint32(manifest, uint32(len(value)))
	proof, originRoot := retirementManifestProof(t, program, manifest, value)
	digest := sha256.Sum256(manifest)
	archives := []ed25519.PrivateKey{
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize)),
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x52}, ed25519.SeedSize)),
	}
	control := make([]byte, 251)
	copy(control, "LXRT1")
	binary.BigEndian.PutUint16(control[5:7], 1)
	binary.BigEndian.PutUint32(control[7:11], 7)
	copy(control[11:43], controller[:])
	copy(control[43:75], archives[0].Public().(ed25519.PublicKey))
	copy(control[75:107], archives[1].Public().(ed25519.PublicKey))
	copy(control[107:139], program[:])
	binary.BigEndian.PutUint64(control[139:147], 1)
	profileDigest := sha256.Sum256(append([]byte("LXP/programs-retirement-profile/v1\x00"), control[:147]...))
	copy(control[147:179], profileDigest[:])
	store := ctx.KVStore(h.key)
	store.Set(append([]byte{1, 0, 9}, []byte("progretire/v1")...), control)
	store.Set(append([]byte{5, 0, 9}, digest[:]...), manifest)
	method := lp.ProgramsABI.Methods["registryRead"]
	payload, err := lp.EncodeRegistryReadPayload(program, []byte("manifest"))
	require.NoError(t, err)
	var currentRoot [32]byte
	for i := 0; i < 2; i++ {
		receipt, err := lp.Submit(ctx, h.executor, lp.NewCallContext(h.caller, &method), 0x00090004, payload, [16]byte{})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		currentRoot = receipt.ResultingStateRoot
	}
	request := make([]byte, 370+149)
	request[0], request[1] = 1, 1
	binary.BigEndian.PutUint32(request[4:8], 7)
	binary.BigEndian.PutUint16(request[8:10], 1)
	copy(request[10:42], profileDigest[:])
	copy(request[42:74], controller[:])
	binary.BigEndian.PutUint64(request[74:82], 3)
	copy(request[82:114], currentRoot[:])
	binary.BigEndian.PutUint64(request[114:122], 2)
	cutoffHeader := sha256.Sum256([]byte("retirement cutoff header"))
	copy(request[122:154], cutoffHeader[:])
	archiveManifest := sha256.Sum256([]byte("retirement archive manifest"))
	copy(request[154:186], archiveManifest[:])
	binary.BigEndian.PutUint64(request[186:194], uint64(len(proof)+len(manifest)+len(value)))
	binary.BigEndian.PutUint64(request[194:202], 1)
	binary.BigEndian.PutUint64(request[202:210], 2)
	copy(request[210:242], originRoot[:])
	candidate := request[370:]
	candidate[0], candidate[1] = 1, 33
	copy(candidate[4:36], program[:])
	candidate[36] = 1
	copy(candidate[37:69], digest[:])
	binary.BigEndian.PutUint32(candidate[69:73], uint32(len(manifest)))
	binary.BigEndian.PutUint64(candidate[73:81], 1)
	copy(candidate[81:113], originRoot[:])
	binary.BigEndian.PutUint16(candidate[145:147], uint16(len(proof)))
	binary.BigEndian.PutUint16(candidate[147:149], uint16(len(manifest)))
	setDigest := sha256.Sum256(append([]byte("LXP/programs-retirement-candidates/v1\x00"), candidate[:145]...))
	certificate := sha256.New()
	certificate.Write([]byte("LXP/programs-retirement-archive/v1\x00"))
	for _, field := range [][]byte{request[4:8], request[10:42], request[154:186], request[186:194], request[194:202], request[202:210], request[210:242], request[122:154], setDigest[:]} {
		certificate.Write(field)
	}
	for i, archive := range archives {
		copy(request[242+i*64:306+i*64], ed25519.Sign(archive, certificate.Sum(nil)))
	}
	request = append(request, proof...)
	request = append(request, manifest...)
	require.NoError(t, lp.CheckRetireStorageRequest(request))
	return request, manifest, digest
}

func TestProgramsRetireStorageEmitsStorageRetired(t *testing.T) {
	h := newHarness(t)
	request, manifest, digest := retirementRequest(t, h)
	before := bytes.Clone(request)
	input, err := lp.ProgramsABI.Pack("retireStorage", request)
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	require.Empty(t, data)
	require.Equal(t, before, request)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 2)
	event := lp.ProgramsABI.Events["StorageRetired"]
	activityID := logs[0].Topics[1]
	require.NotEqual(t, common.Hash{}, activityID)
	for i, log := range logs {
		require.Equal(t, h.precompile.Address(), log.Address)
		require.Equal(t, []common.Hash{event.ID, activityID}, log.Topics)
		require.Equal(t, uint(i), log.Index)
		values, err := event.Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, uint8(i), values[0])
		frame := values[1].([]byte)
		require.Equal(t, []byte{'L', 'X', 'R', 'T', byte(i), 2, 1}, frame[:7])
		if i == 0 {
			require.Len(t, frame, 226)
			require.Equal(t, activityID[:], frame[7:39])
			require.Equal(t, request[82:114], frame[39:71])
			require.Equal(t, request[10:42], frame[71:103])
			require.Equal(t, request[154:186], frame[167:199])
			require.Equal(t, []byte{1, 1, 0}, frame[199:202])
			require.Equal(t, uint64(len(manifest)), binary.BigEndian.Uint64(frame[202:210]))
		} else {
			require.Len(t, frame, 117)
			require.Equal(t, []byte{1, 1}, frame[7:9])
			require.Equal(t, digest[:], frame[9:41])
			require.Equal(t, uint32(len(manifest)), binary.BigEndian.Uint32(frame[41:45]))
			require.Equal(t, request[443:483], frame[45:85])
			require.Equal(t, make([]byte, 32), frame[85:117])
		}
	}
	ctx := h.stateDB.Ctx()
	require.False(t, ctx.KVStore(h.key).Has(append([]byte{5, 0, 9}, digest[:]...)))
	control, err := h.host.ReadRetirementControl(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(control[187:195]))
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(control[195:203]))
	require.Equal(t, uint64(len(manifest)), binary.BigEndian.Uint64(control[203:211]))
}

func TestProgramsRetireStorageOversizeReverts(t *testing.T) {
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	before := h.dump(ctx)
	request := make([]byte, 32769)
	request[0], request[1] = 1, 1
	input, err := lp.ProgramsABI.Pack("retireStorage", request)
	require.NoError(t, err)
	const suppliedGas uint64 = 2000000
	data, remainingGas, err := h.evm.Call(h.caller, h.precompile.Address(), input, suppliedGas, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	kv := storetypes.KVGasConfig()
	wantGas := kv.WriteCostFlat + kv.WriteCostPerByte*uint64(len(input)-4) + 2*addr.LayerXBindSignatureGas + 20000
	require.Equal(t, wantGas, suppliedGas-remainingGas)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.GetAllLogs())
	branch, _ := h.stateDB.Ctx().CacheContext()
	require.Zero(t, lp.NextOrdinal(branch, [32]byte(ctx.EVMTxHash())))
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, "LXP_LENGTH_LIMIT", reason)
}
