//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostEd25519ConformsToGo(t *testing.T) {
	const blockTime = int64(1700000000000)
	for seed := 0; seed < 64; seed++ {
		t.Run(fmt.Sprintf("seed_%02d", seed), func(t *testing.T) {
			chain := newTestChain(t)
			host := chain.host(t)
			actor := newTestActor(t, byte(seed))
			activity := actor.register(t, host, 0, blockTime)
			hash, err := host.SigningHash(activity)
			require.NoError(t, err)
			require.Equal(t, ed25519.Sign(actor.private, hash[:]), activity.Signature)
			require.True(t, ed25519.Verify(actor.public[:], hash[:], activity.Signature))

			receipt, err := host.Execute(chain.context(1, blockTime, 1), activity, actor.authority())
			require.NoError(t, err)
			require.Equal(t, Code(0), receipt.ResultCode)
			require.Equal(t, "LXP_OK", receipt.ResultCode.Error())
		})
	}
}

func TestHostEd25519RejectsNonCanonicalS(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 64)
	activity := actor.register(t, host, 0, blockTime)
	hash, err := host.SigningHash(activity)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(actor.public[:], hash[:], activity.Signature))
	canonical := append([]byte(nil), activity.Signature...)
	groupOrder := [32]byte{
		0xed, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58,
		0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde, 0x14,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
	}
	var carry uint16
	for i, orderByte := range groupOrder {
		sum := uint16(activity.Signature[32+i]) + uint16(orderByte) + carry
		activity.Signature[32+i] = byte(sum)
		carry = sum >> 8
	}
	require.Zero(t, carry)
	require.Equal(t, canonical[:32], activity.Signature[:32])
	require.NotEqual(t, canonical[32:], activity.Signature[32:])
	require.False(t, ed25519.Verify(actor.public[:], hash[:], activity.Signature))

	ctx := chain.context(1, blockTime, 1)
	before := chain.dump()
	_, err = host.Execute(ctx, activity, actor.authority())
	require.Equal(t, Code(-201), err)
	require.EqualError(t, err, "LXP_ERR_BAD_SIGNATURE")
	require.Equal(t, before, chain.dump())

	activity.Signature = canonical
	receipt, err := host.Execute(ctx, activity, actor.authority())
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
}
