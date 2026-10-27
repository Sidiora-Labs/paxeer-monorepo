package keeper

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/stretchr/testify/require"
)

func TestEscrowViewsReadKernelRecord(t *testing.T) {
	h := newPayHarness(t)
	recordBytes, wantRecord := escrowViewRecord()
	resultBytes, wantResult := escrowViewResult(wantRecord)
	idempotencyKey := escrowViewID(9)
	holdKey := append([]byte("hold:"), wantRecord.EscrowID[:]...)
	resultKey := append([]byte("result:"), idempotencyKey[:]...)
	hostHoldKey := append([]byte{0x01, 0x00, 0x02}, holdKey...)
	hostResultKey := append([]byte{0x01, 0x00, 0x02}, resultKey...)
	require.Equal(t, hostHoldKey, types.KernelKVKey(2, holdKey))
	require.Equal(t, hostResultKey, types.KernelKVKey(2, resultKey))
	h.ctx.KVStore(h.hostKey).Set(hostHoldKey, recordBytes)
	h.ctx.KVStore(h.hostKey).Set(hostResultKey, resultBytes)
	h.ctx.KVStore(h.storeKey).Set(hostHoldKey, []byte("wrong store"))
	h.ctx.KVStore(h.storeKey).Set(hostResultKey, []byte("wrong store"))
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(3, holdKey), []byte("wrong module"))
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(3, resultKey), []byte("wrong module"))

	for _, contextBound := range []bool{false, true} {
		if contextBound {
			wantResult.ContextBound = true
			wantResult.ContextDigest = escrowViewID(10)
			resultBytes = append(bytes.Clone(resultBytes), 2)
			resultBytes = append(resultBytes, wantResult.ContextDigest[:]...)
			h.ctx.KVStore(h.hostKey).Set(hostResultKey, resultBytes)
		}
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		record, found := h.keeper.GetEscrow(h.ctx, wantRecord.EscrowID)
		require.True(t, found)
		require.Equal(t, wantRecord, record)
		remaining, found := h.keeper.EscrowRemaining(h.ctx, wantRecord.EscrowID)
		require.True(t, found)
		require.Equal(t, wantRecord.LockedAmount, remaining)
		result, found := h.keeper.GetEscrowResult(h.ctx, idempotencyKey)
		require.True(t, found)
		require.Equal(t, wantResult, result)
		result, found = h.keeper.GetEscrowResult(h.ctx, wantRecord.EscrowID)
		require.False(t, found)
		require.Equal(t, types.EscrowResult{}, result)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	}

	for _, state := range []byte{1, 2, 3, 4, 5, 6, 7} {
		data := bytes.Clone(recordBytes)
		data[224] = state
		clear(data[192:208])
		h.ctx.KVStore(h.hostKey).Set(hostHoldKey, data)
		record, found := h.keeper.GetEscrow(h.ctx, wantRecord.EscrowID)
		require.True(t, found)
		require.Equal(t, state, record.State)
		require.Equal(t, wantRecord.CapturedAmount, record.CapturedAmount)
		remaining, found := h.keeper.EscrowRemaining(h.ctx, wantRecord.EscrowID)
		require.True(t, found)
		require.Equal(t, codec.U128{}, remaining)
	}

	t.Run("malformed records", func(t *testing.T) {
		for _, data := range [][]byte{recordBytes[:len(recordBytes)-1], make([]byte, types.EscrowRecordBytes)} {
			h.ctx.KVStore(h.hostKey).Set(hostHoldKey, data)
			before := payStoreSnapshot(h.ctx, h.hostKey)
			require.Panics(t, func() { h.keeper.GetEscrow(h.ctx, wantRecord.EscrowID) })
			require.Panics(t, func() { h.keeper.EscrowRemaining(h.ctx, wantRecord.EscrowID) })
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		}
		badVersion := bytes.Clone(resultBytes)
		badVersion[243] = 1
		for _, data := range [][]byte{resultBytes[:types.EscrowResultBytes-1], badVersion} {
			h.ctx.KVStore(h.hostKey).Set(hostResultKey, data)
			before := payStoreSnapshot(h.ctx, h.hostKey)
			require.Panics(t, func() { h.keeper.GetEscrowResult(h.ctx, idempotencyKey) })
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		}
	})
}

