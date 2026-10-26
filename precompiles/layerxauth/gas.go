package layerxauth

import pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"

var GasTable = pcommon.GasTable{
	"grantSession":          {Write: true, Surcharge: 36680},
	"grantSessionSuccessor": {Write: true, Surcharge: 38600},
	"grantCapability":       {Write: true, Surcharge: 34760},
	"revokeGrant":           {Write: true, Surcharge: 5000},
	"resultByIdempotency":   {Write: false, Surcharge: 1204},
	"getGrant":              {Write: false, Surcharge: 1108},
	"isLive":                {Write: false, Surcharge: 1300},
	"sessionKey":            {Write: false, Surcharge: 1108},
	"capabilityScope":       {Write: false, Surcharge: 1108},
	"checkActivity":         {Write: false, Surcharge: 1396},
}
