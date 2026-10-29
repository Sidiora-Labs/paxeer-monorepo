package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"sort"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	ics23 "github.com/confio/ics23/go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func deterministicHostCommit(t *testing.T, h *harness) (hostHash, appHash []byte) {
	t.Helper()
	scConfig := config.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	commits := rootmulti.NewStore(t.TempDir(), scConfig, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, commits.Close()) })
	commits.MountStoreWithDB(h.key, sdk.StoreTypeIAVL, nil)
	require.NoError(t, commits.LoadLatestVersion())
	values := h.dump(h.stateDB.Ctx())
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		commits.GetKVStore(h.key).Set([]byte(key), values[key])
	}
	appHash = bytes.Clone(commits.Commit(true).Hash)
	require.NotEmpty(t, keys)
	response := commits.Query(h.stateDB.Ctx().Context(), abci.RequestQuery{
		Path: "/" + h.key.Name() + "/key", Data: []byte(keys[0]), Prove: true,
	})
	require.Zero(t, response.Code, response.Log)
	require.NotEmpty(t, response.Value)
	require.NotNil(t, response.ProofOps)
	require.Len(t, response.ProofOps.Ops, 2)
	var proof ics23.CommitmentProof
	require.NoError(t, proof.Unmarshal(response.ProofOps.Ops[1].Data))
	existence := proof.GetExist()
	require.NotNil(t, existence)
	require.Equal(t, []byte(h.key.Name()), existence.Key)
	require.Equal(t, appHash, commits.LastCommitID().Hash)
	require.True(t, ics23.VerifyMembership(ics23.TendermintSpec, appHash,
		&proof, existence.Key, existence.Value))
	hostHash = bytes.Clone(existence.Value)
	require.Len(t, hostHash, 32)
	require.NotEmpty(t, hostHash)
	require.NotEmpty(t, appHash)
	return hostHash, appHash
}

