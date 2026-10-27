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
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type driverHarness struct {
	h       *sourceHarness
	sb      *sourceHarness
	driver  *sequencer.Driver
	repl    *sequencer.Replicator
	queue   *sequencer.NonceQueue
	w       *wal.WAL
	sw      *wal.WAL
	standby *sequencer.Standby
	dir     string
	sdir    string
	reader  *sdkmetric.ManualReader
	sender  utils.TestAcct
	lastSeq uint64
}

func newDriverHarness(t *testing.T, retain uint64, segmentSize int64) *driverHarness {
	t.Helper()
	sender := utils.NewSigner()
	setup := func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}
	f := &driverHarness{h: newSourceHarness(t, openHeight, setup, sender), sb: newSourceHarness(t, openHeight, setup, sender), sender: sender, lastSeq: sourceLastSeq}
	f.h.exec = newFloor(t, f.h.harness, "")
	f.sb.exec = newFloor(t, f.sb.harness, "")
	f.dir, f.sdir = filepath.Join(t.TempDir(), "primary"), filepath.Join(t.TempDir(), "standby")
	cfg := wal.DefaultConfig()
	cfg.SegmentSizeLimit = segmentSize
	var err error
	f.w, err = wal.OpenWAL(f.dir, cfg)
	require.NoError(t, err)
	t.Cleanup(f.w.Close)
	f.sw, err = wal.OpenWAL(f.sdir, cfg)
	require.NoError(t, err)
	t.Cleanup(f.sw.Close)
	keys := t.TempDir()
	pv, err := privval.GenFilePV(filepath.Join(keys, "primary.json"), filepath.Join(keys, "primary-state.json"), "")
	require.NoError(t, err)
	sv, err := privval.GenFilePV(filepath.Join(keys, "standby.json"), filepath.Join(keys, "standby-state.json"), "")
	require.NoError(t, err)
	pub, err := pv.GetPubKey(context.Background())
	require.NoError(t, err)
	_, receipt, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	primaryReceipt := publicOf(testKeySeed)
	f.standby, err = sequencer.NewStandby(context.Background(), sequencer.StandbyConfig{
		WAL: f.sw, Executor: appReplayer{exec: f.sb.exec}, ConsensusKey: sv,
		ReceiptKey: receipt, PrimaryConsensusKey: pub, PrimaryReceiptKey: primaryReceipt[:],
		CommittedHeight: uint64(committedTip), LastSeq: sourceLastSeq,
	})
	require.NoError(t, err)
	l := listen(t)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
	go func() { _ = f.standby.Serve(l) }()
	f.repl, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: f.w, ReplicationTimeout: replicationTimeout,
		Dial: func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", l.Addr().String())
		},
	})
	require.NoError(t, err)
	t.Cleanup(f.repl.Close)
	f.h.src, err = sequencer.NewConsensusSource(f.h.app, f.h.exec, f.h.key, sequencer.WithConsensusReplicator(f.repl))
	require.NoError(t, err)
	f.h.app.SetOpenBlock(f.h.exec)
	f.queue, err = sequencer.NewNonceQueue(sequencer.NonceQueueConfig{PerSender: 4, Total: 8, Lifetime: time.Minute}, resolutionOpenNonces{a: f.h.app},
		func(_ common.Address, _ uint64, tx []byte) error {
			acc, err := f.h.exec.Accept(tx)
			if err != nil {
				return err
			}
			r, err := receiptBytes(acc)
			if err != nil {
				return err
			}
			_, err = f.repl.Release(context.Background(), &wal.Accept{Height: uint64(acc.Height), Seq: acc.Seq, Tx: tx, Receipt: r})
			if err == nil {
				f.lastSeq = acc.Seq
			}
			return err
		})
	require.NoError(t, err)
	m, reader := newSequencerMetricsReader(t)
	f.reader = reader
	f.driver, err = sequencer.NewDriver(sequencer.DriverConfig{
		App: f.h.app, Executor: f.h.exec.(*sequencer.FeeFloor), Replicator: f.repl,
		NonceQueue: f.queue, Metrics: m, WALRetainBlocks: retain,
	})
	require.NoError(t, err)
	return f
}

