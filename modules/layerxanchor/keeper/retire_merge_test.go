package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRetireAtMergeReturnsBondsAndEmits(t *testing.T) {
	for _, name := range []string{"guarantor_bonds", "mixed_escrow", "empty"} {
		t.Run(name, func(t *testing.T) {
			s := newSuite(t, nil)
			t.Cleanup(func() { require.NoError(t, s.app.Close()) })
			if name != "empty" {
				s.bondAll()
			}
			challengeBond := sdk.ZeroInt()
			if name == "mixed_escrow" {
				_, err := s.submit("batch_1_quorum")
				require.NoError(t, err)
				_, err = s.k.BeginUnbond(s.ctx, s.operators[0], s.ids[0], sdk.NewInt(1_000_000))
				require.NoError(t, err)
				challengeBond = s.k.GetParams(s.ctx).ChallengeBond
				s.fund(s.reporter, challengeBond.Int64())
				_, err = s.k.OpenChallenge(s.ctx, s.reporter, 1, types.ChallengeFraud, [32]byte{1}, challengeBond)
				require.NoError(t, err)
			}
			wantTotal := sdk.ZeroInt()
			if name != "empty" {
				wantTotal = sdk.NewInt(int64(len(s.operators)) * bond).Add(challengeBond)
			}
			require.Equal(t, wantTotal, s.k.ModuleBalance(s.ctx))
			require.Equal(t, wantTotal, s.k.Escrowed(s.ctx))
			balances := make([]sdk.Int, len(s.operators))
			for i, operator := range s.operators {
				balances[i] = s.balance(operator)
			}
			reporterBalance := s.balance(s.reporter)
			supply := s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom)
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

			total, err := s.k.RetireAtMerge(s.ctx)
			require.NoError(t, err)
			require.Equal(t, wantTotal, total)
			for i, operator := range s.operators {
				wantRefund := sdk.ZeroInt()
				if name != "empty" {
					wantRefund = sdk.NewInt(bond)
				}
				require.Equal(t, balances[i].Add(wantRefund), s.balance(operator))
			}
			require.Equal(t, reporterBalance.Add(challengeBond), s.balance(s.reporter))
			for _, guarantor := range s.k.GetGuarantors(s.ctx) {
				require.True(t, guarantor.Bond.IsZero())
			}
			require.Empty(t, s.k.GetUnbondings(s.ctx))
			require.True(t, s.k.ModuleBalance(s.ctx).IsZero())
			require.True(t, s.k.Escrowed(s.ctx).IsZero())
			message, broken := keeper.BalanceInvariant(s.k)(s.ctx)
			require.False(t, broken, message)
			require.Equal(t, supply, s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom))
			var retired sdk.Events
			for _, event := range s.ctx.EventManager().Events() {
				if event.Type == types.EventAnchorRetired {
					retired = append(retired, event)
				}
			}
			require.Equal(t, sdk.Events{sdk.NewEvent("layerx_anchor_retired",
				sdk.NewAttribute("returned_bond", wantTotal.String()))}, retired)
		})
	}

	t.Run("refund_error", func(t *testing.T) {
		s := newSuite(t, nil)
		t.Cleanup(func() { require.NoError(t, s.app.Close()) })
		s.bondAll()
		coins := sdk.NewCoins(sdk.NewInt64Coin(s.k.GetParams(s.ctx).BondDenom, 1))
		require.NoError(t, s.app.BankKeeper.SendCoinsFromAccountToModule(s.ctx, s.operators[0], types.ModuleName, coins))
		state := s.k.ExportGenesis(s.ctx)
		balance := s.k.ModuleBalance(s.ctx)
		balances := make([]sdk.Int, len(s.operators))
		for i, operator := range s.operators {
			balances[i] = s.balance(operator)
		}
		cached, _ := s.ctx.CacheContext()
		_, wantErr := s.k.ReturnAllBonds(cached)
		require.ErrorIs(t, wantErr, types.ErrBond)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

		total, err := s.k.RetireAtMerge(s.ctx)
		require.ErrorIs(t, err, types.ErrBond)
		require.EqualError(t, err, wantErr.Error())
		require.True(t, total.IsZero())
		require.Equal(t, state, s.k.ExportGenesis(s.ctx))
		require.Equal(t, balance, s.k.ModuleBalance(s.ctx))
		for i, operator := range s.operators {
			require.Equal(t, balances[i], s.balance(operator))
		}
		require.Empty(t, s.ctx.EventManager().Events())
	})
}