func deterministicEventProgram() (wasm, iface []byte) {
	section := func(id byte, body []byte) []byte {
		return append([]byte{id, byte(len(body))}, body...)
	}
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	wasm = []byte{0, 97, 115, 109, 1, 0, 0, 0}
	wasm = append(wasm, section(1, []byte{3, 0x60, 4, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	imports := append([]byte{1}, label("layerx_v1")...)
	imports = append(imports, label("event_emit")...)
	wasm = append(wasm, section(2, append(imports, 0, 0))...)
	wasm = append(wasm, section(3, []byte{2, 1, 2})...)
	wasm = append(wasm, section(5, []byte{1, 1, 1, 1})...)
	exports := append([]byte{3}, label("layerx_reserve")...)
	exports = append(exports, 0, 1)
	exports = append(exports, label("layerx_call")...)
	exports = append(exports, 0, 2)
	exports = append(exports, label("memory")...)
	wasm = append(wasm, section(7, append(exports, 2, 0))...)
	body := []byte{0, 0x41, 0, 0x41, 1, 0x41, 1, 0x41, 1, 0x10, 0, 0x1a, 0x41, 0, 0x0b}
	code := []byte{2, 4, 0, 0x41, 16, 0x0b, byte(len(body))}
	wasm = append(wasm, section(10, append(code, body...))...)
	wasm = append(wasm, section(11, []byte{1, 0, 0x41, 0, 0x0b, 2, 't', 'd'})...)
	codeHash := sha256.Sum256(wasm)
	iface = append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = append(iface, 4, 0, 0, 0, 0)
	return wasm, iface
}

func TestProgramsCallLogsDeterministicAcrossRuns(t *testing.T) {
	genesis, callArgs, _ := programsCallFixture(t)
	genesisCtx := genesis.stateDB.Ctx()
	_, did, err := lp.PrincipalOf(genesisCtx, genesis.executor.TestEVMKeeper(), genesis.caller)
	require.NoError(t, err)
	sequence, err := genesis.host.ReadIdentitySequence(genesisCtx, did)
	require.NoError(t, err)
	mainName, mainID, err := lxphost.MainAccountID(genesis.publicKey)
	require.NoError(t, err)
	mainKey := append([]byte{0x06}, mainID[:]...)
	mainRow := bytes.Clone(genesisCtx.KVStore(genesis.key).Get(mainKey))
	sequenceOffset := 4 + 32 + 4 + len(mainName) + 1 + 16 + 4 + 32 + 1
	require.GreaterOrEqual(t, len(mainRow), sequenceOffset+8)
	binary.BigEndian.PutUint64(mainRow[sequenceOffset:sequenceOffset+8], sequence+2)
	genesisCtx.KVStore(genesis.key).Set(mainKey, mainRow)
	mainAccount, err := genesis.host.ReadAccount(genesisCtx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, sequence+2, mainAccount.NextSequence)
	genesisValues := genesis.dump(genesisCtx)
	first, second := newHarness(t), newHarness(t)
	require.NotSame(t, first.host, second.host)
	require.NotSame(t, first.stateDB, second.stateDB)
	require.NotSame(t, first.evm, second.evm)
	for _, h := range []*harness{first, second} {
		for key, value := range genesisValues {
			h.stateDB.Ctx().KVStore(h.key).Set([]byte(key), bytes.Clone(value))
		}
		require.Equal(t, genesisValues, h.dump(h.stateDB.Ctx()))
		require.Empty(t, h.stateDB.GetAllLogs())
	}
	genesisHostHash, genesisAppHash := deterministicHostCommit(t, first)
	secondHostHash, secondAppHash := deterministicHostCommit(t, second)
	require.Equal(t, genesisHostHash, secondHostHash)
	require.Equal(t, genesisAppHash, secondAppHash)

	program := sha256.Sum256([]byte("programs-determinism-deploy-call-transfer"))
	wasm, iface := deterministicEventProgram()
	codeHash := sha256.Sum256(wasm)
	authority, _, err := lp.PrincipalOf(first.stateDB.Ctx(), first.executor.TestEVMKeeper(), first.caller)
	require.NoError(t, err)
	fee, err := first.host.ReadFeeSchedule(first.stateDB.Ctx())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(fee), 97)
	asset := [32]byte(fee[65:97])
	_, payer, err := lxphost.AssetAccountID(first.publicKey, asset)
	require.NoError(t, err)
	treasury, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	amount := big.NewInt(7)
	legs := []lp.TransferLeg{{From: payer, Asset: asset, To: treasury, Amount: amount}}
	callArgs[0] = program
	deployInput, err := lp.ProgramsABI.Pack("deploy", program, uint16(1), uint8(1), authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	callInput, err := lp.ProgramsABI.Pack("call", callArgs...)
	require.NoError(t, err)
	transferInput, err := lp.ProgramsABI.Pack("transfer", program, legs)
	require.NoError(t, err)
	inputs := [][]byte{deployInput, callInput, transferInput}
	methods := []string{"deploy", "call", "transfer"}
	type execution struct {
		outputs           [][]byte
		logs              [][]byte
		states            []map[string][]byte
		hostHash, appHash []byte
	}
	var runs [2]execution
	for run, h := range []*harness{first, second} {
		var payerBefore, treasuryBefore sdk.Int
		for step, input := range inputs {
			if step == 2 {
				sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
				require.NoError(t, err)
				mainAccount, err := h.host.ReadAccount(h.stateDB.Ctx(), mainName, mainID)
				require.NoError(t, err)
				require.Equal(t, sequence, mainAccount.NextSequence)
				payerBefore, err = h.host.ReadBalance(h.stateDB.Ctx(), payer, asset)
				require.NoError(t, err)
				treasuryBefore, err = h.host.ReadBalance(h.stateDB.Ctx(), treasury, asset)
				require.NoError(t, err)
			}
			output, _, err := h.evm.Call(h.caller, h.precompile.Address(), bytes.Clone(input), 20000000, uint256.NewInt(0))
			require.NoError(t, err, "run %d %s must succeed; output: %x", run, methods[step], output)
			if step == 1 {
				values, err := lp.ProgramsABI.Methods["call"].Outputs.Unpack(output)
				require.NoError(t, err)
				require.Equal(t, int32(0), values[0])
				require.Equal(t, uint8(1), values[1])
				require.NotEmpty(t, output)
			} else {
				require.Empty(t, output)
			}
			runs[run].outputs = append(runs[run].outputs, bytes.Clone(output))
			require.Len(t, h.stateDB.GetAllLogs(), []int{1, 3, 4}[step], "%s logs", methods[step])
			logs, err := json.Marshal(h.stateDB.GetAllLogs())
			require.NoError(t, err)
			runs[run].logs = append(runs[run].logs, logs)
			runs[run].states = append(runs[run].states, h.dump(h.stateDB.Ctx()))
			if run == 0 {
				require.Equal(t, genesisValues, second.dump(second.stateDB.Ctx()))
				require.Empty(t, second.stateDB.GetAllLogs())
			}
		}
		payerAfter, err := h.host.ReadBalance(h.stateDB.Ctx(), payer, asset)
		require.NoError(t, err)
		treasuryAfter, err := h.host.ReadBalance(h.stateDB.Ctx(), treasury, asset)
		require.NoError(t, err)
		require.Equal(t, new(big.Int).Sub(payerBefore.BigInt(), amount), payerAfter.BigInt())
		require.Equal(t, new(big.Int).Add(treasuryBefore.BigInt(), amount), treasuryAfter.BigInt())
		logs := h.stateDB.GetAllLogs()
		require.Len(t, logs, 4)
		for index, event := range []string{"ProgramDeployed", "ProgramGuestEvent", "ProgramCallOutcome", "ProgramTransferred"} {
			require.Equal(t, h.precompile.Address(), logs[index].Address)
			require.Equal(t, []common.Hash{lp.ProgramsABI.Events[event].ID, common.Hash(program)}, logs[index].Topics)
			require.Equal(t, uint(index), logs[index].Index)
		}
		runs[run].hostHash, runs[run].appHash = deterministicHostCommit(t, h)
		require.NotEqual(t, genesisHostHash, runs[run].hostHash)
		require.NotEqual(t, genesisAppHash, runs[run].appHash)
	}
	for step, method := range methods {
		require.Equal(t, runs[0].outputs[step], runs[1].outputs[step], "%s output bytes", method)
		require.Equal(t, runs[0].logs[step], runs[1].logs[step], "%s log bytes", method)
		require.Equal(t, runs[0].states[step], runs[1].states[step], "%s host state", method)
	}
	require.Equal(t, runs[0].hostHash, runs[1].hostHash, "host store commit hash")
	require.Equal(t, runs[0].appHash, runs[1].appHash, "host store app hash")
}
