package layerxpay_test

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func streamTestArgs(t *testing.T, f *escrowTestFixture, id [32]byte, mode uint8, funding int64) []interface{} {
	t.Helper()
	account, err := codec.DeriveAccountID([]byte("agent:" + string(f.owner.did) + ":stream:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	now := uint64(f.h.db.Ctx().BlockTime().UnixMilli())
	rate, unit, authorities := int64(2), uint64(1000), [][32]byte{}
	if mode == paytypes.StreamModeMetered {
		rate, unit, authorities = 3, 2, [][32]byte{f.beneficiary.public}
	}
	return []interface{}{id, account, f.beneficiary.account, f.asset, mode, big.NewInt(rate), unit, now, now + 120000, big.NewInt(1000), big.NewInt(funding), authorities}
}

func streamTestFixture(t *testing.T) *escrowTestFixture {
	t.Helper()
	f := newEscrowTestFixture(t)
	f.h.keeper.SetParams(f.h.db.Ctx(), paytypes.Params{StreamEnabled: true})
	return f
}

func streamTestWrite(t *testing.T, h *payPrecompileHarness, caller common.Address, name, event string, want []interface{}, args ...interface{}) paytypes.StreamRecord {
	t.Helper()
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	ordinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey)
	output, _, err := h.evm.StaticCall(caller, h.precompile.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	require.Equal(t, ordinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey))
	output, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%x", output)
	require.Empty(t, output)
	require.Equal(t, ordinal.Big().Uint64()+1, h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64())
	all := h.db.GetAllLogs()
	require.Len(t, all, logs+1)
	log := all[logs]
	require.Equal(t, common.HexToAddress(lp.PayAddress), log.Address)
	require.Equal(t, []common.Hash{a.Events[event].ID, common.Hash(args[0].([32]byte))}, log.Topics)
	encoded, err := a.Events[event].Inputs.NonIndexed().Pack(want...)
	require.NoError(t, err)
	require.Equal(t, encoded, log.Data)
	record, err := h.keeper.GetStream(h.db.Ctx(), args[0].([32]byte))
	require.NoError(t, err)
	return record
}

func streamTestOpen(t *testing.T, f *escrowTestFixture, args []interface{}) paytypes.StreamRecord {
	t.Helper()
	return streamTestWrite(t, f.h, f.owner.caller, "streamOpen", "StreamOpened",
		[]interface{}{f.owner.account, args[1], f.beneficiary.account, f.asset, args[4], args[10]}, args...)
}

func streamTestSignature(t *testing.T, actor escrowTestActor, id [32]byte, reading uint64) []byte {
	t.Helper()
	message := append([]byte("LXP:STREAM:METER:v1"), id[:]...)
	message = binary.BigEndian.AppendUint64(message, reading)
	message = append(message, actor.public[:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	return ed25519.Sign(actor.private, digest[:])
}

func TestPrecompileStreamLifecycle(t *testing.T) {
	t.Run("time", func(t *testing.T) {
		f := streamTestFixture(t)
		h, id := f.h, [32]byte{0xd1}
		args := streamTestArgs(t, f, id, paytypes.StreamModeTime, 100)
		now := int64(args[7].(uint64))
		r := streamTestOpen(t, f, args)
		require.Equal(t, f.owner.account, r.Payer)
		require.Equal(t, args[1], r.StreamAccount)
		require.Equal(t, codec.U128{Lo: 2}, r.Rate)
		budgetTestRevert(t, h, f.arbiter.caller, "streamTopUp", id, big.NewInt(50))
		streamTestWrite(t, h, f.owner.caller, "streamTopUp", "StreamToppedUp", []interface{}{big.NewInt(50), false}, id, big.NewInt(50))
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 5000)))
		budgetTestRevert(t, h, f.arbiter.caller, "streamSettle", id, [32]byte{0xd2})
		r = streamTestWrite(t, h, f.beneficiary.caller, "streamSettle", "StreamSettled", []interface{}{big.NewInt(10), big.NewInt(10), false}, id, [32]byte{0xd2})
		require.Equal(t, codec.U128{Lo: 10}, r.SettledTotal)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 7500)))
		budgetTestRevert(t, h, f.beneficiary.caller, "streamPause", id)
		r = streamTestWrite(t, h, f.owner.caller, "streamPause", "StreamPaused", []interface{}{uint64(now + 7500), big.NewInt(15)}, id)
		require.True(t, r.Paused)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 17500)))
		require.Equal(t, []interface{}{big.NewInt(15), big.NewInt(5)}, budgetTestView(t, h, "streamAccrued", id))
		r = streamTestWrite(t, h, f.owner.caller, "streamResume", "StreamResumed", []interface{}{uint64(now + 17500), big.NewInt(15)}, id)
		require.False(t, r.Paused)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 20000)))
		budgetTestRevert(t, h, f.beneficiary.caller, "streamClose", id, [32]byte{0xd3})
		r = streamTestWrite(t, h, f.owner.caller, "streamClose", "StreamClosed", []interface{}{big.NewInt(10), big.NewInt(130)}, id, [32]byte{0xd3})
		require.True(t, r.Closed)
		require.Equal(t, codec.U128{Lo: 20}, r.SettledTotal)
		budgetTestBalance(t, h, f.owner.account, f.asset, "980")
		budgetTestBalance(t, h, f.beneficiary.account, f.asset, "20")
		budgetTestBalance(t, h, r.StreamAccount, f.asset, "0")
		budgetTestRevert(t, h, f.owner.caller, "streamTopUp", id, big.NewInt(1))
	})
	t.Run("metered", func(t *testing.T) {
		f := streamTestFixture(t)
		h, id := f.h, [32]byte{0xd4}
		streamTestOpen(t, f, streamTestArgs(t, f, id, paytypes.StreamModeMetered, 100))
		signature := streamTestSignature(t, f.beneficiary, id, 15)
		for _, size := range []int{63, 65} {
			bad := append(append([]byte(nil), signature...), 0)[:size]
			budgetTestRevert(t, h, f.owner.caller, "streamMeter", id, uint64(15), f.beneficiary.public, bad)
		}
		bad := append([]byte(nil), signature...)
		bad[0] ^= 1
		budgetTestRevert(t, h, f.owner.caller, "streamMeter", id, uint64(15), f.beneficiary.public, bad)
		r := streamTestWrite(t, h, f.owner.caller, "streamMeter", "StreamMetered", []interface{}{uint64(15), big.NewInt(22), big.NewInt(22)}, id, uint64(15), f.beneficiary.public, signature)
		require.Equal(t, uint64(15), r.CumulativeMeter)
		require.Equal(t, codec.U128{Lo: 1}, r.RemainderCarry)
		require.Equal(t, []interface{}{big.NewInt(22), big.NewInt(22)}, budgetTestView(t, h, "streamAccrued", id))
		streamTestWrite(t, h, f.beneficiary.caller, "streamMeter", "StreamMetered", []interface{}{uint64(17), big.NewInt(3), big.NewInt(25)}, id, uint64(17), f.beneficiary.public, streamTestSignature(t, f.beneficiary, id, 17))
		r = streamTestWrite(t, h, f.owner.caller, "streamClose", "StreamClosed", []interface{}{big.NewInt(25), big.NewInt(75)}, id, [32]byte{0xd5})
		require.True(t, r.Closed)
		budgetTestBalance(t, h, f.owner.account, f.asset, "975")
		budgetTestBalance(t, h, f.beneficiary.account, f.asset, "25")
	})
	t.Run("underfunded", func(t *testing.T) {
		f := streamTestFixture(t)
		h, id := f.h, [32]byte{0xd6}
		args := streamTestArgs(t, f, id, paytypes.StreamModeTime, 10)
		now := int64(args[7].(uint64))
		streamTestOpen(t, f, args)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 10000)))
		r := streamTestWrite(t, h, f.owner.caller, "streamSettle", "StreamSettled", []interface{}{big.NewInt(10), big.NewInt(10), true}, id, [32]byte{0xd7})
		require.True(t, r.Underfunded)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 20000)))
		r = streamTestWrite(t, h, f.owner.caller, "streamTopUp", "StreamToppedUp", []interface{}{big.NewInt(50), true}, id, big.NewInt(50))
		require.False(t, r.Underfunded)
		require.Equal(t, uint64(now+20000), r.LastAccrualTimestamp)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 22000)))
		streamTestWrite(t, h, f.owner.caller, "streamSettle", "StreamSettled", []interface{}{big.NewInt(4), big.NewInt(14), false}, id, [32]byte{0xd8})
	})
}

