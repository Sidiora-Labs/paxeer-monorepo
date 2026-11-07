package layerxauth

import (
	"embed"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed abi.json
var f embed.FS

// GetABI returns the embedded Solidity ABI used by both EVM dispatch and log
// consumers. Each call parses an independent value so callers cannot mutate the
// method or event maps of another precompile instance.
func GetABI() abi.ABI {
	return pcommon.MustGetABI(f, "abi.json")
}
