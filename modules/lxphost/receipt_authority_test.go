//go:build cgo

package lxphost

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestHostFillsHeightAuthorityIndex(t *testing.T) {
	const blockTime = int64(1700000000000)
	source := newTestChain(t)
	producer := source.host(t)
	actor := newTestActor(t, 91)
	receipt := execute(t, producer, source.context(1, blockTime, 1), actor.register(t, producer, 0, blockTime), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	producer.Close()

	chain := newTestChain(t)
	for restart := 0; restart < 2; restart++ {
		host := chain.host(t)
		if restart != 0 {
			previous := newTestActor(t, 92)
			restored, err := host.ReadIdentityState(chain.context(2, blockTime, 3), previous.did)
			require.NoError(t, err)
			require.Equal(t, previous.id, restored.DIDID)
		}
		consumer := newTestActor(t, byte(92+restart*2))
		activity := consumer.register(t, host, 0, blockTime)
		activity.ReceiptAuthorities = []ReceiptAuthority{{CanonicalReceipt: receipt.Encoded, BlockHeight: 1}}
		sequence := uint64(restart*2 + 1)
		got := execute(t, host, chain.context(2, blockTime, sequence), activity, consumer.authority())
		require.Equal(t, Code(0), got.ResultCode)
		index := host.node.execution.verified_receipts
		require.Equal(t, unsafe.Pointer(&host.node.verified_receipts), unsafe.Pointer(index))
		require.NotNil(t, index)
		require.Equal(t, uint64(1), uint64(index.authority_count))
		facts := &index.authority_entries[0]
		require.Equal(t, uint16(2), uint16(facts.version))
		require.Equal(t, uint32(testNetworkID), uint32(facts.network_id))
		require.Equal(t, uint16(stateCommitment), uint16(facts.protocol_version))
		require.Equal(t, uint16(governanceModule), uint16(facts.module_id))
		require.Equal(t, uint64(1), uint64(facts.block_height))
		require.Equal(t, receipt.ResultCode, Code(facts.result_code))
		require.Equal(t, receipt.PreviousStateRoot, *(*[32]byte)(unsafe.Pointer(&facts.previous_state_root[0])))
		require.Equal(t, receipt.ResultingStateRoot, *(*[32]byte)(unsafe.Pointer(&facts.resulting_state_root[0])))
		require.NotEqual(t, [32]byte{}, *(*[32]byte)(unsafe.Pointer(&facts.receipt_digest[0])))

		nextActor := newTestActor(t, byte(93+restart*2))
		next := execute(t, host, chain.context(2, blockTime, sequence+1), nextActor.register(t, host, 0, blockTime), nextActor.authority())
		require.Equal(t, Code(0), next.ResultCode)
		require.Equal(t, uint64(0), uint64(host.node.execution.verified_receipts.authority_count))
		chain.commit()
		host.Close()
	}
}

func TestHostRefusesFutureHeightAuthority(t *testing.T) {
	const blockTime = int64(1700000000000)
	source := newTestChain(t)
	producer := source.host(t)
	actor := newTestActor(t, 98)
	receipt := execute(t, producer, source.context(1, blockTime, 1), actor.register(t, producer, 0, blockTime), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	for _, height := range []uint64{2, 3} {
		chain := newTestChain(t)
		host := chain.host(t)
		consumer := newTestActor(t, 99)
		activity := consumer.register(t, host, 0, blockTime)
		activity.ReceiptAuthorities = []ReceiptAuthority{{CanonicalReceipt: receipt.Encoded, BlockHeight: height}}
		before := chain.dump()
		_, err := host.HostExecute(chain.context(2, blockTime, 1), activity, consumer.authority())
		require.Equal(t, codeNonCanonical, err, "authority height %d", height)
		require.Equal(t, "LXP_ERR_NON_CANONICAL", err.Error())
		require.Equal(t, before, chain.dump())
		activity.ReceiptAuthorities[0].BlockHeight = 1
		accepted := execute(t, host, chain.context(2, blockTime, 1), activity, consumer.authority())
		require.Equal(t, Code(0), accepted.ResultCode)
	}
}
