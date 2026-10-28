//go:build cgo

package lxphost

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func issueReadPayerGrant(t *testing.T) (*testChain, *Host, sdk.Context, []byte, [32]byte) {
	t.Helper()
	const blockTime = int64(10000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 36)
	asset := newTestAsset(t, actor, 0x11)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 41),
		actor.registerAsset(t, host, 0, blockTime, asset, 0x11, 1000), actor.authority()).ResultCode)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 42),
		actor.openAssetAccount(t, host, 1, blockTime, asset), actor.authority()).ResultCode)
	grant, grantID := readBridgePayerGrant(t, actor, asset.account, asset.account, asset.id, uint64(blockTime)+1000)
	issued := actor.activity(t, host, 0x00010007, 2, blockTime, grant)
	require.Equal(t, Code(0), execute(t, host, chain.context(1, blockTime, 43), issued, actor.authority()).ResultCode)
	return chain, host, chain.context(1, blockTime, 43), grant, grantID
}

func TestReadPayerGrantFoldsBudgetDraws(t *testing.T) {
	chain, host, ctx, grant, grantID := issueReadPayerGrant(t)
	key := moduleKVKey(3, append([]byte("budget-grant:"), grantID[:]...))
	draws := make([]byte, 42)
	binary.BigEndian.PutUint16(draws[:2], 1)
	binary.BigEndian.PutUint64(draws[10:18], 40)
	binary.BigEndian.PutUint64(draws[26:34], 15)
	binary.BigEndian.PutUint64(draws[34:42], 7)
	ctx.KVStore(chain.key).Set(key, draws)
	before := chain.dump()
	expected := PayerGrantState{Grant: grant, DrawnTotal: sdk.NewInt(40),
		DrawnThisPeriod: sdk.NewInt(15), WindowStart: 7}
	for _, reader := range []*Host{host, chain.host(t)} {
		actual, err := reader.ReadPayerGrant(ctx, grantID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	require.Equal(t, before, chain.dump())
}

func TestReadPayerGrantWithoutDrawsUnchanged(t *testing.T) {
	chain, host, ctx, grant, grantID := issueReadPayerGrant(t)
	before := chain.dump()
	expected := PayerGrantState{Grant: grant, DrawnTotal: sdk.ZeroInt(), DrawnThisPeriod: sdk.ZeroInt()}
	for _, reader := range []*Host{host, chain.host(t)} {
		actual, err := reader.ReadPayerGrant(ctx, grantID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		missing, err := reader.ReadPayerGrant(ctx, [32]byte{1})
		require.Equal(t, Code(-214), err)
		require.Equal(t, PayerGrantState{}, missing)
	}
	require.Equal(t, before, chain.dump())
}
