//go:build cgo

package lxphost

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNextSequenceAfterExecute(t *testing.T) {
	const blockTime = int64(1700000000123)
	chain := newTestChain(t)
	ctx := chain.context(1, blockTime, 1)
	before := chain.dump()
	next, err := NextSequence(ctx, chain.key)
	require.NoError(t, err)
	require.Equal(t, uint64(1), next)
	require.Equal(t, before, chain.dump())

	host := chain.host(t)
	for i, sequence := range []uint64{1, 257} {
		actor := newTestActor(t, byte(i+1))
		receipt := execute(t, host, chain.context(1, blockTime, sequence),
			actor.register(t, host, 0, blockTime), actor.authority())
		require.Equal(t, Code(0), receipt.ResultCode)
		require.Equal(t, sequence, receipt.GlobalSequence)

		before = chain.dump()
		next, err = NextSequence(ctx, chain.key)
		require.NoError(t, err)
		require.Equal(t, sequence+1, next)
		require.Equal(t, before, chain.dump())

		next, err = NextSequence(ctx, chain.key)
		require.NoError(t, err)
		require.Equal(t, sequence+1, next)
		require.Equal(t, before, chain.dump())
	}
}

func TestNextSequenceRefusesCorruptCounter(t *testing.T) {
	for _, length := range []int{0, 1, 7, 9, 16} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			chain := newTestChain(t)
			ctx := chain.context(1, 1700000000123, 1)
			value := bytes.Repeat([]byte{0x01}, length)
			ctx.KVStore(chain.key).Set([]byte{lastSequenceKey}, value)
			before := chain.dump()

			next, err := NextSequence(ctx, chain.key)
			require.Equal(t, codeFatalInvariant, err)
			require.Zero(t, next)
			require.Equal(t, before, chain.dump())
		})
	}
}
