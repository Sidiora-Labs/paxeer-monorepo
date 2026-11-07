package layerxprograms_test

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/repair_m213_lease_request.hex
var sandboxLeaseRequestHex string

const sandboxLeaseStateDomain = "LayerX/programs/sandbox/lease-state/v3\x00"

func recordedSandboxLeaseRequest(t *testing.T) []byte {
	t.Helper()
	request, err := hex.DecodeString(strings.TrimSpace(sandboxLeaseRequestHex))
	require.NoError(t, err)
	require.Len(t, request, 643)
	require.Equal(t, sandboxLeaseStateDomain, string(request[:len(sandboxLeaseStateDomain)]))
	return request
}

func decodeSandboxLeaseRequest(t *testing.T, request []byte) lp.SandboxLeaseRequest {
	t.Helper()
	require.Len(t, request, 643)
	require.Equal(t, sandboxLeaseStateDomain, string(request[:len(sandboxLeaseStateDomain)]))
	terms := request[len(sandboxLeaseStateDomain):]
	r := lp.SandboxLeaseRequest{
		LeaseID: [32]byte(terms[:32]), Tenant: [32]byte(terms[32:64]),
		Program: [32]byte(terms[64:96]), CodeHash: [32]byte(terms[96:128]),
		Asset: [32]byte(terms[160:192]), Amount: new(big.Int).SetBytes(terms[256:272]),
		OpenedAt: binary.BigEndian.Uint64(terms[336:344]), Expiry: binary.BigEndian.Uint64(terms[344:352]),
		FeeVersion:    binary.BigEndian.Uint32(terms[416:420]),
		ActivityID:    [32]byte(request[len(request)-107 : len(request)-75]),
		ReceiptDigest: [32]byte(request[len(request)-43 : len(request)-11]),
	}
	for i := range r.Limits {
		r.Limits[i] = binary.BigEndian.Uint64(terms[272+8*i : 280+8*i])
	}
	for i := range r.FeeRates {
		r.FeeRates[i] = binary.BigEndian.Uint64(terms[420+8*i : 428+8*i])
	}
	return r
}

