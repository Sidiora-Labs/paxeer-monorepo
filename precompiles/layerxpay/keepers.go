package layerxpay

import paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"

// LayerXPayKeepers supplies the application's shared payment keeper to the
// merge constructor. Reconstructing a precompile must retain this keeper's state.
type LayerXPayKeepers interface {
	LayerXPayK() *paykeeper.Keeper
}
