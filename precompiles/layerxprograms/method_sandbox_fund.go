package layerxprograms

import (
	"math/big"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func sandboxFundHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	amount := decoded[5].([16]byte)
	payload, err := EncodeSandboxFundPayload(
		decoded[0].([32]byte), decoded[1].([32]byte), decoded[2].([32]byte),
		decoded[3].([32]byte), decoded[4].([32]byte), new(big.Int).SetBytes(amount[:]),
		decoded[6].(uint64), decoded[7].(uint32), decoded[8].([7]uint64), decoded[9].([]byte),
		decoded[10].([32]byte), decoded[11].([]TransferLeg),
	)
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090009, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	if err := emitSandboxEvents(call.evm, receipt); err != nil {
		return nil, err
	}
	return nil, nil
}

func init() {
	register("sandboxFund", sandboxFundHandler)
}
