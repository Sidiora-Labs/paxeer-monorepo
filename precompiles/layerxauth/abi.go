package layerxauth

import (
	"embed"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed abi.json
var f embed.FS

func GetABI() abi.ABI {
	return pcommon.MustGetABI(f, "abi.json")
}
