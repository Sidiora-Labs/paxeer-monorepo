package layerx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const ChainsAddress = "0x000000000000000000000000000000000000101e"
const VirtualChainIDBase uint64 = 125000000

type chainsABIEntry struct {
	name    string
	inputs  []string
	outputs []string
	indexed []string
}

var chainsMethods = []chainsABIEntry{
	{name: "createChain", inputs: []string{"string", "uint64", "uint64", "uint16"}, outputs: []string{"uint32"}},
	{name: "fundChain", inputs: []string{"uint32"}},
	{name: "pauseChain", inputs: []string{"uint32"}},
	{name: "resumeChain", inputs: []string{"uint32"}},
	{name: "retireChain", inputs: []string{"uint32"}},
	{name: "governancePause", inputs: []string{"uint32", "bool"}},
	{name: "setOperator", inputs: []string{"uint32", "address"}},
	{name: "getChain", inputs: []string{"uint32"}, outputs: []string{"uint32", "string", "address", "uint8", "uint64", "uint64", "uint64", "uint16", "uint256", "uint256", "uint64"}},
	{name: "chainByName", inputs: []string{"string"}, outputs: []string{"uint32"}},
	{name: "chainCount", outputs: []string{"uint32"}},
	{name: "chainIdOf", inputs: []string{"uint32"}, outputs: []string{"uint64"}},
	{name: "params", outputs: []string{"address", "uint256", "uint256", "uint64", "uint64", "uint64", "uint32"}},
}

var chainsEvents = []chainsABIEntry{
	{name: "ChainCreated", inputs: []string{"uint32", "string", "address", "uint64"}, indexed: []string{"uint32", "address"}, outputs: []string{"string", "uint64"}},
	{name: "ChainStatusChanged", inputs: []string{"uint32", "uint8"}, indexed: []string{"uint32"}, outputs: []string{"uint8"}},
	{name: "ChainFunded", inputs: []string{"uint32", "address", "uint256"}, indexed: []string{"uint32", "address"}, outputs: []string{"uint256"}},
	{name: "ChainOperatorChanged", inputs: []string{"uint32", "address"}, indexed: []string{"uint32", "address"}},
}

func (entry chainsABIEntry) signature() string {
	return entry.name + "(" + strings.Join(entry.inputs, ",") + ")"
}
func chainsMethod(name string) (chainsABIEntry, error) {
	for _, entry := range chainsMethods {
		if name == entry.name {
			return entry, nil
		}
	}
	return chainsABIEntry{}, fmt.Errorf("unknown chains method %q", name)
}

func EncodeChainsCall(method string, args ...any) ([]byte, error) {
	entry, err := chainsMethod(method)
	if err != nil {
		return nil, err
	}
	payload, err := encodeChainsTuple(entry.inputs, args)
	if err != nil {
		return nil, err
	}
	hash := receiptKeccak256([]byte(entry.signature()))
	return append(append([]byte(nil), hash[:4]...), payload...), nil
}

func DecodeChainsCall(data []byte) (string, []any, error) {
	if len(data) < 4 {
		return "", nil, errors.New("truncated chains selector")
	}
	for _, entry := range chainsMethods {
		hash := receiptKeccak256([]byte(entry.signature()))
		if bytes.Equal(data[:4], hash[:4]) {
			values, err := decodeChainsTuple(entry.inputs, data[4:])
			return entry.name, values, err
		}
	}
	return "", nil, errors.New("unknown chains selector")
}

func chainsInteger(value any) (*big.Int, error) {
	var n *big.Int
	switch v := value.(type) {
	case uint8:
		n = new(big.Int).SetUint64(uint64(v))
	case uint16:
		n = new(big.Int).SetUint64(uint64(v))
	case uint32:
		n = new(big.Int).SetUint64(uint64(v))
	case uint64:
		n = new(big.Int).SetUint64(v)
	case uint:
		n = new(big.Int).SetUint64(uint64(v))
	case int:
		n = big.NewInt(int64(v))
	case int8:
		n = big.NewInt(int64(v))
	case int16:
		n = big.NewInt(int64(v))
	case int32:
		n = big.NewInt(int64(v))
	case int64:
		n = big.NewInt(v)
	case json.Number:
		n, _ = new(big.Int).SetString(string(v), 10)
	case *big.Int:
		if v != nil {
			n = new(big.Int).Set(v)
		}
	}
	if n == nil || n.Sign() < 0 {
		return nil, errors.New("invalid chains unsigned integer")
	}
	return n, nil
}

