package layerxchains

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func chainsABI(t *testing.T) abi.ABI {
	t.Helper()
	data, err := os.ReadFile("abi.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestChainsAbiParsesAndEveryMethodHasGas(t *testing.T) {
	a := chainsABI(t)
	expected := map[string]uint64{
		"createChain": 60000, "fundChain": 30000,
		"pauseChain": 20000, "resumeChain": 20000, "retireChain": 20000,
		"governancePause": 20000, "setOperator": 20000,
		"getChain": 3000, "chainByName": 3000, "chainCount": 3000,
		"chainIdOf": 3000, "params": 3000,
	}
	if len(a.Methods) != len(expected) || len(a.Events) != 4 {
		t.Fatalf("unexpected ABI surface: %d methods, %d events", len(a.Methods), len(a.Events))
	}
	for name, method := range a.Methods {
		row, ok := methodGas[name]
		if !ok {
			t.Fatalf("method %s has no gas row", name)
		}
		gas, ok := expected[name]
		if !ok || row.Gas(nil) != gas {
			t.Errorf("%s gas = %d, want %d", name, row.Gas(nil), gas)
		}
		write := method.StateMutability != "view" && method.StateMutability != "pure"
		if row.Write != write {
			t.Errorf("%s Write = %v, want %v", name, row.Write, write)
		}
	}
	for name := range methodGas {
		if _, ok := a.Methods[name]; !ok {
			t.Errorf("gas row %s has no ABI method", name)
		}
	}
	if ChainsAddress != "0x000000000000000000000000000000000000101E" || PrecompileName != "layerxchains" {
		t.Fatal("incorrect chain registry identity")
	}
}

func solidityArguments(t *testing.T, declaration string) abi.Arguments {
	t.Helper()
	args := make(abi.Arguments, 0)
	if strings.TrimSpace(declaration) == "" {
		return args
	}
	for _, parameter := range strings.Split(declaration, ",") {
		fields := strings.Fields(parameter)
		if len(fields) == 0 {
			t.Fatal("empty Solidity parameter")
		}
		typ, err := abi.NewType(fields[0], "", nil)
		if err != nil {
			t.Fatal(err)
		}
		arg := abi.Argument{Type: typ}
		for _, field := range fields[1:] {
			switch field {
			case "indexed":
				arg.Indexed = true
			case "calldata", "memory", "storage":
			default:
				arg.Name = field
			}
		}
		args = append(args, arg)
	}
	return args
}

func argumentTypes(args abi.Arguments) []string {
	types := make([]string, len(args))
	for i, arg := range args {
		types[i] = arg.Type.String()
	}
	return types
}

func TestChainsAbiMatchesTheSolidityInterface(t *testing.T) {
	a := chainsABI(t)
	data, err := os.ReadFile("LayerXChains.sol")
	if err != nil {
		t.Fatal(err)
	}
	source := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(string(data), "")
	if !regexp.MustCompile(`\binterface\s+ILayerXChains\s*\{`).MatchString(source) {
		t.Fatal("ILayerXChains interface is missing")
	}
	address := regexp.MustCompile(`\baddress\s+constant\s+LAYERX_CHAINS_PRECOMPILE_ADDRESS\s*=\s*(0x[0-9a-fA-F]{40})\s*;`).FindStringSubmatch(source)
	if len(address) != 2 || address[1] != ChainsAddress {
		t.Fatal("Solidity precompile address differs from Go")
	}
	declarations := regexp.MustCompile(`(?s)\b(function|event)\s+(\w+)\s*\(([^()]*)\)\s*([^;]*);`).FindAllStringSubmatch(source, -1)
	returns := regexp.MustCompile(`\breturns\s*\(([^()]*)\)`)
	seenMethods, seenEvents := map[string]bool{}, map[string]bool{}
	for _, decl := range declarations {
		kind, name, tail := decl[1], decl[2], strings.TrimSpace(decl[4])
		args := solidityArguments(t, decl[3])
		signature := name + "(" + strings.Join(argumentTypes(args), ",") + ")"
		hash := crypto.Keccak256([]byte(signature))
		if kind == "event" {
			event, ok := a.Events[name]
			if !ok || seenEvents[name] || event.Sig != signature || !bytes.Equal(event.ID.Bytes(), hash) {
				t.Fatalf("Solidity event %s does not match ABI topic0", signature)
			}
			if !reflect.DeepEqual(event.Inputs, args) || event.Anonymous != (tail == "anonymous") {
				t.Fatalf("event %s fields or indexing differ", name)
			}
			seenEvents[name] = true
			continue
		}
		method, ok := a.Methods[name]
		if !ok || seenMethods[name] || method.Sig != signature || !bytes.Equal(method.ID, hash[:4]) {
			t.Fatalf("Solidity function %s does not match ABI selector", signature)
		}
		mutability := "nonpayable"
		for _, field := range strings.Fields(returns.ReplaceAllString(tail, "")) {
			if field == "pure" || field == "view" || field == "payable" {
				mutability = field
			}
		}
		outputs := solidityArguments(t, "")
		if match := returns.FindStringSubmatch(tail); match != nil {
			outputs = solidityArguments(t, match[1])
		}
		if method.StateMutability != mutability || !reflect.DeepEqual(method.Outputs, outputs) {
			t.Fatalf("function %s mutability or return tuple differs", name)
		}
		seenMethods[name] = true
	}
	if len(seenMethods) != len(a.Methods) || len(seenEvents) != len(a.Events) {
		t.Fatalf("Solidity declares %d/%d methods and %d/%d events", len(seenMethods), len(a.Methods), len(seenEvents), len(a.Events))
	}
}

type chainsMethodVector struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Selector  string `json:"selector"`
	Args      []any  `json:"args"`
	Calldata  string `json:"calldata"`
}

