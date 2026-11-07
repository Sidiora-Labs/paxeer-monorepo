package keeper

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func streamViewFixture() types.StreamRecord {
	return types.StreamRecord{
		StreamID: [32]byte{1}, Payer: [32]byte{2}, StreamAccount: [32]byte{3},
		Recipient: [32]byte{4}, AssetID: [32]byte{5}, Mode: types.StreamModeTime,
		Rate: codec.U128{Lo: 2}, RateUnit: 1000,
		StartTimestamp: uint64(payHarnessTimeMs - 30000), LastAccrualTimestamp: uint64(payHarnessTimeMs - 20000),
		TotalCap: codec.U128{Lo: 1000}, AccruedTotal: codec.U128{Lo: 20}, SettledTotal: codec.U128{Lo: 20},
		MeterAuthorities: make([][32]byte, 0),
	}
}

func encodeStreamViewRecord(r types.StreamRecord) []byte {
	b := make([]byte, 541)
	for i, id := range [][32]byte{r.StreamID, r.Payer, r.StreamAccount, r.Recipient, r.AssetID} {
		copy(b[32*i:32*(i+1)], id[:])
	}
	b[160] = r.Mode
	for _, field := range []struct {
		offset int
		value  codec.U128
	}{{161, r.Rate}, {209, r.TotalCap}, {225, r.AccruedTotal}, {241, r.SettledTotal}, {257, r.RemainderCarry}} {
		binary.BigEndian.PutUint64(b[field.offset:field.offset+8], field.value.Hi)
		binary.BigEndian.PutUint64(b[field.offset+8:field.offset+16], field.value.Lo)
	}
	for _, field := range []struct {
		offset int
		value  uint64
	}{{177, r.RateUnit}, {185, r.StartTimestamp}, {193, r.LastAccrualTimestamp}, {201, r.EndTimestamp}, {273, r.CumulativeMeter}} {
		binary.BigEndian.PutUint64(b[field.offset:field.offset+8], field.value)
	}
	b[281] = byte(len(r.MeterAuthorities))
	for i, authority := range r.MeterAuthorities {
		copy(b[282+32*i:314+32*i], authority[:])
	}
	for i, flag := range []bool{r.Underfunded, r.Paused, r.Closed} {
		if flag {
			b[538+i] = 1
		}
	}
	return b
}

