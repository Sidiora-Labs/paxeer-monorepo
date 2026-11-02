package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestWithdrawableUsesWithdrawFloorPpm(t *testing.T) {
	env := newMarginEnv(t)
	env.list(t, 0, "BTC", 5, 1, types.TierClassA)
	mark := e18(1_000)
	mark.AddUint64(&mark, 1)
	env.setPrices(0, mark, uint256.Int{})
	account := marginAccount(1)
	env.putAccount(t, account, 1_000_000_000)
	env.putPosition(t, 1, account, 0, true, e18(1), mark, uint256.Int{}, e18(20))

	for _, tc := range []struct {
		name string
		ppm  uint64
		want uint64
	}{
		{"raised floor", 200_000, 799_999_999},
		{"fractional rate rounds up", 333_333, 666_666_999},
		{"lowered floor rounds up", 50_000, 949_999_999},
		{"initial margin dominates", 10_000, 950_000_000},
		{"zero floor retains initial margin", 0, 950_000_000},
		{"floor exceeds equity", types.PpmDenominator, 0},
		{"subsequent update takes effect", 200_000, 799_999_999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			params.WithdrawFloorPpm = tc.ppm
			require.NoError(t, env.k.UpdateParams(env.ctx, env.k.Authority(), params))
			withdrawable, err := env.k.Withdrawable(env.ctx, account)
			require.NoError(t, err)
			require.Equal(t, *uint256.NewInt(tc.want), withdrawable)
		})
	}
}

func TestWithdrawableDefaultFloorUnchanged(t *testing.T) {
	require.Equal(t, uint64(100_000), types.DefaultParams().WithdrawFloorPpm)
	for _, tc := range []struct {
		name       string
		position   bool
		fractional bool
		leverage   int64
		collateral uint64
		want       uint64
	}{
		{"no positions", false, false, 20, 250_000_000, 250_000_000},
		{"ten percent floor", true, false, 20, 250_000_000, 150_000_000},
		{"fractional notional rounds up", true, true, 20, 250_000_000, 149_999_999},
		{"initial margin dominates", true, false, 5, 250_000_000, 50_000_000},
		{"insufficient equity", true, false, 20, 50_000_000, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newMarginEnv(t)
			env.list(t, 0, "BTC", 5, 1, types.TierClassA)
			account := marginAccount(1)
			env.putAccount(t, account, tc.collateral)
			notional := new(big.Int)
			if tc.position {
				mark := e18(1_000)
				if tc.fractional {
					mark.AddUint64(&mark, 1)
				}
				env.setPrices(0, mark, uint256.Int{})
				env.putPosition(t, 1, account, 0, true, e18(1), mark, uint256.Int{}, e18(tc.leverage))
				notional.Set(mark.ToBig())
			}
			margin, err := env.k.AccountMargin(env.ctx, account)
			require.NoError(t, err)
			required, remainder := new(big.Int).QuoRem(notional, big.NewInt(10), new(big.Int))
			if remainder.Sign() > 0 {
				required.Add(required, big.NewInt(1))
			}
			if initial := margin.InitialMargin.BigInt(); initial.Cmp(required) > 0 {
				required = initial
			}
			legacy := new(big.Int).Sub(margin.Equity.BigInt(), required)
			if legacy.Sign() < 0 {
				legacy.SetInt64(0)
			}
			legacy.Quo(legacy, big.NewInt(1_000_000_000_000))
			require.Zero(t, legacy.Cmp(new(big.Int).SetUint64(tc.want)))

			for _, stored := range []bool{false, true} {
				if stored {
					require.NoError(t, env.k.UpdateParams(env.ctx, env.k.Authority(), types.DefaultParams()))
				}
				withdrawable, err := env.k.Withdrawable(env.ctx, account)
				require.NoError(t, err)
				require.Zero(t, legacy.Cmp(withdrawable.ToBig()), "stored defaults: %t", stored)
			}
		})
	}
}