func TestPrecompileStreamViews(t *testing.T) {
	t.Run("rate two after thirty seconds", func(t *testing.T) {
		f := streamTestFixture(t)
		h, id := f.h, [32]byte{0xe3}
		args := streamTestArgs(t, f, id, paytypes.StreamModeTime, 100)
		streamTestOpen(t, f, args)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(args[7].(uint64) + 30000))))
		require.Equal(t, []interface{}{big.NewInt(60), big.NewInt(60)}, budgetTestView(t, h, "streamAccrued", id))
		got := abi.ConvertType(budgetTestView(t, h, "getStream", id)[0], new(paytypes.StreamTuple)).(*paytypes.StreamTuple)
		require.Equal(t, big.NewInt(2), got.Rate)
	})
	for _, mode := range []uint8{paytypes.StreamModeTime, paytypes.StreamModeMetered} {
		t.Run(map[uint8]string{1: "time", 2: "metered"}[mode], func(t *testing.T) {
			f := streamTestFixture(t)
			h, id, key := f.h, [32]byte{0xe1}, [32]byte{0xe2}
			args := streamTestArgs(t, f, id, mode, 100)
			now := args[7].(uint64)
			streamTestOpen(t, f, args)
			paid, carry, reading := int64(10), int64(500), uint64(0)
			lastAccrual := now + 5250
			if mode == paytypes.StreamModeMetered {
				paid, carry, reading = 22, 1, 15
				lastAccrual = now
				streamTestWrite(t, h, f.owner.caller, "streamMeter", "StreamMetered", []interface{}{reading, big.NewInt(paid), big.NewInt(paid)}, id, reading, f.beneficiary.public, streamTestSignature(t, f.beneficiary, id, reading))
			}
			h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(now + 5250))))
			streamTestWrite(t, h, f.owner.caller, "streamSettle", "StreamSettled", []interface{}{big.NewInt(paid), big.NewInt(paid), false}, id, key)
			h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(now + 7750))))
			before := h.keeper.ExportGenesis(h.db.Ctx())
			logs := len(h.db.GetAllLogs())
			ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
			ordinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey)
			got := abi.ConvertType(budgetTestView(t, h, "getStream", id)[0], new(paytypes.StreamTuple)).(*paytypes.StreamTuple)
			want := paytypes.StreamTuple{
				Payer: f.owner.account, StreamAccount: args[1].([32]byte), Recipient: f.beneficiary.account,
				AssetId: f.asset, Mode: mode, Rate: args[5].(*big.Int), RateUnit: args[6].(uint64),
				StartMs: now, LastAccrualMs: lastAccrual, EndMs: now + 120000, TotalCap: big.NewInt(1000),
				AccruedTotal: big.NewInt(paid), SettledTotal: big.NewInt(paid), RemainderCarry: big.NewInt(carry),
				CumulativeMeter: reading, MeterAuthorities: args[11].([][32]byte),
			}
			require.Equal(t, &want, got)
			result := abi.ConvertType(budgetTestView(t, h, "getStreamResult", key)[0], new(paytypes.StreamResultTuple)).(*paytypes.StreamResultTuple)
			stored, err := h.keeper.GetStreamResult(h.db.Ctx(), key)
			require.NoError(t, err)
			require.NotEqual(t, [32]byte{}, stored.TransferSetRoot)
			wantResultABI, err := h.precompile.GetABI().Methods["getStreamResult"].Outputs.Pack(&paytypes.StreamResultTuple{StreamId: id, TransferSetRoot: stored.TransferSetRoot, Paid: big.NewInt(paid), Refunded: big.NewInt(0), Ordinal: 4, LegCount: 1})
			require.NoError(t, err)
			gotResultABI, err := h.precompile.GetABI().Methods["getStreamResult"].Outputs.Pack(result)
			require.NoError(t, err)
			require.Equal(t, wantResultABI, gotResultABI)
			accrued, unsettled := int64(15), int64(5)
			if mode == paytypes.StreamModeMetered {
				accrued, unsettled = 22, 0
			}
			for i := 0; i < 2; i++ {
				wantAccruedABI, err := h.precompile.GetABI().Methods["streamAccrued"].Outputs.Pack(big.NewInt(accrued), big.NewInt(unsettled))
				require.NoError(t, err)
				gotAccruedABI, err := h.precompile.GetABI().Methods["streamAccrued"].Outputs.Pack(budgetTestView(t, h, "streamAccrued", id)...)
				require.NoError(t, err)
				require.Equal(t, wantAccruedABI, gotAccruedABI)
			}
			a := h.precompile.GetABI()
			for _, name := range []string{"getStream", "getStreamResult", "streamAccrued"} {
				input, err := a.Pack(name, [32]byte{0xff})
				require.NoError(t, err)
				output, _, err := h.evm.StaticCall(common.HexToAddress("0xdead"), h.precompile.Address(), input, 2000000)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, err := abi.UnpackRevert(output)
				require.NoError(t, err)
				require.Contains(t, reason, "LXP_UNKNOWN_FIELD")
			}
			require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
			require.Len(t, h.db.GetAllLogs(), logs)
			require.Equal(t, ordinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey))
		})
	}
}

