package sequencer_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"hash/crc32"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type consensusWALHarness struct {
	*sourceHarness
	repl        *sequencer.Replicator
	primaryWAL  *wal.WAL
	standbyWAL  *wal.WAL
	primaryDir  string
	standbyDir  string
	standby     *sequencer.Standby
	standbyAddr string
	txs         tmtypes.Txs
	lastSeqHash [32]byte
}

func newConsensusWALHarness(t *testing.T) *consensusWALHarness {
	t.Helper()
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	setup := func(ctx sdk.Context, a *app.App) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}
	h := newSourceHarness(t, openHeight, setup, sender)
	sb := newSourceHarness(t, openHeight, setup, sender)
	f := &consensusWALHarness{
		sourceHarness: h,
		primaryDir:    filepath.Join(t.TempDir(), "primary-wal"),
		standbyDir:    filepath.Join(t.TempDir(), "standby-wal"),
		lastSeqHash:   sourcePrevSeqHash,
	}
	var err error
	f.primaryWAL, err = wal.OpenWAL(f.primaryDir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(f.primaryWAL.Close)
	f.standbyWAL, err = wal.OpenWAL(f.standbyDir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(f.standbyWAL.Close)
	keyDir := t.TempDir()
	primaryPV, err := privval.GenFilePV(filepath.Join(keyDir, "primary-key.json"), filepath.Join(keyDir, "primary-state.json"), "")
	require.NoError(t, err)
	standbyPV, err := privval.GenFilePV(filepath.Join(keyDir, "standby-key.json"), filepath.Join(keyDir, "standby-state.json"), "")
	require.NoError(t, err)
	primaryPub, err := primaryPV.GetPubKey(context.Background())
	require.NoError(t, err)
	_, standbyReceipt, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	primaryReceipt := publicOf(testKeySeed)
	f.standby, err = sequencer.NewStandby(context.Background(), sequencer.StandbyConfig{
		WAL:                 f.standbyWAL,
		Executor:            appReplayer{exec: sb.exec},
		ConsensusKey:        standbyPV,
		ReceiptKey:          standbyReceipt,
		PrimaryConsensusKey: primaryPub,
		PrimaryReceiptKey:   primaryReceipt[:],
		CommittedHeight:     uint64(committedTip),
		LastSeq:             sourceLastSeq,
	})
	require.NoError(t, err)
	l := listen(t)
	t.Cleanup(func() { _ = l.Close() })
	go func() { _ = f.standby.Serve(l) }()
	f.standbyAddr = l.Addr().String()
	f.repl, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: f.primaryWAL,
		Dial: func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", f.standbyAddr)
		},
		ReplicationTimeout: replicationTimeout,
	})
	require.NoError(t, err)
	t.Cleanup(f.repl.Close)
	h.src, err = sequencer.NewConsensusSource(h.app, h.exec, h.key, sequencer.WithConsensusReplicator(f.repl))
	require.NoError(t, err)
	ms := h.openAt(time.Now().Add(time.Minute))
	entries := []wal.Entry{&wal.Open{Height: uint64(openHeight), OpenTimeMs: uint64(ms), BaseFee: uint256.NewInt(0)}}
	for nonce := range uint64(3) {
		signed, tx := transfer(t, sender, nonce, recipient, big.NewInt(1))
		acc, err := h.exec.Accept(tx)
		require.NoError(t, err)
		require.Equal(t, uint32(0), acc.Result.Code, acc.Result.Log)
		receipt, err := receiptBytes(acc)
		require.NoError(t, err)
		entries = append(entries, &wal.Accept{Seq: acc.Seq, Height: uint64(acc.Height), Tx: tx, Receipt: receipt})
		f.txs = append(f.txs, tx)
		f.lastSeqHash = expectedSeqHash(f.lastSeqHash, sourceLastSeq+1+nonce, [32]byte(signed.Hash()))
	}
	_, err = f.repl.Release(context.Background(), entries...)
	require.NoError(t, err)
	require.Equal(t, sourceLastSeq+3, f.standby.LastSeq())
	return f
}

