package layerxprograms

import (
	"fmt"
	"math/big"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func exitValueHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	program, programOK := decoded[0].([32]byte)
	account, accountOK := decoded[1].([32]byte)
	if !programOK || !accountOK {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: exitValue requires bytes32 program and account")
	}
	payload, err := EncodeExitValuePayload(program, account)
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090007, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	return nil, emitReceiptEvents(call.evm, 0x00090007, program, receipt)
}

func exitValueBoundedHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	program, programOK := decoded[0].([32]byte)
	account, accountOK := decoded[1].([32]byte)
	maximum, maximumOK := decoded[2].([16]byte)
	if !programOK || !accountOK || !maximumOK {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: exitValueBounded requires decoded program, account and uint128 maximum")
	}
	payload, err := EncodeExitValueBoundedPayload(program, account, new(big.Int).SetBytes(maximum[:]))
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090007, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	return nil, emitReceiptEvents(call.evm, 0x00090007, program, receipt)
}

func init() {
	register("exitValue", exitValueHandler)
	register("exitValueBounded", exitValueBoundedHandler)
}