func TestPrecompileStreamRateUpdatedLog(t *testing.T) {
	t.Run("rate one to three after forty seconds", func(t *testing.T) {
		f := streamTestFixture(t)
		h, id, key := f.h, [32]byte{0xf3}, [32]byte{0xf4}
		args := streamTestArgs(t, f, id, paytypes.StreamModeTime, 100)
		args[5] = big.NewInt(1)
		before := streamTestOpen(t, f, args)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(args[7].(uint64) + 40000))))
		receipt, err := h.keeper.StreamRateChange(h.db.Ctx(), f.owner.caller, id, codec.U128{Lo: 3}, key)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		after, err := h.keeper.GetStream(h.db.Ctx(), id)
		require.NoError(t, err)
		result, err := h.keeper.GetStreamResult(h.db.Ctx(), key)
		require.NoError(t, err)
		require.Equal(t, codec.U128{Lo: 1}, before.Rate)
		require.Equal(t, codec.U128{Lo: 3}, after.Rate)
		require.Equal(t, codec.U128{Lo: 40}, result.Paid)
		logs := len(h.db.GetAllLogs())
		require.NoError(t, lp.EmitStreamRateUpdated(h.evm, id, before.Rate, after.Rate, result.Paid))
		all := h.db.GetAllLogs()
		require.Len(t, all, logs+1)
		log := all[logs]
		require.Equal(t, common.HexToAddress("0x101B"), log.Address)
		a := h.precompile.GetABI()
		require.Equal(t, []common.Hash{a.Events["StreamRateUpdated"].ID, common.Hash(id)}, log.Topics)
		values, err := a.Events["StreamRateUpdated"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{big.NewInt(1), big.NewInt(3), big.NewInt(40)}, values)
	})
	f := streamTestFixture(t)
	h, id, key := f.h, [32]byte{0xf1}, [32]byte{0xf2}
	args := streamTestArgs(t, f, id, paytypes.StreamModeTime, 100)
	before := streamTestOpen(t, f, args)
	h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(args[7].(uint64) + 5000))))
	newRate := codec.U128{Lo: 5}
	receipt, err := h.keeper.StreamRateChange(h.db.Ctx(), f.owner.caller, id, newRate, key)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	after, err := h.keeper.GetStream(h.db.Ctx(), id)
	require.NoError(t, err)
	result, err := h.keeper.GetStreamResult(h.db.Ctx(), key)
	require.NoError(t, err)
	require.Equal(t, newRate, after.Rate)
	require.Equal(t, codec.U128{Lo: 10}, result.Paid)
	require.Equal(t, uint16(8), result.Ordinal)
	state := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	snapshot := h.db.Snapshot()
	require.NoError(t, lp.EmitStreamRateUpdated(h.evm, id, before.Rate, after.Rate, result.Paid))
	all := h.db.GetAllLogs()
	require.Len(t, all, logs+1)
	log := all[logs]
	require.Equal(t, common.HexToAddress("0x101B"), log.Address)
	require.Equal(t, []common.Hash{crypto.Keccak256Hash([]byte("StreamRateUpdated(bytes32,uint128,uint128,uint128)")), common.Hash(id)}, log.Topics)
	a := h.precompile.GetABI()
	values, err := a.Events["StreamRateUpdated"].Inputs.NonIndexed().Unpack(log.Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(2), big.NewInt(5), big.NewInt(10)}, values)
	require.Len(t, log.Data, 96)
	require.Equal(t, state, h.keeper.ExportGenesis(h.db.Ctx()))
	budgetTestBalance(t, h, f.beneficiary.account, f.asset, "10")
	budgetTestBalance(t, h, after.StreamAccount, f.asset, "90")
	h.db.RevertToSnapshot(snapshot)
	require.Len(t, h.db.GetAllLogs(), logs)
	require.Equal(t, state, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Error(t, lp.EmitStreamRateUpdated(nil, id, before.Rate, after.Rate, result.Paid))
	require.Error(t, lp.EmitStreamRateUpdated(&vm.EVM{}, id, before.Rate, after.Rate, result.Paid))
	_, exists := a.Methods["streamRateChange"]
	require.False(t, exists)
	_, exists = a.Methods["updateRate"]
	require.False(t, exists)
}
