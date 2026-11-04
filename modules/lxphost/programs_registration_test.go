//go:build cgo

package lxphost

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func TestHostProgramsModuleAcceptsAccountSandboxAndRetirementActivities(t *testing.T) {
	for _, activity := range []struct {
		name   string
		typeID uint32
	}{
		{"account", 0x00090006},
		{"wind-down", 0x00090007},
		{"sandbox", 0x00090009},
		{"storage-retirement", 0x0009000B},
	} {
		t.Run(activity.name, func(t *testing.T) {
			chain := newTestChain(t)
			host := chain.host(t)
			authority := executeActivityAuthority(t, [32]byte{0x73})
			seedProgramsFeeRecord(t, chain, authority.Actor)
			ctx := chain.context(1, bindingBlockTime, 41)

			receipt, err := ExecuteActivity(ctx, host, authority, activity.typeID, nil, 0)
			require.NoError(t, err)
			require.NotEqual(t, Code(-106), receipt.ResultCode)
			require.Equal(t, Code(-1), receipt.ResultCode)
			require.Equal(t, "LXP_ERR_TRUNCATED", receipt.ResultCode.Error())
		})
	}
}

func TestHostProgramsModuleRefusesUnknownOrdinal(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	authority := executeActivityAuthority(t, [32]byte{0x74})
	seedProgramsFeeRecord(t, chain, authority.Actor)
	ctx := chain.context(1, bindingBlockTime, 41)

	_, err := ExecuteActivity(ctx, host, authority, 0x0009000C, nil, 0)
	require.Equal(t, Code(-106), err)
	require.EqualError(t, err, "LXP_ERR_UNKNOWN_ACTIVITY")
}

func hostProgramsModuleCallGuestABIFive(t *testing.T, chain *testChain, host *Host) Receipt {
	t.Helper()
	actor := newTestActor(t, 0x75)
	authority := actor.authority()
	authority.AuthorityHash = sha256.Sum256(actor.public[:])
	asset := newTestAsset(t, actor, 0x55)
	resolutionSeedMetering(t, chain)
	seedProgramsFeeRecord(t, chain, actor.id)
	for i, activity := range []Activity{
		actor.register(t, host, 0, resolutionBlockTime),
		actor.registerAsset(t, host, 1, resolutionBlockTime, asset, 0x55, 1000000000),
		actor.openAssetAccount(t, host, 2, resolutionBlockTime, asset),
		actor.mint(t, host, 3, resolutionBlockTime, asset, 500000000),
	} {
		receipt := execute(t, host, chain.context(2, resolutionBlockTime, uint64(41+i)), activity, authority)
		require.Equal(t, Code(0), receipt.ResultCode)
	}
	store := chain.context(2, resolutionBlockTime, 45).KVStore(chain.key)
	feeKey := moduleKVKey(9, []byte("progfee/active/v1"))
	fee := append([]byte(nil), store.Get(feeKey)...)
	copy(fee[65:97], asset.id[:])
	store.Set(feeKey, fee)
	store.Set(moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), programsFeeVersion)), fee)
	treasury, err := kernelAccountID([]byte("system:fees"))
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	account := field(nil, treasury[:])
	account = field(account, []byte("system:fees"))
	account = append(account, 10)
	account = append(account, make([]byte, 16)...)
	account = field(account, asset.id[:])
	account = append(account, 1)
	account = append(account, make([]byte, 16)...)
	account = append(account, 1, 0)
	account = field(account, make([]byte, 32))
	account = append(account, 0)
	store.Set(accountKey(treasury), account)

	program := resolutionDIDKey(0x76)
	wasm := resolutionGuest("layerx_v2", "response_write", 3)
	codeHash := sha256.Sum256(wasm)
	deploy := make([]byte, 104)
	copy(deploy, program[:])
	binary.BigEndian.PutUint16(deploy[32:], 2)
	deploy[34] = 1
	copy(deploy[36:], actor.id[:])
	copy(deploy[68:], codeHash[:])
	binary.BigEndian.PutUint32(deploy[100:], uint32(len(wasm)))
	deployed := execute(t, host, chain.context(2, resolutionBlockTime, 45),
		actor.activity(t, host, resolutionDeployType, 4, resolutionBlockTime, append(deploy, wasm...)), authority)
	require.Equal(t, Code(0), deployed.ResultCode)
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(chain.get(resolutionProgramKey(program))[65:67]))

	upgrade := make([]byte, 106)
	copy(upgrade, program[:])
	binary.BigEndian.PutUint16(upgrade[32:], 5)
	copy(upgrade[36:], codeHash[:])
	copy(upgrade[68:], codeHash[:])
	binary.BigEndian.PutUint32(upgrade[102:], uint32(len(wasm)))
	upgraded := execute(t, host, chain.context(2, resolutionBlockTime, 46),
		actor.activity(t, host, 0x00090002, 5, resolutionBlockTime, append(upgrade, wasm...)), authority)
	require.Equal(t, Code(0), upgraded.ResultCode)
	record := chain.get(resolutionProgramKey(program))
	require.Len(t, record, resolutionProgramRecord)
	require.Equal(t, uint16(5), binary.BigEndian.Uint16(record[65:67]))
	require.Equal(t, uint32(2), binary.BigEndian.Uint32(record[67:71]))

	identity, err := host.ReadIdentityState(chain.context(2, resolutionBlockTime, 47), actor.did)
	require.NoError(t, err)
	require.Equal(t, actor.id, identity.DIDID)
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	call := append([]byte(nil), program[:]...)
	call = binary.BigEndian.AppendUint16(call, 5)
	call = binary.BigEndian.AppendUint16(call, uint16(len("layerx_call")))
	call = binary.BigEndian.AppendUint32(call, 0)
	call = binary.BigEndian.AppendUint16(call, 2)
	call = binary.BigEndian.AppendUint32(call, uint32(len(access)))
	call = binary.BigEndian.AppendUint32(call, 16)
	for _, budget := range []uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096} {
		call = binary.BigEndian.AppendUint64(call, budget)
	}
	call = append(call, "layerx_call"...)
	call = append(call, 0, 0)
	call = append(call, access...)
	return execute(t, host, chain.context(2, resolutionBlockTime, 47),
		actor.activity(t, host, 0x00090003, 6, resolutionBlockTime, call), authority)
}

