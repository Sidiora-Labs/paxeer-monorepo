package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func TestMsgUpdateParamsValidateBasic(t *testing.T) {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	msg := types.MsgUpdateParams{Authority: authority.String(), Params: types.DefaultParams()}
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())

	for _, badAuthority := range []string{"", "not-bech32", "0x1234", sdk.ValAddress(authority).String()} {
		t.Run("authority/"+badAuthority, func(t *testing.T) {
			invalid := types.MsgUpdateParams{Authority: badAuthority, Params: types.DefaultParams()}
			require.ErrorIs(t, invalid.ValidateBasic(), types.ErrInvalidParams)
			require.Empty(t, invalid.GetSigners())
		})
	}

	for _, tc := range []struct {
		name   string
		mutate func(*types.Params)
	}{
		{"zero params", func(p *types.Params) { *p = types.Params{} }},
		{"zero funding cap", func(p *types.Params) { p.FundingCapPpmPerHour = 0 }},
		{"maker above taker", func(p *types.Params) { p.FeeTiers[0].MakerFeePpm = 451 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.mutate(&params)
			require.ErrorIs(t, params.Validate(), types.ErrInvalidParams)
			invalid := types.MsgUpdateParams{Authority: authority.String(), Params: params}
			require.ErrorIs(t, invalid.ValidateBasic(), types.ErrInvalidParams)
		})
	}
}

func TestMsgSetMarketValidateBasic(t *testing.T) {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	market := btcMarket()
	require.NoError(t, market.Validate())
	msg := types.MsgSetMarket{Authority: authority.String(), Market: market}
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())

	for _, tc := range []struct {
		name   string
		mutate func(*types.Market)
	}{
		{"zero market", func(m *types.Market) { *m = types.Market{} }},
		{"empty oracle symbol", func(m *types.Market) { m.OracleSymbol = "" }},
		{"zero max leverage", func(m *types.Market) { m.MaxLeverage = 0 }},
		{"fractional collateral step", func(m *types.Market) { m.SzDecimals = 6; m.PxDecimals = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			market := btcMarket()
			tc.mutate(&market)
			require.ErrorIs(t, market.Validate(), types.ErrInvalidParams)
			invalid := types.MsgSetMarket{Authority: authority.String(), Market: market}
			require.ErrorIs(t, invalid.ValidateBasic(), types.ErrInvalidParams)
		})
	}

	for _, paused := range []bool{false, true} {
		msg := types.MsgSetPaused{Authority: authority.String(), Paused: paused}
		require.NoError(t, msg.ValidateBasic())
		require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())
	}

	for _, badAuthority := range []string{"", "not-bech32", "0x1234", sdk.ValAddress(authority).String()} {
		t.Run("authority/"+badAuthority, func(t *testing.T) {
			marketMsg := types.MsgSetMarket{Authority: badAuthority, Market: btcMarket()}
			require.ErrorIs(t, marketMsg.ValidateBasic(), types.ErrInvalidParams)
			require.Empty(t, marketMsg.GetSigners())
			for _, paused := range []bool{false, true} {
				pauseMsg := types.MsgSetPaused{Authority: badAuthority, Paused: paused}
				require.ErrorIs(t, pauseMsg.ValidateBasic(), types.ErrInvalidParams)
				require.Empty(t, pauseMsg.GetSigners())
			}
		})
	}
}
