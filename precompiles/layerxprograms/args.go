package layerxprograms

import (
	"fmt"
	"math/big"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func decodeLegs(arg interface{}) ([]TransferLeg, error) {
	legs, ok := arg.([]struct {
		From   [32]byte `json:"from"`
		Asset  [32]byte `json:"asset"`
		To     [32]byte `json:"to"`
		Amount *big.Int `json:"amount"`
	})
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: expected ABI transfer leg tuples, got %T", arg)
	}
	decoded := make([]TransferLeg, len(legs))
	for i, leg := range legs {
		if _, err := decodeU128(leg.Amount); err != nil {
			return nil, fmt.Errorf("%w; leg %d amount", err, i)
		}
		decoded[i] = TransferLeg{From: leg.From, Asset: leg.Asset, To: leg.To, Amount: new(big.Int).Set(leg.Amount)}
	}
	return decoded, nil
}

func decodeBudget(arg interface{}) ([7]uint64, error) {
	budget, ok := arg.([7]uint64)
	if !ok {
		return [7]uint64{}, fmt.Errorf("LXP_NON_CANONICAL: expected uint64[7] budget, got %T", arg)
	}
	return budget, nil
}

func decodeU128(arg interface{}) ([16]byte, error) {
	var decoded [16]byte
	amount, ok := arg.(*big.Int)
	if !ok || amount == nil {
		return decoded, fmt.Errorf("LXP_NON_CANONICAL: expected non-nil uint128 integer, got %T", arg)
	}
	if amount.Sign() < 0 || amount.BitLen() > 128 {
		return decoded, fmt.Errorf("LXP_OVERFLOW: uint128 out of range")
	}
	amount.FillBytes(decoded[:])
	return decoded, nil
}

func decodeArgs(method *abi.Method, args []interface{}) ([]interface{}, error) {
	if method == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: missing ABI method")
	}
	if err := pcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, err
	}
	decoded := make([]interface{}, len(args))
	for i, input := range method.Inputs {
		var err error
		switch input.Type.String() {
		case "(bytes32,bytes32,bytes32,uint128)[]":
			decoded[i], err = decodeLegs(args[i])
		case "uint64[7]":
			decoded[i], err = decodeBudget(args[i])
		case "uint128":
			decoded[i], err = decodeU128(args[i])
		default:
			decoded[i] = args[i]
		}
		if err != nil {
			return nil, fmt.Errorf("%w; %s argument %s", err, method.Name, input.Name)
		}
	}
	return decoded, nil
}