func encodeChainsTuple(types []string, values []any) ([]byte, error) {
	if len(types) != len(values) {
		return nil, errors.New("wrong chains argument count")
	}
	head := make([]byte, 32*len(types))
	var tail []byte
	for i, kind := range types {
		word := head[i*32 : (i+1)*32]
		switch kind {
		case "string":
			value, ok := values[i].(string)
			if !ok || !utf8.ValidString(value) {
				return nil, errors.New("invalid chains string")
			}
			binary.BigEndian.PutUint64(word[24:], uint64(len(head)+len(tail)))
			length := make([]byte, 32)
			binary.BigEndian.PutUint64(length[24:], uint64(len(value)))
			tail = append(tail, length...)
			tail = append(tail, []byte(value)...)
			tail = append(tail, make([]byte, (32-len(value)%32)%32)...)
		case "address":
			var address []byte
			switch value := values[i].(type) {
			case string:
				if len(value) != 42 || !strings.HasPrefix(value, "0x") {
					return nil, errors.New("invalid chains address")
				}
				var err error
				address, err = hex.DecodeString(value[2:])
				if err != nil {
					return nil, err
				}
			case [20]byte:
				address = value[:]
			default:
				return nil, errors.New("invalid chains address type")
			}
			copy(word[12:], address)
		case "bool":
			value, ok := values[i].(bool)
			if !ok {
				return nil, errors.New("invalid chains boolean")
			}
			if value {
				word[31] = 1
			}
		default:
			width, err := strconv.Atoi(strings.TrimPrefix(kind, "uint"))
			if err != nil || width < 8 || width > 256 || width%8 != 0 {
				return nil, errors.New("invalid chains ABI integer type")
			}
			n, err := chainsInteger(values[i])
			if err != nil {
				return nil, err
			}
			if n.BitLen() > width {
				return nil, fmt.Errorf("chains value overflows %s", kind)
			}
			n.FillBytes(word)
		}
	}
	return append(head, tail...), nil
}

func decodeChainsTuple(types []string, data []byte) ([]any, error) {
	if len(data) < 32*len(types) || len(data)%32 != 0 {
		return nil, errors.New("invalid chains tuple length")
	}
	values := make([]any, len(types))
	for i, kind := range types {
		word := data[i*32 : (i+1)*32]
		n := new(big.Int).SetBytes(word)
		switch kind {
		case "string":
			if !n.IsUint64() || n.Uint64() > uint64(len(data))-32 {
				return nil, errors.New("invalid chains string offset")
			}
			offset := int(n.Uint64())
			length := new(big.Int).SetBytes(data[offset : offset+32])
			if !length.IsUint64() || length.Uint64() > uint64(len(data)-offset-32) {
				return nil, errors.New("invalid chains string length")
			}
			value := data[offset+32 : offset+32+int(length.Uint64())]
			if !utf8.Valid(value) {
				return nil, errors.New("invalid chains UTF-8")
			}
			values[i] = string(value)
		case "address":
			if n.BitLen() > 160 {
				return nil, errors.New("invalid chains address padding")
			}
			values[i] = "0x" + hex.EncodeToString(word[12:])
		case "bool":
			if n.BitLen() > 1 {
				return nil, errors.New("invalid chains boolean word")
			}
			values[i] = n.Sign() != 0
		default:
			width, err := strconv.Atoi(strings.TrimPrefix(kind, "uint"))
			if err != nil || width < 8 || width > 256 || width%8 != 0 || n.BitLen() > width {
				return nil, errors.New("invalid chains integer word")
			}
			switch width {
			case 8:
				values[i] = uint8(n.Uint64())
			case 16:
				values[i] = uint16(n.Uint64())
			case 32:
				values[i] = uint32(n.Uint64())
			case 64:
				values[i] = n.Uint64()
			default:
				values[i] = n
			}
		}
	}
	canonical, err := encodeChainsTuple(types, values)
	if err != nil || !bytes.Equal(canonical, data) {
		return nil, errors.New("noncanonical chains tuple")
	}
	return values, nil
}

type ChainRecord struct {
	ID            uint32
	Name          string
	Operator      string
	Status        uint8
	ChainID       uint64
	GasAllowance  uint64
	ByteAllowance uint64
	FeeShareBps   uint16
	Bond          *big.Int
	BaseFeePerGas *big.Int
	CreatedHeight uint64
}

type ChainsParams struct {
	Authority             string
	MinBond               *big.Int
	CreationFee           *big.Int
	MinGasAllowance       uint64
	MaxGasAllowance       uint64
	TotalGasAllowanceLeft uint64
	MaxChains             uint32
}

func DecodeGetChain(data []byte) (ChainRecord, error) {
	entry, _ := chainsMethod("getChain")
	v, err := decodeChainsTuple(entry.outputs, data)
	if err != nil {
		return ChainRecord{}, err
	}
	return ChainRecord{ID: v[0].(uint32), Name: v[1].(string), Operator: v[2].(string), Status: v[3].(uint8), ChainID: v[4].(uint64), GasAllowance: v[5].(uint64), ByteAllowance: v[6].(uint64), FeeShareBps: v[7].(uint16), Bond: v[8].(*big.Int), BaseFeePerGas: v[9].(*big.Int), CreatedHeight: v[10].(uint64)}, nil
}

func DecodeChainsParams(data []byte) (ChainsParams, error) {
	entry, _ := chainsMethod("params")
	v, err := decodeChainsTuple(entry.outputs, data)
	if err != nil {
		return ChainsParams{}, err
	}
	return ChainsParams{Authority: v[0].(string), MinBond: v[1].(*big.Int), CreationFee: v[2].(*big.Int), MinGasAllowance: v[3].(uint64), MaxGasAllowance: v[4].(uint64), TotalGasAllowanceLeft: v[5].(uint64), MaxChains: v[6].(uint32)}, nil
}

