package layerxpay

import paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"

type LayerXPayKeepers interface {
	LayerXPayK() *paykeeper.Keeper
}