func driverEntries(t *testing.T, dir string) []wal.Entry {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.seg"))
	require.NoError(t, err)
	sort.Strings(files)
	var entries []wal.Entry
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		r := bytes.NewReader(data)
		for r.Len() > 0 {
			var header [8]byte
			_, err = io.ReadFull(r, header[:])
			require.NoError(t, err)
			size := binary.BigEndian.Uint32(header[4:])
			require.LessOrEqual(t, size, uint32(wal.MaxEntrySize))
			payload := make([]byte, size)
			_, err = io.ReadFull(r, payload)
			require.NoError(t, err)
			require.Equal(t, binary.BigEndian.Uint32(header[:4]), crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
			e, err := wal.DecodeEntry(payload)
			require.NoError(t, err)
			entries = append(entries, e)
		}
	}
	return entries
}

func (f *driverHarness) commit(t *testing.T, height int64) []byte {
	t.Helper()
	block, err := f.h.src.Seal(context.Background(), height)
	require.NoError(t, err)
	txs := append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...)
	var hash []byte
	for _, h := range []*sourceHarness{f.h, f.sb} {
		response, err := h.app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: testChainID, Height: height, Time: block.OpenTime, ProposerAddress: f.h.proposer}, Txs: txs,
		})
		require.NoError(t, err)
		require.Len(t, response.TxResults, len(txs))
		for _, result := range response.TxResults {
			require.Zero(t, result.Code, result.Log)
		}
		h.app.SetDeliverStateToCommit()
		_, err = h.app.Commit(context.Background())
		require.NoError(t, err)
		if h == f.h {
			hash = append([]byte(nil), h.app.CommitMultiStore().LastCommitID().Hash...)
		}
	}
	return hash
}

