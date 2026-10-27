package app

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/stretchr/testify/require"
)

type seamsSequencerSource struct {
	activation int64
	sealed     map[int64]tmtypes.SealedBlock
}

func (s *seamsSequencerSource) Sequenced(height int64) bool {
	return height >= s.activation
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
	require.True(t, rcpt == app.sequencerReceipt, "sequencer receipt option must retain its original pointers")
	require.Same(t, forwardURL, app.sequencerReceipt.ForwardURL)
}

func TestSetCommitListenerKeepsListener(t *testing.T) {
	app := Setup(t, false, false, false)
	require.Nil(t, app.commitListener)
	firstCalls, secondCalls := 0, 0
	var gotHeight int64
	var gotHash []byte
	app.SetCommitListener(func(height int64, appHash []byte) {
		firstCalls++
		gotHeight, gotHash = height, appHash
	})
	require.NotNil(t, app.commitListener)
	app.commitListener(11, []byte{1, 2, 3})
	require.Equal(t, 1, firstCalls)
	require.Equal(t, int64(11), gotHeight)
	require.Equal(t, []byte{1, 2, 3}, gotHash)

	app.SetCommitListener(func(height int64, appHash []byte) {
		secondCalls++
		gotHeight, gotHash = height, appHash
	})
	require.NotNil(t, app.commitListener)
	app.commitListener(12, []byte{4, 5, 6})
	require.Equal(t, 1, firstCalls)
	require.Equal(t, 1, secondCalls)
	require.Equal(t, int64(12), gotHeight)
	require.Equal(t, []byte{4, 5, 6}, gotHash)
}
