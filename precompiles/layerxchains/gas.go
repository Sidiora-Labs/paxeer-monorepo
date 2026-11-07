package layerxchains

import pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"

var methodGas = pcommon.GasTable{
	"createChain":     chainsGas(60000, true),
	"fundChain":       chainsGas(30000, true),
	"pauseChain":      chainsGas(20000, true),
	"resumeChain":     chainsGas(20000, true),
	"retireChain":     chainsGas(20000, true),
	"governancePause": chainsGas(20000, true),
	"setOperator":     chainsGas(20000, true),
	"getChain":        chainsGas(3000, false),
	"chainByName":     chainsGas(3000, false),
	"chainCount":      chainsGas(3000, false),
	"chainIdOf":       chainsGas(3000, false),
	"params":          chainsGas(3000, false),
}

func chainsGas(flat uint64, write bool) pcommon.MethodGas {
	return pcommon.MethodGas{Write: write, Surcharge: flat - pcommon.DefaultGasCost(nil, write)}
}
