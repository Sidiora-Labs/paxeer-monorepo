package streams_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type streamOutput struct {
	Id                  *big.Int
	Payer, Payee, Token common.Address
	RatePerSecond, Cap  *big.Int
	StartTime, StopTime uint64
	Settled             *big.Int
	Active              bool
}

func TestStreamsOpenThroughPayKeeper(t *testing.T) {
	h := newStreamsHarness(t)
	require.Equal(t, common.HexToAddress("0x0906"), h.precompile.Address())
	require.Equal(t, "streams", streams.PrecompileName)
	a := h.precompile.GetABI()
	for name, selector := range map[string]string{"open": "93e5b08b", "settle": "8df82800", "close": "0aebeb4e", "updateRate": "405abb41", "accrued": "f52981f4", "getStream": "894e9a0d"} {
		require.Equal(t, common.FromHex(selector), a.Methods[name].ID)
	}
	id := h.open()
	_, payerAccount, err := lxphost.MainAccountID(h.payer.public)
	require.NoError(t, err)
	expected := paytypes.DeriveStreamID([32]byte(h.db.Ctx().EVMTxHash()), 0, payerAccount)
	require.Equal(t, new(big.Int).SetBytes(expected[:]), id)
	require.Greater(t, id.BitLen(), 160)
	r := h.record(id)
	require.Equal(t, expected, r.StreamID)
	require.Equal(t, payerAccount, r.Payer)
	require.Equal(t, h.payee.account, r.Recipient)
	require.Equal(t, h.asset, r.AssetID)
	require.Equal(t, paytypes.StreamModeTime, r.Mode)
	require.Equal(t, codec.U128{Lo: 2}, r.Rate)
	require.Equal(t, uint64(1000), r.RateUnit)
	require.Equal(t, streamsStart*1000, r.StartTimestamp)
	require.Equal(t, (streamsStart+100)*1000, r.EndTimestamp)
	require.Equal(t, codec.U128{Lo: 100}, r.TotalCap)
	require.Len(t, h.db.GetAllLogs(), 1)
	h.log(0, "StreamOpened", "0xc87f2fb1c03b6bebafb5b0fa1b73c2f3665342c2fdc395c1a9df98a364ef6ed1", []common.Hash{common.BigToHash(id), common.BytesToHash(h.payer.caller.Bytes()), common.BytesToHash(h.payee.caller.Bytes())}, h.token, big.NewInt(2), big.NewInt(100))
	for account, expectedBalance := range map[[32]byte]string{r.Payer: "900", r.StreamAccount: "100", r.Recipient: "0"} {
		balance, err := h.keeper.Host().ReadBalance(h.db.Ctx(), account, h.asset)
		require.NoError(t, err)
		require.Equal(t, expectedBalance, balance.String())
	}
	pay, err := lp.NewPrecompile(h.keeper)
	require.NoError(t, err)
	input, err := pay.GetABI().Pack("getStream", r.StreamID)
	require.NoError(t, err)
	out, err := pay.Run(h.evm, h.other.caller, pay.Address(), input, new(big.Int), true, false, nil)
	require.NoError(t, err)
	expectedABI, err := pay.GetABI().Methods["getStream"].Outputs.Pack(r.ABITuple())
	require.NoError(t, err)
	require.Equal(t, expectedABI, out)
	second := h.open()
	require.NotEqual(t, id, second)
	require.Equal(t, new(big.Int).SetBytes(paytypesID(h, 1)), second)
}

func paytypesID(h *streamsHarness, ordinal uint32) []byte {
	id := paytypes.DeriveStreamID([32]byte(h.db.Ctx().EVMTxHash()), ordinal, h.payer.account)
	return id[:]
}

