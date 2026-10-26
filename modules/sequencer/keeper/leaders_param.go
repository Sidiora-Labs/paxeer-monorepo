package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

var KeyLeaders = []byte("Leaders")

type LeaderList [][]byte

func (list LeaderList) toLeaders() ([][32]byte, error) {
	if list == nil {
		return nil, nil
	}
	leaders := make([][32]byte, len(list))
	for i, key := range list {
		if len(key) != 32 {
			return nil, types.ErrInvalidGenesis.Wrapf("leader %d is %d bytes, want 32", i, len(key))
		}
		copy(leaders[i][:], key)
	}
	return leaders, nil
}

func validateLeaders(value interface{}) error {
	list, ok := value.(LeaderList)
	if !ok {
		return fmt.Errorf("%w: invalid parameter type %T", types.ErrInvalidGenesis, value)
	}
	leaders, err := list.toLeaders()
	if err != nil {
		return err
	}
	return types.ValidateLeaders(leaders)
}

func GetLeadersParam(ctx sdk.Context, paramSpace paramtypes.Subspace) [][32]byte {
	var list LeaderList
	paramSpace.GetIfExists(ctx, KeyLeaders, &list)
	leaders, err := list.toLeaders()
	if err != nil {
		panic(err)
	}
	return leaders
}

func SetLeadersParam(ctx sdk.Context, paramSpace paramtypes.Subspace, leaders [][32]byte) error {
	if err := types.ValidateLeaders(leaders); err != nil {
		return err
	}
	list := make(LeaderList, len(leaders))
	for i := range leaders {
		list[i] = leaders[i][:]
	}
	paramSpace.Set(ctx, KeyLeaders, list)
	return nil
}
