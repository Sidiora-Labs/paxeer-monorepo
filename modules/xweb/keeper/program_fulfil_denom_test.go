package keeper

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestFulfilProgramSplitsInTheRecordedDenom(t *testing.T) {
	assets := []programFulfilAsset{paxProgramAsset, sidProgramAsset,
		{types.Hash32{11}, types.ProgramFeeDenomUSDC}, {types.Hash32{12}, types.ProgramFeeDenomUSDL}}
	for _, asset := range assets {
		t.Run(asset.denom, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				amount int64
				first  int
				want   []int64
			}{
				{"floor_and_remainder", 75, 0, []int64{21, 18, 18, 18}},
				{"zero_floor_with_subset", 2, 1, []int64{0, 2, 0, 0}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					s := newProgramFulfilSuite(t, 4)
					s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, asset, sdk.NewInt(tc.amount))
					denom, found := s.k.ProgramFeeDenom(s.ctx, asset.id)
					require.True(t, found)
					require.Equal(t, asset.denom, denom)
					require.Equal(t, asset.id, s.pending(programFulfilP, 3).FeeAsset)

					require.NoError(t, s.k.FulfilProgram(s.ctx,
						s.signed(s.observation(programFulfilP, 3, []byte("answer"), 6), s.keys[tc.first:]...)))
					for index, want := range tc.want {
						require.Equal(t, sdk.NewInt(want), s.balance(s.payout(index), denom), "signer %d", index)
						for _, other := range assets {
							if other.denom != denom {
								require.True(t, s.balance(s.payout(index), other.denom).IsZero())
							}
						}
					}
					require.True(t, s.balance(ProgramFeeAccount(asset.id), denom).IsZero())
					require.Equal(t, []byte("answer"), s.answerPart(programFulfilP, 3, 1))
					require.Equal(t, byte(1), s.fulfilledByte(programFulfilP, 3))
					require.Zero(t, s.k.ProgramPendingCount(s.ctx))
					require.Equal(t, uint64(1), s.k.ProgramCommittedCount(s.ctx))
				})
			}
		})
	}
}

func TestFulfilProgramIgnoresAThirdPartyDepositOfAnotherDenom(t *testing.T) {
	assets := []programFulfilAsset{paxProgramAsset, sidProgramAsset,
		{types.Hash32{11}, types.ProgramFeeDenomUSDC}, {types.Hash32{12}, types.ProgramFeeDenomUSDL}}
	for _, asset := range assets {
		for _, other := range assets {
			if asset.denom == other.denom {
				continue
			}
			t.Run(asset.denom+"_with_"+other.denom, func(t *testing.T) {
				for _, underfunded := range []bool{false, true} {
					name := "funded"
					if underfunded {
						name = "underfunded"
					}
					t.Run(name, func(t *testing.T) {
						s := newProgramFulfilSuite(t, 4)
						s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, asset, sdk.NewInt(75))
						account := ProgramFeeAccount(asset.id)
						donor := sdk.AccAddress(bytes.Repeat([]byte{0xd1}, 20))
						deposit := sdk.NewCoins(sdk.NewInt64Coin(other.denom, 150))
						require.NoError(t, s.bank.MintCoins(s.ctx, programFulfilMinter, deposit))
						require.NoError(t, s.bank.SendCoinsFromModuleToAccount(s.ctx, programFulfilMinter, donor, deposit))
						require.NoError(t, s.bank.SendCoins(s.ctx, donor, account, deposit))
						if underfunded {
							require.NoError(t, s.bank.SendCoins(s.ctx, account, programFulfilPayer,
								sdk.NewCoins(sdk.NewInt64Coin(asset.denom, 30))))
						}
						before := s.state()
						err := s.k.FulfilProgram(s.ctx, s.signed(
							s.observation(programFulfilP, 3, make([]byte, 1500), 1500), s.keys...))
						if underfunded {
							require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
							require.Equal(t, before, s.state())
							for part := uint8(0); part <= 2; part++ {
								require.Nil(t, s.answerPart(programFulfilP, 3, part))
							}
							require.Equal(t, byte(0), s.fulfilledByte(programFulfilP, 3))
							require.Equal(t, uint64(1), s.k.ProgramPendingCount(s.ctx))
							require.Zero(t, s.k.ProgramCommittedCount(s.ctx))
							require.Equal(t, sdk.NewInt(45), s.balance(account, asset.denom))
						} else {
							require.NoError(t, err)
							for index, want := range []int64{21, 18, 18, 18} {
								require.Equal(t, sdk.NewInt(want), s.balance(s.payout(index), asset.denom), "signer %d", index)
							}
							require.True(t, s.balance(account, asset.denom).IsZero())
							require.Equal(t, byte(1), s.fulfilledByte(programFulfilP, 3))
							require.Zero(t, s.k.ProgramPendingCount(s.ctx))
							require.Equal(t, uint64(1), s.k.ProgramCommittedCount(s.ctx))
						}
						require.Equal(t, sdk.NewInt(150), s.balance(account, other.denom))
						require.True(t, s.balance(donor, other.denom).IsZero())
						for index := range s.keys {
							require.True(t, s.balance(s.payout(index), other.denom).IsZero())
						}
					})
				}
			})
		}
	}
}

func TestFulfilProgramRefusesAnAssetWithoutARecordedDenom(t *testing.T) {
	for _, asset := range []programFulfilAsset{paxProgramAsset, sidProgramAsset,
		{types.Hash32{11}, types.ProgramFeeDenomUSDC}, {types.Hash32{12}, types.ProgramFeeDenomUSDL}} {
		t.Run(asset.denom, func(t *testing.T) {
			s := newProgramFulfilSuite(t, 4)
			s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, asset, sdk.NewInt(75))
			s.ctx.KVStore(s.k.storeKey).Delete(ProgramFeeDenomKey(asset.id))
			_, found := s.k.ProgramFeeDenom(s.ctx, asset.id)
			require.False(t, found)
			before := s.state()
			events := s.ctx.EventManager().Events()

			err := s.k.FulfilProgram(s.ctx, s.signed(
				s.observation(programFulfilP, 3, make([]byte, 1500), 1500), s.keys...))
			require.ErrorIs(t, err, errProgramAssetMismatch)
			require.EqualError(t, err, "LXP_ASSET_MISMATCH")
			require.Equal(t, before, s.state())
			require.Equal(t, events, s.ctx.EventManager().Events())
			for part := uint8(0); part <= 2; part++ {
				require.Nil(t, s.answerPart(programFulfilP, 3, part))
			}
			require.Equal(t, byte(0), s.fulfilledByte(programFulfilP, 3))
			require.Equal(t, uint64(1), s.k.ProgramPendingCount(s.ctx))
			require.Zero(t, s.k.ProgramCommittedCount(s.ctx))
			require.Equal(t, sdk.NewInt(75), s.balance(ProgramFeeAccount(asset.id), asset.denom))
			for index := range s.keys {
				require.True(t, s.balance(s.payout(index), asset.denom).IsZero())
			}
		})
	}
}
