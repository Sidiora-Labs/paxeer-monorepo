package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	protoio "github.com/gogo/protobuf/io"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestStandbyReplicatesRecorderGroupsThroughOpenAccount(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	newPV := func(name string) *privval.FilePV {
		t.Helper()
		pv, err := privval.GenFilePV(filepath.Join(dir, name+".json"), filepath.Join(dir, name+"-state.json"), "")
		require.NoError(t, err)
		require.NoError(t, pv.Save())
		return pv
	}
	primaryPV, standbyPV := newPV("primary"), newPV("standby")
	primaryPub, err := primaryPV.GetPubKey(ctx)
	require.NoError(t, err)
	standbyPub, err := standbyPV.GetPubKey(ctx)
	require.NoError(t, err)
	primaryPrivate := ed25519.NewKeyFromSeed(sequencerKeySeed)
	standbySeed := sha256.Sum256([]byte("recorder standby receipt key"))
	standbyPrivate := ed25519.NewKeyFromSeed(standbySeed[:])
	standbyID := sha256.Sum256([]byte("recorder standby receipt id"))
	at := time.Unix(1_800_000_000, 0).UTC()
	w := app.NewTestWrapper(t, at, &sdked25519.PubKey{Key: primaryPub.Bytes()}, true, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
	})
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	activate(t, w)
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
	require.NoError(t, w.App.SequencerKeeper.SetLeaders(w.Ctx, [][32]byte{[32]byte(primaryPub.Bytes()), [32]byte(standbyPub.Bytes())}))
	require.NoError(t, w.App.SequencerKeeper.SetReceiptKeys(w.Ctx, seqkeeper.ReceiptKeys{
		{KeyId: sequencerKeyID, PublicKey: primaryPrivate.Public().(ed25519.PublicKey), FirstHeight: 1, LastHeight: math.MaxUint64},
		{KeyId: standbyID[:], PublicKey: standbyPrivate.Public().(ed25519.PublicKey), FirstHeight: 2, LastHeight: 1000},
	}))
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	w.App.StoreConsensusParams(w.Ctx, params)
	funder, issuer, holder := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	w.App.EvmKeeper.SetAddressMapping(w.Ctx, funder.AccountAddress, funder.EvmAddress)
	w.FundAcc(funder.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", seqCtxFundingUhpx)))
	votes := []abci.VoteInfo{{Validator: abci.Validator{Address: primaryPub.Address(), Power: 100}, SignedLastBlock: true}}
	require.NoError(t, w.App.SequencerKeeper.StoreVotes(w.Ctx, 1, votes))
	commitBeforeSequencedProposal(t, w)
	var snapshot bytes.Buffer
	require.NoError(t, w.App.CommitMultiStore().Snapshot(1, protoio.NewDelimitedWriter(&snapshot)))
	backup := app.Setup(t, true, true, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetStoreLoader(func(ms sdk.CommitMultiStore) error {
			if err := ms.LoadLatestVersion(); err != nil {
				return err
			}
			_, err := ms.Restore(1, 0, protoio.NewDelimitedReader(bytes.NewReader(snapshot.Bytes()), 64<<20))
			return err
		})
	})
	t.Cleanup(func() { require.NoError(t, backup.Close()) })
	backup.ChainID = sequencedChainID
	require.Equal(t, w.App.LastCommitID(), backup.LastCommitID())
	info, err := backup.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	keyBytes, err := os.ReadFile(filepath.Join(dir, "standby.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(consensus.KeyFile(), keyBytes, 0o600))
	stateBytes, err := os.ReadFile(filepath.Join(dir, "standby-state.json"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.StateFile()), 0o700))
	require.NoError(t, os.WriteFile(consensus.StateFile(), stateBytes, 0o600))
	writeSeed := func(name string, seed []byte) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(hex.EncodeToString(seed)), 0o600))
		return path
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := l.Addr().String()
	require.NoError(t, l.Close())
	standbyCtx, cancel := context.WithCancel(ctx)
	standby, err := sequencer.StartStandby(standbyCtx, sequencer.Config{
		Role: sequencer.RoleStandby, WALDir: filepath.Join(dir, "standby-wal"),
		ReceiptKeyFile: writeSeed("standby-receipt", standbySeed[:]),
		StandbyAddress: address, ReplicationTimeout: 2 * time.Second,
	}, backup)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		require.Eventually(t, func() bool { return standby.Err() != nil }, 2*time.Second, time.Millisecond)
	})
	primaryWAL, err := wal.OpenWAL(filepath.Join(dir, "primary-wal"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(primaryWAL.Close)
	repl, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: primaryWAL, Dial: sequencer.DialStandby(address, 2*time.Second), ReplicationTimeout: 2 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(repl.Close)
	keyPins, err := w.App.SequencerKeeper.ReceiptKeyPins(w.App.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(writeSeed("primary-receipt", sequencerKeySeed), filepath.Join(dir, "primary.json"), keyPins, 2)
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(w.App, sequencer.Options{ChainID: sequencedChainID, ProposerAddress: primaryPub.Address()})
	require.NoError(t, err)
	receipter, err := sequencer.NewReceipter(exec, w.App, &w.App.EvmKeeper, key, app.EVMReceiptModuleID)
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(w.App, exec, key, sequencer.WithConsensusReplicator(repl))
	require.NoError(t, err)
	w.App.SetSequencerSource(source)
	release := func(stage string, seq uint64, entries ...wal.Entry) {
		t.Helper()
		ack, err := repl.Release(ctx, entries...)
		require.NoError(t, err, "%s: standby reason: %v; stopped: %v", stage, standby.RefusalReason(), standby.Err())
		require.Equal(t, sequencer.AckOK, ack.Status, "%s: standby reason: %v", stage, standby.RefusalReason())
		require.Equal(t, seq, ack.LastSeq, stage)
	}
	sign := func(sender utils.TestAcct, nonce uint64, to common.Address, value *big.Int, data []byte) []byte {
		t.Helper()
		gas := uint64(1_000_000)
		if len(data) == 0 {
			gas = seqCtxTransferGas
		}
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: nonce, To: &to, Value: value, Data: data, Gas: gas, GasPrice: big.NewInt(seqCtxGasPrice),
		}), ethtypes.LatestSignerForChainID(big.NewInt(config.DefaultChainID)), sender.EvmPrivateKey)
		require.NoError(t, err)
		dataTx, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(dataTx)
		require.NoError(t, err)
		builder := w.App.GetTxConfig().NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(gas)
		tx, err := w.App.GetTxConfig().TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		return tx
	}
	issuerSeed, holderSeed := sha256.Sum256([]byte("recorder issuer")), sha256.Sum256([]byte("recorder holder"))
	issuerDID := ed25519.NewKeyFromSeed(issuerSeed[:])
	holderDID := ed25519.NewKeyFromSeed(holderSeed[:])
	bind := func(sender utils.TestAcct, private ed25519.PrivateKey) []byte {
		t.Helper()
		message, err := evmtypes.LayerXBindMessage(big.NewInt(config.DefaultChainID), sender.EvmAddress, 0)
		require.NoError(t, err)
		p, err := addr.NewPrecompile(w.App.GetPrecompileKeepers())
		require.NoError(t, err)
		data, err := p.GetABI().Pack("bindLayerX", [32]byte(private.Public().(ed25519.PublicKey)), ed25519.Sign(private, message))
		require.NoError(t, err)
		return sign(sender, 0, p.Address(), new(big.Int), data)
	}
	issuerKey, holderKey := [32]byte(issuerDID.Public().(ed25519.PublicKey)), [32]byte(holderDID.Public().(ed25519.PublicKey))
	issuerID, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(issuerKey)))
	require.NoError(t, err)
	salt := [32]byte{0x41}
	material := append(append([]byte("LX:ASSET:v1"), issuerID[:]...), salt[:]...)
	asset := sha256.Sum256(material)
	ledgerABI := layerxledger.GetABI()
	ledgerCall := func(sender utils.TestAcct, nonce uint64, method string, args ...interface{}) []byte {
		t.Helper()
		data, err := ledgerABI.Pack(method, args...)
		require.NoError(t, err)
		return sign(sender, nonce, common.HexToAddress(layerxledger.LedgerAddress), new(big.Int), data)
	}
	stages := []struct {
		name string
		tx   []byte
	}{
		{"fund issuer", sign(funder, 0, issuer.EvmAddress, big.NewInt(1_000_000_000_000_000_000), nil)},
		{"fund holder", sign(funder, 1, holder.EvmAddress, big.NewInt(1_000_000_000_000_000_000), nil)},
		{"bind issuer", bind(issuer, issuerDID)},
		{"bind holder", bind(holder, holderDID)},
		{"register asset", ledgerCall(issuer, 1, "register", asset, salt, "REC", "Recorder replication", uint8(6), big.NewInt(1000), uint8(ledgertypes.AssetIssuerNative), []byte{})},
		{"issuer openAccount", ledgerCall(issuer, 2, "openAccount", asset)},
		{"holder openAccount", ledgerCall(holder, 1, "openAccount", asset)},
	}
	leaseEpoch := uint64(1)
	release("initial LEASE", 0, &wal.Lease{Epoch: leaseEpoch, KeyID: [32]byte(sequencerKeyID), ExpiryMs: uint64(at.Add(3 * time.Second).UnixMilli())})
	for i, stage := range stages {
		height, seq := int64(i+2), uint64(i+1)
		at = at.Add(time.Second)
		require.NoError(t, exec.Open(height, at.UnixMilli()), stage.name)
		release(stage.name+" OPEN", seq-1, &wal.Open{Height: uint64(height), OpenTimeMs: uint64(at.UnixMilli()), BaseFee: uint256.NewInt(0)})
		release(stage.name+" LEASE", seq-1, &wal.Lease{Epoch: leaseEpoch, KeyID: [32]byte(sequencerKeyID), ExpiryMs: uint64(at.Add(3 * time.Second).UnixMilli())})
		if seq == 5 {
			leaseEpoch++
			release(stage.name+" later LEASE", seq-1, &wal.Lease{Epoch: leaseEpoch, KeyID: [32]byte(sequencerKeyID), ExpiryMs: uint64(at.Add(3 * time.Second).UnixMilli())})
			// The retained window now includes the older lease, OPEN and renewal.
			// A reconnect must resend the records without replaying a stale epoch.
			repl.Close()
		}
		accepted, receipt, err := receipter.Accept(stage.tx)
		require.NoError(t, err, stage.name)
		require.Zero(t, accepted.Result.Code, "%s: %s", stage.name, accepted.Result.Log)
		require.NotNil(t, accepted.Result.EvmTxInfo, stage.name)
		require.Empty(t, accepted.Result.EvmTxInfo.VmError, stage.name)
		release(stage.name+" ACCEPT", seq, &wal.Accept{Height: uint64(height), Seq: seq, Tx: stage.tx, Receipt: receipt})
		_, err = source.Seal(ctx, height)
		require.NoError(t, err, "%s SEAL: standby reason: %v; stopped: %v", stage.name, standby.RefusalReason(), standby.Err())
		require.Equal(t, seq, standby.LastSeq(), stage.name)
		sealed := source.Sealed()
		req := &abci.RequestFinalizeBlock{Hash: []byte(stage.name), Header: &tmproto.Header{
			ChainID: sequencedChainID, Height: height, Time: at, ProposerAddress: primaryPub.Address(), AppHash: bytes.Clone(w.App.LastCommitID().Hash),
		}, Txs: sealed.Txs, DecidedLastCommit: abci.CommitInfo{Votes: votes}}
		for _, a := range []*app.App{w.App, backup} {
			response, err := a.FinalizeBlock(ctx, req)
			require.NoError(t, err, stage.name)
			require.Len(t, response.TxResults, 2, stage.name)
			require.Zero(t, response.TxResults[1].Code, "%s: %s", stage.name, response.TxResults[1].Log)
			require.Empty(t, response.TxResults[1].EvmTxInfo.VmError, stage.name)
			if a == w.App {
				require.Same(t, sealed.Results[1], response.TxResults[1], stage.name)
			}
			_, err = a.Commit(ctx)
			require.NoError(t, err, stage.name)
		}
		require.Equal(t, backup.LastCommitID(), w.App.LastCommitID(), stage.name)
		release(stage.name+" COMMITTED", seq, &wal.Committed{Height: uint64(height), LastSeq: seq, AppHash: [32]byte(w.App.LastCommitID().Hash)})
	}
	for _, a := range []*app.App{w.App, backup} {
		for _, did := range [][32]byte{issuerKey, holderKey} {
			name, id, err := lxphost.AssetAccountID(did, asset)
			require.NoError(t, err)
			expectedID, err := ledgerkeeper.AccountID(did, asset)
			require.NoError(t, err)
			require.Equal(t, expectedID, id)
			account, err := a.LayerXHost.ReadAccount(a.GetCheckCtx(), name, id)
			require.NoError(t, err)
			require.Equal(t, asset, account.AssetID)
		}
	}
	require.Equal(t, uint64(len(stages)), standby.LastSeq())
	require.NoError(t, standby.Err())
	// Admission refusals must reach the primary over TCP, including their detail.
	_, err = repl.Release(ctx, &wal.Lease{Epoch: leaseEpoch - 1})
	require.ErrorIs(t, err, sequencer.ErrStandbyRefused)
	require.ErrorIs(t, standby.RefusalReason(), sequencer.ErrStaleLease)
	require.ErrorContains(t, err, standby.RefusalReason().Error())
	require.ErrorContains(t, err, fmt.Sprintf("epoch %d, held %d", leaseEpoch-1, leaseEpoch))
	require.Equal(t, uint64(len(stages)), standby.LastSeq())
	require.NoError(t, standby.Err())
}
