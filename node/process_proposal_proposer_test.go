package app_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func newProposerWrapper(t *testing.T) *app.TestWrapper {
	t.Helper()
	w := newSequencerWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	activate(t, w)
	gs, err := w.App.SequencerKeeper.ExportGenesis(w.Ctx)
	require.NoError(t, err)
	gs.Leaders = append(gs.Leaders, sha256.Sum256([]byte("standby one")), sha256.Sum256([]byte("standby two")))
	require.NoError(t, w.App.SequencerKeeper.InitGenesis(w.Ctx, gs))
	w.Ctx = w.Ctx.WithBlockHeight(w.Ctx.BlockHeight() + 1)
	require.True(t, w.App.Sequenced(w.Ctx, w.Ctx.BlockHeight()))
	return w
}

func TestProposerInLeadersAccepts(t *testing.T) {
	t.Run("consensus addresses", AddressHash)
}

func AddressHash(t *testing.T) {
	w := newProposerWrapper(t)
	leaders, err := w.App.SequencerKeeper.Leaders(w.Ctx)
	require.NoError(t, err)
	require.Len(t, leaders, 3)
	require.Equal(t, sequencerLeader, leaders[0][:])
	for _, leader := range leaders {
		t.Run(hex.EncodeToString(leader[:]), func(t *testing.T) {
			require.NoError(t, w.CheckSequencerProposer(w.Ctx, crypto.AddressHash(leader[:])))
		})
	}
}

func TestProposerOutsideLeadersRefused(t *testing.T) {
	w := newProposerWrapper(t)
	outsider := bytes.Clone(sequencerLeader)
	outsider[len(outsider)-1] ^= 1
	for _, tc := range []struct {
		name     string
		proposer []byte
	}{
		{"outside list", outsider},
		{"outside consensus address", crypto.AddressHash(outsider)},
		{"raw leader key", sequencerLeader},
		{"nil", nil},
		{"empty", []byte{}},
		{"zero hash", make([]byte, 32)},
		{"20 byte leader prefix", sequencerLeader[:20]},
		{"31 byte leader prefix", sequencerLeader[:31]},
		{"extended leader", append(bytes.Clone(sequencerLeader), 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := w.CheckSequencerProposer(w.Ctx, tc.proposer)
			require.ErrorIs(t, err, seqtypes.ErrInvalidSequenceBatch)
			require.Contains(t, err.Error(), "proposer "+hex.EncodeToString(tc.proposer)+" is outside the pinned leader list")
		})
	}
}
