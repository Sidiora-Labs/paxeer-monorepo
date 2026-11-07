package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair475Restart struct {
	a             *app.App
	n             *sequencer.Node
	home          string
	cfg           sequencer.Config
	hashes        map[int64][]byte
	blockHashes   map[int64][]byte
	validatorHash [32]byte
	proposer      []byte
}

func newRepair475Restart(t *testing.T) *repair475Restart {
	t.Helper()
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	keys := t.TempDir()
	pv, err := privval.GenFilePV(filepath.Join(keys, "key.json"), filepath.Join(keys, "state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	w := app.NewTestWrapper(t, time.Now().Add(-time.Minute).UTC(), &sdked25519.PubKey{Key: pv.Key.PubKey.Bytes()}, false,
		func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	info, err := w.App.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	h := &repair475Restart{a: w.App, home: filepath.Dir(filepath.Dir(info)), hashes: make(map[int64][]byte), blockHashes: make(map[int64][]byte)}
	h.validatorHash = sha256.Sum256(pv.Key.PubKey.Bytes())
	h.proposer = bytes.Clone(pv.Key.PubKey.Address())
	t.Cleanup(func() { h.close(t) })
	consensus := tmconfig.DefaultConfig().SetRoot(h.home).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	keyBytes, err := os.ReadFile(filepath.Join(keys, "key.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(consensus.KeyFile(), keyBytes, 0o600))
	seed := bytes.Repeat([]byte{0x74}, ed25519.SeedSize)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	require.NoError(t, w.App.SequencerKeeper.InitGenesis(w.Ctx, seqtypes.GenesisState{
		ActivationHeight: 2,
		Leaders:          [][32]byte{[32]byte(pv.Key.PubKey.Bytes())},
		ReceiptKeys:      []seqtypes.ReceiptKey{{KeyId: [32]byte{0x75}, PublicKey: [32]byte(pub), FirstHeight: 1, LastHeight: 100}},
	}))
	w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	w.App.StoreConsensusParams(w.Ctx, params)
	h.cfg = sequencer.DefaultConfig()
	h.cfg.Enable = true
	h.cfg.WALDir = filepath.Join(h.home, "sequencer-wal")
	h.cfg.ReceiptKeyFile = filepath.Join(h.home, "receipt-key")
	h.cfg.LeaseDuration = 30 * time.Second
	h.cfg.NonceQueueMaxPerSender, h.cfg.NonceQueueMaxTotal = 4, 8
	require.NoError(t, os.WriteFile(h.cfg.ReceiptKeyFile, []byte(hex.EncodeToString(seed)), 0o600))
	// At the first Commit the initialized state becomes visible to NewNode.
	// Use the production driver as the listener, including its durable Release.
	h.a.SetCommitListener(func(height int64, hash []byte) error {
		var err error
		h.n, err = sequencer.NewNode(h.cfg, h.a)
		if err != nil {
			return err
		}
		return h.n.Driver.OnCommit(height, hash)
	})
	return h
}

func (h *repair475Restart) close(t *testing.T) {
	t.Helper()
	if h.n != nil {
		require.NoError(t, h.n.Stop())
		h.n = nil
	}
	if h.a != nil {
		require.NoError(t, h.a.Close())
		h.a = nil
	}
}

func (h *repair475Restart) commit(t *testing.T, height int64) {
	t.Helper()
	parent := bytes.Clone(h.a.LastCommitID().Hash)
	previousID := h.a.LastBlockID()
	header := &tmproto.Header{ChainID: h.a.ChainID, Height: height, Time: time.Now().UTC(), AppHash: parent,
		Version: version.Consensus{Block: version.BlockProtocol}.ToProto(), ProposerAddress: bytes.Clone(h.proposer),
		LastBlockId: previousID, ValidatorsHash: h.validatorHash[:], NextValidatorsHash: h.validatorHash[:]}
	req := &abci.RequestFinalizeBlock{Header: header}
	if height > 1 {
		pending, err := h.n.Executor.Snapshot()
		require.NoError(t, err)
		require.Equal(t, parent, pending.BlockHeader().AppHash, "header must name the parent commit")
		block, err := h.n.ConsensusSource.Seal(context.Background(), height)
		require.NoError(t, err)
		header.Time = block.OpenTime
		header.ProposerAddress = pending.BlockHeader().ProposerAddress
		req.Txs = append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...)
	}
	consensusHeader, err := tmtypes.HeaderFromProto(header)
	require.NoError(t, err)
	req.Hash = consensusHeader.Hash()
	require.Len(t, req.Hash, sha256.Size)
	finalized, err := h.a.FinalizeBlock(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, previousID, h.a.LastBlockID(), "FinalizeBlock must not publish the uncommitted identity")
	for _, result := range finalized.TxResults {
		require.Zero(t, result.Code, result.Log)
	}
	_, err = h.a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, height, h.a.LastBlockHeight())
	require.Equal(t, req.Hash, h.a.LastBlockID().Hash, "Commit must publish the finalized block identity")
	committed := h.n.WAL.LastCommitted()
	require.NotNil(t, committed)
	require.EqualValues(t, height, committed.Height)
	require.Equal(t, finalized.AppHash, committed.AppHash[:])
	require.Equal(t, h.a.LastCommitID().Hash, committed.AppHash[:])
	require.NotEqual(t, parent, committed.AppHash[:], "COMMITTED must name its own height, not the header's parent")
	h.hashes[height] = bytes.Clone(committed.AppHash[:])
	h.blockHashes[height] = bytes.Clone(req.Hash)
	t.Logf("height=%d header_parent=%X finalized=%X committed=%X", height, parent, finalized.AppHash, committed.AppHash)
}

func (h *repair475Restart) reopen(t *testing.T) {
	t.Helper()
	h.a = app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, h.home, 1, false,
		tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil,
		func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	var err error
	h.n, err = sequencer.NewNode(h.cfg, h.a)
	require.NoError(t, err)
}

func (h *repair475Restart) restart(t *testing.T, height int64) {
	t.Helper()
	h.close(t)
	h.reopen(t)
	require.Equal(t, height, h.a.LastBlockHeight())
	require.Equal(t, h.hashes[height], h.a.LastCommitID().Hash)
	require.Equal(t, h.blockHashes[height], h.a.LastBlockID().Hash, "restart must restore the committed block identity")
	require.NoError(t, h.n.Start(context.Background()), "restart handshake at height %d", height)
	require.Equal(t, h.hashes[height], h.n.WAL.LastCommitted().AppHash[:])
	pending, err := h.n.Executor.Snapshot()
	require.NoError(t, err)
	require.Equal(t, height+1, pending.BlockHeight())
	require.Equal(t, h.hashes[height], pending.BlockHeader().AppHash)
}

func TestRepair475_CommitHashesSurviveEveryRestart(t *testing.T) {
	h := newRepair475Restart(t)
	for height := int64(1); height <= 3; height++ {
		h.commit(t, height)
		h.restart(t, height)
	}
}

// Alter the actual producer's last record while preserving valid WAL framing
// and CRC, so refusal must come from the hash handshake rather than decoding.
func repair475AlterCommitted(t *testing.T, dir string, height uint64, change func(*wal.Committed), truncate bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.seg"))
	require.NoError(t, err)
	found := false
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		for offset := 0; offset < len(data); {
			require.GreaterOrEqual(t, len(data)-offset, 8)
			size := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
			end := offset + 8 + size
			require.LessOrEqual(t, end, len(data))
			require.Equal(t, binary.BigEndian.Uint32(data[offset:offset+4]), crc32.Checksum(data[offset+8:end], crc32.MakeTable(crc32.Castagnoli)))
			entry, err := wal.DecodeEntry(data[offset+8 : end])
			require.NoError(t, err)
			if committed, ok := entry.(*wal.Committed); ok && committed.Height == height {
				require.False(t, found, "expected one COMMITTED per height")
				found = true
				if truncate {
					require.NoError(t, os.WriteFile(file, data[:offset], 0o600))
					break
				}
				change(committed)
				frame, err := wal.NewFrame(committed)
				require.NoError(t, err)
				encoded := []byte(frame)
				require.Len(t, encoded, end-offset)
				copy(data[offset:end], encoded)
				require.NoError(t, os.WriteFile(file, data, 0o600))
			}
			offset = end
		}
	}
	require.True(t, found, "production COMMITTED record was not found")
}

func TestRepair475_RejectCorruptedCommittedHash(t *testing.T) {
	for _, corruption := range []string{"parent hash", "changed hash", "changed sequence"} {
		t.Run(corruption, func(t *testing.T) {
			h := newRepair475Restart(t)
			for height := int64(1); height <= 3; height++ {
				h.commit(t, height)
				if height < 3 {
					h.restart(t, height)
				}
			}
			h.close(t)
			repair475AlterCommitted(t, h.cfg.WALDir, 3, func(record *wal.Committed) {
				switch corruption {
				case "parent hash":
					record.AppHash = [32]byte(h.hashes[2])
				case "changed hash":
					record.AppHash[0] ^= 0xff
				case "changed sequence":
					record.LastSeq++
				}
			}, false)
			h.reopen(t)
			require.Equal(t, h.hashes[3], h.a.LastCommitID().Hash, "application state must remain intact")
			require.ErrorContains(t, h.n.Start(context.Background()), "WAL committed tip differs from app")
			require.Nil(t, h.a.SequencerSource(), "a rejected handshake must not install the runtime")
			_, err := h.n.Executor.Snapshot()
			require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
		})
	}
}

func TestRepair475_ReconcileSealAfterCommitCrash(t *testing.T) {
	h := newRepair475Restart(t)
	for height := int64(1); height <= 3; height++ {
		h.commit(t, height)
		if height < 3 {
			h.restart(t, height)
		}
	}
	h.close(t)
	repair475AlterCommitted(t, h.cfg.WALDir, 3, nil, true)
	h.reopen(t)
	require.EqualValues(t, 2, h.n.WAL.LastCommitted().Height)
	require.NoError(t, h.n.Start(context.Background()))
	require.EqualValues(t, 3, h.n.WAL.LastCommitted().Height)
	require.Equal(t, h.hashes[3], h.n.WAL.LastCommitted().AppHash[:])
	h.restart(t, 3)
}

func TestRepair475_DriverRefusesParentHash(t *testing.T) {
	h := newRepair475Restart(t)
	h.commit(t, 1)
	h.restart(t, 1)
	h.commit(t, 2)
	// Seal the next speculative layer so the driver's hash check is exercised
	// without an open-layer refusal masking an invalid COMMITTED producer.
	_, err := h.n.Executor.Seal(3)
	require.NoError(t, err)
	before, err := os.ReadFile(filepath.Join(h.cfg.WALDir, "head.seg"))
	require.NoError(t, err)
	require.ErrorContains(t, h.n.Driver.OnCommit(2, h.hashes[1]), "differs from consensus app hash")
	after, err := os.ReadFile(filepath.Join(h.cfg.WALDir, "head.seg"))
	require.NoError(t, err)
	require.Equal(t, before, after, "invalid producer input must not reach Replicator.Release")
	require.Equal(t, h.hashes[2], h.n.WAL.LastCommitted().AppHash[:])
	copied, err := h.a.CommittedAppHash(2)
	require.NoError(t, err)
	copied[0] ^= 0xff
	require.Equal(t, h.hashes[2], h.a.LastCommitID().Hash)
	_, err = h.a.CommittedAppHash(1)
	require.ErrorContains(t, err, "does not match requested height")
}
