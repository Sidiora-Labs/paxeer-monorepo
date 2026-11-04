package cli

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func getMethodPayload(newAbi abi.ABI, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("method is required")
	}
	method, ok := newAbi.Methods[args[0]]
	if !ok {
		return nil, fmt.Errorf("unknown method %q", args[0])
	}
	if len(args)-1 != len(method.Inputs) {
		return nil, fmt.Errorf("method %s expects %d arguments, got %d", args[0], len(method.Inputs), len(args)-1)
	}
	abiArgs := []interface{}{}
	for i, input := range method.Inputs {
		idx := i + 1
		if idx >= len(args) {
			return nil, errors.New("not enough arguments")
		}
		var arg interface{}
		switch input.Type.T {
		case abi.IntTy, abi.UintTy:
			n, e := parseABIInteger(args[idx], input.Type.Size, input.Type.T == abi.IntTy)
			if e != nil {
				return nil, e
			}
			arg = n
			if input.Type.T == abi.IntTy {
				switch input.Type.Size {
				case 8:
					arg = int8(n.Int64())
				case 16:
					arg = int16(n.Int64())
				case 32:
					arg = int32(n.Int64())
				case 64:
					arg = n.Int64()
				}
			} else {
				switch input.Type.Size {
				case 8:
					arg = uint8(n.Uint64())
				case 16:
					arg = uint16(n.Uint64())
				case 32:
					arg = uint32(n.Uint64())
				case 64:
					arg = n.Uint64()
				}
			}
		case abi.BoolTy:
			if args[idx] != TrueStr && args[idx] != FalseStr { //nolint:gosec
				return nil, fmt.Errorf("boolean argument has to be either \"%s\" or \"%s\"", TrueStr, FalseStr)
			} else {
				arg = args[idx] == TrueStr
			}
		case abi.StringTy:
			arg = args[idx] //nolint:gosec
		case abi.AddressTy:
			arg = common.HexToAddress(args[idx]) //nolint:gosec
		default:
			return nil, errors.New("argument type not supported yet")
		}
		abiArgs = append(abiArgs, arg)
	}

	return newAbi.Pack(args[0], abiArgs...)
}

func parseABIInteger(value string, bits int, signed bool) (*big.Int, error) {
	n, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, fmt.Errorf("invalid decimal integer %q", value)
	}
	limit := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	minimum := new(big.Int)
	kind := "uint"
	if signed {
		kind = "int"
		limit.Rsh(limit, 1)
		minimum.Neg(limit)
	} else if strings.HasPrefix(value, "-") {
		return nil, fmt.Errorf("negative unsigned integer %q", value)
	}
	if n.Cmp(minimum) < 0 || n.Cmp(limit) >= 0 {
		return nil, fmt.Errorf("%s%d out of range: %s", kind, bits, value)
	}
	return n, nil
}
