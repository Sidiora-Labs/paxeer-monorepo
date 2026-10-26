//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostFeeLimitSigned(t *testing.T) {
	const blockTime = int64(1700000000000)
	host := newTestChain(t).host(t)
	actor := newTestActor(t, 64)
	activity := actor.register(t, host, 0, blockTime)
	zero, err := host.SigningHash(activity)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(actor.public[:], zero[:], activity.Signature))
	activity.FeeLimit[15] = 1
	one, err := host.SigningHash(activity)
	require.NoError(t, err)
	require.NotEqual(t, zero, one)
	require.False(t, ed25519.Verify(actor.public[:], one[:], activity.Signature))
	activity.FeeLimit = [16]byte{7: 1}
	high, err := host.SigningHash(activity)
	require.NoError(t, err)
	require.NotEqual(t, zero, high)
	require.NotEqual(t, one, high)
	activity.FeeLimit = [16]byte{}
	restored, err := host.SigningHash(activity)
	require.NoError(t, err)
	require.Equal(t, zero, restored)
}

func TestHostFeeLimitUnpayable(t *testing.T) {
	const blockTime = int64(1700000000000)
	for name, limit := range map[string][16]byte{"low": {15: 1}, "high": {7: 1}} {
		t.Run(name, func(t *testing.T) {
			chain := newTestChain(t)
			host := chain.host(t)
			actor := newTestActor(t, 65)
			activity := actor.register(t, host, 0, blockTime)
			activity.FeeLimit = limit
			hash, err := host.SigningHash(activity)
			require.NoError(t, err)
			activity.Signature = ed25519.Sign(actor.private, hash[:])
			ctx := chain.context(1, blockTime, 1)
			before := chain.dump()
			_, err = host.Execute(ctx, activity, actor.authority())
			require.Equal(t, Code(-602), err)
			require.Equal(t, "LXP_ERR_FEE_UNPAYABLE", err.Error())
			require.Equal(t, before, chain.dump())
			receipt := execute(t, host, ctx, actor.register(t, host, 0, blockTime), actor.authority())
			require.Equal(t, Code(0), receipt.ResultCode)
		})
	}
}

func TestHostAuthorityBoundRecorded(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	first := newTestActor(t, 66)
	authority := first.authority()
	authority.Bound = true
	receipt := execute(t, host, chain.context(1, blockTime, 1), first.register(t, host, 0, blockTime), authority)
	require.Equal(t, Code(0), receipt.ResultCode)
	require.True(t, host.authorityBound())
	second := newTestActor(t, 67)
	receipt = execute(t, host, chain.context(1, blockTime, 2), second.register(t, host, 0, blockTime), second.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	require.False(t, host.authorityBound())
}
