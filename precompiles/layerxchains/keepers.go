package layerxchains

import (
	chainskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
)

type LayerXChainsKeepers interface {
	LayerXChainsK() *chainskeeper.Keeper
}

type chainsEVMKeeper interface {
	utils.EVMKeeper
	BankKeeper() bankkeeper.Keeper
	GetMinimumFeePerGas(sdk.Context) sdk.Dec
}
