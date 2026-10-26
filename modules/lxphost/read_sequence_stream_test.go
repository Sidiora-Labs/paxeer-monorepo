//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestReadSequencesMatchCommittedCounters(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 71)
	other := newTestActor(t, 72)
	ctx := chain.context(1, blockTime, 1)
	before := chain.dump()
	last, err := host.ReadLastSequence(ctx, actor.did)
	require.NoError(t, err)
	require.Zero(t, last)
	next, err := host.ReadIdentitySequence(ctx, actor.did)
	require.NoError(t, err)
	require.Zero(t, next)
	require.Equal(t, before, chain.dump())

	registered := execute(t, host, chain.context(1, blockTime, 41),
		actor.register(t, host, 0, blockTime), actor.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	registered = execute(t, host, chain.context(1, blockTime, 257),
		other.register(t, host, 0, blockTime), other.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	granted := execute(t, host, chain.context(1, blockTime, 1000),
		actor.grant(t, host, 1, blockTime, other.public,
			uint64(blockTime)-1000, uint64(blockTime)+1000, 41), actor.authority())
	require.Equal(t, Code(0), granted.ResultCode)
	beforeRoot := chain.commit()
	before = chain.dump()
	unknown := newTestActor(t, 73)
	for _, h := range []*Host{host, chain.host(t)} {
		for _, test := range []struct {
			actor testActor
			next  uint64
		}{{actor, 2}, {other, 1}, {unknown, 0}} {
			last, err := h.ReadLastSequence(ctx, test.actor.did)
			require.NoError(t, err)
			require.Equal(t, granted.GlobalSequence, last)
			require.Equal(t, binary.BigEndian.Uint64(chain.get([]byte{lastSequenceKey})), last)
			next, err := h.ReadIdentitySequence(ctx, test.actor.did)
			require.NoError(t, err)
			require.Equal(t, test.next, next)
			if test.next != 0 {
				require.Equal(t, binary.BigEndian.Uint64(chain.get(identitySequenceKey(test.actor.id))), next)
			}
		}
	}
	require.Equal(t, before, chain.dump())
	require.Equal(t, beforeRoot, chain.commit())

	branch, _ := ctx.CacheContext()
	registered = execute(t, host, WithSequence(branch, 1001),
		unknown.register(t, host, 0, blockTime), unknown.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	last, err = host.ReadLastSequence(branch, unknown.did)
	require.NoError(t, err)
	require.Equal(t, uint64(1001), last)
	next, err = host.ReadIdentitySequence(branch, unknown.did)
	require.NoError(t, err)
	require.Equal(t, uint64(1), next)
	last, err = host.ReadLastSequence(ctx, unknown.did)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), last)
	next, err = host.ReadIdentitySequence(ctx, unknown.did)
	require.NoError(t, err)
	require.Zero(t, next)
	require.Equal(t, before, chain.dump())

	branch.KVStore(chain.key).Set([]byte{lastSequenceKey}, binary.BigEndian.AppendUint64(nil, math.MaxUint64))
	last, err = host.ReadLastSequence(branch, actor.did)
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), last)
	branch.KVStore(chain.key).Set([]byte{lastSequenceKey}, []byte{1})
	last, err = host.ReadLastSequence(branch, actor.did)
	require.Equal(t, codeFatalInvariant, err)
	require.Zero(t, last)
	branch.KVStore(chain.key).Set(identitySequenceKey(actor.id), []byte{1})
	next, err = host.ReadIdentitySequence(branch, actor.did)
	require.Equal(t, codeFatalInvariant, err)
	require.Zero(t, next)
	next, err = host.ReadIdentitySequence(ctx, make([]byte, 513))
	require.Equal(t, codeNonCanonical, err)
	require.Zero(t, next)
	require.Equal(t, before, chain.dump())
}

