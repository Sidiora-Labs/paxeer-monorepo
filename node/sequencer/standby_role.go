package sequencer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"sync"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// StartStandby opens the node's WAL and serves replication until ctx is cancelled.
// The consensus key and signing state use the standard paths under the app home.
func StartStandby(ctx context.Context, cfg Config, a *app.App) (*Standby, error) {
	if cfg.Role != RoleStandby {
		return nil, fmt.Errorf("%w: StartStandby requires the standby role, got %s", ErrConfigValue, cfg.Role)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("%w: standby needs an app", ErrConfigValue)
	}
	if cfg.WALDir == "" {
		return nil, ErrConfigWALDir
	}
	if cfg.ReceiptKeyFile == "" {
		return nil, ErrConfigReceiptKeyFile
	}
	if _, _, err := net.SplitHostPort(cfg.StandbyAddress); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConfigStandbyAddress, err)
	}
	tlsConfig, err := cfg.ReplicationTLS.TLSConfig(true)
	if err != nil {
		return nil, err
	}
	if cfg.ReplicationTimeout <= 0 {
		return nil, ErrConfigReplicationTimeout
	}
	infoPath, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	if err != nil {
		return nil, fmt.Errorf("sequencer: standby app home: %w", err)
	}
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(infoPath))).PrivValidator
	pv, err := privval.LoadFilePV(consensus.KeyFile(), consensus.StateFile())
	if err != nil {
		return nil, fmt.Errorf("sequencer: standby consensus key: %w", err)
	}
	height := a.LastBlockHeight()
	state := a.NewUncachedContext(false, tmproto.Header{ChainID: a.ChainID, Height: height})
	leaders, err := a.SequencerKeeper.Leaders(state)
	if err != nil {
		return nil, err
	}
	if len(leaders) < 2 {
		return nil, fmt.Errorf("%w: standby needs a pinned primary and standby leader", ErrConfigValue)
	}
	primaryConsensus, err := tmed25519.PublicKeyFromBytes(leaders[0][:])
	if err != nil {
		return nil, err
	}
	localPub, err := pv.GetPubKey(ctx)
	if err != nil {
		return nil, err
	}
	pinned := false
	for _, leader := range leaders[1:] {
		pinned = pinned || bytes.Equal(localPub.Bytes(), leader[:])
	}
	if !pinned {
		return nil, fmt.Errorf("%w: standby consensus key is not a standby leader", ErrConfigValue)
	}
	pins, err := a.SequencerKeeper.ReceiptKeyPins(state)
	if err != nil {
		return nil, err
	}
	key, err := LoadReceiptKey(cfg.ReceiptKeyFile, consensus.KeyFile(), pins, height+1)
	if err != nil {
		return nil, err
	}
	var primaryReceipt ed25519.PublicKey
	for _, pin := range pins {
		if pin.PublicKey == key.public || uint64(height+1) < pin.FirstBatchNumber || uint64(height+1) > pin.LastBatchNumber {
			continue
		}
		if primaryReceipt != nil && !bytes.Equal(primaryReceipt, pin.PublicKey[:]) {
			return nil, fmt.Errorf("%w: ambiguous primary receipt key", ErrConfigValue)
		}
		primaryReceipt = append(ed25519.PublicKey(nil), pin.PublicKey[:]...)
	}
	if primaryReceipt == nil {
		return nil, fmt.Errorf("%w: no distinct primary receipt key", ErrConfigValue)
	}
	lastSeq, err := a.SequencerKeeper.LastSeq(state)
	if err != nil {
		return nil, err
	}
	lastHash, err := a.SequencerKeeper.LastSeqHash(state)
	if err != nil {
		return nil, err
	}
	exec, err := NewExecutor(a, Options{
		Mode: cfg.Mode, ChainID: a.ChainID, ProposerAddress: primaryConsensus.Address(),
		LastSeq: lastSeq, LastSeqHash: lastHash,
		MicroBatch: MicroBatchOptions{Window: cfg.MicroBatchWindow, MaxCount: cfg.MicroBatchMaxCount},
	})
	if err != nil {
		return nil, err
	}
	r, err := NewReceipter(exec, a, &a.EvmKeeper, key, app.EVMReceiptModuleID)
	if err != nil {
		return nil, err
	}
	floor, err := NewFeeFloor(exec, a, a.GetCheckCtx().MinGasPrices().String(), a.ParamsKeeper, a.OracleKeeper, a.PricefeedKeeper, &a.EvmKeeper)
	if err != nil {
		return nil, err
	}
	exec = floor
	r.exec = exec
	w, err := wal.OpenWAL(cfg.WALDir, wal.DefaultConfig())
	if err != nil {
		return nil, err
	}
	source := &standbyRoleSource{app: a}
	s, err := NewStandby(ctx, StandbyConfig{
		WAL: w, Executor: &standbyRoleReplay{AppReplayer: NewAppReplayer(exec, r), pins: pins, local: key.public, source: source},
		ConsensusKey: pv, ReceiptKey: key.private,
		PrimaryConsensusKey: primaryConsensus, PrimaryReceiptKey: primaryReceipt,
		CommittedHeight: uint64(height), LastSeq: lastSeq, ReplicationTimeout: cfg.ReplicationTimeout,
	})
	if err != nil {
		w.Close()
		return nil, err
	}
	l, err := tls.Listen("tcp", cfg.StandbyAddress, tlsConfig)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("sequencer: standby listen: %w", err)
	}
	a.SetOpenBlock(exec)
	a.SetSequencerSource(source)
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	s.serveDone = make(chan struct{})
	go func() {
		defer close(s.serveDone)
		err := s.Serve(standbyRoleListener{Listener: l, ctx: ctx, timeout: cfg.ReplicationTimeout})
		stop()
		_ = l.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.failed == nil {
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			s.failed = err
		}
		w.Close()
	}()
	return s, nil
}

