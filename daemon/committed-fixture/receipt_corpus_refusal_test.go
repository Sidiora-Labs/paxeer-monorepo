package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestReceiptCorpusPreexecutionRefusal(t *testing.T) {
	data, err := os.ReadFile("../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json")
	require.NoError(t, err)
	var source struct {
		Cases []struct {
			ID             string                        `json:"id"`
			RawTransaction hexutil.Bytes                 `json:"rawTransaction"`
			SignedReceipt  hexutil.Bytes                 `json:"signedReceipt"`
			Receipt        evmrpc.SequencerReceiptResult `json:"receiptJson"`
			LXPCode        string                        `json:"lxpCode"`
			PinnedKey      struct {
				PublicKey hexutil.Bytes `json:"publicKey"`
			} `json:"pinnedKey"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(data, &source))
	found := 0
	for _, c := range source.Cases {
		if c.ID != "V2-pax-transfer-insufficient-funds" {
			continue
		}
		found++
		require.NoError(t, c.Receipt.Verify())
		verified, err := verify.ReceiptSignature(c.SignedReceipt, [32]byte(c.PinnedKey.PublicKey))
		require.NoError(t, err)
		require.Equal(t, []byte(c.SignedReceipt), []byte(c.Receipt.Receipt))
		require.EqualValues(t, 0, c.Receipt.Status)
		require.Empty(t, c.Receipt.ReturnData)
		require.Empty(t, c.Receipt.Logs)
		require.Equal(t, "insufficient balance for transfer", c.Receipt.VMError)
		require.Empty(t, c.LXPCode)
		require.EqualValues(t, 1, verified.Receipt.ResultCode)
		require.Len(t, verified.Receipt.Effects, 1)
	}
	require.Equal(t, 1, found, "retained corpus must contain the preexecution refusal exactly once")
}
