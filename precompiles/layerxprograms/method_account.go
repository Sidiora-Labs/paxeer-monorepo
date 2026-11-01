package layerxprograms

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("registerAccount", registerAccountHandler)
	register("getProgramAccount", getProgramAccountHandler)
}

func registerAccountHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	if call.method.Name != "registerAccount" || len(decoded) != 4 {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: expected registerAccount")
	}
	programID, programOK := decoded[0].([32]byte)
	profile, profileOK := decoded[1].(uint8)
	assetID, assetOK := decoded[2].([32]byte)
	seed, seedOK := decoded[3].([]byte)
	if !programOK || !profileOK || !assetOK || !seedOK {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: invalid registerAccount arguments")
	}
	payload, err := EncodeAccountPayload(programID, profile, assetID, seed)
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090006, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	if err := emitReceiptEvents(call.evm, 0x00090006, programID, receipt); err != nil {
		return nil, err
	}
	return call.method.Outputs.Pack()
}