type standbyRoleListener struct {
	net.Listener
	ctx     context.Context
	timeout time.Duration
}

func (l standbyRoleListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		// Authentication finishes before Serve can read even a control frame.
		secured, ok := c.(*tls.Conn)
		if !ok {
			_ = c.Close()
			continue
		}
		ctx, cancel := context.WithTimeout(l.ctx, l.timeout)
		err = secured.HandshakeContext(ctx)
		cancel()
		if err != nil {
			_ = c.Close()
			continue
		}
		return &standbyRoleConn{Conn: c, stop: context.AfterFunc(l.ctx, func() { _ = c.Close() })}, nil
	}

}

type standbyRoleConn struct {
	net.Conn
	stop func() bool
}

func (c *standbyRoleConn) Close() error {
	c.stop()
	return c.Conn.Close()
}

// Replay verifies the primary signature and reproduces the unsigned receipt;
// the standby's own signing key never signs on behalf of the primary.
type standbyRoleReplay struct {
	*AppReplayer
	pins       []verify.SequencerAuthorization
	local      [32]byte
	allowLocal bool
	source     *standbyRoleSource
}

func (r *standbyRoleReplay) Execute(a *wal.Accept) ([]byte, error) {
	pins, err := r.source.app.SequencerKeeper.ReceiptKeyPins(r.source.app.GetCheckCtx())
	if err != nil {
		return nil, err
	}
	r.pins = append([]verify.SequencerAuthorization(nil), pins...)
	var verified *verify.VerifiedReceipt
	for _, pin := range r.pins {
		if (!r.allowLocal && pin.PublicKey == r.local) || a.Height < pin.FirstBatchNumber || a.Height > pin.LastBatchNumber {
			continue
		}
		v, err := verify.ReceiptSignature(a.Receipt, pin.PublicKey)
		if err == nil && v.Receipt.ContextHash == pin.SequencerID {
			verified = v
			break
		}
	}
	if verified == nil {
		return nil, fmt.Errorf("%w: primary receipt signature or pin", wal.ErrReceiptMismatch)
	}
	r.r.mu.Lock()
	defer r.r.mu.Unlock()
	pre, err := r.exec.Snapshot()
	if err != nil {
		return nil, err
	}
	tx, err := r.r.host.GetTxConfig().TxDecoder()(a.Tx)
	if err != nil {
		return nil, err
	}
	var etx *ethtypes.Transaction
	if msg := r.r.host.GetEVMMsg(tx); msg != nil {
		etx, _ = msg.AsTransaction()
	}
	in := &ReceiptInput{ModuleID: r.r.moduleID, PaymentDenom: r.r.evm.GetBaseDenom(pre)}
	if etx != nil {
		err = r.r.beforeEVM(pre, etx, in)
	} else {
		err = beforeCosmos(tx, in)
	}
	if err != nil {
		return nil, err
	}
	in.Accepted, err = r.exec.Accept(a.Tx)
	if err != nil {
		return nil, err
	}
	if in.Accepted.Seq != a.Seq || uint64(in.Accepted.Height) != a.Height {
		return nil, fmt.Errorf("%w: sequence or height", wal.ErrReceiptMismatch)
	}
	if in.Accepted.Height != r.r.height {
		r.r.height, r.r.cumulative = in.Accepted.Height, 0
	}
	if err := r.r.after(etx, in); err != nil {
		return nil, err
	}
	unsigned, err := BuildReceipt(in, verified.Receipt.ContextHash)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(unsigned, verified.Receipt.UnsignedBytes()) {
		return nil, fmt.Errorf("%w: seq %d", wal.ErrReceiptMismatch, a.Seq)
	}
	if r.src != nil {
		send, _, err := r.r.send(a.Tx, in.Accepted, a.Receipt)
		if err != nil {
			return nil, err
		}
		r.src.record(common.Hash(in.Accepted.ActivityID), send, true)
	}
	return bytes.Clone(a.Receipt), nil
}

