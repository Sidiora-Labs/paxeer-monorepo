package layerxprograms

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestWebReaderExampleRoundTripThroughXWeb(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	build := exec.Command("sh", filepath.Join(root, "programs/sdk/rust/examples/web-reader/build.sh"))
	build.Dir = root
	build.Env = append(os.Environ(), "CARGO_TARGET_DIR="+filepath.Join(root, "programs/target"))
	built, err := build.CombinedOutput()
	require.NoError(t, err, "%s", built)
	lines := strings.Split(strings.TrimSpace(string(built)), "\n")
	wasm, err := os.ReadFile(lines[len(lines)-1])
	require.NoError(t, err)
	require.NotEmpty(t, wasm)
	h := newWebCallHarness(t)
	ctx := h.ctx
	var paramsKey, transientKey sdk.StoreKey
	for _, key := range ctx.MultiStore().StoreKeys() {
		switch key.Name() {
		case "params":
			paramsKey = key
		case evmtypes.TransientStoreKey:
			transientKey = key
		}
	}
	require.NotNil(t, paramsKey)
	require.NotNil(t, transientKey)
	accounts := h.executor.evmKeeper.AccountKeeper()
	cdc := accounts.GetCodec()
	*accounts = authkeeper.NewAccountKeeper(cdc, h.authKey,
		paramtypes.NewSubspace(cdc, sdkcodec.NewLegacyAmino(), paramsKey, transientKey, "auth"),
		authtypes.ProtoBaseAccount, map[string][]string{authtypes.FeeCollectorName: nil})
	collector := accounts.GetModuleAccount(ctx, authtypes.FeeCollectorName)
	require.NotNil(t, collector)
	h.executor.evmKeeper.SetAddressMapping(ctx, collector.GetAddress(), evmkeeper.GetCoinbaseAddress())
	authority, did, err := PrincipalOf(ctx, h.executor.evmKeeper, h.caller)
	require.NoError(t, err)
	moduleKey := func(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }
	store := ctx.KVStore(h.key)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Actor[:]...)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)

	execute := func(activityType uint32, payload []byte) {
		t.Helper()
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := h.host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), h.host, authority, activityType, payload, sequence)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	salt := bytes.Repeat([]byte{0x55}, 32)
	assetMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(assetMaterial, salt...))
	register := append([]byte{0, 1}, asset[:]...)
	register = append(register, salt...)
	register = append(register, 1, 'T', 1, 'T', 6)
	register = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(register, 0), 1000000000)
	execute(0x00010001, append(register, 1, 0))
	execute(0x00010004, append([]byte{0, 1}, asset[:]...))
	_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, payer[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	execute(0x0001000a, mint)

	mainName, mainID, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	mainBalance := uint64(500000000)
	mainAsset := asset
	store.Delete(append([]byte{6}, payer[:]...))
	main = binary.BigEndian.AppendUint64(main, mainBalance)
	main = field(main, mainAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, h.publicKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	mainAccount, err := h.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, mainBalance, mainAccount.Balance.Uint64())

	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	treasury := field(nil, treasuryID[:])
	treasury = field(treasury, []byte("system:fees"))
	treasury = append(treasury, 10)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = field(treasury, asset[:])
	treasury = append(treasury, 1)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = append(treasury, 0, 0)
	treasury = field(treasury, make([]byte, 32))
	store.Set(append([]byte{6}, treasuryID[:]...), append(treasury, 0))
	treasuryAccount, err := h.host.ReadAccount(ctx, "system:fees", treasuryID)
	require.NoError(t, err)
	require.False(t, treasuryAccount.Frozen)
	require.True(t, treasuryAccount.Balance.IsZero())
	require.Equal(t, asset, treasuryAccount.AssetID)

	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

	const feeAmount uint64 = 75
	const requestID uint64 = 257
	program := [32]byte{0x73}
	destination := xwebkeeper.ProgramFeeAccountID(xwebtypes.Hash32(asset))
	ctx.KVStore(h.webKey).Set(xwebkeeper.ProgramFeeDenomKey(xwebtypes.Hash32(asset)), []byte(xwebtypes.ProgramFeeDenomPAX))
	keys := make([]*ecdsa.PrivateKey, 3)
	for i := range keys {
		seed := make([]byte, 32)
		seed[0], seed[31] = 0x3c, byte(i+1)
		keys[i], err = crypto.ToECDSA(seed)
		require.NoError(t, err)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := crypto.PubkeyToAddress(keys[i].PublicKey), crypto.PubkeyToAddress(keys[j].PublicKey)
		return bytes.Compare(a[:], b[:]) < 0
	})
	genesis := h.web.ExportGenesis(ctx)
	genesis.Attestors = xwebtypes.AttestorSet{Threshold: 2}
	payouts := make([]sdk.AccAddress, len(keys))
	for i, key := range keys {
		signer := xwebtypes.Address20(crypto.PubkeyToAddress(key.PublicKey))
		payouts[i] = sdk.AccAddress(signer[:])
		genesis.Attestors.Attestors = append(genesis.Attestors.Attestors, xwebtypes.Attestor{
			Signer: signer, Payout: payouts[i].String(), PublicKey: crypto.CompressPubkey(&key.PublicKey),
		})
	}
	require.NoError(t, genesis.Attestors.Validate())
	h.web.InitGenesis(ctx, genesis)
	require.True(t, h.web.ProgramOriginActive(ctx))

	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v4\x00"), codeHash[:]...)
	iface = binary.BigEndian.AppendUint16(iface, 4)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = binary.BigEndian.AppendUint16(iface, uint16(len("layerx_call")))
	iface = append(iface, "layerx_call"...)
	iface = binary.BigEndian.AppendUint32(iface, 0)
	iface = append(iface, 1, 0x20)
	iface = binary.BigEndian.AppendUint32(iface, 4096)
	iface = append(iface, 1, 0x20)
	iface = binary.BigEndian.AppendUint32(iface, 64)
	iface = binary.BigEndian.AppendUint16(iface, 3)
	iface = append(iface, 4, 6)
	iface = append(iface, asset[:]...)
	iface = append(iface, destination[:]...)
	iface = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(iface, 0), feeAmount)
	iface = append(iface, 12, 0, 0, 0, 0)
	deploy, err := EncodeDeployPayload(program, 4, 1, authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	execute(0x00090001, deploy)

	bank := h.executor.evmKeeper.BankKeeper()
	principal := lxphost.BankAddress(mainID)
	denom := xwebtypes.ProgramFeeDenomPAX
	feeAccount := xwebkeeper.ProgramFeeAccount(xwebtypes.Hash32(asset))
	require.NoError(t, bank.AddCoins(ctx, principal, sdk.NewCoins(sdk.NewInt64Coin(denom, 100)), true))
	require.True(t, bank.GetBalance(ctx, feeAccount, denom).Amount.IsZero())

	precompile := pcommon.NewPrecompile(programsABI, h.executor, Address, PrecompileName).
		WithLXPReasons().WithGasTable(gasTable)
	require.Equal(t, common.HexToAddress("0x101d"), precompile.Address())
	statedb := state.GetDBImpl(h.evm.StateDB)
	block, err := h.executor.evmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(h.executor.evmKeeper.ChainID(ctx))
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	capabilities := []byte{0, 2, 3, 5}
	capabilities = append(capabilities, asset[:]...)
	capabilities = append(capabilities, destination[:]...)
	capabilities = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(capabilities, 0), feeAmount)
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	invoke := func(t *testing.T, input []byte, grants []byte) ([]byte, error) {
		t.Helper()
		statedb = state.NewDBImpl(ctx, h.executor.evmKeeper, false)
		t.Cleanup(statedb.Cleanup)
		h.evm = vm.NewEVM(*block, statedb, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{Address: precompile})
		encoded, err := programsABI.Pack("call", program, uint16(4), "layerx_call", input, grants, access, uint32(64), budget, big.NewInt(100000000))
		require.NoError(t, err)
		output, _, err := h.evm.Call(h.caller, Address, encoded, 20000000, uint256.NewInt(0))
		return output, err
	}
	call := func(t *testing.T, input []byte, grants []byte) ([]interface{}, []byte, []byte) {
		t.Helper()
		output, err := invoke(t, input, grants)
		if err != nil {
			reason, decodeErr := abi.UnpackRevert(output)
			require.NoError(t, decodeErr, "precompile output: %x", output)
			require.NoError(t, err, "precompile revert: %s", reason)
		}
		require.NoError(t, err, "precompile output: %x", output)
		values, err := programsABI.Methods["call"].Outputs.Unpack(output)
		require.NoError(t, err)
		guest, legs := webReaderTerminal(t, values[2].([]byte), program, values[1].(uint8))
		ctx = statedb.Ctx()
		return values, guest, legs
	}

	payload := []byte("paxeer web-reader round trip")
	request := binary.BigEndian.AppendUint64([]byte{1, 1}, requestID)
	request = append(request, xwebtypes.KindSearch)
	request = append(request, asset[:]...)
	request = append(request, destination[:]...)
	request = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(request, 0), feeAmount)
	request = append(request, payload...)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	result, guest, legs := call(t, request, capabilities)
	require.Equal(t, int32(0), result[0])
	require.Equal(t, uint8(1), result[1])
	require.Empty(t, guest)
	expectedLeg := append([]byte{0}, mainID[:]...)
	expectedLeg = append(expectedLeg, destination[:]...)
	expectedLeg = append(expectedLeg, asset[:]...)
	expectedLeg = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(expectedLeg, 0), feeAmount)
	expectedLeg = append(expectedLeg, 0, 1)
	require.Equal(t, expectedLeg, legs)
	ctx = statedb.Ctx()
	record, found, err := h.web.GetProgramPendingRecord(ctx, xwebtypes.Hash32(program), requestID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, xwebtypes.KindSearch, record.Kind)
	require.Equal(t, xwebtypes.Keccak(payload), record.PayloadHash)
	require.Equal(t, sequence, record.Sequence)
	require.Equal(t, xwebtypes.Hash32(asset), record.FeeAsset)
	require.Equal(t, destination, record.FeeAccount)
	require.True(t, sdk.NewIntFromUint64(feeAmount).Equal(record.Amount))
	require.False(t, record.Fulfilled)
	require.Equal(t, uint64(1), h.web.ProgramPendingCount(ctx))
	require.True(t, sdk.NewInt(25).Equal(bank.GetBalance(ctx, principal, denom).Amount))
	require.True(t, sdk.NewIntFromUint64(feeAmount).Equal(bank.GetBalance(ctx, feeAccount, denom).Amount))
	logs := statedb.GetAllLogs()
	require.Len(t, logs, 2)
	require.Equal(t, Address, logs[0].Address)
	require.Equal(t, programsABI.Events["ProgramGuestEvent"].ID, logs[0].Topics[0])
	event, err := programsABI.Events["ProgramGuestEvent"].Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(append([]byte("LayerX/programs/event-topic/v1"), []byte(xwebkeeper.ProgramRequestTopic)...)), event[6])
	eventData := binary.BigEndian.AppendUint64(nil, requestID)
	eventData = append(eventData, xwebtypes.KindSearch)
	eventData = binary.BigEndian.AppendUint32(eventData, uint32(len(payload)))
	eventData = append(eventData, payload...)
	require.Equal(t, uint32(len(eventData)), event[7])
	require.Equal(t, sha256.Sum256(append([]byte("LayerX/programs/event-data/v1"), eventData...)), event[8])

	response := []byte("the committed web answer")
	observation := xwebtypes.Observation{Origin: xwebtypes.OriginProgram, NetworkID: 7,
		ProgramID: xwebtypes.Hash32(program), RequestID: requestID, Kind: record.Kind,
		PayloadHash: record.PayloadHash, ContentDigest: xwebtypes.Hash32(sha256.Sum256(response)),
		FullLength: uint32(len(response)), Response: response}
	digest := xwebtypes.Digest(xwebtypes.Attestation{Origin: observation.Origin, NetworkID: big.NewInt(7),
		Requester: observation.ProgramID, RequestID: observation.RequestID, Kind: observation.Kind,
		PayloadHash: observation.PayloadHash, ContentDigest: observation.ContentDigest,
		ResponseHash: xwebtypes.Keccak(response), FullLength: observation.FullLength})
	for _, key := range keys[:2] {
		signature, err := crypto.Sign(digest[:], key)
		require.NoError(t, err)
		signature[64] += 27
		observation.Signatures = append(observation.Signatures, signature)
	}
	raw, err := xwebtypes.EncodeObservation(observation)
	require.NoError(t, err)
	require.NoError(t, h.web.FulfilProgram(ctx, raw))
	record, found, err = h.web.GetProgramPendingRecord(ctx, observation.ProgramID, requestID)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, record.Fulfilled)
	require.Zero(t, h.web.ProgramPendingCount(ctx))
	require.Equal(t, uint64(1), h.web.ProgramCommittedCount(ctx))
	header := append([]byte(nil), observation.ContentDigest[:]...)
	header = binary.BigEndian.AppendUint32(header, observation.FullLength)
	header = binary.BigEndian.AppendUint32(header, uint32(len(response)))
	require.Equal(t, header, ctx.KVStore(h.webKey).Get(xwebkeeper.ProgramAnswerKey(observation.ProgramID, requestID, 0)))
	require.Equal(t, response, ctx.KVStore(h.webKey).Get(xwebkeeper.ProgramAnswerKey(observation.ProgramID, requestID, 1)))
	assertCommittedAnswer := func(t *testing.T, ctx sdk.Context) {
		t.Helper()
		answer, err := h.web.GetProgramResult(ctx, xwebtypes.Hash32(program), requestID)
		require.NoError(t, err)
		require.Equal(t, xwebkeeper.ProgramResult{
			ContentDigest: observation.ContentDigest,
			FullLength:    observation.FullLength, ResponseLength: uint32(len(response)), Response: response,
		}, answer)
		require.Equal(t, []xwebtypes.ProgramAnswerEntry{
			{ProgramID: xwebtypes.Hash32(program), RequestID: requestID, Part: 0, Value: header},
			{ProgramID: xwebtypes.Hash32(program), RequestID: requestID, Part: 1, Value: response},
		}, h.web.ProgramAnswerEntries(ctx))
		require.Zero(t, h.web.ProgramPendingCount(ctx))
		require.Equal(t, uint64(1), h.web.ProgramCommittedCount(ctx))
	}
	assertCommittedAnswer(t, ctx)
	require.True(t, bank.GetBalance(ctx, feeAccount, denom).Amount.IsZero())
	require.True(t, sdk.NewInt(38).Equal(bank.GetBalance(ctx, payouts[0], denom).Amount))
	require.True(t, sdk.NewInt(37).Equal(bank.GetBalance(ctx, payouts[1], denom).Amount))

	read := binary.BigEndian.AppendUint64([]byte{1, 2}, requestID)
	read = append(read, observation.ContentDigest[:]...)
	read = binary.BigEndian.AppendUint32(read, observation.FullLength)
	read = append(read, response...)
	assertReadReverts := func(t *testing.T, input []byte) {
		t.Helper()
		output, err := invoke(t, input, []byte{0, 0})
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(output)
		require.NoError(t, err)
		require.Equal(t, "LXP_NON_CANONICAL: call: LXP_ERR_NON_CANONICAL", reason)
		require.Empty(t, statedb.GetAllLogs())
		assertCommittedAnswer(t, statedb.Ctx())
	}
	t.Run("committed_answer", func(t *testing.T) {
		assertReadReverts(t, read)
	})
	t.Run("different_digest", func(t *testing.T) {
		wrong := bytes.Clone(read)
		wrong[10] ^= 1
		assertReadReverts(t, wrong)
		require.Equal(t, header, statedb.Ctx().KVStore(h.webKey).Get(xwebkeeper.ProgramAnswerKey(observation.ProgramID, requestID, 0)))
		require.Equal(t, response, statedb.Ctx().KVStore(h.webKey).Get(xwebkeeper.ProgramAnswerKey(observation.ProgramID, requestID, 1)))
	})
}

