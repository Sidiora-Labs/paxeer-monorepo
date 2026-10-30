package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func prepareSandboxExecute(t *testing.T, h *harness, callArgs []interface{}, callPayload []byte) ([]interface{}, func()) {
	t.Helper()
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	accountID := func(name string) [32]byte {
		material := binary.BigEndian.AppendUint32([]byte("LX:ACCOUNT:v1"), uint32(len(name)))
		return sha256.Sum256(append(material, name...))
	}
	feeRecord, err := h.host.ReadFeeSchedule(ctx)
	require.NoError(t, err)
	asset := [32]byte(feeRecord[65:97])
	_, tenantAccount, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	program, leaseID := callArgs[0].([32]byte), [32]byte{0x11}
	record, err := h.host.ReadProgram(ctx, program)
	require.NoError(t, err)
	codeHash := record.CodeHash
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), leaseID[:]...)
	material := append([]byte("LayerX/programs/program-account/v1\x00"), program[:]...)
	material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
	escrow := sha256.Sum256(append(material, seed...))
	store := ctx.KVStore(h.key)
	moduleKey := func(prefix string) []byte {
		return append(append([]byte{1, 0, 9}, prefix...), program[:]...)
	}
	store.Set(moduleKey("program-owner\x00"), append([]byte{1}, authority.Principal[:]...))
	store.Set(moduleKey("program-account-profile\x00"), append([]byte{2}, authority.Principal[:]...))
	register, err := lp.ProgramsABI.Pack("registerAccount", program, uint8(1), asset, seed)
	require.NoError(t, err)
	registered, _, err := h.evm.Call(h.caller, h.precompile.Address(), register, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "registerAccount reverted: %x", registered)
	require.Empty(t, registered)
	ctx = h.stateDB.Ctx()
	account, err := h.host.ReadProgramAccount(ctx, program, asset, seed)
	require.NoError(t, err)
	require.Equal(t, program, account.ProgramID)
	require.Equal(t, asset, account.AssetID)
	require.Equal(t, seed, account.Seed)
	require.Equal(t, escrow, account.AccountID)
	h.stateDB.ResetForTracer()
	fees := accountID("system:fees")
	store = ctx.KVStore(h.key)
	seedAccount := func(id [32]byte, name string, kind byte, balance uint64, accountAsset [32]byte, hasAsset byte, next uint64, key [32]byte, hasKey byte) {
		field := func(out, value []byte) []byte {
			return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
		}
		row := field(nil, id[:])
		row = field(row, []byte(name))
		row = append(row, kind)
		row = binary.BigEndian.AppendUint64(row, 0)
		row = binary.BigEndian.AppendUint64(row, balance)
		row = field(row, accountAsset[:])
		row = append(row, hasAsset)
		row = binary.BigEndian.AppendUint64(row, next)
		row = binary.BigEndian.AppendUint64(row, 1)
		row = append(row, 0, 0)
		row = field(row, key[:])
		row = append(row, hasKey)
		store.Set(append([]byte{6}, id[:]...), row)
	}
	mainName, mainID, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	main, err := h.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	mainAsset := main.AssetID
	require.NotEqual(t, asset, mainAsset)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	seedAccount(mainID, mainName, 1, 0, [32]byte{}, 0, sequence, h.publicKey, 1)
	seedAccount(fees, "system:fees", 10, 0, asset, 1, 0, [32]byte{}, 0)
	material = append([]byte("LayerX/programs/sandbox/namespace/v1\x00"), program[:]...)
	namespace := sha256.Sum256(append(material, leaseID[:]...))
	var terms []byte
	for _, word := range [][32]byte{leaseID, authority.Principal, program, codeHash, namespace, asset, escrow, fees} {
		terms = append(terms, word[:]...)
	}
	terms = binary.BigEndian.AppendUint64(terms, 0)
	terms = binary.BigEndian.AppendUint64(terms, 100000000)
	budget := callArgs[7].([7]uint64)
	for _, bound := range append(budget[:], 1048576, 100, 200) {
		terms = binary.BigEndian.AppendUint64(terms, bound)
	}
	schedule := binary.BigEndian.AppendUint32(nil, 1)
	for range 7 {
		schedule = binary.BigEndian.AppendUint64(schedule, 1)
	}
	requestMaterial := append([]byte("LayerX/programs/sandbox/request/v3\x00"), terms...)
	requestDigest := sha256.Sum256(append(requestMaterial, schedule...))
	request := append([]byte("LayerX/programs/sandbox/lease-state/v3\x00"), terms...)
	request = append(request, make([]byte, 64)...)
	request = append(request, schedule...)
	request = append(request, make([]byte, 16)...)
	request = append(request, 0, 1, 0, 0, 0)
	request = append(request, bytes.Repeat([]byte{0x81}, 32)...)
	request = append(request, requestDigest[:]...)
	requestReceipt := sha256.Sum256(request)
	request = append(request, requestReceipt[:]...)
	request = binary.BigEndian.AppendUint64(request, 100)
	request = append(request, 0, 0, 0)
	fund, err := lp.ProgramsABI.Pack("sandboxFund", leaseID, authority.Principal, program, escrow, asset,
		big.NewInt(100000000), uint64(200), uint32(1), [7]uint64{1, 1, 1, 1, 1, 1, 1}, request, program,
		[]lp.TransferLeg{{From: tenantAccount, Asset: asset, To: escrow, Amount: big.NewInt(100000000)}})
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), fund, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "sandboxFund reverted: %x", data)
	funded, err := h.host.ReadSandbox(h.stateDB.Ctx(), leaseID, 0)
	require.NoError(t, err)
	stateOffset := len("LayerX/programs/sandbox/lease-state/v3\x00") + len(terms) + 64 + len(schedule) + 16
	require.Equal(t, byte(1), funded[stateOffset])
	activate, err := lp.ProgramsABI.Pack("sandboxActivate", leaseID, sha256.Sum256(funded), uint64(3))
	require.NoError(t, err)
	data, _, err = h.evm.Call(h.caller, h.precompile.Address(), activate, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "sandboxActivate reverted: %x; funded lease: %x", data, funded)
	active, err := h.host.ReadSandbox(h.stateDB.Ctx(), leaseID, 0)
	require.NoError(t, err)
	require.Equal(t, byte(2), active[stateOffset])
	for _, kind := range []uint8{1, 2} {
		state, err := h.host.ReadSandbox(h.stateDB.Ctx(), leaseID, kind)
		require.NoError(t, err)
		require.NotEmpty(t, state)
	}
	storedTerms := active[len("LayerX/programs/sandbox/lease-state/v3\x00"):]
	storedSchedule := storedTerms[len(terms)+64:]
	var rates [7]uint64
	for i := range rates {
		rates[i] = binary.BigEndian.Uint64(storedSchedule[4+8*i : 12+8*i])
	}
	args := []interface{}{
		[32]byte(storedTerms[:32]), uint64(1), sha256.Sum256(active),
		[32]byte(storedTerms[192:224]), [32]byte(storedTerms[160:192]), [32]byte(storedTerms[224:256]),
		binary.BigEndian.Uint32(storedSchedule[:4]), rates, callPayload, big.NewInt(0),
	}
	return args, func() {
		t.Helper()
		last, err := h.host.ReadLastSequence(h.stateDB.Ctx(), did)
		require.NoError(t, err)
		sequence := last + 1
		store = h.stateDB.Ctx().KVStore(h.key)
		seedAccount(mainID, mainName, 1, 100000000, mainAsset, 1, sequence, h.publicKey, 1)
		main, err := h.host.ReadAccount(h.stateDB.Ctx(), mainName, mainID)
		require.NoError(t, err)
		require.Equal(t, mainAsset, main.AssetID)
		require.Equal(t, int64(100000000), main.Balance.Int64())
		require.Equal(t, sequence, main.NextSequence)
	}
}

