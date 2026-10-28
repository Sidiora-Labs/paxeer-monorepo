package sequencer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"
	"net"
	"path/filepath"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
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
	w, err := wal.OpenWAL(cfg.WALDir, wal.DefaultConfig())
	if err != nil {
		return nil, err
	}
	s, err := NewStandby(ctx, StandbyConfig{
		WAL: w, Executor: &standbyRoleReplay{AppReplayer: NewAppReplayer(exec, r), pins: pins, local: key.public},
		ConsensusKey: pv, ReceiptKey: key.private,
		PrimaryConsensusKey: primaryConsensus, PrimaryReceiptKey: primaryReceipt,
		CommittedHeight: uint64(height), LastSeq: lastSeq, ReplicationTimeout: cfg.ReplicationTimeout,
	})
	if err != nil {
		w.Close()
		return nil, err
	}
	l, err := net.Listen("tcp", cfg.StandbyAddress)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("sequencer: standby listen: %w", err)
	}
	a.SetOpenBlock(exec)
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	go func() {
		err := s.Serve(standbyRoleListener{Listener: l, ctx: ctx})
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
	ctx context.Context
}

func (l standbyRoleListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &standbyRoleConn{Conn: c, stop: context.AfterFunc(l.ctx, func() { _ = c.Close() })}, nil
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
	pins  []verify.SequencerAuthorization
	local [32]byte
}

func (r *standbyRoleReplay) Execute(a *wal.Accept) ([]byte, error) {
	var verified *verify.VerifiedReceipt
	for _, pin := range r.pins {
		if pin.PublicKey == r.local || a.Height < pin.FirstBatchNumber || a.Height > pin.LastBatchNumber {
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
	in := &ReceiptInput{ModuleID: r.r.moduleID}
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
	return bytes.Clone(a.Receipt), nil
}
