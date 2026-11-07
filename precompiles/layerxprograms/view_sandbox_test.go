package layerxprograms

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func sandboxViewContext(t *testing.T) (*PrecompileExecutor, sdk.Context, *sdk.KVStoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	key := sdk.NewKVStoreKey("programs_sandbox_view")
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 100, Time: time.UnixMilli(1700000000123)}, false)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	return &PrecompileExecutor{host: host}, ctx, key
}

func sandboxViewExecute(p *PrecompileExecutor, ctx sdk.Context, name string, args []interface{}) ([]byte, error) {
	method := programsABI.Methods[name]
	return p.Execute(ctx, &method, common.Address{}, common.Address{}, args, nil, true, nil, nil)
}

func sandboxViewStoreKey(kernelKey []byte) []byte {
	return append([]byte{1, 0, 9}, kernelKey...)
}

func sandboxViewSnapshot(ctx sdk.Context, key *sdk.KVStoreKey) map[string][]byte {
	values := make(map[string][]byte)
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func assertSandboxViewOutput(t *testing.T, name string, packed, record []byte) {
	t.Helper()
	decoded, err := programsABI.Methods[name].Outputs.Unpack(packed)
	require.NoError(t, err)
	require.Equal(t, []interface{}{record}, decoded)
	require.Equal(t, uint64(32), binary.BigEndian.Uint64(packed[24:32]))
	require.Equal(t, uint64(len(record)), binary.BigEndian.Uint64(packed[56:64]))
	require.Equal(t, record, packed[64:64+len(record)])
	require.Equal(t, make([]byte, (32-len(record)%32)%32), packed[64+len(record):])
}

func TestProgramsViewSandboxRecord(t *testing.T) {
	p, ctx, key := sandboxViewContext(t)
	lease := [32]byte{0x51, 0, 0xff, 0x71}
	otherLease := [32]byte{0x52}
	kv := ctx.KVStore(key)
	records := [][]byte{
		append([]byte("LayerX/programs/sandbox/lease-state/v3\x00"), bytes.Repeat([]byte{0, 0xff, 0x81}, 191)...),
		append([]byte("LayerX/programs/sandbox/escrow-state/v1\x00"), bytes.Repeat([]byte{0xff, 0, 0x42}, 71)...),
		append([]byte("LayerX/programs/sandbox/usage-ledger/v1\x00"), bytes.Repeat([]byte{0x82, 0, 0xff}, 328)...),
	}
	require.Len(t, records[2], 1024)
	for kind, record := range records {
		kv.Set(sandboxViewStoreKey(append([]byte{'s', byte(kind)}, lease[:]...)), record)
		kv.Set(sandboxViewStoreKey(append([]byte{'s', byte(kind)}, otherLease[:]...)), []byte{0x91, byte(kind)})
	}
	before := sandboxViewSnapshot(ctx, key)
	for kind, record := range records {
		args := []interface{}{lease, uint8(kind)}
		packed, err := sandboxViewExecute(p, ctx, "getSandbox", args)
		require.NoError(t, err)
		assertSandboxViewOutput(t, "getSandbox", packed, record)
		packed[64] ^= 0xff
		packed, err = sandboxViewExecute(p, ctx, "getSandbox", args)
		require.NoError(t, err)
		assertSandboxViewOutput(t, "getSandbox", packed, record)
		packed, err = sandboxViewExecute(p, ctx, "getSandbox", []interface{}{otherLease, uint8(kind)})
		require.NoError(t, err)
		assertSandboxViewOutput(t, "getSandbox", packed, []byte{0x91, byte(kind)})
		packed, err = sandboxViewExecute(p, ctx, "getSandbox", []interface{}{[32]byte{0x99}, uint8(kind)})
		require.Nil(t, packed)
		require.ErrorIs(t, err, lxphost.Code(-7))
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Contains(t, reason, "LXP_NOT_FOUND:")
	}
	for _, args := range [][]interface{}{nil, {lease}, {lease, uint8(0), uint8(0)}, {lease, uint8(3)}, {lease, uint8(255)}, {lease, uint16(0)}, {[]byte{1}, uint8(0)}} {
		packed, err := sandboxViewExecute(p, ctx, "getSandbox", args)
		require.Error(t, err)
		require.Nil(t, packed)
	}
	require.Equal(t, before, sandboxViewSnapshot(ctx, key))
	p.host.Close()
	packed, err := sandboxViewExecute(p, ctx, "getSandbox", []interface{}{lease, uint8(0)})
	require.Nil(t, packed)
	require.ErrorIs(t, err, lxphost.Code(-3))
}

func TestProgramsViewRetirementControl(t *testing.T) {
	p, ctx, key := sandboxViewContext(t)
	kv := ctx.KVStore(key)
	controlKey := sandboxViewStoreKey([]byte("progretire/v1"))
	packed, err := sandboxViewExecute(p, ctx, "getRetirementControl", nil)
	require.Nil(t, packed)
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.False(t, kv.Has(controlKey))
	genesis := make([]byte, 251)
	copy(genesis, "LXRT1")
	binary.BigEndian.PutUint16(genesis[5:7], 1)
	binary.BigEndian.PutUint32(genesis[7:11], 7)
	copy(genesis[11:43], bytes.Repeat([]byte{0x31}, 32))
	for i, seed := range []byte{0x41, 0x42} {
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
		copy(genesis[43+32*i:75+32*i], private.Public().(ed25519.PublicKey))
	}
	copy(genesis[107:139], bytes.Repeat([]byte{0x71}, 32))
	binary.BigEndian.PutUint64(genesis[139:147], 1000)
	digest := sha256.Sum256(append([]byte("LXP/programs-retirement-profile/v1\x00"), genesis[:147]...))
	copy(genesis[147:179], digest[:])
	kv.Set(controlKey, genesis)
	before := sandboxViewSnapshot(ctx, key)
	packed, err = sandboxViewExecute(p, ctx, "getRetirementControl", nil)
	require.NoError(t, err)
	assertSandboxViewOutput(t, "getRetirementControl", packed, genesis)
	require.Equal(t, make([]byte, 72), packed[64+179:64+251])
	packed[64] ^= 0xff
	packed, err = sandboxViewExecute(p, ctx, "getRetirementControl", nil)
	require.NoError(t, err)
	assertSandboxViewOutput(t, "getRetirementControl", packed, genesis)
	require.Equal(t, before, sandboxViewSnapshot(ctx, key))
	updated := bytes.Clone(genesis)
	for i, value := range []uint64{2000, 1, 3, 0xfedcba9876543210, 2} {
		binary.BigEndian.PutUint64(updated[179+8*i:187+8*i], value)
	}
	copy(updated[219:], bytes.Repeat([]byte{0xff}, 32))
	kv.Set(controlKey, updated)
	before = sandboxViewSnapshot(ctx, key)
	packed, err = sandboxViewExecute(p, ctx, "getRetirementControl", []interface{}{})
	require.NoError(t, err)
	assertSandboxViewOutput(t, "getRetirementControl", packed, updated)
	packed, err = sandboxViewExecute(p, ctx, "getRetirementControl", []interface{}{uint8(0)})
	require.Error(t, err)
	require.Nil(t, packed)
	require.Equal(t, before, sandboxViewSnapshot(ctx, key))
	p.host.Close()
	packed, err = sandboxViewExecute(p, ctx, "getRetirementControl", nil)
	require.Nil(t, packed)
	require.ErrorIs(t, err, lxphost.Code(-3))
}