func TestStreamAccruedAtBlockOpen(t *testing.T) {
	h := newPayHarness(t)
	r := streamViewFixture()
	key := types.KernelKVKey(4, append([]byte("stream:"), r.StreamID[:]...))
	h.ctx.KVStore(h.hostKey).Set(key, encodeStreamViewRecord(r))
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	payBefore := payStoreSnapshot(h.ctx, h.storeKey)
	rootBefore := bytes.Clone(h.cms.Commit(true).Hash)
	for _, tc := range []struct {
		offset             int64
		accrued, unsettled int64
	}{{0, 60, 40}, {0, 60, 40}, {500, 61, 41}, {-20000, 20, 0}} {
		ctx := h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + tc.offset))
		accrued, unsettled, err := h.keeper.StreamAccrued(ctx, r.StreamID)
		require.NoError(t, err)
		require.True(t, sdk.NewInt(tc.accrued).Equal(accrued), "accrued = %s; want %d", accrued, tc.accrued)
		require.True(t, sdk.NewInt(tc.unsettled).Equal(unsettled), "unsettled = %s; want %d", unsettled, tc.unsettled)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
	}
	require.Equal(t, rootBefore, h.cms.Commit(true).Hash)

	for _, tc := range []struct {
		name               string
		edit               func(*types.StreamRecord)
		accrued, unsettled int64
	}{
		{"paused", func(r *types.StreamRecord) { r.Paused = true }, 20, 0},
		{"closed", func(r *types.StreamRecord) { r.Closed = true }, 20, 0},
		{"underfunded", func(r *types.StreamRecord) { r.Underfunded = true }, 20, 0},
		{"cap", func(r *types.StreamRecord) { r.TotalCap.Lo = 35 }, 35, 15},
		{"end", func(r *types.StreamRecord) { r.EndTimestamp = uint64(payHarnessTimeMs - 10000) }, 40, 20},
		{"carry", func(r *types.StreamRecord) {
			r.Rate.Lo, r.RateUnit, r.RemainderCarry.Lo = 7, 3, 1
			r.LastAccrualTimestamp = uint64(payHarnessTimeMs - 4)
		}, 29, 9},
		{"metered", func(r *types.StreamRecord) {
			r.Mode, r.CumulativeMeter = types.StreamModeMetered, 100
			r.MeterAuthorities = [][32]byte{{6}}
			r.AccruedTotal.Lo = 50
		}, 50, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			branch, _ := h.ctx.CacheContext()
			record := r
			tc.edit(&record)
			branch.KVStore(h.hostKey).Set(key, encodeStreamViewRecord(record))
			before := payStoreSnapshot(branch, h.hostKey)
			for i := 0; i < 2; i++ {
				accrued, unsettled, err := h.keeper.StreamAccrued(branch, r.StreamID)
				require.NoError(t, err)
				require.True(t, sdk.NewInt(tc.accrued).Equal(accrued), "accrued = %s; want %d", accrued, tc.accrued)
				require.True(t, sdk.NewInt(tc.unsettled).Equal(unsettled), "unsettled = %s; want %d", unsettled, tc.unsettled)
				require.Equal(t, before, payStoreSnapshot(branch, h.hostKey))
			}
		})
	}

	t.Run("full uint128", func(t *testing.T) {
		branch, _ := h.ctx.CacheContext()
		record := r
		record.AccruedTotal = codec.U128{Hi: 2, Lo: 3}
		record.SettledTotal = codec.U128{Hi: 1, Lo: 1}
		record.TotalCap = codec.U128{Hi: 3}
		branch.KVStore(h.hostKey).Set(key, encodeStreamViewRecord(record))
		before := payStoreSnapshot(branch, h.hostKey)
		accrued, unsettled, err := h.keeper.StreamAccrued(branch, r.StreamID)
		require.NoError(t, err)
		require.Equal(t, "36893488147419103275", accrued.String())
		require.Equal(t, "18446744073709551658", unsettled.String())
		require.Equal(t, before, payStoreSnapshot(branch, h.hostKey))
	})

	for _, tc := range []struct {
		ctx  sdk.Context
		id   [32]byte
		code lxphost.Code
	}{
		{h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs - 20001)), r.StreamID, lxphost.Code(-717)},
		{h.ctx.WithBlockTime(time.UnixMilli(-1)), r.StreamID, lxphost.Code(-3)},
		{h.ctx, [32]byte{}, lxphost.Code(-3)},
		{h.ctx, [32]byte{99}, lxphost.Code(-7)},
	} {
		accrued, unsettled, err := h.keeper.StreamAccrued(tc.ctx, tc.id)
		require.Equal(t, tc.code, err)
		require.True(t, sdk.ZeroInt().Equal(accrued))
		require.True(t, sdk.ZeroInt().Equal(unsettled))
	}
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
}