func webReaderTerminal(t *testing.T, encoded []byte, program [32]byte, kind uint8) ([]byte, []byte) {
	t.Helper()
	remaining := encoded
	take := func(length uint64) []byte {
		t.Helper()
		require.LessOrEqual(t, length, uint64(len(remaining)))
		value := remaining[:int(length)]
		remaining = remaining[int(length):]
		return value
	}
	sized32 := func() []byte {
		t.Helper()
		return take(uint64(binary.BigEndian.Uint32(take(4))))
	}
	sized64 := func() []byte {
		t.Helper()
		return take(binary.BigEndian.Uint64(take(8)))
	}
	domain := []byte("LXP/programs/terminal-applied-legs/v1\x00")
	require.Equal(t, domain, take(uint64(len(domain))))
	detail, legs := sized32(), sized32()
	require.Empty(t, remaining)
	remaining = detail
	authorityDomain := []byte("LXP/program-execution-with-transfer-authority/v2\x00")
	if bytes.HasPrefix(remaining, authorityDomain) {
		take(uint64(len(authorityDomain)))
		inner := sized32()
		require.NotEmpty(t, sized32())
		root := take(32)
		if len(legs) == 0 {
			require.Equal(t, make([]byte, 32), root)
		} else {
			require.Len(t, legs, 115)
			leaf := sha256.Sum256(append([]byte("LXP/v1/merkle-leaf\x00"), legs...))
			require.Equal(t, leaf[:], root)
		}
		require.Empty(t, remaining)
		remaining = inner
	}
	occupancyDomain := []byte("LXP/program-execution-with-occupancy/v1\x00")
	if bytes.HasPrefix(remaining, occupancyDomain) {
		take(uint64(len(occupancyDomain)))
		inner := sized32()
		require.NotEmpty(t, sized32())
		require.Empty(t, remaining)
		remaining = inner
	}
	executionDomain := []byte("LXP/program-execution/v5\x00")
	require.Equal(t, executionDomain, take(uint64(len(executionDomain))))
	require.NotZero(t, binary.BigEndian.Uint16(take(2)))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(take(4)))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(take(4)))
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(take(8)))
	require.Equal(t, byte(1), take(1)[0])
	code := int32(binary.BigEndian.Uint32(take(4)))
	if kind == 1 {
		require.Zero(t, code)
	} else {
		require.Equal(t, int32(-2), code)
	}
	take(4*8 + 4 + 8 + 16)
	trace := take(1)[0]
	require.LessOrEqual(t, trace, byte(1))
	if trace == 1 {
		require.NotEmpty(t, sized64())
	}
	require.Equal(t, program[:], take(32))
	require.Equal(t, uint16(4), binary.BigEndian.Uint16(take(2)))
	var guest []byte
	switch tag := take(1)[0]; tag {
	case 0:
		require.Equal(t, uint8(1), kind)
		require.Zero(t, binary.BigEndian.Uint32(take(4)))
		guest = sized64()
	case 1:
		require.Equal(t, uint8(2), kind)
		failure := binary.BigEndian.AppendUint32(bytes.Clone(program[:]), 255)
		failure = binary.BigEndian.AppendUint32(failure, 0)
		require.Equal(t, failure, sized64())
	default:
		t.Fatalf("unexpected execution outcome tag %d", tag)
	}
	graph := sized64()
	require.True(t, bytes.HasPrefix(graph, []byte("LayerX/programs/call-graph/v1\x00")))
	require.Empty(t, remaining)
	return guest, legs
}
