//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

const bindingBlockTime = int64(1700000000000)

func bindingAssetRegister(t *testing.T, host *Host, actor testActor, sequence uint64) (Activity, [32]byte) {
	t.Helper()
	salt := [32]byte{8}
	preimage := append([]byte("LX:ASSET:v1"), actor.id[:]...)
	asset := sha256.Sum256(append(preimage, salt[:]...))
	payload := make([]byte, 89)
	payload[1] = 1
	copy(payload[2:34], asset[:])
	copy(payload[34:66], salt[:])
	payload[66], payload[67] = 1, 'T'
	payload[68], payload[69] = 1, 'T'
	payload[86] = 100
	payload[87] = 1
	activity := actor.activity(t, host, 0x00010001, sequence, bindingBlockTime, payload)
	activity.Signature = nil
	return activity, asset
}

func TestHostBindingAuthorityUnsignedRegister(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 71)
	activity, asset := bindingAssetRegister(t, host, actor, 0)
	authority := actor.authority()
	authority.Bound = true
	ctx := chain.context(1, bindingBlockTime, 41)
	before := chain.dump()
	for _, candidate := range []KernelAuthority{
		actor.authority(),
		{Bound: true, Actor: actor.id, Principal: actor.id, VerifiedKey: newTestActor(t, 72).public},
	} {
		_, err := host.HostExecute(ctx, activity, candidate)
		require.Equal(t, Code(-201), err)
		require.Equal(t, "LXP_ERR_BAD_SIGNATURE", err.Error())
		require.Equal(t, before, chain.dump())
	}
	receipt, err := host.HostExecute(ctx, activity, authority)
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
	require.True(t, bool(host.node.asset_runtime.binding_authority))
	require.True(t, signature_valid(host.node))
	require.NotEmpty(t, chain.get(moduleKVKey(1, append([]byte("asset:"), asset[:]...))))
	require.Contains(t, receipt.Events, ReceiptEvent{Code: 1, Body: asset[:]})
	require.NotNil(t, host.node.programs_runtime.accounts)
	require.NotNil(t, host.node.programs_runtime.assets)
	require.NotNil(t, host.node.programs_runtime.resolve_metering_schedule)
	require.NotNil(t, host.node.programs_runtime.resolve_occupancy_parameters)

	other := newTestActor(t, 73)
	signed := other.register(t, host, 0, bindingBlockTime)
	receipt, err = host.HostExecute(chain.context(1, bindingBlockTime, 42), signed, other.authority())
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
	require.False(t, bool(host.node.asset_runtime.binding_authority))
	require.True(t, signature_valid(host.node))
}

func TestHostRejectsForgedSignature(t *testing.T) {
	for _, bound := range []bool{false, true} {
		name := "unbound"
		if bound {
			name = "bound"
		}
		t.Run(name, func(t *testing.T) {
			chain := newTestChain(t)
			host := chain.host(t)
			actor := newTestActor(t, 74)
			intruder := newTestActor(t, 75)
			activity := actor.register(t, host, 0, bindingBlockTime)
			preimage, err := host.SigningHash(activity)
			require.NoError(t, err)
			activity.Signature = ed25519.Sign(intruder.private, preimage[:])
			require.True(t, ed25519.Verify(intruder.public[:], preimage[:], activity.Signature))
			require.False(t, ed25519.Verify(actor.public[:], preimage[:], activity.Signature))
			authority := actor.authority()
			authority.Bound = bound
			before := chain.dump()
			ctx := chain.context(1, bindingBlockTime, 41)
			_, err = host.HostExecute(ctx, activity, authority)
			require.Equal(t, Code(-201), err)
			require.Equal(t, "LXP_ERR_BAD_SIGNATURE", err.Error())
			require.False(t, signature_valid(host.node))
			require.False(t, bool(host.node.asset_runtime.binding_authority))
			require.Equal(t, before, chain.dump())

			activity.Signature = ed25519.Sign(actor.private, preimage[:])
			receipt, err := host.HostExecute(ctx, activity, authority)
			require.NoError(t, err)
			require.Equal(t, Code(0), receipt.ResultCode)
			require.True(t, signature_valid(host.node))
			require.False(t, bool(host.node.asset_runtime.binding_authority))
		})
	}
}

func TestHostBindingAuthorityKeepsPayloadHash(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 76)
	receipt := execute(t, host, chain.context(1, bindingBlockTime, 41),
		actor.register(t, host, 0, bindingBlockTime), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	activity, asset := bindingAssetRegister(t, host, actor, 1)
	authority := actor.authority()
	authority.Bound = true
	branch, err := newBranchCtx(chain.context(1, bindingBlockTime, 42), chain.key)
	require.NoError(t, err)
	host.mu.Lock()
	defer host.mu.Unlock()
	require.NoError(t, writeEnvelope(&host.node.envelope, host.cfg.NetworkID, activity))
	lxphost_authority_write(host.node, authority)
	bound := binding_authority(activity, authority)
	require.True(t, bound)
	require.NoError(t, lxphost_bind_assets(host.node, host.cfg.NetworkID, bound))
	host.node.envelope.activity.payload_hash[0] ^= 1
	before := chain.dump()
	beforeKV, err := host.moduleKV()
	require.NoError(t, err)
	err = lxphost_execute(host.node, host.cfg, branch, bound)
	require.Equal(t, Code(-104), err)
	require.Equal(t, "LXP_ERR_PAYLOAD_HASH_MISMATCH", err.Error())
	require.True(t, signature_valid(host.node))
	require.True(t, bool(host.node.asset_runtime.binding_authority))
	require.Equal(t, before, chain.dump())
	afterKV, err := host.moduleKV()
	require.NoError(t, err)
	require.Equal(t, beforeKV, afterKV)
	require.Empty(t, chain.get(moduleKVKey(1, append([]byte("asset:"), asset[:]...))))
}