func TestDriverCommitReleasesCommittedThenOpen(t *testing.T) {
	f := newDriverHarness(t, 2, wal.DefaultConfig().SegmentSizeLimit)
	hash := f.h.app.CommitMultiStore().LastCommitID().Hash
	require.Error(t, f.driver.OnCommit(0, hash))
	require.Error(t, f.driver.OnCommit(committedTip, hash[:31]))
	require.Empty(t, driverEntries(t, f.dir))
	before := time.Now().UnixMilli()
	require.NoError(t, f.driver.OnCommit(committedTip, hash))
	entries := driverEntries(t, f.dir)
	require.Len(t, entries, 2)
	require.Equal(t, &wal.Committed{Height: uint64(committedTip), LastSeq: sourceLastSeq, AppHash: [32]byte(hash)}, entries[0])
	opened, ok := entries[1].(*wal.Open)
	require.True(t, ok)
	require.Equal(t, uint64(openHeight), opened.Height)
	require.GreaterOrEqual(t, opened.OpenTimeMs, uint64(before))
	require.LessOrEqual(t, opened.OpenTimeMs, uint64(time.Now().UnixMilli()))
	ctx := f.h.openCtx()
	require.Equal(t, openHeight, ctx.BlockHeight())
	require.Equal(t, opened.OpenTimeMs, uint64(ctx.BlockTime().UnixMilli()))
	fee, err := f.h.exec.(*sequencer.FeeFloor).BaseFeePerGas()
	require.NoError(t, err)
	require.Equal(t, fee.TruncateInt().BigInt(), opened.BaseFee.ToBig())
	require.Equal(t, entries[1:], driverEntries(t, f.sdir))
	_, tx := transfer(t, f.sender, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
	held, err := f.queue.Hold(f.sender.EvmAddress, 0, tx)
	require.NoError(t, err)
	require.False(t, held)
	_, err = f.queue.Hold(f.sender.EvmAddress, 0, tx)
	require.ErrorIs(t, err, sequencer.ErrSameNonce)
	hash = f.commit(t, openHeight)
	require.NoError(t, f.driver.OnCommit(openHeight, hash))
	entries = driverEntries(t, f.dir)
	require.Len(t, entries, 6)
	require.Equal(t, &wal.Committed{Height: uint64(openHeight), LastSeq: sourceLastSeq + 1, AppHash: [32]byte(hash)}, entries[4])
	require.Equal(t, uint64(openHeight+1), entries[5].(*wal.Open).Height)
	require.Equal(t, entries[1:], driverEntries(t, f.sdir))
	require.Equal(t, openHeight+1, f.h.openCtx().BlockHeight())
	_, err = f.queue.Hold(f.sender.EvmAddress, 0, tx)
	require.ErrorIs(t, err, sequencer.ErrNonceTooLow)
	metrics := collectSequencerMetrics(t, f.reader)
	for name, want := range map[string]int64{"sequencer_sealed_height": openHeight, "sequencer_open_seq": int64(sourceLastSeq + 1)} {
		gauge := metrics[name].Data.(metricdata.Gauge[int64])
		require.Len(t, gauge.DataPoints, 1)
		require.Equal(t, want, gauge.DataPoints[0].Value)
	}
	syncs := metrics["sequencer_wal_sync_seconds"].Data.(metricdata.Histogram[float64])
	require.Len(t, syncs.DataPoints, 1)
	require.Equal(t, uint64(6), syncs.DataPoints[0].Count)
	require.Greater(t, syncs.DataPoints[0].Sum, float64(0))
	_, nextTx := transfer(t, f.sender, 1, utils.NewSigner().EvmAddress, big.NewInt(1))
	_, err = f.queue.Hold(f.sender.EvmAddress, 1, nextTx)
	require.NoError(t, err)
	hash = f.commit(t, openHeight+1)
	f.sw.Close()
	require.ErrorIs(t, f.driver.OnCommit(openHeight+1, hash), sequencer.ErrStandbyRefused)
	_, err = f.h.exec.Snapshot()
	require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
	require.ErrorIs(t, f.driver.OnCommit(openHeight+1, hash), sequencer.ErrDriverHalted)
}

func TestDriverPrunesWALBelowRetention(t *testing.T) {
	for _, retain := range []uint64{2, 10} {
		t.Run(strconv.FormatUint(retain, 10), func(t *testing.T) {
			f := newDriverHarness(t, retain, 1)
			require.NoError(t, f.driver.OnCommit(committedTip, f.h.app.CommitMultiStore().LastCommitID().Hash))
			initialCommit := driverEntries(t, f.dir)[0]
			for height := openHeight; height <= 5; height++ {
				_, tx := transfer(t, f.sender, uint64(height-openHeight), utils.NewSigner().EvmAddress, big.NewInt(1))
				_, err := f.queue.Hold(f.sender.EvmAddress, uint64(height-openHeight), tx)
				require.NoError(t, err)
				hash := f.commit(t, height)
				require.NoError(t, f.driver.OnCommit(height, hash))
				var cutoff uint64
				if uint64(height) > retain {
					cutoff = uint64(height) - retain
				}
				files, err := filepath.Glob(filepath.Join(f.dir, "*-*.seg"))
				require.NoError(t, err)
				require.NotEmpty(t, files)
				boundary := false
				for _, file := range files {
					parts := strings.Split(strings.TrimSuffix(filepath.Base(file), ".seg"), "-")
					maxHeight, err := strconv.ParseUint(parts[1], 16, 64)
					require.NoError(t, err)
					require.GreaterOrEqual(t, maxHeight, cutoff)
					if maxHeight == max(cutoff, uint64(committedTip)) {
						boundary = true
					}
				}
				require.True(t, boundary, "segment at retention boundary must survive")
				all := append([]wal.Entry{initialCommit}, driverEntries(t, f.sdir)...)
				kept := driverEntries(t, f.dir)
				var want []wal.Entry
				for _, entry := range all {
					var h uint64
					switch e := entry.(type) {
					case *wal.Open:
						h = e.Height
					case *wal.Accept:
						h = e.Height
					case *wal.Seal:
						h = e.Height
					case *wal.Committed:
						h = e.Height
					default:
						t.Fatalf("unexpected WAL entry %T", entry)
					}
					if h >= cutoff {
						want = append(want, entry)
					}
				}
				require.Equal(t, want, kept)
				require.Equal(t, uint64(height), f.w.LastCommitted().Height)
				require.Equal(t, height+1, f.h.openCtx().BlockHeight())
			}
		})
	}
}
