//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

const feeRecordLoadTime = int64(1700000000000)
const feeRecordLoadVersion = uint32(0x01020304)

type feeRecordLoadFixture struct {
	chain    *testChain
	host     *Host
	actor    testActor
	delegate testActor
	asset    [32]byte
	grantID  [32]byte
	body     []byte
	record   []byte
	activity Activity
}

func newFeeRecordLoadFixture(t *testing.T, withRecord bool) feeRecordLoadFixture {
	t.Helper()
	f := feeRecordLoadFixture{
		chain: newTestChain(t), actor: newTestActor(t, 81),
		delegate: newTestActor(t, 82), asset: repeated(0x83),
	}
	f.host = f.chain.host(t)
	registered := execute(t, f.host, f.chain.context(1, feeRecordLoadTime, 1),
		f.actor.register(t, f.host, 0, feeRecordLoadTime), f.actor.authority())
	require.Equal(t, Code(0), registered.ResultCode)

	store := f.chain.context(1, feeRecordLoadTime, 2).KVStore(f.chain.key)
	policyKey := make([]byte, 32)
	copy(policyKey, "native-fee-authority-version")
	policy := make([]byte, 32)
	policy[31] = 2
	store.Set(moduleKVKey(governanceModule, policyKey), policy)

	action := repeated(0x84)
	payload, _, _ := readBridgeSession(t, f.actor, f.delegate, action,
		uint64(feeRecordLoadTime)-1000, uint64(feeRecordLoadTime)+1000, 1, 100)
	length := binary.BigEndian.Uint32(payload[4:8])
	f.body = append([]byte(nil), payload[8:8+length]...)
	f.body[4] = 2
	field := func(value []byte) {
		f.body = binary.BigEndian.AppendUint32(f.body, uint32(len(value)))
		f.body = append(f.body, value...)
	}
	u64 := func(value uint64) { f.body = binary.BigEndian.AppendUint64(f.body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(f.asset[:])
	u128(4)
	u128(20)
	u128(0)
	u64(1000)
	u128(8)
	u128(0)
	u64(uint64(feeRecordLoadTime) - 1000)
	var err error
	f.grantID, err = codec.DomainHash(codec.DomainAuthorityHash, f.body)
	require.NoError(t, err)
	payload, err = authtypes.EncodeSessionPayload(f.body, 100, action, nil)
	require.NoError(t, err)
	f.activity = f.actor.activity(t, f.host, 0x00070005, 1, feeRecordLoadTime, payload)

	f.record = binary.BigEndian.AppendUint32([]byte("LXFR1"), feeRecordLoadVersion)
	for i := 0; i < 7; i++ {
		f.record = binary.BigEndian.AppendUint64(f.record, 1)
	}
	f.record = append(f.record, f.asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		f.record = binary.BigEndian.AppendUint64(f.record, value)
	}
	f.record = append(f.record, bytes.Repeat([]byte{0x85}, 32)...)
	f.record = append(f.record, make([]byte, 16)...)
	require.Len(t, f.record, 217)
	if withRecord {
		store.Set(moduleKVKey(9, []byte("progfee/active/v1")), f.record)
		store.Set(feeRecordHistoryKey(feeRecordLoadVersion), f.record)
	}
	return f
}

func feeRecordHistoryKey(version uint32) []byte {
	return moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), version))
}

func TestHostLoadsProgramsFeeRecordForGovernance(t *testing.T) {
	f := newFeeRecordLoadFixture(t, true)
	ctx := f.chain.context(1, feeRecordLoadTime, 2)
	receipt, err := f.host.Execute(ctx, f.activity, f.actor.authority())
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, int32(0), decoded.ResultCode)
	require.Equal(t, uint16(governanceModule), decoded.ModuleID)
	var recordedGrant []byte
	for _, effect := range decoded.Effects {
		if effect.ModuleID == governanceModule && effect.Kind == 3 &&
			(effect.EventType == 0x7105 || effect.EventType == 0x7125) {
			recordedGrant = append(recordedGrant, effect.Body...)
		}
	}
	require.Equal(t, f.body, recordedGrant)
	grant, err := f.host.ReadGrant(ctx, f.grantID)
	require.NoError(t, err)
	require.Equal(t, f.grantID, grant.GrantID)
	require.Equal(t, f.actor.id, grant.Grantor)
	require.Equal(t, f.delegate.public, grant.Key)
	require.Equal(t, uint8(2), grant.Kind)
	require.Equal(t, AuthorityFeeBudget{
		Present: true, AssetID: f.asset,
		MaximumPerActivity: sdk.NewInt(4), MaximumTotal: sdk.NewInt(20),
		SpentTotal: sdk.ZeroInt(), PeriodLength: 1000,
		MaximumPerPeriod: sdk.NewInt(8), SpentThisPeriod: sdk.ZeroInt(),
		PeriodStart: uint64(feeRecordLoadTime) - 1000,
	}, grant.FeeBudget)
	require.Equal(t, f.record, f.chain.get(moduleKVKey(9, []byte("progfee/active/v1"))))
	require.Equal(t, f.record, f.chain.get(feeRecordHistoryKey(feeRecordLoadVersion)))
}

func TestHostFeeRecordAbsentRefusesVersion(t *testing.T) {
	f := newFeeRecordLoadFixture(t, false)
	ctx := f.chain.context(1, feeRecordLoadTime, 2)
	receipt, err := f.host.Execute(ctx, f.activity, f.actor.authority())
	require.NoError(t, err)
	require.Equal(t, Code(-101), receipt.ResultCode)
	require.Equal(t, "LXP_ERR_VERSION_UNSUPPORTED", receipt.ResultCode.Error())
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, int32(-101), decoded.ResultCode)
	require.Empty(t, receipt.Events)
	require.Nil(t, f.chain.get(moduleKVKey(governanceModule, append([]byte{5}, f.grantID[:]...))))
	_, err = f.host.ReadGrant(ctx, f.grantID)
	require.Equal(t, Code(-7), err)
}

func TestHostUnrelatedModuleNineRowsStillIgnored(t *testing.T) {
	f := newFeeRecordLoadFixture(t, true)
	ctx := f.chain.context(1, feeRecordLoadTime, 2)
	store := ctx.KVStore(f.chain.key)
	untouched := map[string][]byte{
		string(feeRecordHistoryKey(feeRecordLoadVersion + 1)): []byte("unselected history"),
		string(moduleKVKey(9, []byte("progfee/pending/v1"))):  []byte("pending record"),
	}
	for i := uint32(0); i < 600; i++ {
		key := moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("unrelated/"), i))
		untouched[string(key)] = []byte{0x91, byte(i)}
	}
	for key, value := range untouched {
		store.Set([]byte(key), value)
	}
	receipt, err := f.host.Execute(ctx, f.activity, f.actor.authority())
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
	entries, err := f.host.moduleKV()
	require.NoError(t, err)
	loaded := map[string][]byte{}
	for _, entry := range entries {
		require.Contains(t, []uint16{1, governanceModule, 9}, entry.module)
		if entry.module == 9 {
			loaded[string(moduleKVKey(entry.module, entry.key))] = entry.value
		}
	}
	require.Equal(t, map[string][]byte{
		string(moduleKVKey(9, []byte("progfee/active/v1"))): f.record,
		string(feeRecordHistoryKey(feeRecordLoadVersion)):   f.record,
	}, loaded)
	for key, value := range untouched {
		require.Equal(t, value, f.chain.get([]byte(key)), "module row %x changed", key)
	}
}