func TestStreamsSettleCloseAuthority(t *testing.T) {
	h := newStreamsHarness(t)
	id := h.open()
	h.advance(5)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	out, err := h.call(h.other.caller, "settle", id)
	requireStreamsRevert(t, out, err, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	require.Equal(t, []interface{}{big.NewInt(10)}, h.write(h.payee.caller, "settle", id))
	h.log(logs, "StreamSettled", "0x0ad296c73d151b1f325c282f0de78c0efb4588bbd70ea579b30b25a16f62917a", []common.Hash{common.BigToHash(id), common.BytesToHash(h.payee.caller.Bytes())}, big.NewInt(10), big.NewInt(10))
	h.advance(8)
	require.Equal(t, []interface{}{big.NewInt(6)}, h.write(h.payer.caller, "settle", id))
	h.log(logs+1, "StreamSettled", "0x0ad296c73d151b1f325c282f0de78c0efb4588bbd70ea579b30b25a16f62917a", []common.Hash{common.BigToHash(id), common.BytesToHash(h.payee.caller.Bytes())}, big.NewInt(6), big.NewInt(6))
	h.advance(10)
	before = h.keeper.ExportGenesis(h.db.Ctx())
	logs = len(h.db.GetAllLogs())
	for _, caller := range []common.Address{h.payee.caller, h.other.caller} {
		out, err := h.call(caller, "close", id)
		requireStreamsRevert(t, out, err, "LXP_UNAUTHORIZED_DEBIT")
		require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
		require.Len(t, h.db.GetAllLogs(), logs)
	}
	require.Equal(t, []interface{}{big.NewInt(4)}, h.write(h.payer.caller, "close", id))
	h.log(logs, "StreamClosed", "0xbd03ee89416a2d44203c48d8655c11fea0add5437f20b8026356e9a22481aca0", []common.Hash{common.BigToHash(id)}, big.NewInt(4), big.NewInt(80))
	r := h.record(id)
	require.True(t, r.Closed)
	require.Equal(t, codec.U128{Lo: 20}, r.SettledTotal)
	for account, want := range map[[32]byte]string{r.Payer: "980", r.Recipient: "20", r.StreamAccount: "0"} {
		balance, err := h.keeper.Host().ReadBalance(h.db.Ctx(), account, h.asset)
		require.NoError(t, err)
		require.Equal(t, want, balance.String())
	}
}

func TestStreamsUpdateRateLogsBothAddresses(t *testing.T) {
	h := newStreamsHarness(t)
	id := h.write(h.payer.caller, "open", h.payee.caller, h.token, big.NewInt(1), streamsStart, streamsStart+100, big.NewInt(100))[0].(*big.Int)
	h.advance(40)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	for _, caller := range []common.Address{h.payee.caller, h.other.caller} {
		out, err := h.call(caller, "updateRate", id, big.NewInt(3))
		requireStreamsRevert(t, out, err, "LXP_UNAUTHORIZED_DEBIT")
		require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
		require.Len(t, h.db.GetAllLogs(), logs)
	}
	require.Empty(t, h.write(h.payer.caller, "updateRate", id, big.NewInt(3)))
	require.Len(t, h.db.GetAllLogs(), logs+2)
	h.log(logs, "StreamRateUpdated", "0xd3372d2ac6b969b6649a339fe08ef2b791ab59c8818e02c48568c85f6571dbbc", []common.Hash{common.BigToHash(id)}, big.NewInt(1), big.NewInt(3))
	payLog := h.db.GetAllLogs()[logs+1]
	require.Equal(t, common.HexToAddress("0x101B"), payLog.Address)
	require.Equal(t, []common.Hash{crypto.Keccak256Hash([]byte("StreamRateUpdated(bytes32,uint128,uint128,uint128)")), common.BigToHash(id)}, payLog.Topics)
	pay, err := lp.NewPrecompile(h.keeper)
	require.NoError(t, err)
	values, err := pay.GetABI().Events["StreamRateUpdated"].Inputs.NonIndexed().Unpack(payLog.Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(1), big.NewInt(3), big.NewInt(40)}, values)
	r := h.record(id)
	require.Equal(t, codec.U128{Lo: 3}, r.Rate)
	require.Equal(t, codec.U128{Lo: 40}, r.SettledTotal)
	balance, err := h.keeper.Host().ReadBalance(h.db.Ctx(), r.Recipient, h.asset)
	require.NoError(t, err)
	require.Equal(t, "40", balance.String())
	h.advance(42)
	require.Equal(t, []interface{}{big.NewInt(6)}, h.write(h.payee.caller, "settle", id))
}

