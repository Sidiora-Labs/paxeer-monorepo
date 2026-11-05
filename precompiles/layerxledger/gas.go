package layerxledger

import pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"

var GasTable = pcommon.GasTable{
	"register":      {Write: true, Surcharge: 18440},
	"pause":         {Write: true, Surcharge: 3080},
	"unpause":       {Write: true, Surcharge: 3080},
	"openAccount":   {Write: true, Surcharge: 3080},
	"send":          {Write: true, Surcharge: 17480},
	"draw":          {Write: true, Surcharge: 8840},
	"payExact":      {Write: true, Surcharge: 7880},
	"issueGrant":    {Write: true, Surcharge: 16520},
	"revokeGrant":   {Write: true, Surcharge: 4040},
	"mint":          {Write: true, Surcharge: 5000},
	"burn":          {Write: true, Surcharge: 5000},
	"getAsset":      {Write: false, Surcharge: 1108},
	"balanceOf":     {Write: false, Surcharge: 1204},
	"totalUnits":    {Write: false, Surcharge: 1108},
	"accountId":     {Write: false, Surcharge: 1204},
	"accountOwner":  {Write: false, Surcharge: 1108},
	"nextSequence":  {Write: false, Surcharge: 1204},
	"getPayerGrant": {Write: false, Surcharge: 1108},
}