func consensusWALSeals(t *testing.T, dir string) []*wal.Seal {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "head.seg"))
	require.NoError(t, err)
	r := bytes.NewReader(data)
	var seals []*wal.Seal
	for r.Len() > 0 {
		var header [8]byte
		_, err := io.ReadFull(r, header[:])
		require.NoError(t, err)
		size := binary.BigEndian.Uint32(header[4:])
		require.LessOrEqual(t, size, uint32(wal.MaxEntrySize))
		payload := make([]byte, size)
		_, err = io.ReadFull(r, payload)
		require.NoError(t, err)
		require.Equal(t, binary.BigEndian.Uint32(header[:4]), crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
		entry, err := wal.DecodeEntry(payload)
		require.NoError(t, err)
		if seal, ok := entry.(*wal.Seal); ok {
			seals = append(seals, seal)
		}
	}
	return seals
}

func TestConsensusSourceSealWritesWALSeal(t *testing.T) {
	f := newConsensusWALHarness(t)
	require.Empty(t, consensusWALSeals(t, f.primaryDir))
	require.Empty(t, consensusWALSeals(t, f.standbyDir))
	block, err := f.src.Seal(context.Background(), openHeight)
	require.NoError(t, err)
	require.Equal(t, f.txs, block.Txs)
	msg, err := app.DecodeSequenceBatch(block.Batch)
	require.NoError(t, err)
	want := &wal.Seal{
		Height:      uint64(openHeight),
		FromSeq:     sourceLastSeq + 1,
		ToSeq:       sourceLastSeq + 3,
		LastSeqHash: f.lastSeqHash,
	}
	require.Equal(t, want.Height, msg.Height)
	require.Equal(t, want.FromSeq, msg.FromSeq)
	require.Equal(t, want.ToSeq, msg.ToSeq)
	require.Equal(t, want.LastSeqHash[:], msg.LastSeqHash)
	require.Equal(t, []*wal.Seal{want}, consensusWALSeals(t, f.primaryDir))
	require.Equal(t, []*wal.Seal{want}, consensusWALSeals(t, f.standbyDir))
	require.NotNil(t, f.src.Sealed())
	require.Equal(t, want.LastSeqHash, f.src.Sealed().Batch.LastSeqHash)
	require.NoError(t, f.standby.Err())
}

func TestConsensusSourceSealFailsWhenReleaseFails(t *testing.T) {
	for _, failure := range []string{"primary_wal_closed", "standby_wal_closed", "standby_unreachable"} {
		t.Run(failure, func(t *testing.T) {
			f := newConsensusWALHarness(t)
			wantErr := wal.ErrClosed
			liveAddr := f.standbyAddr
			switch failure {
			case "primary_wal_closed":
				f.primaryWAL.Close()
			case "standby_wal_closed":
				f.standbyWAL.Close()
				wantErr = sequencer.ErrStandbyRefused
			case "standby_unreachable":
				unavailable := listen(t)
				f.standbyAddr = unavailable.Addr().String()
				require.NoError(t, unavailable.Close())
				f.repl.Close()
				wantErr = sequencer.ErrNoAck
			}
			block, err := f.src.Seal(context.Background(), openHeight)
			require.ErrorIs(t, err, wantErr)
			require.Equal(t, tmtypes.SealedBlock{}, block)
			require.Nil(t, f.src.Sealed())
			require.Empty(t, consensusWALSeals(t, f.standbyDir))
			if failure == "primary_wal_closed" {
				require.Empty(t, consensusWALSeals(t, f.primaryDir))
			}
			if failure == "standby_unreachable" {
				f.standbyAddr = liveAddr
				block, err = f.src.Seal(context.Background(), openHeight)
				require.NoError(t, err)
				require.Equal(t, f.txs, block.Txs)
				require.NotNil(t, f.src.Sealed())
				seals := consensusWALSeals(t, f.standbyDir)
				require.Len(t, seals, 1)
				require.Equal(t, f.lastSeqHash, seals[0].LastSeqHash)
			}
		})
	}
}
