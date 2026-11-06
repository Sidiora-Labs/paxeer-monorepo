package evmrpc_test

import (
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

const repair2231491CaseID = "V2-pax-transfer-insufficient-funds"

func repair2231491Corpus(t *testing.T) (seqVecFile, seqVecCase) {
	t.Helper()
	data, err := os.ReadFile(seqVecFixture)
	require.NoError(t, err)
	var corpus seqVecFile
	require.NoError(t, json.Unmarshal(data, &corpus))
	var matches []seqVecCase
	for _, c := range corpus.Cases {
		if c.ID == repair2231491CaseID {
			matches = append(matches, c)
		}
	}
	require.Len(t, matches, 1, "decision 2537 requires the committed preexecution refusal; gotcha 3024")
	return corpus, matches[0]
}

func TestRepair_22_31_491_PreexecutionRefusal(t *testing.T) {
	_, c := repair2231491Corpus(t)
	require.Equal(t, "V2", c.Vector)
	require.Equal(t, seqVecExpected{Rung: "receipted", Verifier: "accepts"}, c.Expected)
	check, err := seqVecClientCheck(c.RawTransaction, c.ReceiptJSON, c.PrevSeqHash, c.PinnedKey)
	require.NoError(t, err, check)
	var result evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(c.ReceiptJSON, &result))
	require.EqualValues(t, ethtypes.ReceiptStatusFailed, result.Status)
	require.Empty(t, result.ReturnData)
	require.Empty(t, result.Logs)
	require.NotEmpty(t, result.VMError)
	require.Equal(t, vm.ErrInsufficientBalance.Error(), result.VMError)
	require.Empty(t, c.LXPCode)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(c.ReceiptJSON, &fields))
	require.NotContains(t, fields, "ledgerResultCode")
	require.NotContains(t, fields, "lxpCode")
	verified, err := verify.ReceiptSignature(c.SignedReceipt, [32]byte(c.PinnedKey.PublicKey))
	require.NoError(t, err)
	require.Equal(t, c.ReceiptDigest, common.Hash(verified.Digest))
	decoded := verified.Receipt
	require.EqualValues(t, 1, decoded.ResultCode, "sequencer failure code is separate from a ledger result code")
	require.Equal(t, sha256.Sum256(nil), decoded.ActivityRoot)
	require.Len(t, decoded.Effects, 1)
	require.Equal(t, seqVecModuleID, decoded.Effects[0].ModuleID)
	kernelError := sha256.Sum256([]byte(result.VMError))
	require.Equal(t, kernelError[:], decoded.Effects[0].Body[68:100])
	var tx ethtypes.Transaction
	require.NoError(t, tx.UnmarshalBinary(c.RawTransaction))
	require.Equal(t, seqVecRecipient, *tx.To())
	require.Empty(t, tx.Data())
	before := decoded.FromBalanceBefore.Bytes()
	amount := decoded.Amount.Bytes()
	require.NotEqual(t, [16]byte{}, before, "sender is funded")
	require.Positive(t, new(big.Int).SetBytes(amount[:]).Cmp(new(big.Int).SetBytes(before[:])), "transfer value exceeds the authenticated sender balance")
	require.Equal(t, decoded.FromBalanceBefore, decoded.FromBalanceAfter)
	require.Equal(t, decoded.ToBalanceBefore, decoded.ToBalanceAfter)
}

func repair2231491Terminal0Cases(t *testing.T, corpus seqVecFile) []string {
	t.Helper()
	var matches []string
	for _, c := range corpus.Cases {
		if c.Vector == "V3" {
			continue // These cases deliberately fail receipt authentication.
		}
		check, err := seqVecClientCheck(c.RawTransaction, c.ReceiptJSON, c.PrevSeqHash, c.PinnedKey)
		require.NoError(t, err, "%s: %s", c.ID, check)
		var result evmrpc.SequencerReceiptResult
		require.NoError(t, json.Unmarshal(c.ReceiptJSON, &result))
		if result.Status == 0 && len(result.ReturnData) == 0 && len(result.Logs) == 0 && result.VMError != "" {
			matches = append(matches, c.ID)
		}
	}
	return matches
}

