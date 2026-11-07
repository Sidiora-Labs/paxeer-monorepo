package sequencer_test

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func newAdmitNode(t *testing.T, sender utils.TestAcct) (*harness, *sequencer.Node, int64) {
	t.Helper()
	h := newNodeHarness(t, sender)
	// The harness activates at height 3. Commit the final plain block before
	// starting the primary so its first open layer is sequenced.
	_, err := h.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: openHeight, Time: h.genesis.Add(time.Second)},
	})
	require.NoError(t, err)
	h.app.SetDeliverStateToCommit()
	_, err = h.app.Commit(t.Context())
	require.NoError(t, err)
	height := h.app.LastBlockHeight() + 1
	require.True(t, h.app.Sequenced(h.app.GetCheckCtx(), height))

	info, err := h.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.StandbyAddress = ""
	cfg.WALDir = filepath.Join(t.TempDir(), "wal")
	cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
	cfg.LeaseDuration = 3 * replicationTimeout
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 1, 2
	n, err := sequencer.NewNode(cfg, h.app)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, n.Start(ctx))
	require.True(t, n.Replicator.Standalone())
	require.True(t, n.ConsensusSource.Sequenced(height))
	h.exec = n.Executor
	return h, n, height
}

func TestConsensusSourceAdmitsCosmosTransaction(t *testing.T) {
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	h, n, height := newAdmitNode(t, sender)
	before := h.openCtx()
	nonce := h.app.AccountKeeper.GetAccount(before, sender.AccountAddress).GetSequence()
	tx := signCosmos(t, before, h, sender, 200_000,
		banktypes.NewMsgSend(sender.AccountAddress, recipient.AccountAddress,
			sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(7)))))
	hash := sha256.Sum256(tx)

	admitted, err := n.ConsensusSource.Admit(t.Context(), height, tmtypes.Txs{tx})
	require.NoError(t, err)
	require.Equal(t, 1, admitted)
	require.Equal(t, nonce+1, h.app.AccountKeeper.GetAccount(h.openCtx(), sender.AccountAddress).GetSequence())
	require.Equal(t, sdk.NewInt(7), h.app.BankKeeper.GetBalance(h.openCtx(), recipient.AccountAddress, "uhpx").Amount)

	block, err := n.ConsensusSource.Seal(t.Context(), height)
	require.NoError(t, err)
	require.NotEmpty(t, block.Batch)
	require.Equal(t, tmtypes.Txs{tx}, block.Txs)
	require.Equal(t, [][]byte{block.Batch, tx}, n.ConsensusSource.Sealed().Txs)
	require.Equal(t, hash, sha256.Sum256(block.Txs[0]))
	send, err := n.ReceiptSource.SequencerSend(common.Hash(hash))
	require.NoError(t, err)
	require.NotNil(t, send)
	receipt, err := verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, hash, receipt.Receipt.ActivityID)
	require.Equal(t, sourceLastSeq+1, receipt.Receipt.GlobalSequence)
	require.Equal(t, nonce, receipt.Receipt.FromSequence)
	require.Equal(t, sequencer.CosmosOperation, receipt.Receipt.Operation)
	require.Zero(t, receipt.Receipt.ResultCode)

	admitted, err = n.ConsensusSource.Admit(t.Context(), height, tmtypes.Txs{tx})
	require.NoError(t, err)
	require.Zero(t, admitted, "a sealed height cannot admit on a proposal retry")
}

func TestConsensusSourceAdmitRefusalLeavesTheLayer(t *testing.T) {
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	h, n, height := newAdmitNode(t, sender)
	before := h.openCtx()
	nonce := h.app.AccountKeeper.GetAccount(before, sender.AccountAddress).GetSequence()
	msg := banktypes.NewMsgSend(sender.AccountAddress, recipient.AccountAddress,
		sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(7))))
	good := signCosmos(t, before, h, sender, 200_000, msg)
	wrongSequence, _ := before.CacheContext()
	account := h.app.AccountKeeper.GetAccount(wrongSequence, sender.AccountAddress)
	require.NoError(t, account.SetSequence(nonce+1))
	h.app.AccountKeeper.SetAccount(wrongSequence, account)
	bad := signCosmos(t, wrongSequence, h, sender, 200_000, msg)
	require.NotEqual(t, good, bad)

	admitted, err := n.ConsensusSource.Admit(t.Context(), height, tmtypes.Txs{bad, good})
	require.NoError(t, err)
	require.Equal(t, 1, admitted)
	require.Equal(t, nonce+1, h.app.AccountKeeper.GetAccount(h.openCtx(), sender.AccountAddress).GetSequence())
	require.Equal(t, sdk.NewInt(7), h.app.BankKeeper.GetBalance(h.openCtx(), recipient.AccountAddress, "uhpx").Amount)

	block, err := n.ConsensusSource.Seal(t.Context(), height)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{good}, block.Txs)
	require.Equal(t, [][]byte{block.Batch, good}, n.ConsensusSource.Sealed().Txs)
	require.Equal(t, sourceLastSeq+1, n.ConsensusSource.Sealed().Batch.ToSeq)
	refused, err := n.ReceiptSource.SequencerSend(common.Hash(sha256.Sum256(bad)))
	require.NoError(t, err)
	require.Nil(t, refused)
	accepted, err := n.ReceiptSource.SequencerSend(common.Hash(sha256.Sum256(good)))
	require.NoError(t, err)
	require.NotNil(t, accepted)
}