func sandboxExecuteArguments(t *testing.T, asset [32]byte, limit *big.Int) []interface{} {
	t.Helper()
	call, err := lp.EncodeCallPayload([32]byte{0xa1}, 3, "layerx_call", nil, []byte{0, 0},
		[]byte("LayerX/programs/access-declaration/v1\x00\x00"), 64,
		[7]uint64{100000, 65536, 1024, 1024, 16, 1024, 16})
	require.NoError(t, err)
	return []interface{}{
		[32]byte{0x11}, uint64(1), [32]byte{0x22}, [32]byte{0x33}, asset,
		[32]byte{0x55}, uint32(1), [7]uint64{1, 1, 1, 1, 1, 1, 1}, call, limit,
	}
}

func TestProgramsSandboxExecuteEmitsUsage(t *testing.T) {
	h, callArgs, _ := programsCallFixture(t)
	authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	seedDeployMetering(t, h, authority.Principal)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	program := sha256.Sum256([]byte(t.Name()))
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v3\x00"), codeHash[:]...)
	iface = binary.BigEndian.AppendUint16(iface, 3)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = binary.BigEndian.AppendUint16(iface, uint16(len("layerx_call")))
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	iface = append(iface, 0, 0, 0, 0, 0, 0)
	require.Equal(t, uint16(3), binary.BigEndian.Uint16(iface[len("LayerX/program-interface/v3\x00")+32:]))
	deploy, err := lp.ProgramsABI.Pack("deploy", program, uint16(3), uint8(1), authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	deployed, _, err := h.evm.Call(h.caller, h.precompile.Address(), deploy, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "deploy returned %x", deployed)
	require.Empty(t, deployed)
	record, err := h.host.ReadProgram(h.stateDB.Ctx(), program)
	require.NoError(t, err)
	require.Equal(t, uint16(3), record.ABIVersion)
	require.Equal(t, codeHash, record.CodeHash)
	require.True(t, record.Active)
	deployLogs := h.stateDB.GetAllLogs()
	require.Len(t, deployLogs, 1)
	require.Equal(t, []common.Hash{lp.ProgramsABI.Events["ProgramDeployed"].ID, common.Hash(program)}, deployLogs[0].Topics)
	h.stateDB.ResetForTracer()
	storedInterface, err := h.host.ReadProgramInterface(h.stateDB.Ctx(), program)
	require.NoError(t, err)
	require.Equal(t, iface, storedInterface.Encoding)
	h.stateDB.ResetForTracer()
	callArgs[0], callArgs[1] = program, uint16(3)
	access, budget := callArgs[5].([]byte), callArgs[7].([7]uint64)
	callPayload, err := lp.EncodeCallPayload(program, 3, "layerx_call", []byte{0, 0xff}, []byte{0, 1, 3}, access, 64, budget)
	require.NoError(t, err)
	method := lp.ProgramsABI.Methods["sandboxExecute"]
	args := sandboxExecuteArguments(t, [32]byte{0x44}, big.NewInt(0))
	payload, err := lp.EncodeSandboxExecutePayload(
		args[0].([32]byte), args[1].(uint64), args[2].([32]byte), args[3].([32]byte),
		args[4].([32]byte), args[5].([32]byte), args[6].(uint32), args[7].([7]uint64), args[8].([]byte))
	require.NoError(t, err)
	require.Equal(t, []byte{1, 1, 0, 0}, payload[:4])
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(payload[36:44]))
	require.Equal(t, args[8], payload[236:])

	args, fundFeeLimit := prepareSandboxExecute(t, h, callArgs, callPayload)
	if !t.Run("fee limit reaches submission", func(t *testing.T) {
		for _, limit := range []*big.Int{
			big.NewInt(9), new(big.Int).Lsh(big.NewInt(1), 64),
			new(big.Int).Lsh(big.NewInt(1), 127),
		} {
			args[9] = limit
			input, err := lp.ProgramsABI.Pack(method.Name, args...)
			require.NoError(t, err)
			before := h.dump(h.stateDB.Ctx())
			data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(data)
			require.NoError(t, err)
			// Admission refuses an unpayable fee limit before a receipt exists.
			require.Equal(t, lxphost.Code(-602).Error(), reason)
			require.Equal(t, before, h.dump(h.stateDB.Ctx()))
			require.Empty(t, h.stateDB.GetAllLogs())
		}
	}) {
		t.FailNow()
	}

	fundFeeLimit()
	args[9] = big.NewInt(100000000)
	escrow, asset := args[3].([32]byte), args[4].([32]byte)
	escrowAccount, err := h.host.ReadAccount(h.stateDB.Ctx(), "module:programs:value:"+hex.EncodeToString(escrow[:]), escrow)
	require.NoError(t, err, "sandbox assertion ladder: read escrow account")
	require.Equal(t, uint8(13), escrowAccount.Kind, "sandbox assertion ladder: escrow must be a module-value account")
	require.True(t, escrowAccount.HasAsset, "sandbox assertion ladder: escrow must hold an asset")
	require.Equal(t, asset, escrowAccount.AssetID, "sandbox assertion ladder: escrow asset must match the active lease")
	require.False(t, escrowAccount.HasAuthorityKey, "sandbox assertion ladder: escrow must have no authority key")
	require.Equal(t, [32]byte{}, escrowAccount.AuthorityKey, "sandbox assertion ladder: escrow authority key must be empty")
	require.Equal(t, int64(100000000), escrowAccount.Balance.Int64(), "sandbox assertion ladder: escrow must retain its full funding")
	leaseID := args[0].([32]byte)
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), leaseID[:]...)
	binding, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), program, asset, seed)
	require.NoError(t, err, "sandbox assertion ladder: read program-account binding")
	require.Equal(t, escrow, binding.AccountID, "sandbox assertion ladder: program-account binding must identify the escrow")
	record, err = h.host.ReadProgram(h.stateDB.Ctx(), program)
	require.NoError(t, err, "sandbox assertion ladder: read guest version")
	require.Equal(t, uint16(3), record.ABIVersion, "sandbox assertion ladder: registerAccount must have admitted guest ABI 3")
	t.Log("sandbox assertion ladder: funded escrow and program-account binding verified; registerAccount admitted guest ABI 3")

	plainPayload, err := lp.EncodeCallPayload(callArgs[0].([32]byte), callArgs[1].(uint16), callArgs[2].(string),
		callArgs[3].([]byte), callArgs[4].([]byte), callArgs[5].([]byte), callArgs[6].(uint32), callArgs[7].([7]uint64))
	require.NoError(t, err)
	require.Equal(t, callPayload, plainPayload, "sandbox assertion ladder: ordinary call must execute the identical ABI-v3 payload")
	plainInput, err := lp.ProgramsABI.Pack("call", callArgs...)
	require.NoError(t, err)
	beforePlainCall := h.dump(h.stateDB.Ctx())
	beforePlainLogs := h.stateDB.GetAllLogs()
	snapshot := h.stateDB.Snapshot()
	plainData, _, plainErr := h.evm.Call(h.caller, h.precompile.Address(), plainInput, 2000000, uint256.NewInt(0))
	if plainErr != nil {
		reason, unpackErr := abi.UnpackRevert(plainData)
		require.NoError(t, plainErr, "sandbox assertion ladder: ordinary ABI-v3 call reverted: %s (revert decoding: %v)", reason, unpackErr)
	}
	plainOutput, err := lp.ProgramsABI.Methods["call"].Outputs.Unpack(plainData)
	require.NoError(t, err)
	require.Equal(t, int32(0), plainOutput[0], "sandbox assertion ladder: ordinary ABI-v3 call must settle successfully")
	require.Equal(t, uint8(1), plainOutput[1], "sandbox assertion ladder: ordinary ABI-v3 call must settle successfully")
	h.stateDB.RevertToSnapshot(snapshot)
	require.Equal(t, beforePlainCall, h.dump(h.stateDB.Ctx()))
	require.Equal(t, beforePlainLogs, h.stateDB.GetAllLogs())
	t.Log("sandbox assertion ladder: identical ABI-v3 payload settled through the registered call handler")

	input, err := lp.ProgramsABI.Pack(method.Name, args...)
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	if err != nil {
		reason, unpackErr := abi.UnpackRevert(data)
		require.NoError(t, unpackErr)
		var states [3][]byte
		for kind := range states {
			state, readErr := h.host.ReadSandbox(h.stateDB.Ctx(), args[0].([32]byte), uint8(kind))
			require.NoError(t, readErr)
			states[kind] = state
		}
		require.NoError(t, err, "sandboxExecute reverted: %s; stored lease: %x; stored escrow: %x; stored ledger: %x",
			reason, states[0], states[1], states[2])
	}
	output, err := method.Outputs.Unpack(data)
	require.NoError(t, err)
	require.Equal(t, int32(0), output[0])
	require.Equal(t, uint8(1), output[1])
	logs := h.stateDB.GetAllLogs()
	require.NotEmpty(t, logs)
	var usage []byte
	var count uint16
	var total uint32
	var activity common.Hash
	var chunks uint16
	for _, log := range logs {
		if len(log.Topics) == 0 || log.Topics[0] != lp.ProgramsABI.Events["SandboxUsageReceipt"].ID {
			continue
		}
		require.Equal(t, h.precompile.Address(), log.Address)
		require.Len(t, log.Topics, 2)
		values, err := lp.ProgramsABI.Events["SandboxUsageReceipt"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, uint64(1), values[0])
		require.Equal(t, chunks, values[1])
		if chunks == 0 {
			activity, count, total = log.Topics[1], values[2].(uint16), values[3].(uint32)
			require.NotEqual(t, common.Hash{}, activity)
			require.Positive(t, count)
		} else {
			require.Equal(t, activity, log.Topics[1])
			require.Equal(t, count, values[2])
			require.Equal(t, total, values[3])
		}
		usage = append(usage, values[4].([]byte)...)
		chunks++
	}
	require.Positive(t, chunks)
	require.Equal(t, count, chunks)
	require.Equal(t, int(total), len(usage))
	require.Contains(t, string(usage), "LayerX/programs/sandbox/usage-receipt/v4\x00")
}

func TestProgramsSandboxExecuteZeroAssetReverts(t *testing.T) {
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	before := h.dump(ctx)
	args := sandboxExecuteArguments(t, [32]byte{}, new(big.Int).Lsh(big.NewInt(1), 127))
	input, err := lp.ProgramsABI.Pack("sandboxExecute", args...)
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, "LXP_NON_CANONICAL", reason)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.GetAllLogs())
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Zero(t, sequence)
	branch, _ := h.stateDB.Ctx().CacheContext()
	require.Zero(t, lp.NextOrdinal(branch, [32]byte(ctx.EVMTxHash())))
}