func TestStreamRecordAndResultViews(t *testing.T) {
	h := newPayHarness(t)
	want := streamViewFixture()
	want.Mode, want.CumulativeMeter = types.StreamModeMetered, 12345
	want.MeterAuthorities = [][32]byte{{6}, {7}}
	want.Rate = codec.U128{Hi: 1, Lo: 2}
	want.TotalCap = codec.U128{Hi: 4, Lo: 5}
	want.AccruedTotal = codec.U128{Hi: 3, Lo: 4}
	want.SettledTotal = codec.U128{Hi: 2, Lo: 3}
	want.RemainderCarry.Lo = 7
	want.EndTimestamp = uint64(payHarnessTimeMs + 30000)
	want.Underfunded, want.Paused, want.Closed = true, true, true
	raw := encodeStreamViewRecord(want)
	require.Len(t, raw, 541)
	idempotencyKey := [32]byte{9}
	resultWant := types.StreamResult{
		StreamID: want.StreamID, TransferSetRoot: [32]byte{8}, Paid: want.SettledTotal,
		Refunded: codec.U128{Hi: 1, Lo: 2}, Ordinal: 7, LegCount: 2,
	}
	resultRaw := append([]byte(nil), resultWant.TransferSetRoot[:]...)
	for _, amount := range []codec.U128{resultWant.Paid, resultWant.Refunded} {
		encoded := amount.Bytes()
		resultRaw = append(resultRaw, encoded[:]...)
	}
	resultRaw = binary.BigEndian.AppendUint16(resultRaw, resultWant.Ordinal)
	resultRaw = append(resultRaw, resultWant.LegCount)
	resultRaw = append(resultRaw, resultWant.StreamID[:]...)
	require.Len(t, resultRaw, 99)
	streamKey := types.KernelKVKey(4, append([]byte("stream:"), want.StreamID[:]...))
	resultKey := types.KernelKVKey(4, append([]byte("result:"), idempotencyKey[:]...))
	require.Len(t, streamKey, 3+39)
	require.Len(t, resultKey, 3+39)
	h.ctx.KVStore(h.hostKey).Set(streamKey, raw)
	h.ctx.KVStore(h.hostKey).Set(resultKey, resultRaw)
	h.ctx.KVStore(h.storeKey).Set(streamKey, []byte("module store decoy"))
	h.ctx.KVStore(h.storeKey).Set(resultKey, []byte("module store decoy"))
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(3, streamKey[3:]), []byte("wrong module decoy"))
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(3, resultKey[3:]), []byte("wrong module decoy"))
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	payBefore := payStoreSnapshot(h.ctx, h.storeKey)
	for i := 0; i < 2; i++ {
		record, err := h.keeper.GetStream(h.ctx, want.StreamID)
		require.NoError(t, err)
		require.Equal(t, want, record)
		result, err := h.keeper.GetStreamResult(h.ctx, idempotencyKey)
		require.NoError(t, err)
		require.Equal(t, resultWant, result)
		record.MeterAuthorities[0][0] ^= 0xff
	}

	for _, id := range [][32]byte{{}, {99}} {
		code := lxphost.Code(-7)
		if id == ([32]byte{}) {
			code = lxphost.Code(-3)
		}
		record, err := h.keeper.GetStream(h.ctx, id)
		require.Equal(t, code, err)
		require.Equal(t, types.StreamRecord{}, record)
		result, err := h.keeper.GetStreamResult(h.ctx, id)
		require.Equal(t, code, err)
		require.Equal(t, types.StreamResult{}, result)
	}
	badFlag := bytes.Clone(raw)
	badFlag[539] = 2
	for _, malformed := range [][]byte{raw[:540], append(bytes.Clone(raw), 0), badFlag} {
		branch, _ := h.ctx.CacheContext()
		branch.KVStore(h.hostKey).Set(streamKey, malformed)
		before := payStoreSnapshot(branch, h.hostKey)
		record, err := h.keeper.GetStream(branch, want.StreamID)
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, types.StreamRecord{}, record)
		accrued, unsettled, err := h.keeper.StreamAccrued(branch, want.StreamID)
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.True(t, sdk.ZeroInt().Equal(accrued))
		require.True(t, sdk.ZeroInt().Equal(unsettled))
		require.Equal(t, before, payStoreSnapshot(branch, h.hostKey))
	}
	badOrdinal := bytes.Clone(resultRaw)
	badOrdinal[64], badOrdinal[65] = 0, 0
	for _, malformed := range [][]byte{resultRaw[:98], append(bytes.Clone(resultRaw), 0), badOrdinal} {
		branch, _ := h.ctx.CacheContext()
		branch.KVStore(h.hostKey).Set(resultKey, malformed)
		before := payStoreSnapshot(branch, h.hostKey)
		result, err := h.keeper.GetStreamResult(branch, idempotencyKey)
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, types.StreamResult{}, result)
		require.Equal(t, before, payStoreSnapshot(branch, h.hostKey))
	}
	branch, _ := h.ctx.CacheContext()
	mismatch := bytes.Clone(raw)
	mismatch[0] = 99
	branch.KVStore(h.hostKey).Set(streamKey, mismatch)
	record, err := h.keeper.GetStream(branch, want.StreamID)
	require.Equal(t, lxphost.Code(-1001), err)
	require.Equal(t, types.StreamRecord{}, record)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
}
