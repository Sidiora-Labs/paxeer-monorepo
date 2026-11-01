//go:build cgo

package lxphost

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func assetMintStoreGas(t *testing.T, extraRows map[string][]byte) [2]uint64 {
	t.Helper()
	const blockTime = int64(1700000000000)
	actor := newTestActor(t, 71)
	asset := newTestAsset(t, actor, 0x71)
	chains := [2]*testChain{newTestChain(t), newTestChain(t)}
	var hosts [2]*Host
	for i, chain := range chains {
		host := chain.host(t)
		hosts[i] = host
		registered := execute(t, host, chain.context(1, blockTime, 1),
			actor.registerAsset(t, host, 0, blockTime, asset, 0x71, 1000), actor.authority())
		require.Equal(t, Code(0), registered.ResultCode)
		opened := execute(t, host, chain.context(1, blockTime, 2),
			actor.openAssetAccount(t, host, 1, blockTime, asset), actor.authority())
		require.Equal(t, Code(0), opened.ResultCode)
	}
	require.Equal(t, chains[0].dump(), chains[1].dump())
	store := chains[1].context(1, blockTime, 2).KVStore(chains[1].key)
	expected := chains[0].dump()
	for key, value := range extraRows {
		require.NotContains(t, expected, key)
		store.Set([]byte(key), value)
		expected[key] = value
	}
	for _, chain := range chains {
		chain.commit()
	}
	require.Equal(t, expected, chains[1].dump())

	activity := actor.mint(t, hosts[0], 2, blockTime+1000, asset, 250)
	var gas [2]uint64
	var receipts [2]Receipt
	for i, chain := range chains {
		ctx := chain.context(2, blockTime+1000, 3).WithGasMeter(sdk.NewGasMeter(1000000000, 1, 1))
		require.Zero(t, ctx.GasMeter().GasConsumed())
		receipt, err := hosts[i].Execute(ctx, activity, actor.authority())
		gas[i] = ctx.GasMeter().GasConsumed()
		require.NoError(t, err)
		require.Equal(t, Code(0), receipt.ResultCode)
		require.Positive(t, gas[i])
		receipts[i] = receipt
		rows := accountRows(t, chain)
		require.Equal(t, uint64(750), rows[asset.issuance].balance)
		require.Equal(t, uint64(250), rows[asset.account].balance)
	}
	require.Equal(t, receipts[0], receipts[1])
	for key, value := range extraRows {
		require.Equal(t, value, chains[1].get([]byte(key)), "unrelated row %x changed", key)
	}
	return gas
}

func TestHostExecuteGasIndependentOfUnrelatedRows(t *testing.T) {
	rows := make(map[string][]byte, 2200)
	for i := uint32(0); i < 2000; i++ {
		key := moduleKVKey(9, binary.BigEndian.AppendUint32(nil, i))
		rows[string(key)] = binary.BigEndian.AppendUint32([]byte{0x91}, i)
	}
	for i := uint32(0); i < 200; i++ {
		value := binary.BigEndian.AppendUint32([]byte{0x95}, i)
		hash := sha256.Sum256(value)
		rows[string(blobKey(9, hash[:]))] = value
	}
	require.Len(t, rows, 2200)
	gas := assetMintStoreGas(t, rows)
	require.Equal(t, gas[0], gas[1], "unrelated module KV and blob rows must add no store gas to a mint")
}

func TestHostExecuteGasIndependentOfLegacyIdempotencyRows(t *testing.T) {
	rows := make(map[string][]byte, 2000)
	for i := uint32(0); i < 2000; i++ {
		hash := sha256.Sum256(binary.BigEndian.AppendUint32(nil, i))
		key := append([]byte{idempotencyPrefix}, hash[:]...)
		rows[string(key)] = binary.BigEndian.AppendUint64(nil, uint64(i+1))
	}
	require.Len(t, rows, 2000)
	gas := assetMintStoreGas(t, rows)
	require.Equal(t, gas[0], gas[1], "legacy idempotency rows must add no store gas to a mint")
}