func TestStreamsViewsAndGuards(t *testing.T) {
	h := newStreamsHarness(t)
	zero := &streamOutput{Id: new(big.Int), RatePerSecond: new(big.Int), Cap: new(big.Int), Settled: new(big.Int)}
	for _, id := range []*big.Int{new(big.Int), big.NewInt(123)} {
		require.Zero(t, h.view("accrued", id)[0].(*big.Int).Cmp(new(big.Int)))
		got := abi.ConvertType(h.view("getStream", id)[0], new(streamOutput)).(*streamOutput)
		requireStreamTuple(t, h, zero, got)
	}
	id := h.open()
	h.advance(5)
	got := abi.ConvertType(h.view("getStream", id)[0], new(streamOutput)).(*streamOutput)
	requireStreamTuple(t, h, &streamOutput{Id: id, Payer: h.payer.caller, Payee: h.payee.caller, Token: h.token, RatePerSecond: big.NewInt(2), Cap: big.NewInt(100), StartTime: streamsStart, StopTime: streamsStart + 100, Settled: new(big.Int), Active: true}, got)
	var key [32]byte
	id.FillBytes(key[:])
	accrued, _, err := h.keeper.StreamAccrued(h.db.Ctx(), key)
	require.NoError(t, err)
	require.Equal(t, []interface{}{accrued.BigInt()}, h.view("accrued", id))
	require.Equal(t, big.NewInt(10), accrued.BigInt())
	h.write(h.payee.caller, "settle", id)
	h.advance(8)
	got = abi.ConvertType(h.view("getStream", id)[0], new(streamOutput)).(*streamOutput)
	require.Equal(t, big.NewInt(10), got.Settled)
	accrued, _, err = h.keeper.StreamAccrued(h.db.Ctx(), key)
	require.NoError(t, err)
	require.Equal(t, []interface{}{accrued.BigInt()}, h.view("accrued", id))
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	writes := []struct {
		name string
		args []interface{}
	}{
		{"open", []interface{}{h.payee.caller, h.token, big.NewInt(2), streamsStart, streamsStart + 100, big.NewInt(100)}},
		{"settle", []interface{}{id}}, {"close", []interface{}{id}}, {"updateRate", []interface{}{id, big.NewInt(3)}},
	}
	for _, call := range writes {
		t.Run(call.name, func(t *testing.T) {
			input := h.input(call.name, call.args...)
			out, _, err := h.evm.StaticCall(h.payer.caller, h.precompile.Address(), input, 2000000)
			requireStreamsRevert(t, out, err, string(agentcommon.ErrReadOnly))
			out, err = h.precompile.Run(h.evm, h.payer.caller, h.other.caller, input, new(big.Int), false, true, nil)
			requireStreamsRevert(t, out, err, string(agentcommon.ErrDelegatecall))
			ctx := h.db.Ctx()
			h.db.WithCtx(ctx.WithEVMPrecompileCalledFromDelegateCall(true))
			out, err = h.precompile.Run(h.evm, h.payer.caller, h.other.caller, input, new(big.Int), false, false, nil)
			h.db.WithCtx(ctx)
			requireStreamsRevert(t, out, err, string(agentcommon.ErrDelegatecall))
			out, _, err = h.evm.Call(h.payer.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(1))
			requireStreamsRevert(t, out, err, string(agentcommon.ErrNonPayable))
			require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
			require.Len(t, h.db.GetAllLogs(), logs)
		})
	}
	overflow := new(big.Int).Lsh(big.NewInt(1), 128)
	for _, args := range [][]interface{}{
		{h.payee.caller, h.token, overflow, streamsStart, streamsStart + 100, big.NewInt(100)},
		{h.payee.caller, h.token, big.NewInt(2), streamsStart, streamsStart + 100, overflow},
		{h.payee.caller, h.token, big.NewInt(2), uint64(math.MaxUint64), uint64(0), big.NewInt(100)},
		{h.payee.caller, h.token, big.NewInt(2), streamsStart, uint64(math.MaxUint64), big.NewInt(100)},
	} {
		out, err := h.call(h.payer.caller, "open", args...)
		requireStreamsRevert(t, out, err, string(agentcommon.ErrOverflow))
	}
	out, err := h.call(h.payer.caller, "updateRate", id, overflow)
	requireStreamsRevert(t, out, err, string(agentcommon.ErrOverflow))
	out, err = h.call(common.HexToAddress("0xdead"), "open", writes[0].args...)
	requireStreamsRevert(t, out, err, "LXP_PAY_UNBOUND_CALLER")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	params := h.keeper.GetParams(h.db.Ctx())
	params.StreamEnabled = false
	h.keeper.SetParams(h.db.Ctx(), params)
	out, err = h.call(h.payer.caller, "open", writes[0].args...)
	requireStreamsRevert(t, out, err, "LXP_MODULE_DISABLED")
	require.NotEmpty(t, h.view("getStream", id))
	require.NotEmpty(t, h.view("accrued", id))
}

func requireStreamTuple(t *testing.T, h *streamsHarness, want, got *streamOutput) {
	t.Helper()
	outputs := h.precompile.GetABI().Methods["getStream"].Outputs
	expected, err := outputs.Pack(want)
	require.NoError(t, err)
	actual, err := outputs.Pack(got)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
