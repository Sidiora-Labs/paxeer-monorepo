package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/stretchr/testify/require"
)

type seamsSequencerSource struct {
	activation int64
	admitted   map[int64]tmtypes.Txs
	sealed     map[int64]tmtypes.SealedBlock
}

func (s *seamsSequencerSource) Sequenced(height int64) bool {
	return height >= s.activation
}

func (s *seamsSequencerSource) Admit(ctx context.Context, height int64, txs tmtypes.Txs) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !s.Sequenced(height) {
		return 0, fmt.Errorf("height %d is not sequenced", height)
	}
	if s.admitted == nil {
		s.admitted = make(map[int64]tmtypes.Txs)
	}
	s.admitted[height] = append(s.admitted[height], txs...)
	return len(txs), nil
}

func (s *seamsSequencerSource) Seal(ctx context.Context, height int64) (tmtypes.SealedBlock, error) {
	if err := ctx.Err(); err != nil {
		return tmtypes.SealedBlock{}, err
	}
	block, ok := s.sealed[height]
	if !ok {
		return tmtypes.SealedBlock{}, fmt.Errorf("no sealed block at height %d", height)
	}
	return block, nil
}

func TestSetSequencerSourceReturnsInstalledSource(t *testing.T) {
	app := Setup(t, false, false, false)
	require.Nil(t, app.SequencerSource())
	block := tmtypes.SealedBlock{
		Batch: tmtypes.Tx("sequence-batch-11"),
		Txs:   tmtypes.Txs{tmtypes.Tx("seq-a-11=1")},
	}
	source := &seamsSequencerSource{
		activation: 11,
		sealed:     map[int64]tmtypes.SealedBlock{11: block},
	}

	app.SetSequencerSource(source)

	require.Same(t, source, app.SequencerSource())
	require.False(t, app.SequencerSource().Sequenced(10))
	require.True(t, app.SequencerSource().Sequenced(11))
	admitted, err := app.SequencerSource().Admit(context.Background(), 10, block.Txs)
	require.EqualError(t, err, "height 10 is not sequenced")
	require.Zero(t, admitted)
	require.NotContains(t, source.admitted, int64(10))
	admitted, err = app.SequencerSource().Admit(context.Background(), 11, block.Txs)
	require.NoError(t, err)
	require.Equal(t, 1, admitted)
	require.Equal(t, block.Txs, source.admitted[11])
	sealed, err := app.SequencerSource().Seal(context.Background(), 11)
	require.NoError(t, err)
	require.Equal(t, block, sealed)
}

func TestSetSequencerRPCKeepsOptions(t *testing.T) {
	app := Setup(t, false, false, false)
	opt := evmrpc.NewSequencerOption(nil, evmrpc.NewReceiptLogFeed(0))
	forwardURL, err := url.Parse("https://sequencer.example/evm")
	require.NoError(t, err)
	rcpt := evmrpc.SequencerReceiptOption{ForwardURL: forwardURL}

	app.SetSequencerRPC(opt, rcpt)

	require.True(t, opt == app.sequencerRPC, "sequencer RPC option must retain its original pointers")
	require.Same(t, forwardURL, app.sequencerReceipt.ForwardURL)
	require.Nil(t, rcpt.Keeper, "the caller's receipt option is passed by value")
	require.Same(t, &app.SequencerKeeper, app.sequencerReceipt.Keeper,
		"forwarded receipts use the application's sequencer keeper")

	providedKeeper := new(seqkeeper.Keeper)
	providedURL, err := url.Parse("https://sequencer.example/receipts")
	require.NoError(t, err)
	provided := Setup(t, false, false, false)
	provided.SetSequencerRPC(opt, evmrpc.SequencerReceiptOption{ForwardURL: providedURL, Keeper: providedKeeper})
	require.Same(t, providedKeeper, provided.sequencerReceipt.Keeper,
		"an explicitly supplied keeper is retained")
	require.Same(t, providedURL, provided.sequencerReceipt.ForwardURL)

	withoutForward := Setup(t, false, false, false)
	withoutForward.SetSequencerRPC(opt, evmrpc.SequencerReceiptOption{})
	require.Nil(t, withoutForward.sequencerReceipt.Keeper,
		"a receipt option without forwarding keeps no implicit keeper")
}

func TestSetCommitListenerKeepsListener(t *testing.T) {
	app := Setup(t, false, false, false)
	require.Nil(t, app.commitListener)
	firstCalls, secondCalls := 0, 0
	var gotHeight int64
	var gotHash []byte
	app.SetCommitListener(func(height int64, appHash []byte) error {
		firstCalls++
		gotHeight, gotHash = height, appHash
		return nil
	})
	require.NotNil(t, app.commitListener)
	require.NoError(t, app.commitListener(11, []byte{1, 2, 3}))
	require.Equal(t, 1, firstCalls)
	require.Equal(t, int64(11), gotHeight)
	require.Equal(t, []byte{1, 2, 3}, gotHash)

	listenerErr := errors.New("replacement listener failed")
	app.SetCommitListener(func(height int64, appHash []byte) error {
		secondCalls++
		gotHeight, gotHash = height, appHash
		return listenerErr
	})
	require.NotNil(t, app.commitListener)
	require.ErrorIs(t, app.commitListener(12, []byte{4, 5, 6}), listenerErr)
	require.Equal(t, 1, firstCalls)
	require.Equal(t, 1, secondCalls)
	require.Equal(t, int64(12), gotHeight)
	require.Equal(t, []byte{4, 5, 6}, gotHash)
}