type ChainsEvent struct {
	Name      string
	ID        uint32
	ChainName string
	Operator  string
	Status    uint8
	ChainID   uint64
	From      string
	Amount    *big.Int
}

func ChainsEventTopic(name string) ([32]byte, error) {
	for _, entry := range chainsEvents {
		if entry.name == name {
			return receiptKeccak256([]byte(entry.signature())), nil
		}
	}
	return [32]byte{}, fmt.Errorf("unknown chains event %q", name)
}

func DecodeChainsEvent(topics [][32]byte, data []byte) (ChainsEvent, error) {
	if len(topics) == 0 {
		return ChainsEvent{}, errors.New("missing chains event topic")
	}
	for _, entry := range chainsEvents {
		topic := receiptKeccak256([]byte(entry.signature()))
		if topics[0] != topic {
			continue
		}
		if len(topics) != len(entry.indexed)+1 {
			return ChainsEvent{}, errors.New("wrong chains event topic count")
		}
		var indexed []byte
		for _, value := range topics[1:] {
			indexed = append(indexed, value[:]...)
		}
		fields, err := decodeChainsTuple(entry.indexed, indexed)
		if err != nil {
			return ChainsEvent{}, err
		}
		v, err := decodeChainsTuple(entry.outputs, data)
		if err != nil {
			return ChainsEvent{}, err
		}
		event := ChainsEvent{Name: entry.name, ID: fields[0].(uint32)}
		switch entry.name {
		case "ChainCreated":
			event.ChainName = v[0].(string)
			event.Operator = fields[1].(string)
			event.ChainID = v[1].(uint64)
		case "ChainStatusChanged":
			event.Status = v[0].(uint8)
		case "ChainFunded":
			event.From = fields[1].(string)
			event.Amount = v[0].(*big.Int)
		case "ChainOperatorChanged":
			event.Operator = fields[1].(string)
		}
		return event, nil
	}
	return ChainsEvent{}, errors.New("unknown chains event topic")
}

func VirtualChainID(id uint32) uint64 { return VirtualChainIDBase + uint64(id) }
func ChainRPCURL(base string, id uint32) string {
	return strings.TrimRight(base, "/") + "/chain/" + strconv.FormatUint(uint64(id), 10)
}

type ChainsClient struct{ transport *ProgramRPCTransport }

func NewChainsClient(endpoint string, client *http.Client) (*ChainsClient, error) {
	transport, err := NewProgramRPCTransport(endpoint, client)
	if err != nil {
		return nil, err
	}
	return &ChainsClient{transport: transport}, nil
}

func (client *ChainsClient) view(ctx context.Context, method string, args ...any) ([]byte, error) {
	if client == nil || client.transport == nil || ctx == nil {
		return nil, lifecycleInvalid()
	}
	data, err := EncodeChainsCall(method, args...)
	if err != nil {
		return nil, err
	}
	params, err := json.Marshal([]any{map[string]string{"to": ChainsAddress, "data": "0x" + hex.EncodeToString(data)}, "latest"})
	if err != nil {
		return nil, err
	}
	raw, err := client.transport.Call(ctx, TransportCall{Plane: PlaneAgent, Operation: "eth_call", Request: params})
	if err != nil {
		return nil, err
	}
	var result string
	if json.Unmarshal(raw, &result) != nil {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	decoded, ok := receiptHex(result)
	if !ok {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return decoded, nil
}

func (client *ChainsClient) GetChain(ctx context.Context, id uint32) (ChainRecord, error) {
	data, err := client.view(ctx, "getChain", id)
	if err != nil {
		return ChainRecord{}, err
	}
	return DecodeGetChain(data)
}
func (client *ChainsClient) Params(ctx context.Context) (ChainsParams, error) {
	data, err := client.view(ctx, "params")
	if err != nil {
		return ChainsParams{}, err
	}
	return DecodeChainsParams(data)
}
func (client *ChainsClient) readUint(ctx context.Context, method string, args ...any) (uint32, error) {
	data, err := client.view(ctx, method, args...)
	if err != nil {
		return 0, err
	}
	v, err := decodeChainsTuple([]string{"uint32"}, data)
	if err != nil {
		return 0, err
	}
	return v[0].(uint32), nil
}
func (client *ChainsClient) ChainByName(ctx context.Context, name string) (uint32, error) {
	return client.readUint(ctx, "chainByName", name)
}
func (client *ChainsClient) ChainCount(ctx context.Context) (uint32, error) {
	return client.readUint(ctx, "chainCount")
}
func (client *ChainsClient) ChainIDOf(ctx context.Context, id uint32) (uint64, error) {
	data, err := client.view(ctx, "chainIdOf", id)
	if err != nil {
		return 0, err
	}
	v, err := decodeChainsTuple([]string{"uint64"}, data)
	if err != nil {
		return 0, err
	}
	return v[0].(uint64), nil
}