func TestReadStreamAccruedDoesNotWrite(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	ctx := chain.context(1, 1001, 1)
	id := [32]byte{1}
	record := append([]byte(nil), id[:]...)
	for _, value := range []byte{2, 3, 4, 5} {
		field := [32]byte{value}
		record = append(record, field[:]...)
	}
	record = append(record, 1)
	record = binary.BigEndian.AppendUint64(record, 0)
	record = binary.BigEndian.AppendUint64(record, 7)
	for _, value := range []uint64{3, 1000, 1001, 1010} {
		record = binary.BigEndian.AppendUint64(record, value)
	}
	for _, value := range []uint64{1000, 2, 1, 1} {
		record = binary.BigEndian.AppendUint64(record, 0)
		record = binary.BigEndian.AppendUint64(record, value)
	}
	record = append(record, make([]byte, 8+1+8*32+3)...)
	require.Len(t, record, 541)
	key := moduleKVKey(4, append([]byte("stream:"), id[:]...))
	ctx.KVStore(chain.key).Set(key, record)
	beforeRoot := chain.commit()
	before := chain.dump()
	for _, h := range []*Host{host, chain.host(t)} {
		for _, test := range []struct {
			at   uint64
			want int64
		}{{1001, 0}, {1005, 9}, {1005, 9}, {1010, 21}, {2000, 21}, {1002, 2}} {
			amount, err := h.ReadStreamAccrued(ctx, id, test.at)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(test.want), amount)
			require.Equal(t, record, chain.get(key))
			require.Equal(t, before, chain.dump())
		}
		amount, err := h.ReadStreamAccrued(ctx, id, 1000)
		require.Equal(t, Code(-717), err)
		require.Equal(t, sdk.ZeroInt(), amount)
		amount, err = h.ReadStreamAccrued(ctx, [32]byte{9}, 1005)
		require.Equal(t, Code(-7), err)
		require.Equal(t, sdk.ZeroInt(), amount)
	}
	require.Equal(t, beforeRoot, chain.commit())
	for _, flag := range []int{538, 539, 540} {
		branch, _ := ctx.CacheContext()
		inactive := append([]byte(nil), record...)
		inactive[flag] = 1
		branch.KVStore(chain.key).Set(key, inactive)
		amount, err := host.ReadStreamAccrued(branch, id, 1005)
		require.NoError(t, err)
		require.Equal(t, sdk.ZeroInt(), amount)
		require.Equal(t, inactive, branch.KVStore(chain.key).Get(key))
	}
	branch, _ := ctx.CacheContext()
	capped := append([]byte(nil), record...)
	binary.BigEndian.PutUint64(capped[217:225], 5)
	branch.KVStore(chain.key).Set(key, capped)
	amount, err := host.ReadStreamAccrued(branch, id, 1005)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(3), amount)
	require.Equal(t, capped, branch.KVStore(chain.key).Get(key))
	require.Equal(t, before, chain.dump())
}

func TestReadFeeAndMeteringSchedules(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	resolutionSeedMetering(t, chain)
	ctx := chain.context(1, resolutionBlockTime, 1)
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for _, price := range []uint64{1, 2, 3, 4, 5, 6, 7} {
		fee = binary.BigEndian.AppendUint64(fee, price)
	}
	fee = append(fee, bytes.Repeat([]byte{0xa5}, 32)...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, bytes.Repeat([]byte{0xa5}, 32)...)
	fee = append(fee, make([]byte, 16)...)
	require.Len(t, fee, 217)
	ctx.KVStore(chain.key).Set(moduleKVKey(9, []byte("progfee/active/v1")), fee)
	ctx.KVStore(chain.key).Set(moduleKVKey(9, append([]byte("progfee/history/v1/"), 0, 0, 0, 1)), fee)
	metering := append([]byte(nil), chain.get(moduleKVKey(9, []byte("progmet/active/v1")))...)
	beforeRoot := chain.commit()
	before := chain.dump()
	for _, h := range []*Host{host, chain.host(t)} {
		for _, test := range []struct {
			read func(sdk.Context) ([]byte, error)
			want []byte
		}{{h.ReadFeeSchedule, fee}, {h.ReadMeteringSchedule, metering}} {
			encoded, err := test.read(ctx)
			require.NoError(t, err)
			require.Equal(t, test.want, encoded)
			encoded[0] ^= 0xff
			again, err := test.read(ctx)
			require.NoError(t, err)
			require.Equal(t, test.want, again)
		}
		pending, err := h.ReadPendingFeeSchedule(ctx)
		require.Equal(t, Code(-7), err)
		require.Nil(t, pending)
		request, err := h.ReadWebRequest(ctx, [32]byte{1}, 257)
		require.Equal(t, Code(-7), err)
		require.Nil(t, request)
	}
	require.Equal(t, before, chain.dump())
	require.Equal(t, beforeRoot, chain.commit())

	branch, _ := ctx.CacheContext()
	programID := [32]byte{1}
	requestKey := binary.BigEndian.AppendUint64(append([]byte("web/pending"), programID[:]...), 257)
	request := append([]byte{1, 1}, bytes.Repeat([]byte{0xa5}, 96)...)
	request = binary.BigEndian.AppendUint64(request, 0)
	request = binary.BigEndian.AppendUint64(request, 7)
	request = binary.BigEndian.AppendUint64(request, 41)
	request = append(request, 0)
	require.Len(t, request, 123)
	branch.KVStore(chain.key).Set(moduleKVKey(9, requestKey), request)
	encoded, err := host.ReadWebRequest(branch, programID, 257)
	require.NoError(t, err)
	require.Equal(t, request, encoded)
	encoded[0] ^= 1
	again, err := host.ReadWebRequest(branch, programID, 257)
	require.NoError(t, err)
	require.Equal(t, request, again)
	for _, test := range []struct {
		program [32]byte
		id      uint64
	}{{programID, 258}, {[32]byte{2}, 257}} {
		missing, err := host.ReadWebRequest(branch, test.program, test.id)
		require.Equal(t, Code(-7), err)
		require.Nil(t, missing)
	}
	missing, err := host.ReadWebRequest(ctx, programID, 257)
	require.Equal(t, Code(-7), err)
	require.Nil(t, missing)
	require.Equal(t, before, chain.dump())
}