func prepareSandboxExecute(t *testing.T, h *harness, callArgs []interface{}, callPayload []byte, leaseID, activityID, receiptDigest [32]byte) ([]interface{}, []byte, func()) {
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
	program := callArgs[0].([32]byte)
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
	budget := callArgs[7].([7]uint64)
	request, err := lp.EncodeSandboxLeaseRequest(lp.SandboxLeaseRequest{
		LeaseID: leaseID, Tenant: authority.Principal, Program: program, CodeHash: codeHash, Asset: asset,
		Amount:   big.NewInt(100000000),
		Limits:   [8]uint64{budget[0], budget[1], budget[2], budget[3], budget[4], budget[5], budget[6], 1048576},
		OpenedAt: 100, Expiry: 200, FeeVersion: 1, FeeRates: [7]uint64{1, 1, 1, 1, 1, 1, 1},
		ActivityID: activityID, ReceiptDigest: receiptDigest,
	})
	require.NoError(t, err)
	fund, err := lp.ProgramsABI.Pack("sandboxFund", leaseID, authority.Principal, program, escrow, asset,
		big.NewInt(100000000), uint64(200), uint32(1), [7]uint64{1, 1, 1, 1, 1, 1, 1}, request, program,
		[]lp.TransferLeg{{From: tenantAccount, Asset: asset, To: escrow, Amount: big.NewInt(100000000)}})
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), fund, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "sandboxFund reverted: %x", data)
	funded, err := h.host.ReadSandbox(h.stateDB.Ctx(), leaseID, 0)
	require.NoError(t, err)
	stateOffset := len("LayerX/programs/sandbox/lease-state/v3\x00") + (8*32 + 16 + 10*8) + 64 + (4 + 7*8) + 16
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
	storedSchedule := storedTerms[(8*32+16+10*8)+64:]
	var rates [7]uint64
	for i := range rates {
		rates[i] = binary.BigEndian.Uint64(storedSchedule[4+8*i : 12+8*i])
	}
	args := []interface{}{
		[32]byte(storedTerms[:32]), uint64(1), sha256.Sum256(active),
		[32]byte(storedTerms[192:224]), [32]byte(storedTerms[160:192]), [32]byte(storedTerms[224:256]),
		binary.BigEndian.Uint32(storedSchedule[:4]), rates, callPayload, big.NewInt(0),
	}
	return args, request, func() {
		t.Helper()
		main, err := h.host.ReadAccount(h.stateDB.Ctx(), mainName, mainID)
		require.NoError(t, err)
		sequence := main.NextSequence
		store = h.stateDB.Ctx().KVStore(h.key)
		seedAccount(mainID, mainName, 1, 100000000, mainAsset, 1, sequence, h.publicKey, 1)
		main, err = h.host.ReadAccount(h.stateDB.Ctx(), mainName, mainID)
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
	recorded := recordedSandboxLeaseRequest(t)
	recordedRequest := decodeSandboxLeaseRequest(t, recorded)
	program := recordedRequest.Program
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
	callArgs[0], callArgs[1], callArgs[4] = program, uint16(3), []byte{0, 2, 1, 2}
	access, budget := callArgs[5].([]byte), callArgs[7].([7]uint64)
	callPayload, err := lp.EncodeCallPayload(program, 3, "layerx_call", []byte{0, 0xff}, callArgs[4].([]byte), access, 64, budget)
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

	args, request, fundFeeLimit := prepareSandboxExecute(t, h, callArgs, callPayload,
		recordedRequest.LeaseID, recordedRequest.ActivityID, recordedRequest.ReceiptDigest)
	require.Equal(t, recorded, request, "the request codec must match the committed native submission byte for byte")
	decodedRequest := decodeSandboxLeaseRequest(t, request)
	require.Equal(t, program, decodedRequest.Program)
	require.Equal(t, recordedRequest.Asset, decodedRequest.Asset)
	require.Equal(t, recordedRequest.Limits, decodedRequest.Limits)
	require.Equal(t, recordedRequest.ActivityID, decodedRequest.ActivityID)
	require.Equal(t, recordedRequest.ReceiptDigest, decodedRequest.ReceiptDigest)
	roundTrip, err := lp.EncodeSandboxLeaseRequest(decodedRequest)
	require.NoError(t, err)
	require.Equal(t, recorded, roundTrip, "the recording must round-trip through the request codec byte for byte")
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
	plainDecoded, err := lp.DecodeProgramExecutionResult(plainData, callArgs[6].(uint32), [32]byte{})
	require.NoError(t, err)
	require.Empty(t, plainDecoded.GuestResponse, "the deployed guest returns zero without calling response_write")
	h.stateDB.RevertToSnapshot(snapshot)
	require.Equal(t, beforePlainCall, h.dump(h.stateDB.Ctx()))
	require.Equal(t, beforePlainLogs, h.stateDB.GetAllLogs())
	t.Log("sandbox assertion ladder: identical ABI-v3 payload settled through the registered call handler")

	payerName, payerID, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	payerBefore, err := h.host.ReadAccount(h.stateDB.Ctx(), payerName, payerID)
	require.NoError(t, err)
	_, did, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	last, err := h.host.ReadLastSequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.NotEqual(t, last+1, payerBefore.NextSequence, "ledger admission must use the payer sequence independently of the legacy global sequence")

	payload, err = lp.EncodeSandboxExecutePayload(args[0].([32]byte), args[1].(uint64), args[2].([32]byte),
		args[3].([32]byte), args[4].([32]byte), args[5].([32]byte), args[6].(uint32), args[7].([7]uint64), args[8].([]byte))
	require.NoError(t, err)
	input, err := lp.ProgramsABI.Pack(method.Name, args...)
	require.NoError(t, err)
	prepared, preparedMethod, wireArgs, err := h.precompile.Prepare(h.evm, input)
	require.NoError(t, err)
	require.Equal(t, method.ID, preparedMethod.ID)
	// Prepare obtains the same state DB context used by registered dispatch.
	// Run supplies a fresh event manager and the delegate-call flag before Execute.
	prepared = prepared.WithEventManager(sdk.NewEventManager()).WithEVMPrecompileCalledFromDelegateCall(false)
	reference, _ := prepared.CacheContext()
	txHash := [32]byte(reference.EVMTxHash())
	require.NotEqual(t, [32]byte{}, txHash, "the native reference must retain the EVM transaction hash")
	var transient *sdk.TransientStoreKey
	for _, key := range reference.MultiStore().StoreKeys() {
		if key.Name() == evmtypes.TransientStoreKey {
			var ok bool
			transient, ok = key.(*sdk.TransientStoreKey)
			require.True(t, ok)
			break
		}
	}
	require.NotNil(t, transient, "the native reference must mount the EVM transient store")
	ordinalProbe, _ := reference.CacheContext()
	ordinal := lp.NextOrdinal(ordinalProbe, txHash)
	decodedArgs, err := lp.DecodeArgs(&method, wireArgs)
	require.NoError(t, err)
	handlerPayload, err := lp.EncodeSandboxExecutePayload(
		decodedArgs[0].([32]byte), decodedArgs[1].(uint64), decodedArgs[2].([32]byte),
		decodedArgs[3].([32]byte), decodedArgs[4].([32]byte), decodedArgs[5].([32]byte),
		decodedArgs[6].(uint32), decodedArgs[7].([7]uint64), decodedArgs[8].([]byte))
	require.NoError(t, err)
	require.Equal(t, payload, handlerPayload, "the native reference must submit exactly what the registered sandboxExecute handler decodes")
	require.Equal(t, []byte{1, 1, 0, 0}, handlerPayload[:4])
	active, err := h.host.ReadSandbox(reference, leaseID, 0)
	require.NoError(t, err)
	leaseDigest := sha256.Sum256(active)
	require.Equal(t, leaseID[:], handlerPayload[4:36])
	require.Equal(t, leaseDigest[:], handlerPayload[44:76])
	terms := active[len(sandboxLeaseStateDomain):]
	require.Equal(t, terms[192:224], handlerPayload[76:108], "the request escrow must match the active lease in the reference context")
	require.Equal(t, terms[160:192], handlerPayload[108:140])
	require.Equal(t, terms[224:256], handlerPayload[140:172])
	require.Equal(t, terms[(8*32+16+10*8)+64:(8*32+16+10*8)+64+60], handlerPayload[172:232])
	require.Equal(t, callPayload, handlerPayload[236:])
	feeLimit := decodedArgs[9].([16]byte)
	receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method), 0x00090009, handlerPayload, feeLimit)
	ordinalKey := append([]byte("layerxprograms/ordinal/v1/"), txHash[:]...)
	assigned := reference.TransientStore(transient).Get(ordinalKey)
	require.Len(t, assigned, 4, "Submit must assign the call ordinal before native execution")
	require.Equal(t, ordinal, binary.BigEndian.Uint32(assigned))
	t.Logf("sandbox reference context: EVM transaction %x, transient store %s, assigned call ordinal %d", txHash, transient.Name(), ordinal)
	require.NoError(t, err, "independent native sandbox submission")
	require.Zero(t, receipt.ResultCode)
	transferRoot := repair251TransferRoot(t, receipt)
	require.NotEqual(t, [32]byte{}, transferRoot)

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
	decoded, err := lp.DecodeProgramExecutionResult(data, callArgs[6].(uint32), transferRoot)
	require.NoError(t, err)
	require.Empty(t, decoded.GuestResponse, "the deployed guest returns zero without calling response_write")
	require.Equal(t, plainDecoded.GuestResponse, decoded.GuestResponse)
	require.Equal(t, output[2], decoded.TerminalFrame)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	payerAfter, err := h.host.ReadAccount(h.stateDB.Ctx(), payerName, payerID)
	require.NoError(t, err)
	require.Equal(t, payerBefore.NextSequence+1, payerAfter.NextSequence, "sandbox execution must advance the sequence captured by ledger admission")
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
	require.Equal(t, usage, decoded.SandboxUsageReceipt)
	ledger, err := h.host.ReadSandbox(h.stateDB.Ctx(), leaseID, 2)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(ledger), string(usage)), "the committed ledger must contain the emitted native usage frame")
	// Capture the real producer through its ABI and protobuf transport encoders.
	response := evmtypes.MsgEVMTransactionResponse{ReturnData: data}
	responseBytes, err := response.Marshal()
	require.NoError(t, err)
	messages := sdk.TxMsgData{Data: []*sdk.MsgData{{MsgType: sdk.MsgTypeURL(&evmtypes.MsgEVMTransaction{}), Data: responseBytes}}}
	resultBytes, err := messages.Marshal()
	require.NoError(t, err)
	t.Logf("repair_m213_execute_result=%x", resultBytes)
	t.Logf("repair_m213_usage_ledger=%x", ledger)
	recordedResult, err := hex.DecodeString(strings.TrimSpace(repairM213ExecuteResultHex))
	require.NoError(t, err)
	require.Equal(t, recordedResult, resultBytes, "the consumer fixture must be the production-encoded live EVM result")
	recordedLedger, err := hex.DecodeString(strings.TrimSpace(repairM213UsageLedgerHex))
	require.NoError(t, err)
	require.Equal(t, recordedLedger, ledger, "the independently recorded ledger must match the live host state")
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