type chainsEventVector struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Topic0    string `json:"topic0"`
}

type chainsReturnVector struct {
	Returndata string         `json:"returndata"`
	Decoded    map[string]any `json:"decoded"`
}

type chainsVectors struct {
	Address  string               `json:"address"`
	Methods  []chainsMethodVector `json:"methods"`
	Events   []chainsEventVector  `json:"events"`
	GetChain chainsReturnVector   `json:"getChain"`
	Params   chainsReturnVector   `json:"params"`
}

func vectorValue(value any) any {
	switch v := value.(type) {
	case common.Address:
		return strings.ToLower(v.Hex())
	case *big.Int:
		return v.String()
	default:
		return v
	}
}

func vectorHex(value []byte) string { return "0x" + hex.EncodeToString(value) }

func chainsReturnExample(t *testing.T, method abi.Method, values ...any) chainsReturnVector {
	t.Helper()
	data, err := method.Outputs.Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := method.Outputs.Unpack(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, values) {
		t.Fatalf("%s output round trip: got %v, want %v", method.Name, decoded, values)
	}
	fields := make(map[string]any, len(decoded))
	for i, value := range decoded {
		fields[method.Outputs[i].Name] = vectorValue(value)
	}
	return chainsReturnVector{Returndata: vectorHex(data), Decoded: fields}
}

func TestChainsVectorFileIsCurrent(t *testing.T) {
	a := chainsABI(t)
	operator := common.HexToAddress("0x1111111111111111111111111111111111111111")
	examples := []struct {
		name string
		args []any
	}{
		{"createChain", []any{"demo", uint64(500000), uint64(65536), uint16(100)}},
		{"fundChain", []any{uint32(1)}},
		{"pauseChain", []any{uint32(1)}},
		{"resumeChain", []any{uint32(1)}},
		{"retireChain", []any{uint32(1)}},
		{"governancePause", []any{uint32(1), true}},
		{"setOperator", []any{uint32(1), operator}},
		{"getChain", []any{uint32(1)}},
		{"chainByName", []any{"demo"}},
		{"chainCount", []any{}},
		{"chainIdOf", []any{uint32(1)}},
		{"params", []any{}},
	}
	vectors := chainsVectors{Address: strings.ToLower(ChainsAddress)}
	seen := map[string]bool{}
	for _, example := range examples {
		method, ok := a.Methods[example.name]
		if !ok || seen[example.name] {
			t.Fatalf("missing or repeated method %s", example.name)
		}
		data, err := a.Pack(example.name, example.args...)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := a.MethodById(data[:4])
		if err != nil || resolved.Sig != method.Sig {
			t.Fatalf("%s selector lookup failed: %v", example.name, err)
		}
		decoded, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != len(example.args) || (len(decoded) > 0 && !reflect.DeepEqual(decoded, example.args)) {
			t.Fatalf("%s calldata round trip differs", example.name)
		}
		args := make([]any, 0, len(example.args))
		for _, value := range example.args {
			args = append(args, vectorValue(value))
		}
		vectors.Methods = append(vectors.Methods, chainsMethodVector{
			Name: method.Name, Signature: method.Sig, Selector: vectorHex(method.ID), Args: args, Calldata: vectorHex(data),
		})
		seen[example.name] = true
	}
	if len(seen) != len(a.Methods) {
		t.Fatal("not every ABI method has a vector")
	}
	eventNames := []string{"ChainCreated", "ChainStatusChanged", "ChainFunded", "ChainOperatorChanged"}
	for _, name := range eventNames {
		event, ok := a.Events[name]
		if !ok {
			t.Fatalf("missing event %s", name)
		}
		vectors.Events = append(vectors.Events, chainsEventVector{Name: event.Name, Signature: event.Sig, Topic0: event.ID.Hex()})
	}
	if len(vectors.Events) != len(a.Events) {
		t.Fatal("not every ABI event has a vector")
	}
	bond := new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)
	vectors.GetChain = chainsReturnExample(t, a.Methods["getChain"],
		uint32(1), "demo", operator, uint8(1), uint64(125000001), uint64(500000), uint64(65536),
		uint16(100), bond, big.NewInt(1000000000), uint64(42))
	vectors.Params = chainsReturnExample(t, a.Methods["params"],
		operator, bond, big.NewInt(1000000000000000000), uint64(100000), uint64(10000000), uint64(9500000), uint32(1024))
	want, err := json.MarshalIndent(vectors, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	path := filepath.Join("..", "..", "tests", "vectors", "chains", "abi-vectors.json")
	if os.Getenv("UPDATE_VECTORS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from ABI-generated vectors; regenerate with UPDATE_VECTORS=1", path)
	}
}
