package sequencer

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/spf13/cast"
)

// ApplyCheck uses the node's admission, execution and sealing code for an
// offline comparison. Sequenced blocks retain their signed batch at index zero.
type ApplyCheck struct {
	ReceiptKey *ReceiptKey
}

// PrepareApplyCheckState replaces both receipt-key records in the disposable
// application with the ephemeral key used by the check.
func (s ApplyCheck) PrepareApplyCheckState(a *app.App, height int64) error {
	if s.ReceiptKey == nil {
		return errors.New("stand-in receipt key is unavailable")
	}
	public := s.ReceiptKey.PublicKey()
	keyID := sha256.Sum256(public[:])
	keys := seqkeeper.ReceiptKeys{{
		KeyId:       keyID[:],
		PublicKey:   public[:],
		FirstHeight: uint64(height),
		LastHeight:  math.MaxUint64,
	}}
	// Block execution branches from the working store, independently of CheckTx.
	state := a.NewUncachedContext(false, tmproto.Header{Height: height})
	if err := a.SequencerKeeper.InitReceiptKeyState(state, a.GetSubspace(seqtypes.ModuleName), keys); err != nil {
		return err
	}
	// Apply the pending changes to the disposable working tree before any
	// execution branch reads it. Computing its hash does not commit a version.
	if _, err := a.CommitMultiStore().GetWorkingHash(); err != nil {
		return err
	}
	if err := a.SequencerKeeper.InitReceiptKeyState(a.GetCheckCtx(), a.GetSubspace(seqtypes.ModuleName), keys); err != nil {
		return err
	}
	return s.ReceiptKey.SetPins([]verify.SequencerAuthorization{{
		SequencerID:      keyID,
		PublicKey:        public,
		FirstBatchNumber: uint64(height),
		LastBatchNumber:  math.MaxUint64,
	}})
}

func (s ApplyCheck) BuildBlock(a *app.App, req *abci.RequestFinalizeBlock, home string, opts servertypes.AppOptions) error {
	ctx := a.GetCheckCtx()
	if !a.Sequenced(ctx, req.Header.Height) {
		return nil
	}
	pins, err := a.SequencerKeeper.ReceiptKeyPins(ctx)
	if err != nil {
		return err
	}
	key := s.ReceiptKey
	if key == nil {
		keyFile := cast.ToString(opts.Get(FlagReceiptKeyFile))
		if keyFile == "" {
			return fmt.Errorf("sequenced home requires sequencer.receipt-key-file in config/app.toml")
		}
		key, err = LoadReceiptKey(keyFile, tmconfig.DefaultConfig().SetRoot(home).PrivValidator.KeyFile(), pins, req.Header.Height)
		if err != nil {
			return err
		}
	}
	keyID, err := key.KeyID(req.Header.Height)
	if err != nil {
		return err
	}
	last, err := a.SequencerKeeper.LastSeq(ctx)
	if err != nil {
		return err
	}
	previous, err := a.SequencerKeeper.LastSeqHash(ctx)
	if err != nil {
		return err
	}
	if uint64(len(req.Txs)) > ^uint64(0)-last {
		return ErrSeqExhausted
	}
	typed := make([]sdk.Tx, len(req.Txs))
	ending := previous
	for i, raw := range req.Txs {
		typed[i], err = a.GetTxConfig().TxDecoder()(raw)
		if err != nil {
			return fmt.Errorf("transaction %d: %w", i+1, err)
		}
		activity := sha256.Sum256(raw)
		if msg := a.GetEVMMsg(typed[i]); msg != nil {
			if tx, _ := msg.AsTransaction(); tx != nil {
				activity = tx.Hash()
			}
		}
		ending = seqtypes.SeqHash(ending, last+uint64(i)+1, activity)
	}
	root := a.SequencedTxRoot(req.Txs, typed)
	msg := seqtypes.MsgSequenceBatch{Height: uint64(req.Header.Height), FromSeq: last + 1, ToSeq: last + uint64(len(req.Txs)),
		PrevSeqHash: previous[:], LastSeqHash: ending[:], TxRoot: root[:], KeyId: keyID[:]}
	digest, err := msg.SignBytes(req.Header.ChainID)
	if err != nil {
		return err
	}
	msg.Sig = ed25519.Sign(key.private, digest[:])
	batch, err := app.EncodeSequenceBatch(msg)
	if err != nil {
		return err
	}
	req.Txs = append([][]byte{batch}, req.Txs...)
	return nil
}