func TestHostProgramsModuleRegistersGuestABIFive(t *testing.T) {
	chain := newTestChain(t)
	host, err := NewHost(Config{
		StoreKey: chain.key, NetworkID: testNetworkID, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000, ProgramsGuestABI: 5,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	receipt := hostProgramsModuleCallGuestABIFive(t, chain, host)
	require.NotEqual(t, "LXP_ERR_VERSION_UNSUPPORTED", receipt.ResultCode.Error())
	require.NotEqual(t, Code(-106), receipt.ResultCode)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(9), decoded.ModuleID)
	require.Equal(t, uint32(5), decoded.ModuleVersion)
}

func TestHostProgramsModuleKeepsGuestABIFourByDefault(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	receipt := hostProgramsModuleCallGuestABIFive(t, chain, host)
	require.Equal(t, "LXP_ERR_VERSION_UNSUPPORTED", receipt.ResultCode.Error())
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint32(4), decoded.ModuleVersion)
}

func TestHostProgramsModuleKeepsExplicitGuestABIFour(t *testing.T) {
	chain := newTestChain(t)
	host, err := NewHost(Config{
		StoreKey: chain.key, NetworkID: testNetworkID, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000, ProgramsGuestABI: 4,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	receipt := hostProgramsModuleCallGuestABIFive(t, chain, host)
	require.Equal(t, "LXP_ERR_VERSION_UNSUPPORTED", receipt.ResultCode.Error())
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint32(4), decoded.ModuleVersion)
}

func TestHostProgramsModuleRefusesUnsupportedGuestABI(t *testing.T) {
	chain := newTestChain(t)
	for _, abi := range []uint16{1, 2, 3, 6, 65535} {
		host, err := NewHost(Config{
			StoreKey: chain.key, NetworkID: testNetworkID, ParameterVersion: 1,
			GasLimit: 1000000, ProgramsGuestABI: abi,
		})
		require.Nil(t, host)
		require.Equal(t, codeNonCanonical, err)
	}
}
