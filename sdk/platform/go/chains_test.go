package layerx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestChainsVectorsMatchAndRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/vectors/chains/abi-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	type tupleVector struct {
		ReturnData string         `json:"returndata"`
		Decoded    map[string]any `json:"decoded"`
	}
	var vectors struct {
		Address string `json:"address"`
		Methods []struct {
			Name      string `json:"name"`
			Signature string `json:"signature"`
			Selector  string `json:"selector"`
			Args      []any  `json:"args"`
			Calldata  string `json:"calldata"`
		} `json:"methods"`
		Events []struct {
			Name      string `json:"name"`
			Signature string `json:"signature"`
			Topic0    string `json:"topic0"`
		} `json:"events"`
		GetChain tupleVector `json:"getChain"`
		Params   tupleVector `json:"params"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&vectors); err != nil {
		t.Fatal(err)
	}
	decodeHex := func(value string) []byte {
		t.Helper()
		if !strings.HasPrefix(value, "0x") {
			t.Fatalf("missing hex prefix: %q", value)
		}
		data, err := hex.DecodeString(value[2:])
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if vectors.Address != ChainsAddress {
		t.Fatalf("registry address: %s", vectors.Address)
	}
	if len(vectors.Methods) != 12 || len(vectors.Methods) != len(chainsMethods) {
		t.Fatalf("method coverage: %d", len(vectors.Methods))
	}
	seen := map[string]bool{}
	for _, vector := range vectors.Methods {
		t.Run(vector.Name, func(t *testing.T) {
			if seen[vector.Name] {
				t.Fatal("duplicate method vector")
			}
			seen[vector.Name] = true
			entry, err := chainsMethod(vector.Name)
			if err != nil {
				t.Fatal(err)
			}
			if entry.signature() != vector.Signature {
				t.Fatalf("signature: %s", entry.signature())
			}
			call, err := EncodeChainsCall(vector.Name, vector.Args...)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(call[:4], decodeHex(vector.Selector)) {
				t.Fatalf("selector: %x", call[:4])
			}
			if !bytes.Equal(call, decodeHex(vector.Calldata)) {
				t.Fatalf("calldata: %x", call)
			}
			name, args, err := DecodeChainsCall(call)
			if err != nil {
				t.Fatal(err)
			}
			if name != vector.Name {
				t.Fatalf("decoded method: %s", name)
			}
			again, err := EncodeChainsCall(name, args...)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(call, again) {
				t.Fatal("call round trip changed bytes")
			}
			expectedArgs, err := encodeChainsTuple(entry.inputs, vector.Args)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again[4:], expectedArgs) {
				t.Fatal("decoded arguments changed")
			}
		})
	}
	chainBytes := decodeHex(vectors.GetChain.ReturnData)
	chain, err := DecodeGetChain(chainBytes)
	if err != nil {
		t.Fatal(err)
	}
	chainValues := map[string]any{
		"id": json.Number(new(big.Int).SetUint64(uint64(chain.ID)).String()), "name": chain.Name, "operator": chain.Operator,
		"status": json.Number(new(big.Int).SetUint64(uint64(chain.Status)).String()), "chainId": json.Number(new(big.Int).SetUint64(chain.ChainID).String()),
		"gasAllowance": json.Number(new(big.Int).SetUint64(chain.GasAllowance).String()), "byteAllowance": json.Number(new(big.Int).SetUint64(chain.ByteAllowance).String()),
		"feeShareBps": json.Number(new(big.Int).SetUint64(uint64(chain.FeeShareBps)).String()), "bond": chain.Bond.String(), "baseFeePerGas": chain.BaseFeePerGas.String(),
		"createdHeight": json.Number(new(big.Int).SetUint64(chain.CreatedHeight).String()),
	}
	if !reflect.DeepEqual(chainValues, vectors.GetChain.Decoded) {
		t.Fatalf("getChain tuple: got %#v, want %#v", chainValues, vectors.GetChain.Decoded)
	}
	chainEntry, _ := chainsMethod("getChain")
	encoded, err := encodeChainsTuple(chainEntry.outputs, []any{chain.ID, chain.Name, chain.Operator, chain.Status, chain.ChainID, chain.GasAllowance, chain.ByteAllowance, chain.FeeShareBps, chain.Bond, chain.BaseFeePerGas, chain.CreatedHeight})
	if err != nil || !bytes.Equal(encoded, chainBytes) {
		t.Fatalf("getChain re-encoding: %x, %v", encoded, err)
	}
	roundTripChain, err := DecodeGetChain(encoded)
	if err != nil || !reflect.DeepEqual(roundTripChain, chain) {
		t.Fatalf("getChain round trip: %+v, %v", roundTripChain, err)
	}
	paramsBytes := decodeHex(vectors.Params.ReturnData)
	params, err := DecodeChainsParams(paramsBytes)
	if err != nil {
		t.Fatal(err)
	}
	paramsValues := map[string]any{
		"authority": params.Authority, "minBond": params.MinBond.String(), "creationFee": params.CreationFee.String(),
		"minGasAllowance": json.Number(new(big.Int).SetUint64(params.MinGasAllowance).String()), "maxGasAllowance": json.Number(new(big.Int).SetUint64(params.MaxGasAllowance).String()),
		"totalGasAllowanceLeft": json.Number(new(big.Int).SetUint64(params.TotalGasAllowanceLeft).String()), "maxChains": json.Number(new(big.Int).SetUint64(uint64(params.MaxChains)).String()),
	}
	if !reflect.DeepEqual(paramsValues, vectors.Params.Decoded) {
		t.Fatalf("params tuple: got %#v, want %#v", paramsValues, vectors.Params.Decoded)
	}
	paramsEntry, _ := chainsMethod("params")
	encoded, err = encodeChainsTuple(paramsEntry.outputs, []any{params.Authority, params.MinBond, params.CreationFee, params.MinGasAllowance, params.MaxGasAllowance, params.TotalGasAllowanceLeft, params.MaxChains})
	if err != nil || !bytes.Equal(encoded, paramsBytes) {
		t.Fatalf("params re-encoding: %x, %v", encoded, err)
	}
	roundTripParams, err := DecodeChainsParams(encoded)
	if err != nil || !reflect.DeepEqual(roundTripParams, params) {
		t.Fatalf("params round trip: %+v, %v", roundTripParams, err)
	}
	if len(vectors.Events) != 4 || len(vectors.Events) != len(chainsEvents) {
		t.Fatalf("event coverage: %d", len(vectors.Events))
	}
	seenEvents := map[string]bool{}
	for _, vector := range vectors.Events {
		t.Run(vector.Name, func(t *testing.T) {
			if seenEvents[vector.Name] {
				t.Fatal("duplicate event vector")
			}
			seenEvents[vector.Name] = true
			topic, err := ChainsEventTopic(vector.Name)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(topic[:], decodeHex(vector.Topic0)) {
				t.Fatalf("topic0: %x", topic)
			}
			var entry chainsABIEntry
			for _, candidate := range chainsEvents {
				if candidate.name == vector.Name {
					entry = candidate
				}
			}
			if entry.signature() != vector.Signature {
				t.Fatalf("event signature: %s", entry.signature())
			}
			indexed := []any{chain.ID}
			if len(entry.indexed) == 2 {
				indexed = append(indexed, chain.Operator)
			}
			indexedBytes, err := encodeChainsTuple(entry.indexed, indexed)
			if err != nil {
				t.Fatal(err)
			}
			topics := [][32]byte{topic}
			for i := 0; i < len(indexedBytes); i += 32 {
				var word [32]byte
				copy(word[:], indexedBytes[i:i+32])
				topics = append(topics, word)
			}
			want := ChainsEvent{Name: vector.Name, ID: chain.ID}
			var values []any
			switch vector.Name {
			case "ChainCreated":
				values = []any{chain.Name, chain.ChainID}
				want.ChainName = chain.Name
				want.ChainID = chain.ChainID
				want.Operator = chain.Operator
			case "ChainStatusChanged":
				values = []any{chain.Status}
				want.Status = chain.Status
			case "ChainFunded":
				values = []any{chain.Bond}
				want.From = chain.Operator
				want.Amount = chain.Bond
			case "ChainOperatorChanged":
				want.Operator = chain.Operator
			default:
				t.Fatalf("unexpected event: %s", vector.Name)
			}
			payload, err := encodeChainsTuple(entry.outputs, values)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeChainsEvent(topics, payload)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("event: got %+v, want %+v", got, want)
			}
			if _, err = DecodeChainsEvent(topics[1:], payload); err == nil {
				t.Fatal("missing topic0 accepted")
			}
			if _, err = DecodeChainsEvent(append(topics, [32]byte{}), payload); err == nil {
				t.Fatal("extra topic accepted")
			}
			if _, err = DecodeChainsEvent(topics, append(payload, make([]byte, 32)...)); err == nil {
				t.Fatal("trailing event data accepted")
			}
		})
	}
	if VirtualChainID(chain.ID) != chain.ChainID || VirtualChainID(math.MaxUint32) != uint64(math.MaxUint32)+125000000 {
		t.Fatal("virtual chain ID derivation")
	}
	if ChainRPCURL("https://rpc.example/", chain.ID) != "https://rpc.example/chain/1" || ChainRPCURL("https://rpc.example/base///", math.MaxUint32) != "https://rpc.example/base/chain/4294967295" {
		t.Fatal("chain RPC URL derivation")
	}
	t.Run("boundariesAndRefusals", func(t *testing.T) {
		max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
		types := []string{"uint8", "uint16", "uint32", "uint64", "uint256", "bool", "address", "string"}
		values := []any{uint8(math.MaxUint8), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), max, false, chain.Operator, "a multibyte name: \u03bb"}
		payload, err := encodeChainsTuple(types, values)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeChainsTuple(types, payload)
		if err != nil || !reflect.DeepEqual(decoded, values) {
			t.Fatalf("ABI boundary round trip: %#v, %v", decoded, err)
		}
		for _, name := range []string{"", "a", "12345678901234567890123456789012", "12345678901234567890123456789012345", "\u03bb\u03bb"} {
			call, err := EncodeChainsCall("createChain", name, uint64(math.MaxUint64), uint64(math.MaxUint64), uint16(math.MaxUint16))
			if err != nil {
				t.Fatal(err)
			}
			_, args, err := DecodeChainsCall(call)
			if err != nil || args[0] != name {
				t.Fatalf("string round trip: %q, %v", name, err)
			}
		}
		invalid := []struct {
			method string
			args   []any
		}{
			{"unknown", nil}, {"getChain", nil}, {"getChain", []any{-1}}, {"getChain", []any{uint64(math.MaxUint32) + 1}},
			{"createChain", []any{"demo", new(big.Int).Lsh(big.NewInt(1), 64), 1, 1}}, {"createChain", []any{"demo", 1, 1, 65536}},
			{"governancePause", []any{1, 1}}, {"setOperator", []any{1, "0x11"}}, {"chainByName", []any{string([]byte{0xff})}},
		}
		for _, value := range invalid {
			if _, err := EncodeChainsCall(value.method, value.args...); err == nil {
				t.Fatalf("invalid call accepted: %+v", value)
			}
		}
		for _, data := range [][]byte{nil, {0, 0, 0}, {0, 0, 0, 0}} {
			if _, _, err := DecodeChainsCall(data); err == nil {
				t.Fatal("invalid selector accepted")
			}
		}
		for n := 0; n < len(chainBytes); n++ {
			if _, err := DecodeGetChain(chainBytes[:n]); err == nil {
				t.Fatalf("truncated getChain accepted: %d", n)
			}
		}
		for n := 0; n < len(paramsBytes); n++ {
			if _, err := DecodeChainsParams(paramsBytes[:n]); err == nil {
				t.Fatalf("truncated params accepted: %d", n)
			}
		}
		corrupt := func(offset int, value byte) []byte {
			data := append([]byte(nil), chainBytes...)
			data[offset] = value
			return data
		}
		for _, data := range [][]byte{corrupt(0, 1), corrupt(32, 255), corrupt(63, 0), corrupt(64, 1), corrupt(96, 1), corrupt(128, 1), corrupt(len(chainBytes)-1, 1), append(append([]byte(nil), chainBytes...), make([]byte, 32)...)} {
			if _, err := DecodeGetChain(data); err == nil {
				t.Fatalf("malformed getChain accepted: %x", data)
			}
		}
		boolean, err := EncodeChainsCall("governancePause", 1, true)
		if err != nil {
			t.Fatal(err)
		}
		boolean[len(boolean)-1] = 2
		if _, _, err := DecodeChainsCall(boolean); err == nil {
			t.Fatal("invalid boolean accepted")
		}
		if _, err := ChainsEventTopic("unknown"); err == nil {
			t.Fatal("unknown event accepted")
		}
		if _, err := DecodeChainsEvent(nil, nil); err == nil {
			t.Fatal("missing topics accepted")
		}
		var unknown [32]byte
		if _, err := DecodeChainsEvent([][32]byte{unknown}, nil); err == nil {
			t.Fatal("unknown topic accepted")
		}
		if _, err := NewChainsClient("not a URL", nil); err == nil {
			t.Fatal("invalid RPC endpoint accepted")
		}
		client, err := NewChainsClient("https://rpc.example", nil)
		if err != nil || client.transport == nil {
			t.Fatalf("client transport: %v", err)
		}
		if _, err := client.GetChain(nil, 1); err == nil {
			t.Fatal("nil context accepted")
		}
	})
}
