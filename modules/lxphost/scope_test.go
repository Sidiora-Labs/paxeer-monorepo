//go:build cgo

package lxphost

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostLoadScopedToActivityModules(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 61)
	asset := newTestAsset(t, actor, 0x61)
	ctx := chain.context(1, blockTime, 1)
	require.Equal(t, Code(0), execute(t, host, ctx, actor.register(t, host, 0, blockTime), actor.authority()).ResultCode)
	governance := chain.get(moduleKVKey(governanceModule, actor.id[:]))
	require.NotEmpty(t, governance)
	for i := uint32(0); i < 600; i++ {
		ctx.KVStore(chain.key).Set(moduleKVKey(9, binary.BigEndian.AppendUint32(nil, i)), []byte{0x91, byte(i)})
	}
	receipt := execute(t, host, chain.context(1, blockTime, 2),
		actor.registerAsset(t, host, 1, blockTime, asset, 0x61, 1000), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	opened := execute(t, host, chain.context(1, blockTime, 3),
		actor.openAssetAccount(t, host, 2, blockTime, asset), actor.authority())
	require.Equal(t, Code(0), opened.ResultCode)
	require.NotEmpty(t, chain.get(accountKey(asset.account)))
	entries, err := host.moduleKV()
	require.NoError(t, err)
	modules := map[uint16]bool{}
	for _, entry := range entries {
		require.Contains(t, []uint16{1, governanceModule}, entry.module)
		modules[entry.module] = true
		if entry.module == governanceModule && bytes.Equal(entry.key, actor.id[:]) {
			require.Equal(t, governance, entry.value)
		}
	}
	require.Equal(t, map[uint16]bool{1: true, governanceModule: true}, modules)
}

func TestHostUnrelatedModuleRowsIgnored(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 62)
	ctx := chain.context(1, blockTime, 1)
	untouched := map[string][]byte{}
	for _, module := range []uint16{8, 9, 10, 256, 65535} {
		untouched[string(moduleKVKey(module, nil))] = []byte{0xff, 0, 0x80}
		untouched[string(moduleKVKey(module, []byte("opaque")))] = bytes.Repeat([]byte{byte(module)}, 8192)
	}
	for key, value := range untouched {
		ctx.KVStore(chain.key).Set([]byte(key), value)
	}
	receipt := execute(t, host, ctx, actor.register(t, host, 0, blockTime), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	for key, value := range untouched {
		require.Equal(t, value, chain.get([]byte(key)), "module row %x changed", key)
	}
	entries, err := host.moduleKV()
	require.NoError(t, err)
	for _, entry := range entries {
		require.Contains(t, []uint16{1, governanceModule}, entry.module)
	}
}

func TestHostBlobLoadScopedToModule(t *testing.T) {
	const blockTime = int64(1700000000000)
	for _, activityModule := range []uint16{1, governanceModule, 9} {
		t.Run(map[uint16]string{1: "asset", governanceModule: "governance", 9: "programs"}[activityModule], func(t *testing.T) {
			chain := newTestChain(t)
			host := chain.host(t)
			actor := newTestActor(t, 63)
			ctx := chain.context(1, blockTime, 1)
			all := map[string][]byte{}
			expected := map[string][]byte{}
			for _, module := range []uint16{1, governanceModule, 8, 9} {
				count := uint32(1)
				if module == 9 && activityModule != 9 {
					count = 600
				}
				for i := uint32(0); i < count; i++ {
					value := binary.BigEndian.AppendUint32([]byte{byte(module), 0xff}, i)
					hash := sha256.Sum256(value)
					key := string(blobKey(module, hash[:]))
					all[key] = value
					if module == 1 || module == governanceModule || module == activityModule {
						expected[key] = value
					}
				}
			}
			all[string(blobKey(8, []byte("malformed")))] = []byte("opaque")
			for key, value := range all {
				ctx.KVStore(chain.key).Set([]byte(key), value)
			}
			var activity Activity
			switch activityModule {
			case 1:
				asset := newTestAsset(t, actor, 0x63)
				activity = actor.registerAsset(t, host, 0, blockTime, asset, 0x63, 1000)
			case governanceModule:
				activity = actor.register(t, host, 0, blockTime)
			case 9:
				activity = actor.registry(t, host, 0, blockTime, 0x63, []byte("manifest"))
			}
			receipt := execute(t, host, ctx, activity, actor.authority())
			require.Equal(t, Code(0), receipt.ResultCode)
			loaded, err := host.moduleBlobs()
			require.NoError(t, err)
			require.Equal(t, expected, loaded)
			for key, value := range all {
				require.Equal(t, value, chain.get([]byte(key)), "blob %x changed", key)
			}
		})
	}
}