type standbyRoleSource struct {
	app     *app.App
	mu      sync.Mutex
	pending *Batch
	sealed  *SealedLayer
}

var _ tmtypes.SequencerSource = (*standbyRoleSource)(nil)

func (s *standbyRoleSource) Sequenced(height int64) bool {
	return s.app.Sequenced(s.app.GetCheckCtx(), height)
}

func (s *standbyRoleSource) Seal(ctx context.Context, height int64) (tmtypes.SealedBlock, error) {
	if err := ctx.Err(); err != nil {
		return tmtypes.SealedBlock{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed == nil || s.sealed.Height != height {
		return tmtypes.SealedBlock{}, ErrNoSealedLayer
	}
	txs := make(tmtypes.Txs, len(s.sealed.Txs)-1)
	for i := range txs {
		txs[i] = bytes.Clone(s.sealed.Txs[i+1])
	}
	return tmtypes.SealedBlock{Batch: bytes.Clone(s.sealed.Txs[0]), Txs: txs,
		OpenTime: s.sealed.Context().BlockTime()}, nil
}

func (s *standbyRoleSource) Sealed() *SealedLayer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed
}

func (s *standbyRoleSource) SealForFinalize(height int64, txs [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed != nil && s.sealed.Height == height {
		if len(txs) != len(s.sealed.Txs) {
			return ErrBatchFieldsMismatch
		}
		for i := range txs {
			if !bytes.Equal(txs[i], s.sealed.Txs[i]) {
				return ErrBatchFieldsMismatch
			}
		}
		return nil
	}
	batch := s.pending
	if batch == nil {
		return ErrNoSealedLayer
	}
	if batch.Height != height || len(txs) != len(batch.Txs)+1 {
		return ErrBatchFieldsMismatch
	}
	for i, tx := range batch.Txs {
		if !bytes.Equal(tx, txs[i+1]) {
			return ErrBatchFieldsMismatch
		}
	}
	msg, err := app.DecodeSequenceBatch(txs[0])
	if err != nil {
		return err
	}
	sealed, _, err := SealBatch(s.app, batch, msg)
	if err != nil {
		return err
	}
	s.sealed = sealed
	s.pending = nil
	return nil
}

func (r *standbyRoleReplay) Apply(entry wal.Entry) error {
	if _, ok := entry.(*wal.Open); ok {
		pins, err := r.source.app.SequencerKeeper.ReceiptKeyPins(r.source.app.GetCheckCtx())
		if err != nil {
			return err
		}
		if err := r.r.key.SetPins(pins); err != nil {
			return err
		}
		r.pins = append([]verify.SequencerAuthorization(nil), pins...)
	}
	seal, ok := entry.(*wal.Seal)
	if !ok {
		return r.AppReplayer.Apply(entry)
	}
	if seal.Height > math.MaxInt64 {
		return fmt.Errorf("%w: SEAL height exceeds int64", wal.ErrMalformed)
	}
	s := r.source
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, err := r.exec.Seal(int64(seal.Height))
	if err != nil {
		return err
	}
	fields := BatchFields(batch)
	if fields.Height != seal.Height || fields.FromSeq != seal.FromSeq || fields.ToSeq != seal.ToSeq || fields.LastSeqHash != seal.LastSeqHash {
		return ErrBatchFieldsMismatch
	}
	s.pending = batch
	return nil
}