func TestRepair_22_31_491_Terminal0(t *testing.T) {
	corpus, refusal := repair2231491Corpus(t)
	require.Equal(t, []string{repair2231491CaseID}, repair2231491Terminal0Cases(t, corpus))
	chain := newSeqReceiptChain9005(t)
	installSeqVecReceiptKey(t, chain, refusal.PinnedKey)
	for _, c := range corpus.Cases {
		if c.Vector == "V3" {
			continue
		}
		verified, err := verify.ReceiptSignature(c.SignedReceipt, [32]byte(c.PinnedKey.PublicKey))
		require.NoError(t, err)
		decoded := verified.Receipt
		require.Equal(t, chain.seqHashes[decoded.GlobalSequence-1], decoded.PreviousStateRoot)
		chain.seqHashes[decoded.GlobalSequence] = decoded.ResultingStateRoot
		chain.activities[decoded.GlobalSequence] = decoded.ActivityID
	}
	var retained evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(refusal.ReceiptJSON, &retained))
	source := &seqVecSource{sends: map[common.Hash]*evmrpc.SequencerSend{
		retained.TxHash: {
			Nonce: uint64(retained.Nonce), Receipt: refusal.SignedReceipt,
			ReturnData: retained.ReturnData, VMError: retained.VMError,
		},
	}}
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider,
		evmrpc.SequencerReceiptOption{Source: source, Keeper: &chain.keeper})
	_, before, _ := seqReceiptResult9005(t, port, retained.TxHash)
	require.Equal(t, evmrpc.RungReceipted, before.Rung)
	for height := chain.latest + 1; height < int64(retained.Height); height++ {
		chain.commit9005(height)
	}
	chain.commitBatch9005(int64(retained.Height), uint64(retained.Seq))
	_, committed, _ := seqReceiptResult9005(t, port, retained.TxHash)
	require.Equal(t, evmrpc.RungCommitted, committed.Rung)
	require.NoError(t, committed.Verify())
	require.Equal(t, retained.Receipt, committed.Receipt, "commit preserves the authenticated refusal")
	require.NotNil(t, committed.BlockNumber)
	require.Equal(t, retained.Height, *committed.BlockNumber)
	require.NotNil(t, committed.BlockHash)
	require.NotEqual(t, common.Hash{}, *committed.BlockHash)
	require.Zero(t, uint64(committed.Status))
	require.Empty(t, committed.ReturnData)
	require.Empty(t, committed.Logs)
	require.Equal(t, vm.ErrInsufficientBalance.Error(), committed.VMError)
	require.Zero(t, uint64(committed.GasUsed))
	require.NotNil(t, committed.FeeAmount)
	require.Zero(t, committed.FeeAmount.ToInt().Sign())
	verified, err := verify.ReceiptSignature(committed.Receipt, [32]byte(refusal.PinnedKey.PublicKey))
	require.NoError(t, err)
	decoded := verified.Receipt
	require.Equal(t, decoded.FromBalanceBefore, decoded.FromBalanceAfter)
	require.Equal(t, decoded.ToBalanceBefore, decoded.ToBalanceAfter)
	require.Zero(t, decoded.FeeCharged)
	require.Equal(t, sha256.Sum256(nil), decoded.ActivityRoot)
	require.Len(t, decoded.Effects, 1, "no execution logs or additional effects")
	require.Equal(t, seqVecModuleID, decoded.Effects[0].ModuleID)
	kernelError := sha256.Sum256([]byte(committed.VMError))
	require.Equal(t, kernelError[:], decoded.Effects[0].Body[68:100])
	var fields map[string]json.RawMessage
	encoded, err := json.Marshal(committed)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.NotContains(t, fields, "ledgerResultCode")
	require.NotContains(t, fields, "lxpCode")
	var remaining []seqVecCase
	for _, c := range corpus.Cases {
		if c.ID != repair2231491CaseID {
			remaining = append(remaining, c)
		}
	}
	corpus.Cases = remaining
	require.Empty(t, repair2231491Terminal0Cases(t, corpus), "terminal-0 requires the new retained case")
}