func TestEscrowViewsUnknownIDEmpty(t *testing.T) {
	h := newPayHarness(t)
	recordBytes, record := escrowViewRecord()
	resultBytes, _ := escrowViewResult(record)
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(2, append([]byte("hold:"), record.EscrowID[:]...)), recordBytes)
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(2, append([]byte("result:"), record.EscrowID[:]...)), resultBytes)
	for _, id := range [][32]byte{{}, escrowViewID(99)} {
		holdKey := append([]byte("hold:"), id[:]...)
		resultKey := append([]byte("result:"), id[:]...)
		h.ctx.KVStore(h.storeKey).Set(types.KernelKVKey(2, holdKey), recordBytes)
		h.ctx.KVStore(h.storeKey).Set(types.KernelKVKey(2, resultKey), resultBytes)
		h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(3, holdKey), recordBytes)
		h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(3, resultKey), resultBytes)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		require.Equal(t, h.sequence, binary.BigEndian.Uint64(hostBefore[string([]byte{0x02})]))
		for i := 0; i < 2; i++ {
			got, found := h.keeper.GetEscrow(h.ctx, id)
			require.False(t, found)
			require.Equal(t, types.EscrowRecord{}, got)
			remaining, found := h.keeper.EscrowRemaining(h.ctx, id)
			require.False(t, found)
			require.Equal(t, codec.U128{}, remaining)
			result, found := h.keeper.GetEscrowResult(h.ctx, id)
			require.False(t, found)
			require.Equal(t, types.EscrowResult{}, result)
		}
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	}
}

func escrowViewID(seed byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = seed + byte(i)
	}
	return id
}

func escrowViewPut128(data []byte, amount codec.U128) {
	binary.BigEndian.PutUint64(data[:8], amount.Hi)
	binary.BigEndian.PutUint64(data[8:16], amount.Lo)
}

func escrowViewRecord() ([]byte, types.EscrowRecord) {
	record := types.EscrowRecord{
		EscrowID: escrowViewID(1), Owner: escrowViewID(2), EscrowAccount: escrowViewID(3),
		Beneficiary: escrowViewID(4), Arbiter: escrowViewID(5), AssetID: escrowViewID(6),
		LockedAmount:   codec.U128{Hi: 0x0102030405060708, Lo: 0x1112131415161718},
		CapturedAmount: codec.U128{Hi: 0x2122232425262728, Lo: 0x3132333435363738},
		State:          2, ExpiryMs: 0x8182838485868788, DisputeWindowMs: 0x9192939495969798,
		TermsHash: escrowViewID(7), AgreementReference: escrowViewID(8),
	}
	data := make([]byte, types.EscrowRecordBytes)
	for i, id := range [][32]byte{record.EscrowID, record.Owner, record.EscrowAccount, record.Beneficiary, record.Arbiter, record.AssetID} {
		copy(data[i*32:(i+1)*32], id[:])
	}
	escrowViewPut128(data[192:208], record.LockedAmount)
	escrowViewPut128(data[208:224], record.CapturedAmount)
	data[224] = record.State
	binary.BigEndian.PutUint64(data[225:233], record.ExpiryMs)
	binary.BigEndian.PutUint64(data[233:241], record.DisputeWindowMs)
	copy(data[241:273], record.TermsHash[:])
	copy(data[273:305], record.AgreementReference[:])
	return data, record
}

func escrowViewResult(record types.EscrowRecord) ([]byte, types.EscrowResult) {
	result := types.EscrowResult{
		EscrowID: record.EscrowID, Ordinal: 2, StateAfter: record.State,
		CapturedAfter: record.CapturedAmount, LockedAfter: record.LockedAmount,
		AssetID: record.AssetID, From: record.EscrowAccount, To: record.Beneficiary,
		Amount:          codec.U128{Hi: 0x4142434445464748, Lo: 0x5152535455565758},
		SecondaryAmount: codec.U128{Hi: 0x6162636465666768, Lo: 0x7172737475767778},
		TransferSetRoot: escrowViewID(11), GlobalSequence: 0x8182838485868788, TimestampMs: 0x9192939495969798,
	}
	data := make([]byte, types.EscrowResultBytes)
	copy(data[:32], result.EscrowID[:])
	binary.BigEndian.PutUint16(data[32:34], result.Ordinal)
	data[34] = result.StateAfter
	escrowViewPut128(data[35:51], result.CapturedAfter)
	escrowViewPut128(data[51:67], result.LockedAfter)
	copy(data[67:99], result.AssetID[:])
	copy(data[99:131], result.From[:])
	copy(data[131:163], result.To[:])
	escrowViewPut128(data[163:179], result.Amount)
	escrowViewPut128(data[179:195], result.SecondaryAmount)
	copy(data[195:227], result.TransferSetRoot[:])
	binary.BigEndian.PutUint64(data[227:235], result.GlobalSequence)
	binary.BigEndian.PutUint64(data[235:243], result.TimestampMs)
	return data, result
}
