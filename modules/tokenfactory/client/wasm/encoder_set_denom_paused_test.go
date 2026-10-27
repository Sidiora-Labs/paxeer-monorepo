package wasm

import (
	"encoding/json"
	"testing"

	tfbindings "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm/bindings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestEncodeTokenFactorySetDenomPaused(t *testing.T) {
	sender, err := sdk.AccAddressFromBech32("pax1y3pxq5dp900czh0mkudhjdqjq5m8cpmmsnt288")
	require.NoError(t, err)
	denom := "factory/" + sender.String() + "/a"

	for _, tc := range []struct {
		name   string
		paused bool
		raw    string
	}{
		{name: "pause", paused: true, raw: `{"denom":"` + denom + `","paused":true}`},
		{name: "unpause", paused: false, raw: `{"denom":"` + denom + `","paused":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tfbindings.SetDenomPaused{Denom: denom, Paused: tc.paused})
			require.NoError(t, err)
			require.JSONEq(t, tc.raw, string(encoded))

			msgs, err := EncodeTokenFactorySetDenomPaused(json.RawMessage(tc.raw), sender)
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			msg, ok := msgs[0].(*types.MsgSetDenomPaused)
			require.True(t, ok)
			require.Equal(t, &types.MsgSetDenomPaused{
				Sender: sender.String(),
				Denom:  denom,
				Paused: tc.paused,
			}, msg)
		})
	}

	t.Run("sender comes from contract", func(t *testing.T) {
		raw := json.RawMessage(`{"denom":"` + denom + `","paused":true,"sender":"another-account"}`)
		msgs, err := EncodeTokenFactorySetDenomPaused(raw, sender)
		require.NoError(t, err)
		require.Equal(t, []sdk.Msg{&types.MsgSetDenomPaused{
			Sender: sender.String(),
			Denom:  denom,
			Paused: true,
		}}, msgs)
	})

	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "invalid denom type", raw: `{"denom":1}`},
		{name: "invalid paused type", raw: `{"denom":"` + denom + `","paused":"true"}`},
		{name: "malformed JSON", raw: `{"denom":`},
		{name: "empty JSON", raw: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msgs, err := EncodeTokenFactorySetDenomPaused(json.RawMessage(tc.raw), sender)
			require.ErrorIs(t, err, types.ErrEncodeTokenFactorySetDenomPaused)
			require.Equal(t, []sdk.Msg{}, msgs)
		})
	}
}
