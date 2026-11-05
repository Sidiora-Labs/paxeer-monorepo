//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostWritesNoIdempotencyRows(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	for sequence := uint64(1); sequence <= 2; sequence++ {
		actor := newTestActor(t, byte(sequence))
		activity := actor.register(t, host, 0, blockTime)
		require.NotEqual(t, [32]byte{}, activity.IdempotencyKey)

		receipt, err := host.Execute(chain.context(1, blockTime, sequence), activity, actor.authority())
		require.NoError(t, err)
		require.Equal(t, Code(0), receipt.ResultCode)
		require.Equal(t, sequence, receipt.GlobalSequence)
		require.NotEmpty(t, receipt.Encoded)
		chain.commit()

		require.Equal(t, binary.BigEndian.AppendUint64(nil, 1), chain.get(identitySequenceKey(actor.id)))
		for key := range chain.dump() {
			require.False(t, bytes.HasPrefix([]byte(key), []byte{idempotencyPrefix}), "unexpected idempotency row %x", key)
		}
	}
}

func TestHostLegacyIdempotencyRowsIgnored(t *testing.T) {
	const blockTime = int64(1700000000000)
	for _, test := range []struct {
		name string
		key  []byte
	}{
		{"prefix only", []byte{idempotencyPrefix}},
		{"short key", []byte{idempotencyPrefix, 1}},
		{"malformed snapshot", append([]byte{idempotencyPrefix}, bytes.Repeat([]byte{1}, 32)...)},
		{"long key", append([]byte{idempotencyPrefix}, bytes.Repeat([]byte{1}, 33)...)},
	} {
		t.Run(test.name, func(t *testing.T) {
			chain := newTestChain(t)
			legacy := []byte{0xff}
			chain.context(1, blockTime, 1).KVStore(chain.key).Set(test.key, legacy)
			chain.commit()

			for sequence := uint64(1); sequence <= 2; sequence++ {
				host := chain.host(t)
				actor := newTestActor(t, byte(sequence))
				activity := actor.register(t, host, 0, blockTime)
				receipt, err := host.Execute(chain.context(int64(sequence), blockTime, sequence), activity, actor.authority())
				require.NoError(t, err)
				require.Equal(t, Code(0), receipt.ResultCode)
				require.Equal(t, sequence, receipt.GlobalSequence)
				require.NotEmpty(t, receipt.Encoded)
				chain.commit()

				require.Equal(t, binary.BigEndian.AppendUint64(nil, 1), chain.get(identitySequenceKey(actor.id)))
				rows := map[string][]byte{}
				for key, value := range chain.dump() {
					if bytes.HasPrefix([]byte(key), []byte{idempotencyPrefix}) {
						rows[key] = value
					}
				}
				require.Equal(t, map[string][]byte{string(test.key): legacy}, rows)
			}
		})
	}
}
