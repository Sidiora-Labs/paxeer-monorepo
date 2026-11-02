//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestReadBalanceScopedLoad(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 64)
	asset := newTestAsset(t, actor, 0x64)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 1),
		actor.registerAsset(t, host, 0, blockTime, asset, 0x64, 1000), actor.authority()).ResultCode)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 2),
		actor.openAssetAccount(t, host, 1, blockTime, asset), actor.authority()).ResultCode)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 3),
		actor.mint(t, host, 2, blockTime, asset, 250), actor.authority()).ResultCode)
	chain.commit()
	ctx := chain.context(1, blockTime, 3)
	for i := uint32(0); i < 600; i++ {
		ctx.KVStore(chain.key).Set(moduleKVKey(9, binary.BigEndian.AppendUint32(nil, i)), []byte{0x91, byte(i)})
	}
	before := chain.dump()
	fresh := chain.host(t)
	for _, reader := range []*Host{host, fresh} {
		balance, err := reader.ReadBalance(ctx, asset.account, asset.id)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(250), balance)
		total, err := reader.ReadTotalUnits(ctx, asset.id)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(250), total)
		account, err := reader.ReadAccount(ctx, asset.name, asset.account)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(250), account.Balance)
		owner, err := reader.ReadAccountOwner(ctx, asset.account)
		require.NoError(t, err)
		require.Equal(t, actor.id, owner)
	}
	require.Equal(t, before, chain.dump())
}

func TestReadAssetIgnoresUnrelatedModules(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 65)
	asset := newTestAsset(t, actor, 0x65)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 1),
		actor.registerAsset(t, host, 0, blockTime, asset, 0x65, 1000), actor.authority()).ResultCode)
	ctx := chain.context(1, blockTime, 1)
	expected, err := host.ReadAsset(ctx, asset.id)
	require.NoError(t, err)
	require.Equal(t, asset.id, expected.AssetID)
	require.Equal(t, actor.id, expected.IssuerDID)
	require.Equal(t, sdk.NewInt(1000), expected.SupplyCap)
	for _, module := range []uint16{2, 4, governanceModule, 8, 9, 10, 256, 65535} {
		ctx.KVStore(chain.key).Set(moduleKVKey(module, nil), []byte{0xff})
		ctx.KVStore(chain.key).Set(moduleKVKey(module, []byte("opaque")), bytes.Repeat([]byte{0xff}, 8192))
	}
	for i := uint32(0); i < 600; i++ {
		ctx.KVStore(chain.key).Set(moduleKVKey(9, binary.BigEndian.AppendUint32(nil, i)), []byte{0x91, byte(i)})
	}
	ctx.KVStore(chain.key).Set([]byte{idempotencyPrefix, 1}, []byte{0xff})
	before := chain.dump()
	fresh := chain.host(t)
	for _, reader := range []*Host{host, fresh} {
		actual, err := reader.ReadAsset(ctx, asset.id)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	require.Equal(t, before, chain.dump())
}