type applyCheckHost struct {
	*app.App
	req       *abci.RequestFinalizeBlock
	sequenced bool
}

func (h applyCheckHost) BeginBlock(ctx sdk.Context, height int64, _ []abci.VoteInfo, evidence []abci.Misbehavior, check bool) abci.ResponseBeginBlock {
	return h.App.BeginBlock(ctx.WithBlockHeader(*h.req.Header).WithHeaderHash(h.req.Hash), height, h.req.DecidedLastCommit.Votes, evidence, check)
}

func (h applyCheckHost) CacheTxContext(ctx sdk.Context, checksum [32]byte) (sdk.Context, sdk.CacheMultiStore) {
	// Before activation there is no sequence batch, so the first supplied
	// transaction occupies index zero in the block being compared.
	if !h.sequenced {
		ctx = ctx.WithTxIndex(ctx.TxIndex() - 1)
	}
	return h.App.CacheTxContext(ctx.WithBlockHeader(*h.req.Header).WithHeaderHash(h.req.Hash), checksum)
}

func (ApplyCheck) Execute(ctx context.Context, a *app.App, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	sequenced := a.Sequenced(a.GetCheckCtx(), req.Header.Height)
	last, err := a.SequencerKeeper.LastSeq(a.GetCheckCtx())
	if err != nil {
		return nil, err
	}
	hash, err := a.SequencerKeeper.LastSeqHash(a.GetCheckCtx())
	if err != nil {
		return nil, err
	}
	host := applyCheckHost{App: a, req: req, sequenced: sequenced}
	executor, err := NewExecutor(host, Options{ChainID: req.Header.ChainID, ProposerAddress: req.Header.ProposerAddress, LastSeq: last, LastSeqHash: hash})
	if err != nil {
		return nil, err
	}
	floor, err := NewFeeFloor(executor, host, a.GetCheckCtx().MinGasPrices().String(), a.ParamsKeeper, a.OracleKeeper, a.PricefeedKeeper, &a.EvmKeeper)
	if err != nil {
		return nil, err
	}
	if err := floor.Open(req.Header.Height, req.Header.Time.UnixMilli()); err != nil {
		if errors.Is(err, ErrBeginBlock) {
			return nil, &app.ApplyCheckPanicError{Message: err.Error()}
		}
		return nil, err
	}
	a.SetOpenBlock(floor)
	defer a.SetOpenBlock(nil)
	first := 0
	if sequenced {
		first = 1
	}
	for i, tx := range req.Txs[first:] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := floor.Accept(tx); err != nil {
			if errors.Is(err, sdkerrors.ErrPanic) {
				return nil, &app.ApplyCheckPanicError{Message: err.Error()}
			}
			return nil, &app.ApplyCheckTransactionError{Transaction: i + 1, Err: err}
		}
	}
	batch, err := floor.Seal(req.Header.Height)
	if err != nil {
		return nil, err
	}
	batch.Ctx = batch.Ctx.WithBlockHeader(*req.Header).WithHeaderHash(req.Hash)
	if sequenced {
		msg, err := app.DecodeSequenceBatch(req.Txs[0])
		if err != nil {
			return nil, err
		}
		layer, _, err := SealBatch(a, batch, msg)
		if err != nil {
			if errors.Is(err, ErrSealPanicked) {
				return nil, &app.ApplyCheckPanicError{Message: err.Error()}
			}
			return nil, err
		}
		a.SetSequencerSource(&ConsensusSource{app: a, exec: floor, sealed: layer})
		return a.FinalizeBlock(ctx, req)
	}
	return a.FinalizeApplyCheckLayer(ctx, batch.Ctx, req, batch.Results, append([]*evmtypes.MsgEVMTransaction(nil), batch.EVMMsgs...))
}

var _ app.ApplyCheckSequencer = ApplyCheck{}
