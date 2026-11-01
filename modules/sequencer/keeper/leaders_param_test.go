package keeper_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/stretchr/testify/require"
)

func TestParamKeyTableRegistersLeaders(t *testing.T) {
	env := newReceiptKeysEnv(t, 6000)
	require.Equal(t, []byte("Leaders"), keeper.KeyLeaders)
	require.Nil(t, keeper.GetLeadersParam(env.ctx, env.paramSpace))
	require.False(t, env.paramSpace.Has(env.ctx, keeper.KeyLeaders))
	require.NoError(t, env.paramSpace.Validate(env.ctx, keeper.KeyReceiptKeys, keeper.ReceiptKeys{}))

	leaders := [][32]byte{{1}, {0: 1, 31: 1}, {31: 1}}
	require.NoError(t, keeper.SetLeadersParam(env.ctx, env.paramSpace, leaders))
	require.Equal(t, leaders, keeper.GetLeadersParam(env.ctx, env.paramSpace))
	read := keeper.GetLeadersParam(env.ctx, env.paramSpace)
	read[0][0] = 9
	require.Equal(t, leaders, keeper.GetLeadersParam(env.ctx, env.paramSpace))

	rotated := keeper.LeaderList{bytes.Repeat([]byte{2}, 32), bytes.Repeat([]byte{3}, 32)}
	require.NoError(t, env.paramSpace.Validate(env.ctx, keeper.KeyLeaders, rotated))
	value, err := env.amino.MarshalAsJSON(rotated)
	require.NoError(t, err)
	p := proposal.NewParameterChangeProposal("leaders", "rotate leaders", []proposal.ParamChange{
		proposal.NewParamChange(types.ModuleName, string(keeper.KeyLeaders), string(value)),
	}, false)
	require.NoError(t, env.handler(env.ctx, p))
	require.Equal(t, [][32]byte{[32]byte(rotated[0]), [32]byte(rotated[1])}, keeper.GetLeadersParam(env.ctx, env.paramSpace))

	for name, invalid := range map[string]interface{}{
		"wrong type": [][]byte{leaders[0][:]},
		"nil":        keeper.LeaderList(nil),
		"empty":      keeper.LeaderList{},
		"zero key":   keeper.LeaderList{make([]byte, 32)},
		"short key":  keeper.LeaderList{bytes.Repeat([]byte{1}, 31)},
		"long key":   keeper.LeaderList{bytes.Repeat([]byte{1}, 33)},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, env.paramSpace.Validate(env.ctx, keeper.KeyLeaders, invalid))
		})
	}
}

func TestLeadersParamRefusesDuplicate(t *testing.T) {
	key1, key2 := [32]byte{1}, [32]byte{2}
	for name, duplicate := range map[string]keeper.LeaderList{
		"adjacent":  {key1[:], key1[:]},
		"separated": {key1[:], key2[:], key1[:]},
	} {
		for _, pinned := range []bool{false, true} {
			state := "unset"
			if pinned {
				state = "pinned"
			}
			t.Run(name+"/"+state, func(t *testing.T) {
				env := newReceiptKeysEnv(t, 6000)
				var original [][32]byte
				if pinned {
					original = [][32]byte{key2}
					require.NoError(t, keeper.SetLeadersParam(env.ctx, env.paramSpace, original))
				}
				before := bytes.Clone(env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
				err := env.paramSpace.Validate(env.ctx, keeper.KeyLeaders, duplicate)
				require.ErrorContains(t, err, "repeats an earlier leader")
				value, err := env.amino.MarshalAsJSON(duplicate)
				require.NoError(t, err)
				p := proposal.NewParameterChangeProposal("leaders", "refuse duplicate leaders", []proposal.ParamChange{
					proposal.NewParamChange(types.ModuleName, string(keeper.KeyLeaders), string(value)),
				}, false)
				err = env.handler(env.ctx, p)
				require.ErrorIs(t, err, proposal.ErrSettingParameter)
				require.ErrorContains(t, err, "repeats an earlier leader")
				require.Equal(t, before, env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
				require.Equal(t, original, keeper.GetLeadersParam(env.ctx, env.paramSpace))

				leaders := make([][32]byte, len(duplicate))
				for i := range duplicate {
					copy(leaders[i][:], duplicate[i])
				}
				require.ErrorIs(t, keeper.SetLeadersParam(env.ctx, env.paramSpace, leaders), types.ErrInvalidGenesis)
				require.Equal(t, before, env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
				require.Equal(t, pinned, env.paramSpace.Has(env.ctx, keeper.KeyLeaders))
				require.Equal(t, pinned, env.paramSpace.Modified(env.ctx, keeper.KeyLeaders))
			})
		}
	}
}
