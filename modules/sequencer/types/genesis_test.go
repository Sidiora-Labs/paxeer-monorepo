package types_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisActivationHeightZeroAccepted(t *testing.T) {
	_, public := receiptKey()
	gs := types.GenesisState{
		Leaders: [][32]byte{{1}},
		ReceiptKeys: []types.ReceiptKey{{
			KeyId:       [types.KeyIDLength]byte{1},
			PublicKey:   public,
			FirstHeight: 10,
			LastHeight:  20,
		}},
		ActivationHeight: 0,
	}

	require.False(t, gs.ReceiptKeys[0].Covers(gs.ActivationHeight))
	require.NoError(t, gs.Validate())
}

func TestGenesisRefusesActivationWithoutCoveringKey(t *testing.T) {
	_, public := receiptKey()
	for _, tc := range []struct {
		name    string
		height  uint64
		covered bool
	}{
		{name: "before_first_key", height: 9},
		{name: "first_key_start", height: 10, covered: true},
		{name: "first_key_middle", height: 15, covered: true},
		{name: "first_key_end", height: 20, covered: true},
		{name: "gap_after_first_key", height: 21},
		{name: "gap_before_second_key", height: 29},
		{name: "second_key_start", height: 30, covered: true},
		{name: "second_key_middle", height: 35, covered: true},
		{name: "second_key_end", height: 40, covered: true},
		{name: "after_last_key", height: 41},
		{name: "maximum_height", height: ^uint64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.GenesisState{
				Leaders: [][32]byte{{1}},
				ReceiptKeys: []types.ReceiptKey{
					{KeyId: [types.KeyIDLength]byte{1}, PublicKey: public, FirstHeight: 10, LastHeight: 20},
					{KeyId: [types.KeyIDLength]byte{2}, PublicKey: public, FirstHeight: 30, LastHeight: 40},
				},
				ActivationHeight: tc.height,
			}

			err := gs.Validate()
			if tc.covered {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.Contains(t, err.Error(), fmt.Sprintf("no receipt key covers activation height %d", tc.height))
		})
	}
}
