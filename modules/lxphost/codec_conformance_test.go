//go:build cgo

package lxphost

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func codecHostReceipts(t *testing.T) []struct {
	name    string
	receipt Receipt
} {
	t.Helper()
	const blockTime = int64(1700000000123)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 0x71)
	asset := newTestAsset(t, actor, 0x72)

	governance := execute(t, host, chain.context(1, blockTime, 41),
		actor.register(t, host, 0, blockTime), actor.authority())
	require.Equal(t, Code(0), governance.ResultCode)
	registered := execute(t, host, chain.context(2, blockTime+1000, 42),
		actor.registerAsset(t, host, 1, blockTime+1000, asset, 0x72, 1000), actor.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	opened := execute(t, host, chain.context(3, blockTime+2000, 43),
		actor.openAssetAccount(t, host, 2, blockTime+2000, asset), actor.authority())
	require.Equal(t, Code(0), opened.ResultCode)
	minted := execute(t, host, chain.context(4, blockTime+3000, 44),
		actor.mint(t, host, 3, blockTime+3000, asset, 250), actor.authority())
	require.Equal(t, Code(0), minted.ResultCode)

	return []struct {
		name    string
		receipt Receipt
	}{
		{"register", registered},
		{"mint", minted},
		{"governance", governance},
	}
}

func TestCodecDecodesHostReceipts(t *testing.T) {
	for _, execution := range codecHostReceipts(t) {
		t.Run(execution.name, func(t *testing.T) {
			receipt := execution.receipt
			decoded, err := codec.DecodeReceipt(receipt.Encoded)
			require.NoError(t, err)
			require.Equal(t, receipt.GlobalSequence, decoded.GlobalSequence)
			require.Equal(t, receipt.Timestamp, decoded.Timestamp)
			require.Equal(t, int32(receipt.ResultCode), decoded.ResultCode)
			require.Equal(t, receipt.PreviousStateRoot, decoded.PreviousStateRoot)
			require.Equal(t, receipt.ResultingStateRoot, decoded.ResultingStateRoot)
			require.True(t, bytes.HasPrefix(receipt.Encoded, decoded.UnsignedBytes()),
				"unsigned codec receipt must be a prefix of the kernel encoding")
		})
	}
}

func TestCodecProtocolReceiptShapeOnHostReceipts(t *testing.T) {
	for _, execution := range codecHostReceipts(t) {
		t.Run(execution.name, func(t *testing.T) {
			encoded := execution.receipt.Encoded
			require.NotEmpty(t, encoded)
			require.True(t, codec.IsProtocolReceiptShape(encoded),
				"kernel receipt must have the protocol receipt shape")
			_, err := codec.DecodeReceipt(encoded[:len(encoded)-1])
			require.ErrorIs(t, err, codec.ErrTruncated,
				"codec must reject a kernel receipt truncated by one byte")
		})
	}
}
