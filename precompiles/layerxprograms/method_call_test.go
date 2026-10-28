package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func programsCallFixture(t *testing.T) (*harness, []interface{}, []byte) {
	t.Helper()
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
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
	main = binary.BigEndian.AppendUint64(main, 100000000)
	mainAsset := [32]byte{0x91}
	main = field(main, mainAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, h.publicKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	mainAccount, err := h.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, int64(100000000), mainAccount.Balance.Int64())

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

	section := func(id byte, body []byte) []byte {
		return append([]byte{id, byte(len(body))}, body...)
	}
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0}
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
	program := [32]byte{0x71}
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = append(iface, 4, 0, 0, 0, 0)
	deploy, err := lp.EncodeDeployPayload(program, 1, 1, authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	execute(0x00090001, deploy)

	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	capabilities := []byte{0, 1, 3}
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	args := []interface{}{program, uint16(1), "layerx_call", []byte{0, 0xff}, capabilities, access, uint32(64), budget, big.NewInt(100000000)}
	payload, err := lp.EncodeCallPayload(program, 1, "layerx_call", args[3].([]byte), capabilities, access, 64, budget)
	require.NoError(t, err)
	return h, args, payload
}

func TestProgramsCallReturnsTerminalOutcome(t *testing.T) {
	h, args, payload := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	reference, _ := ctx.CacheContext()
	method := lp.ProgramsABI.Methods["call"]
	var limit [16]byte
	args[8].(*big.Int).FillBytes(limit[:])
	receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method), 0x00090003, payload, limit)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.NotNil(t, receipt.ProgramOutcome)
	require.Zero(t, receipt.ProgramOutcome.Status)
	require.Equal(t, uint8(1), receipt.ProgramOutcome.Kind)
	require.Positive(t, receipt.FeeCharged.Sign())
	input, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 20000000, uint256.NewInt(0))
	require.NoError(t, err)
	expected, err := method.Outputs.Pack(receipt.ProgramOutcome.Status, receipt.ProgramOutcome.Kind, receipt.ProgramOutcome.Response)
	require.NoError(t, err)
	require.Equal(t, expected, output)
	values, err := method.Outputs.Unpack(output)
	require.NoError(t, err)
	require.Equal(t, []interface{}{receipt.ProgramOutcome.Status, receipt.ProgramOutcome.Kind, receipt.ProgramOutcome.Response}, values)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 2)
	for i, name := range []string{"ProgramGuestEvent", "ProgramCallOutcome"} {
		require.Equal(t, h.precompile.Address(), logs[i].Address)
		require.Equal(t, []common.Hash{lp.ProgramsABI.Events[name].ID, common.Hash(args[0].([32]byte))}, logs[i].Topics)
		require.Equal(t, uint(i), logs[i].Index)
	}
	guest, err := lp.ProgramsABI.Events["ProgramGuestEvent"].Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	require.Equal(t, authority.Principal, guest[0])
	require.NotEqual(t, [32]byte{}, guest[1])
	require.Equal(t, uint64(0), guest[2])
	require.Equal(t, uint8(0), guest[3])
	require.Equal(t, uint32(0), guest[4])
	require.Equal(t, uint16(1), guest[5])
	require.Equal(t, sha256.Sum256([]byte("LayerX/programs/event-topic/v1t")), guest[6])
	require.Equal(t, uint32(1), guest[7])
	require.Equal(t, sha256.Sum256([]byte("LayerX/programs/event-data/v1d")), guest[8])
	terminal, err := lp.ProgramsABI.Events["ProgramCallOutcome"].Inputs.NonIndexed().Unpack(logs[1].Data)
	require.NoError(t, err)
	require.Equal(t, guest[:4], terminal[:4])
	require.Equal(t, args[1], terminal[4])
	require.Equal(t, uint32(receipt.ProgramOutcome.FeeScheduleVersion), terminal[6])
	require.Equal(t, uint32(receipt.ProgramOutcome.MeteringScheduleVersion), terminal[7])
	require.Equal(t, receipt.ProgramOutcome.Status, terminal[8])
}

func TestProgramsCallFeeLimitBelowFeeReverts(t *testing.T) {
	h, args, _ := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	fee, err := h.host.ReadFeeSchedule(ctx)
	require.NoError(t, err)
	ceiling := new(big.Int)
	budget := args[7].([7]uint64)
	for i := 0; i < 6; i++ {
		price := binary.BigEndian.Uint64(fee[9+i*8 : 17+i*8])
		ceiling.Add(ceiling, new(big.Int).Mul(new(big.Int).SetUint64(price), new(big.Int).SetUint64(budget[i])))
	}
	require.Positive(t, ceiling.Sign())
	before := h.dump(ctx)
	asset := [32]byte(fee[65:97])
	_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	balanceBefore, err := h.host.ReadBalance(ctx, payer, asset)
	require.NoError(t, err)
	require.Positive(t, balanceBefore.Sign())
	mainName, mainID, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	mainBefore, err := h.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	for _, limit := range []*big.Int{big.NewInt(0), big.NewInt(1), new(big.Int).Sub(ceiling, big.NewInt(1))} {
		t.Run(limit.String(), func(t *testing.T) {
			args[8] = limit
			require.Negative(t, limit.Cmp(ceiling))
			require.GreaterOrEqual(t, mainBefore.Balance.BigInt().Cmp(limit), 0)
			input, err := lp.ProgramsABI.Pack("call", args...)
			require.NoError(t, err)
			data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 20000000, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(data)
			require.NoError(t, err)
			require.Equal(t, lp.KernelError(lxphost.Code(-207), "call").Error(), reason)
			require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
			require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-207))
			require.Equal(t, before, h.dump(h.stateDB.Ctx()))
			balanceAfter, err := h.host.ReadBalance(h.stateDB.Ctx(), payer, asset)
			require.NoError(t, err)
			require.Equal(t, balanceBefore, balanceAfter)
			mainAfter, err := h.host.ReadAccount(h.stateDB.Ctx(), mainName, mainID)
			require.NoError(t, err)
			require.Equal(t, mainBefore, mainAfter)
			require.Empty(t, h.stateDB.GetAllLogs())
		})
	}
}
