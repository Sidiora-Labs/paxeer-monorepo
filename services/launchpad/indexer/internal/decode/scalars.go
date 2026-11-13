package decode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const (
	scalarTypeFailure   = "type"
	scalarOffsetFailure = "offset_or_type"
)

func newBig(s string) (*big.Int, bool) { return new(big.Int).SetString(s, 10) }
func exactArgs(ev abi.Event, topics []common.Hash, data []byte, approved []string) (map[string]any, string, error) {
	if out, handled, reason, err := exactFixedScalars(ev, topics, data); handled {
		return out, reason, err
	}
	return exactGeneralArgs(ev, topics, data, approved)
}

func exactFixedScalars(ev abi.Event, topics []common.Hash, data []byte) (map[string]any, bool, string, error) {
	indexed, nonIndexed := 0, 0
	for _, in := range ev.Inputs {
		if in.Type.T != abi.AddressTy && (in.Type.T != abi.UintTy || in.Type.Size != 256) {
			return nil, false, "", nil
		}
		if in.Indexed {
			indexed++
		} else {
			nonIndexed++
		}
	}
	if len(topics) != indexed+1 {
		return nil, true, valueArity, fmt.Errorf("topic count %d, expected %d", len(topics), indexed+1)
	}
	if len(data) != nonIndexed*32 {
		if nonIndexed == 0 {
			return nil, true, valueArity, fmt.Errorf("unexpected data")
		}
		return nil, true, scalarOffsetFailure, fmt.Errorf("noncanonical event data")
	}
	out := make(map[string]any, len(ev.Inputs))
	for _, indexedPass := range []bool{true, false} {
		position := 0
		for _, in := range ev.Inputs {
			if in.Indexed != indexedPass {
				continue
			}
			var word []byte
			if indexedPass {
				word = topics[position+1][:]
			} else {
				word = data[position*32 : (position+1)*32]
			}
			position++
			if in.Type.T == abi.AddressTy {
				for _, padding := range word[:12] {
					if padding != 0 {
						if indexedPass {
							return nil, true, scalarTypeFailure, fmt.Errorf("noncanonical indexed %s", in.Name)
						}
						return nil, true, scalarOffsetFailure, fmt.Errorf("noncanonical event data")
					}
				}
				out[in.Name] = "0x" + hex.EncodeToString(word[12:])
			} else {
				out[in.Name] = new(big.Int).SetBytes(word).String()
			}
		}
	}
	return out, true, "", nil
}

func exactGeneralArgs(ev abi.Event, topics []common.Hash, data []byte, approved []string) (map[string]any, string, error) {
	indexed := indexedSubset(ev.Inputs)
	if len(topics) != 1+len(indexed) {
		return nil, valueArity, fmt.Errorf("topic count %d, expected %d", len(topics), 1+len(indexed))
	}
	permitted := map[string]bool{}
	for _, field := range approved {
		permitted[field] = true
	}
	out := map[string]any{}
	for i, in := range indexed {
		if in.Type.T == abi.StringTy || in.Type.T == abi.BytesTy || in.Type.T == abi.ArrayTy || in.Type.T == abi.SliceTy || in.Type.T == abi.TupleTy {
			out[in.Name] = topics[i+1].Hex()
			continue
		}
		typ := in.Type
		if typ.T == abi.BoolTy && permitted[in.Name] {
			typ = abiUint256
		}
		arg := abi.Arguments{{Name: in.Name, Type: typ}}
		values, err := arg.Unpack(topics[i+1].Bytes())
		if err != nil {
			return nil, scalarTypeFailure, err
		}
		packed, err := arg.Pack(values...)
		if err != nil || !bytes.Equal(packed, topics[i+1].Bytes()) {
			return nil, scalarTypeFailure, fmt.Errorf("noncanonical indexed %s", in.Name)
		}
		value := values[0]
		if in.Type.T == abi.BoolTy && permitted[in.Name] {
			value = value.(*big.Int).Sign() != 0
		}
		out[in.Name] = humanizeExact(value, in.Type)
	}
	original := ev.Inputs.NonIndexed()
	inputs := make(abi.Arguments, len(original))
	copy(inputs, original)
	for i, in := range inputs {
		if in.Type.T == abi.BoolTy && permitted[in.Name] {
			inputs[i].Type = abiUint256
		}
	}
	if len(inputs) == 0 {
		if len(data) != 0 {
			return nil, valueArity, fmt.Errorf("unexpected data")
		}
		return out, "", nil
	}
	values, err := inputs.Unpack(data)
	if err != nil {
		return nil, scalarOffsetFailure, err
	}
	packed, err := inputs.Pack(values...)
	if err != nil || !bytes.Equal(packed, data) {
		return nil, scalarOffsetFailure, fmt.Errorf("noncanonical event data")
	}
	for i, in := range original {
		if err := boundedValue(reflect.ValueOf(values[i])); err != nil {
			return nil, "bounds", err
		}
		value := values[i]
		if in.Type.T == abi.BoolTy && permitted[in.Name] {
			value = value.(*big.Int).Sign() != 0
		}
		out[in.Name] = humanizeExact(value, in.Type)
	}
	return out, "", nil
}
func boundedValue(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return boundedValue(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return fmt.Errorf("invalid UTF-8 string")
		}
		if v.Len() > MaxFieldBytes {
			return fmt.Errorf("string exceeds 64 KiB")
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 && v.Len() > MaxFieldBytes {
			return fmt.Errorf("bytes exceeds 64 KiB")
		}
		fallthrough
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := boundedValue(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := boundedValue(v.Field(i)); err != nil {
				return err
			}
		}
	}
	return nil
}
func humanizeExact(v any, t abi.Type) any {
	if t.T == abi.IntTy || t.T == abi.UintTy {
		return fmt.Sprint(v)
	}
	if t.T == abi.SliceTy || t.T == abi.ArrayTy {
		rv := reflect.ValueOf(v)
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = humanizeExact(rv.Index(i).Interface(), *t.Elem)
		}
		return out
	}
	if t.T == abi.TupleTy {
		rv := reflect.ValueOf(v)
		out := map[string]any{}
		for i, typ := range t.TupleElems {
			out[t.TupleRawNames[i]] = humanizeExact(rv.Field(i).Interface(), *typ)
		}
		return out
	}
	return humanize(v, t)
}

func validateDecodedPayload(args map[string]any) error {
	remaining := MaxEnvelopeBytes - 2
	bounded := true
	for name, value := range args {
		text, ok := value.(string)
		if !ok || remaining < 6 {
			bounded = false
			break
		}
		remaining -= 6
		if len(name) > remaining/6 {
			bounded = false
			break
		}
		remaining -= 6 * len(name)
		if len(text) > remaining/6 {
			bounded = false
			break
		}
		remaining -= 6 * len(text)
	}
	if bounded {
		return nil
	}
	payload, err := json.Marshal(args)
	if err != nil || len(payload) > MaxEnvelopeBytes {
		return fmt.Errorf("decoded payload exceeds envelope limit")
	}
	return nil
}
