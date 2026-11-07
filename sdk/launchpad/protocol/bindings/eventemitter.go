// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// IEventEmitterAddressArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterAddressArrayKeyValue struct {
	Key   string
	Value []common.Address
}

// IEventEmitterAddressItems is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterAddressItems struct {
	Items      []IEventEmitterAddressKeyValue
	ArrayItems []IEventEmitterAddressArrayKeyValue
}

// IEventEmitterAddressKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterAddressKeyValue struct {
	Key   string
	Value common.Address
}

// IEventEmitterBoolArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBoolArrayKeyValue struct {
	Key   string
	Value []bool
}

// IEventEmitterBoolItems is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBoolItems struct {
	Items      []IEventEmitterBoolKeyValue
	ArrayItems []IEventEmitterBoolArrayKeyValue
}

// IEventEmitterBoolKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBoolKeyValue struct {
	Key   string
	Value bool
}

// IEventEmitterBytes32ArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBytes32ArrayKeyValue struct {
	Key   string
	Value [][32]byte
}

// IEventEmitterBytes32Items is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBytes32Items struct {
	Items      []IEventEmitterBytes32KeyValue
	ArrayItems []IEventEmitterBytes32ArrayKeyValue
}

// IEventEmitterBytes32KeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBytes32KeyValue struct {
	Key   string
	Value [32]byte
}

// IEventEmitterBytesArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBytesArrayKeyValue struct {
	Key   string
	Value [][]byte
}

// IEventEmitterBytesItems is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBytesItems struct {
	Items      []IEventEmitterBytesKeyValue
	ArrayItems []IEventEmitterBytesArrayKeyValue
}

// IEventEmitterBytesKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterBytesKeyValue struct {
	Key   string
	Value []byte
}

// IEventEmitterEventData is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterEventData struct {
	AddressItems IEventEmitterAddressItems
	UintItems    IEventEmitterUintItems
	IntItems     IEventEmitterIntItems
	BoolItems    IEventEmitterBoolItems
	Bytes32Items IEventEmitterBytes32Items
	BytesItems   IEventEmitterBytesItems
	StringItems  IEventEmitterStringItems
}

// IEventEmitterIntArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterIntArrayKeyValue struct {
	Key   string
	Value []*big.Int
}

// IEventEmitterIntItems is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterIntItems struct {
	Items      []IEventEmitterIntKeyValue
	ArrayItems []IEventEmitterIntArrayKeyValue
}

// IEventEmitterIntKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterIntKeyValue struct {
	Key   string
	Value *big.Int
}

// IEventEmitterStringArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterStringArrayKeyValue struct {
	Key   string
	Value []string
}

// IEventEmitterStringItems is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterStringItems struct {
	Items      []IEventEmitterStringKeyValue
	ArrayItems []IEventEmitterStringArrayKeyValue
}

// IEventEmitterStringKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterStringKeyValue struct {
	Key   string
	Value string
}

// IEventEmitterUintArrayKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterUintArrayKeyValue struct {
	Key   string
	Value []*big.Int
}

// IEventEmitterUintItems is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterUintItems struct {
	Items      []IEventEmitterUintKeyValue
	ArrayItems []IEventEmitterUintArrayKeyValue
}

// IEventEmitterUintKeyValue is an auto generated low-level Go binding around an user-defined struct.
type IEventEmitterUintKeyValue struct {
	Key   string
	Value *big.Int
}

// EventEmitterMetaData contains all meta data concerning the EventEmitter contract.
var EventEmitterMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"AlreadyInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidImplementation\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidImplementation\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidRoleAccount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"LastDefaultAdmin\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"}],\"name\":\"MissingRole\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NonPayableUpgrade\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitializing\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UUPSNotThroughActiveProxy\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UUPSNotThroughProxy\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"Unauthorized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UnauthorizedUpgrade\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UpgradeInitializationFailed\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"valueOrTokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isNft\",\"type\":\"bool\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"AssetApproval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"routeId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"hops\",\"type\":\"address[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"protocolFeeBps\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"BestRouteSwap\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"sourceId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"reportedPrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"referencePrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"deviationBps\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"CircuitBreakerTriggered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"key\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldValue\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newValue\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"ConfigUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"proxy\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"ContractUpgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"msgSender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"eventNameHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"eventName\",\"type\":\"string\"},{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"value\",\"type\":\"address\"}],\"internalType\":\"structIEventEmitter.AddressKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address[]\",\"name\":\"value\",\"type\":\"address[]\"}],\"internalType\":\"structIEventEmitter.AddressArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.AddressItems\",\"name\":\"addressItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"internalType\":\"structIEventEmitter.UintKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256[]\",\"name\":\"value\",\"type\":\"uint256[]\"}],\"internalType\":\"structIEventEmitter.UintArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.UintItems\",\"name\":\"uintItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256\",\"name\":\"value\",\"type\":\"int256\"}],\"internalType\":\"structIEventEmitter.IntKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256[]\",\"name\":\"value\",\"type\":\"int256[]\"}],\"internalType\":\"structIEventEmitter.IntArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.IntItems\",\"name\":\"intItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"value\",\"type\":\"bool\"}],\"internalType\":\"structIEventEmitter.BoolKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool[]\",\"name\":\"value\",\"type\":\"bool[]\"}],\"internalType\":\"structIEventEmitter.BoolArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BoolItems\",\"name\":\"boolItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"value\",\"type\":\"bytes32\"}],\"internalType\":\"structIEventEmitter.Bytes32KeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32[]\",\"name\":\"value\",\"type\":\"bytes32[]\"}],\"internalType\":\"structIEventEmitter.Bytes32ArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.Bytes32Items\",\"name\":\"bytes32Items\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"internalType\":\"structIEventEmitter.BytesKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"value\",\"type\":\"bytes[]\"}],\"internalType\":\"structIEventEmitter.BytesArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BytesItems\",\"name\":\"bytesItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structIEventEmitter.StringKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"value\",\"type\":\"string[]\"}],\"internalType\":\"structIEventEmitter.StringArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.StringItems\",\"name\":\"stringItems\",\"type\":\"tuple\"}],\"indexed\":false,\"internalType\":\"structIEventEmitter.EventData\",\"name\":\"eventData\",\"type\":\"tuple\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"EventLog\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"msgSender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"eventNameHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"topic1\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"eventName\",\"type\":\"string\"},{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"value\",\"type\":\"address\"}],\"internalType\":\"structIEventEmitter.AddressKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address[]\",\"name\":\"value\",\"type\":\"address[]\"}],\"internalType\":\"structIEventEmitter.AddressArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.AddressItems\",\"name\":\"addressItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"internalType\":\"structIEventEmitter.UintKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256[]\",\"name\":\"value\",\"type\":\"uint256[]\"}],\"internalType\":\"structIEventEmitter.UintArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.UintItems\",\"name\":\"uintItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256\",\"name\":\"value\",\"type\":\"int256\"}],\"internalType\":\"structIEventEmitter.IntKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256[]\",\"name\":\"value\",\"type\":\"int256[]\"}],\"internalType\":\"structIEventEmitter.IntArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.IntItems\",\"name\":\"intItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"value\",\"type\":\"bool\"}],\"internalType\":\"structIEventEmitter.BoolKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool[]\",\"name\":\"value\",\"type\":\"bool[]\"}],\"internalType\":\"structIEventEmitter.BoolArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BoolItems\",\"name\":\"boolItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"value\",\"type\":\"bytes32\"}],\"internalType\":\"structIEventEmitter.Bytes32KeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32[]\",\"name\":\"value\",\"type\":\"bytes32[]\"}],\"internalType\":\"structIEventEmitter.Bytes32ArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.Bytes32Items\",\"name\":\"bytes32Items\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"internalType\":\"structIEventEmitter.BytesKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"value\",\"type\":\"bytes[]\"}],\"internalType\":\"structIEventEmitter.BytesArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BytesItems\",\"name\":\"bytesItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structIEventEmitter.StringKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"value\",\"type\":\"string[]\"}],\"internalType\":\"structIEventEmitter.StringArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.StringItems\",\"name\":\"stringItems\",\"type\":\"tuple\"}],\"indexed\":false,\"internalType\":\"structIEventEmitter.EventData\",\"name\":\"eventData\",\"type\":\"tuple\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"EventLog1\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"msgSender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"eventNameHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"topic1\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"topic2\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"eventName\",\"type\":\"string\"},{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"value\",\"type\":\"address\"}],\"internalType\":\"structIEventEmitter.AddressKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address[]\",\"name\":\"value\",\"type\":\"address[]\"}],\"internalType\":\"structIEventEmitter.AddressArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.AddressItems\",\"name\":\"addressItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"internalType\":\"structIEventEmitter.UintKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256[]\",\"name\":\"value\",\"type\":\"uint256[]\"}],\"internalType\":\"structIEventEmitter.UintArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.UintItems\",\"name\":\"uintItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256\",\"name\":\"value\",\"type\":\"int256\"}],\"internalType\":\"structIEventEmitter.IntKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256[]\",\"name\":\"value\",\"type\":\"int256[]\"}],\"internalType\":\"structIEventEmitter.IntArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.IntItems\",\"name\":\"intItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"value\",\"type\":\"bool\"}],\"internalType\":\"structIEventEmitter.BoolKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool[]\",\"name\":\"value\",\"type\":\"bool[]\"}],\"internalType\":\"structIEventEmitter.BoolArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BoolItems\",\"name\":\"boolItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"value\",\"type\":\"bytes32\"}],\"internalType\":\"structIEventEmitter.Bytes32KeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32[]\",\"name\":\"value\",\"type\":\"bytes32[]\"}],\"internalType\":\"structIEventEmitter.Bytes32ArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.Bytes32Items\",\"name\":\"bytes32Items\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"internalType\":\"structIEventEmitter.BytesKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"value\",\"type\":\"bytes[]\"}],\"internalType\":\"structIEventEmitter.BytesArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BytesItems\",\"name\":\"bytesItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structIEventEmitter.StringKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"value\",\"type\":\"string[]\"}],\"internalType\":\"structIEventEmitter.StringArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.StringItems\",\"name\":\"stringItems\",\"type\":\"tuple\"}],\"indexed\":false,\"internalType\":\"structIEventEmitter.EventData\",\"name\":\"eventData\",\"type\":\"tuple\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"EventLog2\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"strategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeDistributed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"protocolCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"poolCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"epoch\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeFlow\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"feeAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"protocolCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"poolCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeRecorded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"oldStrategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"newStrategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeStrategyChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"action\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"id\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"actor\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"Governance\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"version\",\"type\":\"uint64\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"MarketCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"strategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"NftMint\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nft\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"NftTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"hookName\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"OpticalExecuted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"action\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"name\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"OpticalLifecycle\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"sourceId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"adapter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"phase\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"priority\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"OracleAdapterLifecycle\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pausedContract\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"paused\",\"type\":\"bool\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PauseToggle\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"orderId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"orderKind\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"orderType\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"targetPrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stopPrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"limitPrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PecorOrderCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"orderId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"orderKind\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"phase\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PecorOrderLifecycle\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"priceIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"priceOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"volumeUSD\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"feeBps\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"feeAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"impactBps\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"swapKind\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PecorSwap\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PoolRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"virtualReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"realReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tokenReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PoolStateUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"roundId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"relayer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"confidence\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sourceId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PriceUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"previousAdminRole\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"newAdminRole\",\"type\":\"bytes32\"}],\"name\":\"RoleAdminChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"action\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"previousAdminRole\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"RoleChange\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"RoleGranted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"RoleRevoked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"intermediateUsdl\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"RouterTrade\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isBuy\",\"type\":\"bool\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"Swap\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"salt\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"symbol\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"decimals\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalSupply\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"TokenDeployed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"TokenTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"direction\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"TreasuryFlow\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"emitter\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"bytes4\",\"name\":\"selector\",\"type\":\"bytes4\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"allowed\",\"type\":\"bool\"}],\"name\":\"TypedEmitterPermission\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"Upgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"flowType\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"VaultFlow\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DEFAULT_ADMIN_ROLE\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"EMITTER_ADMIN_ROLE\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"EVENT_EMITTER_ROLE\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VERSION\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"}],\"name\":\"deregisterToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"valueOrTokenId\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"isNft\",\"type\":\"bool\"}],\"name\":\"emitAssetApproval\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"routeId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"internalType\":\"address[]\",\"name\":\"hops\",\"type\":\"address[]\"},{\"internalType\":\"uint256\",\"name\":\"protocolFeeBps\",\"type\":\"uint256\"}],\"name\":\"emitBestRouteSwap\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"sourceId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"reportedPrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"referencePrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deviationBps\",\"type\":\"uint256\"}],\"name\":\"emitCircuitBreaker\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"key\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"oldValue\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newValue\",\"type\":\"uint256\"}],\"name\":\"emitConfigUpdated\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"eventName\",\"type\":\"string\"},{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"value\",\"type\":\"address\"}],\"internalType\":\"structIEventEmitter.AddressKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address[]\",\"name\":\"value\",\"type\":\"address[]\"}],\"internalType\":\"structIEventEmitter.AddressArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.AddressItems\",\"name\":\"addressItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"internalType\":\"structIEventEmitter.UintKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256[]\",\"name\":\"value\",\"type\":\"uint256[]\"}],\"internalType\":\"structIEventEmitter.UintArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.UintItems\",\"name\":\"uintItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256\",\"name\":\"value\",\"type\":\"int256\"}],\"internalType\":\"structIEventEmitter.IntKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256[]\",\"name\":\"value\",\"type\":\"int256[]\"}],\"internalType\":\"structIEventEmitter.IntArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.IntItems\",\"name\":\"intItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"value\",\"type\":\"bool\"}],\"internalType\":\"structIEventEmitter.BoolKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool[]\",\"name\":\"value\",\"type\":\"bool[]\"}],\"internalType\":\"structIEventEmitter.BoolArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BoolItems\",\"name\":\"boolItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"value\",\"type\":\"bytes32\"}],\"internalType\":\"structIEventEmitter.Bytes32KeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32[]\",\"name\":\"value\",\"type\":\"bytes32[]\"}],\"internalType\":\"structIEventEmitter.Bytes32ArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.Bytes32Items\",\"name\":\"bytes32Items\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"internalType\":\"structIEventEmitter.BytesKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"value\",\"type\":\"bytes[]\"}],\"internalType\":\"structIEventEmitter.BytesArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BytesItems\",\"name\":\"bytesItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structIEventEmitter.StringKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"value\",\"type\":\"string[]\"}],\"internalType\":\"structIEventEmitter.StringArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.StringItems\",\"name\":\"stringItems\",\"type\":\"tuple\"}],\"internalType\":\"structIEventEmitter.EventData\",\"name\":\"data\",\"type\":\"tuple\"}],\"name\":\"emitEventLog\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"eventName\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"topic1\",\"type\":\"bytes32\"},{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"value\",\"type\":\"address\"}],\"internalType\":\"structIEventEmitter.AddressKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address[]\",\"name\":\"value\",\"type\":\"address[]\"}],\"internalType\":\"structIEventEmitter.AddressArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.AddressItems\",\"name\":\"addressItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"internalType\":\"structIEventEmitter.UintKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256[]\",\"name\":\"value\",\"type\":\"uint256[]\"}],\"internalType\":\"structIEventEmitter.UintArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.UintItems\",\"name\":\"uintItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256\",\"name\":\"value\",\"type\":\"int256\"}],\"internalType\":\"structIEventEmitter.IntKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256[]\",\"name\":\"value\",\"type\":\"int256[]\"}],\"internalType\":\"structIEventEmitter.IntArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.IntItems\",\"name\":\"intItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"value\",\"type\":\"bool\"}],\"internalType\":\"structIEventEmitter.BoolKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool[]\",\"name\":\"value\",\"type\":\"bool[]\"}],\"internalType\":\"structIEventEmitter.BoolArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BoolItems\",\"name\":\"boolItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"value\",\"type\":\"bytes32\"}],\"internalType\":\"structIEventEmitter.Bytes32KeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32[]\",\"name\":\"value\",\"type\":\"bytes32[]\"}],\"internalType\":\"structIEventEmitter.Bytes32ArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.Bytes32Items\",\"name\":\"bytes32Items\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"internalType\":\"structIEventEmitter.BytesKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"value\",\"type\":\"bytes[]\"}],\"internalType\":\"structIEventEmitter.BytesArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BytesItems\",\"name\":\"bytesItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structIEventEmitter.StringKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"value\",\"type\":\"string[]\"}],\"internalType\":\"structIEventEmitter.StringArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.StringItems\",\"name\":\"stringItems\",\"type\":\"tuple\"}],\"internalType\":\"structIEventEmitter.EventData\",\"name\":\"data\",\"type\":\"tuple\"}],\"name\":\"emitEventLog1\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"eventName\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"topic1\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"topic2\",\"type\":\"bytes32\"},{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"value\",\"type\":\"address\"}],\"internalType\":\"structIEventEmitter.AddressKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"address[]\",\"name\":\"value\",\"type\":\"address[]\"}],\"internalType\":\"structIEventEmitter.AddressArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.AddressItems\",\"name\":\"addressItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"internalType\":\"structIEventEmitter.UintKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256[]\",\"name\":\"value\",\"type\":\"uint256[]\"}],\"internalType\":\"structIEventEmitter.UintArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.UintItems\",\"name\":\"uintItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256\",\"name\":\"value\",\"type\":\"int256\"}],\"internalType\":\"structIEventEmitter.IntKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"int256[]\",\"name\":\"value\",\"type\":\"int256[]\"}],\"internalType\":\"structIEventEmitter.IntArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.IntItems\",\"name\":\"intItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"value\",\"type\":\"bool\"}],\"internalType\":\"structIEventEmitter.BoolKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bool[]\",\"name\":\"value\",\"type\":\"bool[]\"}],\"internalType\":\"structIEventEmitter.BoolArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BoolItems\",\"name\":\"boolItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"value\",\"type\":\"bytes32\"}],\"internalType\":\"structIEventEmitter.Bytes32KeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes32[]\",\"name\":\"value\",\"type\":\"bytes32[]\"}],\"internalType\":\"structIEventEmitter.Bytes32ArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.Bytes32Items\",\"name\":\"bytes32Items\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"internalType\":\"structIEventEmitter.BytesKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"value\",\"type\":\"bytes[]\"}],\"internalType\":\"structIEventEmitter.BytesArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.BytesItems\",\"name\":\"bytesItems\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structIEventEmitter.StringKeyValue[]\",\"name\":\"items\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"value\",\"type\":\"string[]\"}],\"internalType\":\"structIEventEmitter.StringArrayKeyValue[]\",\"name\":\"arrayItems\",\"type\":\"tuple[]\"}],\"internalType\":\"structIEventEmitter.StringItems\",\"name\":\"stringItems\",\"type\":\"tuple\"}],\"internalType\":\"structIEventEmitter.EventData\",\"name\":\"data\",\"type\":\"tuple\"}],\"name\":\"emitEventLog2\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"strategy\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"emitFeeDistributed\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"protocolCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"poolCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"epoch\",\"type\":\"uint256\"}],\"name\":\"emitFeeFlow\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"feeAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"protocolCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"poolCut\",\"type\":\"uint256\"}],\"name\":\"emitFeeRecorded\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"oldStrategy\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"newStrategy\",\"type\":\"uint8\"}],\"name\":\"emitFeeStrategyChanged\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"action\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"id\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"actor\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"}],\"name\":\"emitGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"}],\"name\":\"emitMarketCreated\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"strategy\",\"type\":\"uint8\"}],\"name\":\"emitNftMint\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nft\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"emitNftTransfer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"hookName\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"emitOpticalExecuted\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"action\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"name\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"}],\"name\":\"emitOpticalLifecycle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"sourceId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"adapter\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"phase\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"priority\",\"type\":\"uint256\"}],\"name\":\"emitOracleAdapterLifecycle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"pausedContract\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"paused\",\"type\":\"bool\"}],\"name\":\"emitPauseToggle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"orderId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"orderKind\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"orderType\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"targetPrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopPrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"limitPrice\",\"type\":\"uint256\"}],\"name\":\"emitPecorOrderCreated\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"orderId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"orderKind\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"phase\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"}],\"name\":\"emitPecorOrderLifecycle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"priceIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"priceOut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"volumeUSD\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"feeBps\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"feeAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"impactBps\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"swapKind\",\"type\":\"uint8\"}],\"name\":\"emitPecorSwap\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"}],\"name\":\"emitPoolRegistered\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"virtualReserve\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"realReserve\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"tokenReserve\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"}],\"name\":\"emitPoolStateUpdated\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"roundId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"relayer\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"confidence\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"sourceId\",\"type\":\"bytes32\"}],\"name\":\"emitPriceUpdated\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"action\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"previousAdminRole\",\"type\":\"bytes32\"}],\"name\":\"emitRoleChange\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"intermediateUsdl\",\"type\":\"uint256\"}],\"name\":\"emitRouterTrade\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"isBuy\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"}],\"name\":\"emitSwap\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"salt\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"symbol\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"decimals\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"totalSupply\",\"type\":\"uint256\"}],\"name\":\"emitTokenDeployed\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"emitTokenTransfer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"direction\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"emitTreasuryFlow\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"proxy\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"}],\"name\":\"emitUpgraded\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"flowType\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newReserve\",\"type\":\"uint256\"}],\"name\":\"emitVaultFlow\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"}],\"name\":\"getRoleAdmin\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"grantRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"hasRole\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_admin\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"emitter\",\"type\":\"address\"}],\"name\":\"isAuthorizedEmitter\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"}],\"name\":\"isRegisteredToken\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"emitter\",\"type\":\"address\"},{\"internalType\":\"bytes4\",\"name\":\"selector\",\"type\":\"bytes4\"}],\"name\":\"isTypedEmitter\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"metaAGRouter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"opticalRegistry\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"poolRegistry\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proxiableUUID\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"registerToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"registeredDefaultAdminCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"adminWithEmitterRole\",\"type\":\"address\"}],\"name\":\"reinitializeV2\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"callerConfirmation\",\"type\":\"address\"}],\"name\":\"renounceRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"revokeRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"emitter\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"authorized\",\"type\":\"bool\"}],\"name\":\"setAuthorizedEmitter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"router\",\"type\":\"address\"}],\"name\":\"setMetaAGRouter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"registry\",\"type\":\"address\"}],\"name\":\"setOpticalRegistry\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_poolRegistry\",\"type\":\"address\"}],\"name\":\"setPoolRegistry\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"factory\",\"type\":\"address\"}],\"name\":\"setSidioraFactory\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"setTokenRegistry\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"emitter\",\"type\":\"address\"},{\"internalType\":\"bytes4\",\"name\":\"selector\",\"type\":\"bytes4\"},{\"internalType\":\"bool\",\"name\":\"allowed\",\"type\":\"bool\"}],\"name\":\"setTypedEmitter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sidioraFactory\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeToAndCall\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// EventEmitterABI is the input ABI used to generate the binding from.
// Deprecated: Use EventEmitterMetaData.ABI instead.
var EventEmitterABI = EventEmitterMetaData.ABI

// EventEmitter is an auto generated Go binding around an Ethereum contract.
type EventEmitter struct {
	EventEmitterCaller     // Read-only binding to the contract
	EventEmitterTransactor // Write-only binding to the contract
	EventEmitterFilterer   // Log filterer for contract events
}

// EventEmitterCaller is an auto generated read-only Go binding around an Ethereum contract.
type EventEmitterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventEmitterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EventEmitterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventEmitterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EventEmitterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventEmitterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EventEmitterSession struct {
	Contract     *EventEmitter     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EventEmitterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EventEmitterCallerSession struct {
	Contract *EventEmitterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// EventEmitterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EventEmitterTransactorSession struct {
	Contract     *EventEmitterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// EventEmitterRaw is an auto generated low-level Go binding around an Ethereum contract.
type EventEmitterRaw struct {
	Contract *EventEmitter // Generic contract binding to access the raw methods on
}

// EventEmitterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EventEmitterCallerRaw struct {
	Contract *EventEmitterCaller // Generic read-only contract binding to access the raw methods on
}

// EventEmitterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EventEmitterTransactorRaw struct {
	Contract *EventEmitterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEventEmitter creates a new instance of EventEmitter, bound to a specific deployed contract.
func NewEventEmitter(address common.Address, backend bind.ContractBackend) (*EventEmitter, error) {
	contract, err := bindEventEmitter(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &EventEmitter{EventEmitterCaller: EventEmitterCaller{contract: contract}, EventEmitterTransactor: EventEmitterTransactor{contract: contract}, EventEmitterFilterer: EventEmitterFilterer{contract: contract}}, nil
}

// NewEventEmitterCaller creates a new read-only instance of EventEmitter, bound to a specific deployed contract.
func NewEventEmitterCaller(address common.Address, caller bind.ContractCaller) (*EventEmitterCaller, error) {
	contract, err := bindEventEmitter(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EventEmitterCaller{contract: contract}, nil
}

// NewEventEmitterTransactor creates a new write-only instance of EventEmitter, bound to a specific deployed contract.
func NewEventEmitterTransactor(address common.Address, transactor bind.ContractTransactor) (*EventEmitterTransactor, error) {
	contract, err := bindEventEmitter(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EventEmitterTransactor{contract: contract}, nil
}

// NewEventEmitterFilterer creates a new log filterer instance of EventEmitter, bound to a specific deployed contract.
func NewEventEmitterFilterer(address common.Address, filterer bind.ContractFilterer) (*EventEmitterFilterer, error) {
	contract, err := bindEventEmitter(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EventEmitterFilterer{contract: contract}, nil
}

// bindEventEmitter binds a generic wrapper to an already deployed contract.
func bindEventEmitter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := EventEmitterMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EventEmitter *EventEmitterRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EventEmitter.Contract.EventEmitterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EventEmitter *EventEmitterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EventEmitter.Contract.EventEmitterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EventEmitter *EventEmitterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EventEmitter.Contract.EventEmitterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EventEmitter *EventEmitterCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EventEmitter.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EventEmitter *EventEmitterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EventEmitter.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EventEmitter *EventEmitterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EventEmitter.Contract.contract.Transact(opts, method, params...)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterCaller) DEFAULTADMINROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "DEFAULT_ADMIN_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _EventEmitter.Contract.DEFAULTADMINROLE(&_EventEmitter.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterCallerSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _EventEmitter.Contract.DEFAULTADMINROLE(&_EventEmitter.CallOpts)
}

// EMITTERADMINROLE is a free data retrieval call binding the contract method 0xc2d11c38.
//
// Solidity: function EMITTER_ADMIN_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterCaller) EMITTERADMINROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "EMITTER_ADMIN_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// EMITTERADMINROLE is a free data retrieval call binding the contract method 0xc2d11c38.
//
// Solidity: function EMITTER_ADMIN_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterSession) EMITTERADMINROLE() ([32]byte, error) {
	return _EventEmitter.Contract.EMITTERADMINROLE(&_EventEmitter.CallOpts)
}

// EMITTERADMINROLE is a free data retrieval call binding the contract method 0xc2d11c38.
//
// Solidity: function EMITTER_ADMIN_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterCallerSession) EMITTERADMINROLE() ([32]byte, error) {
	return _EventEmitter.Contract.EMITTERADMINROLE(&_EventEmitter.CallOpts)
}

// EVENTEMITTERROLE is a free data retrieval call binding the contract method 0xd7056c20.
//
// Solidity: function EVENT_EMITTER_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterCaller) EVENTEMITTERROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "EVENT_EMITTER_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// EVENTEMITTERROLE is a free data retrieval call binding the contract method 0xd7056c20.
//
// Solidity: function EVENT_EMITTER_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterSession) EVENTEMITTERROLE() ([32]byte, error) {
	return _EventEmitter.Contract.EVENTEMITTERROLE(&_EventEmitter.CallOpts)
}

// EVENTEMITTERROLE is a free data retrieval call binding the contract method 0xd7056c20.
//
// Solidity: function EVENT_EMITTER_ROLE() view returns(bytes32)
func (_EventEmitter *EventEmitterCallerSession) EVENTEMITTERROLE() ([32]byte, error) {
	return _EventEmitter.Contract.EVENTEMITTERROLE(&_EventEmitter.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() pure returns(string)
func (_EventEmitter *EventEmitterCaller) VERSION(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "VERSION")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() pure returns(string)
func (_EventEmitter *EventEmitterSession) VERSION() (string, error) {
	return _EventEmitter.Contract.VERSION(&_EventEmitter.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() pure returns(string)
func (_EventEmitter *EventEmitterCallerSession) VERSION() (string, error) {
	return _EventEmitter.Contract.VERSION(&_EventEmitter.CallOpts)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_EventEmitter *EventEmitterCaller) GetRoleAdmin(opts *bind.CallOpts, role [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "getRoleAdmin", role)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_EventEmitter *EventEmitterSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _EventEmitter.Contract.GetRoleAdmin(&_EventEmitter.CallOpts, role)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_EventEmitter *EventEmitterCallerSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _EventEmitter.Contract.GetRoleAdmin(&_EventEmitter.CallOpts, role)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_EventEmitter *EventEmitterCaller) HasRole(opts *bind.CallOpts, role [32]byte, account common.Address) (bool, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "hasRole", role, account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_EventEmitter *EventEmitterSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _EventEmitter.Contract.HasRole(&_EventEmitter.CallOpts, role, account)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_EventEmitter *EventEmitterCallerSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _EventEmitter.Contract.HasRole(&_EventEmitter.CallOpts, role, account)
}

// IsAuthorizedEmitter is a free data retrieval call binding the contract method 0x66706fd2.
//
// Solidity: function isAuthorizedEmitter(address emitter) view returns(bool)
func (_EventEmitter *EventEmitterCaller) IsAuthorizedEmitter(opts *bind.CallOpts, emitter common.Address) (bool, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "isAuthorizedEmitter", emitter)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsAuthorizedEmitter is a free data retrieval call binding the contract method 0x66706fd2.
//
// Solidity: function isAuthorizedEmitter(address emitter) view returns(bool)
func (_EventEmitter *EventEmitterSession) IsAuthorizedEmitter(emitter common.Address) (bool, error) {
	return _EventEmitter.Contract.IsAuthorizedEmitter(&_EventEmitter.CallOpts, emitter)
}

// IsAuthorizedEmitter is a free data retrieval call binding the contract method 0x66706fd2.
//
// Solidity: function isAuthorizedEmitter(address emitter) view returns(bool)
func (_EventEmitter *EventEmitterCallerSession) IsAuthorizedEmitter(emitter common.Address) (bool, error) {
	return _EventEmitter.Contract.IsAuthorizedEmitter(&_EventEmitter.CallOpts, emitter)
}

// IsRegisteredToken is a free data retrieval call binding the contract method 0x7639a230.
//
// Solidity: function isRegisteredToken(address token) view returns(bool)
func (_EventEmitter *EventEmitterCaller) IsRegisteredToken(opts *bind.CallOpts, token common.Address) (bool, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "isRegisteredToken", token)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsRegisteredToken is a free data retrieval call binding the contract method 0x7639a230.
//
// Solidity: function isRegisteredToken(address token) view returns(bool)
func (_EventEmitter *EventEmitterSession) IsRegisteredToken(token common.Address) (bool, error) {
	return _EventEmitter.Contract.IsRegisteredToken(&_EventEmitter.CallOpts, token)
}

// IsRegisteredToken is a free data retrieval call binding the contract method 0x7639a230.
//
// Solidity: function isRegisteredToken(address token) view returns(bool)
func (_EventEmitter *EventEmitterCallerSession) IsRegisteredToken(token common.Address) (bool, error) {
	return _EventEmitter.Contract.IsRegisteredToken(&_EventEmitter.CallOpts, token)
}

// IsTypedEmitter is a free data retrieval call binding the contract method 0x48c3830a.
//
// Solidity: function isTypedEmitter(address emitter, bytes4 selector) view returns(bool)
func (_EventEmitter *EventEmitterCaller) IsTypedEmitter(opts *bind.CallOpts, emitter common.Address, selector [4]byte) (bool, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "isTypedEmitter", emitter, selector)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsTypedEmitter is a free data retrieval call binding the contract method 0x48c3830a.
//
// Solidity: function isTypedEmitter(address emitter, bytes4 selector) view returns(bool)
func (_EventEmitter *EventEmitterSession) IsTypedEmitter(emitter common.Address, selector [4]byte) (bool, error) {
	return _EventEmitter.Contract.IsTypedEmitter(&_EventEmitter.CallOpts, emitter, selector)
}

// IsTypedEmitter is a free data retrieval call binding the contract method 0x48c3830a.
//
// Solidity: function isTypedEmitter(address emitter, bytes4 selector) view returns(bool)
func (_EventEmitter *EventEmitterCallerSession) IsTypedEmitter(emitter common.Address, selector [4]byte) (bool, error) {
	return _EventEmitter.Contract.IsTypedEmitter(&_EventEmitter.CallOpts, emitter, selector)
}

// MetaAGRouter is a free data retrieval call binding the contract method 0xfa0dced9.
//
// Solidity: function metaAGRouter() view returns(address)
func (_EventEmitter *EventEmitterCaller) MetaAGRouter(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "metaAGRouter")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// MetaAGRouter is a free data retrieval call binding the contract method 0xfa0dced9.
//
// Solidity: function metaAGRouter() view returns(address)
func (_EventEmitter *EventEmitterSession) MetaAGRouter() (common.Address, error) {
	return _EventEmitter.Contract.MetaAGRouter(&_EventEmitter.CallOpts)
}

// MetaAGRouter is a free data retrieval call binding the contract method 0xfa0dced9.
//
// Solidity: function metaAGRouter() view returns(address)
func (_EventEmitter *EventEmitterCallerSession) MetaAGRouter() (common.Address, error) {
	return _EventEmitter.Contract.MetaAGRouter(&_EventEmitter.CallOpts)
}

// OpticalRegistry is a free data retrieval call binding the contract method 0x73b5708c.
//
// Solidity: function opticalRegistry() view returns(address)
func (_EventEmitter *EventEmitterCaller) OpticalRegistry(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "opticalRegistry")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OpticalRegistry is a free data retrieval call binding the contract method 0x73b5708c.
//
// Solidity: function opticalRegistry() view returns(address)
func (_EventEmitter *EventEmitterSession) OpticalRegistry() (common.Address, error) {
	return _EventEmitter.Contract.OpticalRegistry(&_EventEmitter.CallOpts)
}

// OpticalRegistry is a free data retrieval call binding the contract method 0x73b5708c.
//
// Solidity: function opticalRegistry() view returns(address)
func (_EventEmitter *EventEmitterCallerSession) OpticalRegistry() (common.Address, error) {
	return _EventEmitter.Contract.OpticalRegistry(&_EventEmitter.CallOpts)
}

// PoolRegistry is a free data retrieval call binding the contract method 0xafcff50f.
//
// Solidity: function poolRegistry() view returns(address)
func (_EventEmitter *EventEmitterCaller) PoolRegistry(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "poolRegistry")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PoolRegistry is a free data retrieval call binding the contract method 0xafcff50f.
//
// Solidity: function poolRegistry() view returns(address)
func (_EventEmitter *EventEmitterSession) PoolRegistry() (common.Address, error) {
	return _EventEmitter.Contract.PoolRegistry(&_EventEmitter.CallOpts)
}

// PoolRegistry is a free data retrieval call binding the contract method 0xafcff50f.
//
// Solidity: function poolRegistry() view returns(address)
func (_EventEmitter *EventEmitterCallerSession) PoolRegistry() (common.Address, error) {
	return _EventEmitter.Contract.PoolRegistry(&_EventEmitter.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_EventEmitter *EventEmitterCaller) ProxiableUUID(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "proxiableUUID")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_EventEmitter *EventEmitterSession) ProxiableUUID() ([32]byte, error) {
	return _EventEmitter.Contract.ProxiableUUID(&_EventEmitter.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_EventEmitter *EventEmitterCallerSession) ProxiableUUID() ([32]byte, error) {
	return _EventEmitter.Contract.ProxiableUUID(&_EventEmitter.CallOpts)
}

// RegisteredDefaultAdminCount is a free data retrieval call binding the contract method 0xc0cb71b6.
//
// Solidity: function registeredDefaultAdminCount() view returns(uint256)
func (_EventEmitter *EventEmitterCaller) RegisteredDefaultAdminCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "registeredDefaultAdminCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RegisteredDefaultAdminCount is a free data retrieval call binding the contract method 0xc0cb71b6.
//
// Solidity: function registeredDefaultAdminCount() view returns(uint256)
func (_EventEmitter *EventEmitterSession) RegisteredDefaultAdminCount() (*big.Int, error) {
	return _EventEmitter.Contract.RegisteredDefaultAdminCount(&_EventEmitter.CallOpts)
}

// RegisteredDefaultAdminCount is a free data retrieval call binding the contract method 0xc0cb71b6.
//
// Solidity: function registeredDefaultAdminCount() view returns(uint256)
func (_EventEmitter *EventEmitterCallerSession) RegisteredDefaultAdminCount() (*big.Int, error) {
	return _EventEmitter.Contract.RegisteredDefaultAdminCount(&_EventEmitter.CallOpts)
}

// SidioraFactory is a free data retrieval call binding the contract method 0xb182ba64.
//
// Solidity: function sidioraFactory() view returns(address)
func (_EventEmitter *EventEmitterCaller) SidioraFactory(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EventEmitter.contract.Call(opts, &out, "sidioraFactory")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SidioraFactory is a free data retrieval call binding the contract method 0xb182ba64.
//
// Solidity: function sidioraFactory() view returns(address)
func (_EventEmitter *EventEmitterSession) SidioraFactory() (common.Address, error) {
	return _EventEmitter.Contract.SidioraFactory(&_EventEmitter.CallOpts)
}

// SidioraFactory is a free data retrieval call binding the contract method 0xb182ba64.
//
// Solidity: function sidioraFactory() view returns(address)
func (_EventEmitter *EventEmitterCallerSession) SidioraFactory() (common.Address, error) {
	return _EventEmitter.Contract.SidioraFactory(&_EventEmitter.CallOpts)
}

// DeregisterToken is a paid mutator transaction binding the contract method 0xbab2af1d.
//
// Solidity: function deregisterToken(address token) returns()
func (_EventEmitter *EventEmitterTransactor) DeregisterToken(opts *bind.TransactOpts, token common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "deregisterToken", token)
}

// DeregisterToken is a paid mutator transaction binding the contract method 0xbab2af1d.
//
// Solidity: function deregisterToken(address token) returns()
func (_EventEmitter *EventEmitterSession) DeregisterToken(token common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.DeregisterToken(&_EventEmitter.TransactOpts, token)
}

// DeregisterToken is a paid mutator transaction binding the contract method 0xbab2af1d.
//
// Solidity: function deregisterToken(address token) returns()
func (_EventEmitter *EventEmitterTransactorSession) DeregisterToken(token common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.DeregisterToken(&_EventEmitter.TransactOpts, token)
}

// EmitAssetApproval is a paid mutator transaction binding the contract method 0x4f2bdd09.
//
// Solidity: function emitAssetApproval(address asset, address owner, address spender, uint256 valueOrTokenId, bool isNft) returns()
func (_EventEmitter *EventEmitterTransactor) EmitAssetApproval(opts *bind.TransactOpts, asset common.Address, owner common.Address, spender common.Address, valueOrTokenId *big.Int, isNft bool) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitAssetApproval", asset, owner, spender, valueOrTokenId, isNft)
}

// EmitAssetApproval is a paid mutator transaction binding the contract method 0x4f2bdd09.
//
// Solidity: function emitAssetApproval(address asset, address owner, address spender, uint256 valueOrTokenId, bool isNft) returns()
func (_EventEmitter *EventEmitterSession) EmitAssetApproval(asset common.Address, owner common.Address, spender common.Address, valueOrTokenId *big.Int, isNft bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitAssetApproval(&_EventEmitter.TransactOpts, asset, owner, spender, valueOrTokenId, isNft)
}

// EmitAssetApproval is a paid mutator transaction binding the contract method 0x4f2bdd09.
//
// Solidity: function emitAssetApproval(address asset, address owner, address spender, uint256 valueOrTokenId, bool isNft) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitAssetApproval(asset common.Address, owner common.Address, spender common.Address, valueOrTokenId *big.Int, isNft bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitAssetApproval(&_EventEmitter.TransactOpts, asset, owner, spender, valueOrTokenId, isNft)
}

// EmitBestRouteSwap is a paid mutator transaction binding the contract method 0xb3defa3c.
//
// Solidity: function emitBestRouteSwap(address user, address tokenIn, address tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps) returns()
func (_EventEmitter *EventEmitterTransactor) EmitBestRouteSwap(opts *bind.TransactOpts, user common.Address, tokenIn common.Address, tokenOut common.Address, routeId [32]byte, amountIn *big.Int, amountOut *big.Int, hops []common.Address, protocolFeeBps *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitBestRouteSwap", user, tokenIn, tokenOut, routeId, amountIn, amountOut, hops, protocolFeeBps)
}

// EmitBestRouteSwap is a paid mutator transaction binding the contract method 0xb3defa3c.
//
// Solidity: function emitBestRouteSwap(address user, address tokenIn, address tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps) returns()
func (_EventEmitter *EventEmitterSession) EmitBestRouteSwap(user common.Address, tokenIn common.Address, tokenOut common.Address, routeId [32]byte, amountIn *big.Int, amountOut *big.Int, hops []common.Address, protocolFeeBps *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitBestRouteSwap(&_EventEmitter.TransactOpts, user, tokenIn, tokenOut, routeId, amountIn, amountOut, hops, protocolFeeBps)
}

// EmitBestRouteSwap is a paid mutator transaction binding the contract method 0xb3defa3c.
//
// Solidity: function emitBestRouteSwap(address user, address tokenIn, address tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitBestRouteSwap(user common.Address, tokenIn common.Address, tokenOut common.Address, routeId [32]byte, amountIn *big.Int, amountOut *big.Int, hops []common.Address, protocolFeeBps *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitBestRouteSwap(&_EventEmitter.TransactOpts, user, tokenIn, tokenOut, routeId, amountIn, amountOut, hops, protocolFeeBps)
}

// EmitCircuitBreaker is a paid mutator transaction binding the contract method 0xd6278d9c.
//
// Solidity: function emitCircuitBreaker(address token, bytes32 sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps) returns()
func (_EventEmitter *EventEmitterTransactor) EmitCircuitBreaker(opts *bind.TransactOpts, token common.Address, sourceId [32]byte, reportedPrice *big.Int, referencePrice *big.Int, deviationBps *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitCircuitBreaker", token, sourceId, reportedPrice, referencePrice, deviationBps)
}

// EmitCircuitBreaker is a paid mutator transaction binding the contract method 0xd6278d9c.
//
// Solidity: function emitCircuitBreaker(address token, bytes32 sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps) returns()
func (_EventEmitter *EventEmitterSession) EmitCircuitBreaker(token common.Address, sourceId [32]byte, reportedPrice *big.Int, referencePrice *big.Int, deviationBps *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitCircuitBreaker(&_EventEmitter.TransactOpts, token, sourceId, reportedPrice, referencePrice, deviationBps)
}

// EmitCircuitBreaker is a paid mutator transaction binding the contract method 0xd6278d9c.
//
// Solidity: function emitCircuitBreaker(address token, bytes32 sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitCircuitBreaker(token common.Address, sourceId [32]byte, reportedPrice *big.Int, referencePrice *big.Int, deviationBps *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitCircuitBreaker(&_EventEmitter.TransactOpts, token, sourceId, reportedPrice, referencePrice, deviationBps)
}

// EmitConfigUpdated is a paid mutator transaction binding the contract method 0x85ceceb5.
//
// Solidity: function emitConfigUpdated(bytes32 key, uint256 oldValue, uint256 newValue) returns()
func (_EventEmitter *EventEmitterTransactor) EmitConfigUpdated(opts *bind.TransactOpts, key [32]byte, oldValue *big.Int, newValue *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitConfigUpdated", key, oldValue, newValue)
}

// EmitConfigUpdated is a paid mutator transaction binding the contract method 0x85ceceb5.
//
// Solidity: function emitConfigUpdated(bytes32 key, uint256 oldValue, uint256 newValue) returns()
func (_EventEmitter *EventEmitterSession) EmitConfigUpdated(key [32]byte, oldValue *big.Int, newValue *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitConfigUpdated(&_EventEmitter.TransactOpts, key, oldValue, newValue)
}

// EmitConfigUpdated is a paid mutator transaction binding the contract method 0x85ceceb5.
//
// Solidity: function emitConfigUpdated(bytes32 key, uint256 oldValue, uint256 newValue) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitConfigUpdated(key [32]byte, oldValue *big.Int, newValue *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitConfigUpdated(&_EventEmitter.TransactOpts, key, oldValue, newValue)
}

// EmitEventLog is a paid mutator transaction binding the contract method 0x906c49f6.
//
// Solidity: function emitEventLog(string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterTransactor) EmitEventLog(opts *bind.TransactOpts, eventName string, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitEventLog", eventName, data)
}

// EmitEventLog is a paid mutator transaction binding the contract method 0x906c49f6.
//
// Solidity: function emitEventLog(string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterSession) EmitEventLog(eventName string, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitEventLog(&_EventEmitter.TransactOpts, eventName, data)
}

// EmitEventLog is a paid mutator transaction binding the contract method 0x906c49f6.
//
// Solidity: function emitEventLog(string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitEventLog(eventName string, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitEventLog(&_EventEmitter.TransactOpts, eventName, data)
}

// EmitEventLog1 is a paid mutator transaction binding the contract method 0x24de01e4.
//
// Solidity: function emitEventLog1(string eventName, bytes32 topic1, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterTransactor) EmitEventLog1(opts *bind.TransactOpts, eventName string, topic1 [32]byte, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitEventLog1", eventName, topic1, data)
}

// EmitEventLog1 is a paid mutator transaction binding the contract method 0x24de01e4.
//
// Solidity: function emitEventLog1(string eventName, bytes32 topic1, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterSession) EmitEventLog1(eventName string, topic1 [32]byte, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitEventLog1(&_EventEmitter.TransactOpts, eventName, topic1, data)
}

// EmitEventLog1 is a paid mutator transaction binding the contract method 0x24de01e4.
//
// Solidity: function emitEventLog1(string eventName, bytes32 topic1, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitEventLog1(eventName string, topic1 [32]byte, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitEventLog1(&_EventEmitter.TransactOpts, eventName, topic1, data)
}

// EmitEventLog2 is a paid mutator transaction binding the contract method 0x63d16363.
//
// Solidity: function emitEventLog2(string eventName, bytes32 topic1, bytes32 topic2, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterTransactor) EmitEventLog2(opts *bind.TransactOpts, eventName string, topic1 [32]byte, topic2 [32]byte, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitEventLog2", eventName, topic1, topic2, data)
}

// EmitEventLog2 is a paid mutator transaction binding the contract method 0x63d16363.
//
// Solidity: function emitEventLog2(string eventName, bytes32 topic1, bytes32 topic2, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterSession) EmitEventLog2(eventName string, topic1 [32]byte, topic2 [32]byte, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitEventLog2(&_EventEmitter.TransactOpts, eventName, topic1, topic2, data)
}

// EmitEventLog2 is a paid mutator transaction binding the contract method 0x63d16363.
//
// Solidity: function emitEventLog2(string eventName, bytes32 topic1, bytes32 topic2, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) data) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitEventLog2(eventName string, topic1 [32]byte, topic2 [32]byte, data IEventEmitterEventData) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitEventLog2(&_EventEmitter.TransactOpts, eventName, topic1, topic2, data)
}

// EmitFeeDistributed is a paid mutator transaction binding the contract method 0x06a7cedb.
//
// Solidity: function emitFeeDistributed(bytes32 poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient) returns()
func (_EventEmitter *EventEmitterTransactor) EmitFeeDistributed(opts *bind.TransactOpts, poolId [32]byte, nftId *big.Int, strategy uint8, amount *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitFeeDistributed", poolId, nftId, strategy, amount, recipient)
}

// EmitFeeDistributed is a paid mutator transaction binding the contract method 0x06a7cedb.
//
// Solidity: function emitFeeDistributed(bytes32 poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient) returns()
func (_EventEmitter *EventEmitterSession) EmitFeeDistributed(poolId [32]byte, nftId *big.Int, strategy uint8, amount *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeDistributed(&_EventEmitter.TransactOpts, poolId, nftId, strategy, amount, recipient)
}

// EmitFeeDistributed is a paid mutator transaction binding the contract method 0x06a7cedb.
//
// Solidity: function emitFeeDistributed(bytes32 poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitFeeDistributed(poolId [32]byte, nftId *big.Int, strategy uint8, amount *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeDistributed(&_EventEmitter.TransactOpts, poolId, nftId, strategy, amount, recipient)
}

// EmitFeeFlow is a paid mutator transaction binding the contract method 0xfa2e1d5e.
//
// Solidity: function emitFeeFlow(uint8 kind, address pool, address party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch) returns()
func (_EventEmitter *EventEmitterTransactor) EmitFeeFlow(opts *bind.TransactOpts, kind uint8, pool common.Address, party common.Address, amount *big.Int, protocolCut *big.Int, poolCut *big.Int, epoch *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitFeeFlow", kind, pool, party, amount, protocolCut, poolCut, epoch)
}

// EmitFeeFlow is a paid mutator transaction binding the contract method 0xfa2e1d5e.
//
// Solidity: function emitFeeFlow(uint8 kind, address pool, address party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch) returns()
func (_EventEmitter *EventEmitterSession) EmitFeeFlow(kind uint8, pool common.Address, party common.Address, amount *big.Int, protocolCut *big.Int, poolCut *big.Int, epoch *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeFlow(&_EventEmitter.TransactOpts, kind, pool, party, amount, protocolCut, poolCut, epoch)
}

// EmitFeeFlow is a paid mutator transaction binding the contract method 0xfa2e1d5e.
//
// Solidity: function emitFeeFlow(uint8 kind, address pool, address party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitFeeFlow(kind uint8, pool common.Address, party common.Address, amount *big.Int, protocolCut *big.Int, poolCut *big.Int, epoch *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeFlow(&_EventEmitter.TransactOpts, kind, pool, party, amount, protocolCut, poolCut, epoch)
}

// EmitFeeRecorded is a paid mutator transaction binding the contract method 0xd6d28200.
//
// Solidity: function emitFeeRecorded(bytes32 poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut) returns()
func (_EventEmitter *EventEmitterTransactor) EmitFeeRecorded(opts *bind.TransactOpts, poolId [32]byte, feeAmount *big.Int, protocolCut *big.Int, poolCut *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitFeeRecorded", poolId, feeAmount, protocolCut, poolCut)
}

// EmitFeeRecorded is a paid mutator transaction binding the contract method 0xd6d28200.
//
// Solidity: function emitFeeRecorded(bytes32 poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut) returns()
func (_EventEmitter *EventEmitterSession) EmitFeeRecorded(poolId [32]byte, feeAmount *big.Int, protocolCut *big.Int, poolCut *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeRecorded(&_EventEmitter.TransactOpts, poolId, feeAmount, protocolCut, poolCut)
}

// EmitFeeRecorded is a paid mutator transaction binding the contract method 0xd6d28200.
//
// Solidity: function emitFeeRecorded(bytes32 poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitFeeRecorded(poolId [32]byte, feeAmount *big.Int, protocolCut *big.Int, poolCut *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeRecorded(&_EventEmitter.TransactOpts, poolId, feeAmount, protocolCut, poolCut)
}

// EmitFeeStrategyChanged is a paid mutator transaction binding the contract method 0xb2c8dc72.
//
// Solidity: function emitFeeStrategyChanged(bytes32 poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy) returns()
func (_EventEmitter *EventEmitterTransactor) EmitFeeStrategyChanged(opts *bind.TransactOpts, poolId [32]byte, nftId *big.Int, oldStrategy uint8, newStrategy uint8) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitFeeStrategyChanged", poolId, nftId, oldStrategy, newStrategy)
}

// EmitFeeStrategyChanged is a paid mutator transaction binding the contract method 0xb2c8dc72.
//
// Solidity: function emitFeeStrategyChanged(bytes32 poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy) returns()
func (_EventEmitter *EventEmitterSession) EmitFeeStrategyChanged(poolId [32]byte, nftId *big.Int, oldStrategy uint8, newStrategy uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeStrategyChanged(&_EventEmitter.TransactOpts, poolId, nftId, oldStrategy, newStrategy)
}

// EmitFeeStrategyChanged is a paid mutator transaction binding the contract method 0xb2c8dc72.
//
// Solidity: function emitFeeStrategyChanged(bytes32 poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitFeeStrategyChanged(poolId [32]byte, nftId *big.Int, oldStrategy uint8, newStrategy uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitFeeStrategyChanged(&_EventEmitter.TransactOpts, poolId, nftId, oldStrategy, newStrategy)
}

// EmitGovernance is a paid mutator transaction binding the contract method 0x98616097.
//
// Solidity: function emitGovernance(uint8 action, bytes32 id, address actor, bytes payload) returns()
func (_EventEmitter *EventEmitterTransactor) EmitGovernance(opts *bind.TransactOpts, action uint8, id [32]byte, actor common.Address, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitGovernance", action, id, actor, payload)
}

// EmitGovernance is a paid mutator transaction binding the contract method 0x98616097.
//
// Solidity: function emitGovernance(uint8 action, bytes32 id, address actor, bytes payload) returns()
func (_EventEmitter *EventEmitterSession) EmitGovernance(action uint8, id [32]byte, actor common.Address, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitGovernance(&_EventEmitter.TransactOpts, action, id, actor, payload)
}

// EmitGovernance is a paid mutator transaction binding the contract method 0x98616097.
//
// Solidity: function emitGovernance(uint8 action, bytes32 id, address actor, bytes payload) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitGovernance(action uint8, id [32]byte, actor common.Address, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitGovernance(&_EventEmitter.TransactOpts, action, id, actor, payload)
}

// EmitMarketCreated is a paid mutator transaction binding the contract method 0xe4617ca5.
//
// Solidity: function emitMarketCreated(bytes32 poolId, address token, address creator, address pool, address optical) returns()
func (_EventEmitter *EventEmitterTransactor) EmitMarketCreated(opts *bind.TransactOpts, poolId [32]byte, token common.Address, creator common.Address, pool common.Address, optical common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitMarketCreated", poolId, token, creator, pool, optical)
}

// EmitMarketCreated is a paid mutator transaction binding the contract method 0xe4617ca5.
//
// Solidity: function emitMarketCreated(bytes32 poolId, address token, address creator, address pool, address optical) returns()
func (_EventEmitter *EventEmitterSession) EmitMarketCreated(poolId [32]byte, token common.Address, creator common.Address, pool common.Address, optical common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitMarketCreated(&_EventEmitter.TransactOpts, poolId, token, creator, pool, optical)
}

// EmitMarketCreated is a paid mutator transaction binding the contract method 0xe4617ca5.
//
// Solidity: function emitMarketCreated(bytes32 poolId, address token, address creator, address pool, address optical) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitMarketCreated(poolId [32]byte, token common.Address, creator common.Address, pool common.Address, optical common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitMarketCreated(&_EventEmitter.TransactOpts, poolId, token, creator, pool, optical)
}

// EmitNftMint is a paid mutator transaction binding the contract method 0x75c48038.
//
// Solidity: function emitNftMint(uint256 tokenId, address creator, address pool, uint8 strategy) returns()
func (_EventEmitter *EventEmitterTransactor) EmitNftMint(opts *bind.TransactOpts, tokenId *big.Int, creator common.Address, pool common.Address, strategy uint8) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitNftMint", tokenId, creator, pool, strategy)
}

// EmitNftMint is a paid mutator transaction binding the contract method 0x75c48038.
//
// Solidity: function emitNftMint(uint256 tokenId, address creator, address pool, uint8 strategy) returns()
func (_EventEmitter *EventEmitterSession) EmitNftMint(tokenId *big.Int, creator common.Address, pool common.Address, strategy uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitNftMint(&_EventEmitter.TransactOpts, tokenId, creator, pool, strategy)
}

// EmitNftMint is a paid mutator transaction binding the contract method 0x75c48038.
//
// Solidity: function emitNftMint(uint256 tokenId, address creator, address pool, uint8 strategy) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitNftMint(tokenId *big.Int, creator common.Address, pool common.Address, strategy uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitNftMint(&_EventEmitter.TransactOpts, tokenId, creator, pool, strategy)
}

// EmitNftTransfer is a paid mutator transaction binding the contract method 0xeeaf8bcd.
//
// Solidity: function emitNftTransfer(address nft, address from, address to, uint256 tokenId) returns()
func (_EventEmitter *EventEmitterTransactor) EmitNftTransfer(opts *bind.TransactOpts, nft common.Address, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitNftTransfer", nft, from, to, tokenId)
}

// EmitNftTransfer is a paid mutator transaction binding the contract method 0xeeaf8bcd.
//
// Solidity: function emitNftTransfer(address nft, address from, address to, uint256 tokenId) returns()
func (_EventEmitter *EventEmitterSession) EmitNftTransfer(nft common.Address, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitNftTransfer(&_EventEmitter.TransactOpts, nft, from, to, tokenId)
}

// EmitNftTransfer is a paid mutator transaction binding the contract method 0xeeaf8bcd.
//
// Solidity: function emitNftTransfer(address nft, address from, address to, uint256 tokenId) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitNftTransfer(nft common.Address, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitNftTransfer(&_EventEmitter.TransactOpts, nft, from, to, tokenId)
}

// EmitOpticalExecuted is a paid mutator transaction binding the contract method 0xd4756325.
//
// Solidity: function emitOpticalExecuted(bytes32 poolId, address optical, string hookName, bytes data) returns()
func (_EventEmitter *EventEmitterTransactor) EmitOpticalExecuted(opts *bind.TransactOpts, poolId [32]byte, optical common.Address, hookName string, data []byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitOpticalExecuted", poolId, optical, hookName, data)
}

// EmitOpticalExecuted is a paid mutator transaction binding the contract method 0xd4756325.
//
// Solidity: function emitOpticalExecuted(bytes32 poolId, address optical, string hookName, bytes data) returns()
func (_EventEmitter *EventEmitterSession) EmitOpticalExecuted(poolId [32]byte, optical common.Address, hookName string, data []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitOpticalExecuted(&_EventEmitter.TransactOpts, poolId, optical, hookName, data)
}

// EmitOpticalExecuted is a paid mutator transaction binding the contract method 0xd4756325.
//
// Solidity: function emitOpticalExecuted(bytes32 poolId, address optical, string hookName, bytes data) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitOpticalExecuted(poolId [32]byte, optical common.Address, hookName string, data []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitOpticalExecuted(&_EventEmitter.TransactOpts, poolId, optical, hookName, data)
}

// EmitOpticalLifecycle is a paid mutator transaction binding the contract method 0x82b678c3.
//
// Solidity: function emitOpticalLifecycle(uint8 action, address optical, address pool, bytes32 name, bytes payload) returns()
func (_EventEmitter *EventEmitterTransactor) EmitOpticalLifecycle(opts *bind.TransactOpts, action uint8, optical common.Address, pool common.Address, name [32]byte, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitOpticalLifecycle", action, optical, pool, name, payload)
}

// EmitOpticalLifecycle is a paid mutator transaction binding the contract method 0x82b678c3.
//
// Solidity: function emitOpticalLifecycle(uint8 action, address optical, address pool, bytes32 name, bytes payload) returns()
func (_EventEmitter *EventEmitterSession) EmitOpticalLifecycle(action uint8, optical common.Address, pool common.Address, name [32]byte, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitOpticalLifecycle(&_EventEmitter.TransactOpts, action, optical, pool, name, payload)
}

// EmitOpticalLifecycle is a paid mutator transaction binding the contract method 0x82b678c3.
//
// Solidity: function emitOpticalLifecycle(uint8 action, address optical, address pool, bytes32 name, bytes payload) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitOpticalLifecycle(action uint8, optical common.Address, pool common.Address, name [32]byte, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitOpticalLifecycle(&_EventEmitter.TransactOpts, action, optical, pool, name, payload)
}

// EmitOracleAdapterLifecycle is a paid mutator transaction binding the contract method 0x176af80c.
//
// Solidity: function emitOracleAdapterLifecycle(bytes32 sourceId, address adapter, uint8 phase, uint256 priority) returns()
func (_EventEmitter *EventEmitterTransactor) EmitOracleAdapterLifecycle(opts *bind.TransactOpts, sourceId [32]byte, adapter common.Address, phase uint8, priority *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitOracleAdapterLifecycle", sourceId, adapter, phase, priority)
}

// EmitOracleAdapterLifecycle is a paid mutator transaction binding the contract method 0x176af80c.
//
// Solidity: function emitOracleAdapterLifecycle(bytes32 sourceId, address adapter, uint8 phase, uint256 priority) returns()
func (_EventEmitter *EventEmitterSession) EmitOracleAdapterLifecycle(sourceId [32]byte, adapter common.Address, phase uint8, priority *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitOracleAdapterLifecycle(&_EventEmitter.TransactOpts, sourceId, adapter, phase, priority)
}

// EmitOracleAdapterLifecycle is a paid mutator transaction binding the contract method 0x176af80c.
//
// Solidity: function emitOracleAdapterLifecycle(bytes32 sourceId, address adapter, uint8 phase, uint256 priority) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitOracleAdapterLifecycle(sourceId [32]byte, adapter common.Address, phase uint8, priority *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitOracleAdapterLifecycle(&_EventEmitter.TransactOpts, sourceId, adapter, phase, priority)
}

// EmitPauseToggle is a paid mutator transaction binding the contract method 0xd4925be5.
//
// Solidity: function emitPauseToggle(address pausedContract, bool paused) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPauseToggle(opts *bind.TransactOpts, pausedContract common.Address, paused bool) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPauseToggle", pausedContract, paused)
}

// EmitPauseToggle is a paid mutator transaction binding the contract method 0xd4925be5.
//
// Solidity: function emitPauseToggle(address pausedContract, bool paused) returns()
func (_EventEmitter *EventEmitterSession) EmitPauseToggle(pausedContract common.Address, paused bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPauseToggle(&_EventEmitter.TransactOpts, pausedContract, paused)
}

// EmitPauseToggle is a paid mutator transaction binding the contract method 0xd4925be5.
//
// Solidity: function emitPauseToggle(address pausedContract, bool paused) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPauseToggle(pausedContract common.Address, paused bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPauseToggle(&_EventEmitter.TransactOpts, pausedContract, paused)
}

// EmitPecorOrderCreated is a paid mutator transaction binding the contract method 0x7944994b.
//
// Solidity: function emitPecorOrderCreated(uint256 orderId, address user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPecorOrderCreated(opts *bind.TransactOpts, orderId *big.Int, user common.Address, orderKind uint8, orderType uint8, tokenIn common.Address, tokenOut common.Address, amount *big.Int, targetPrice *big.Int, stopPrice *big.Int, limitPrice *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPecorOrderCreated", orderId, user, orderKind, orderType, tokenIn, tokenOut, amount, targetPrice, stopPrice, limitPrice)
}

// EmitPecorOrderCreated is a paid mutator transaction binding the contract method 0x7944994b.
//
// Solidity: function emitPecorOrderCreated(uint256 orderId, address user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice) returns()
func (_EventEmitter *EventEmitterSession) EmitPecorOrderCreated(orderId *big.Int, user common.Address, orderKind uint8, orderType uint8, tokenIn common.Address, tokenOut common.Address, amount *big.Int, targetPrice *big.Int, stopPrice *big.Int, limitPrice *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPecorOrderCreated(&_EventEmitter.TransactOpts, orderId, user, orderKind, orderType, tokenIn, tokenOut, amount, targetPrice, stopPrice, limitPrice)
}

// EmitPecorOrderCreated is a paid mutator transaction binding the contract method 0x7944994b.
//
// Solidity: function emitPecorOrderCreated(uint256 orderId, address user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPecorOrderCreated(orderId *big.Int, user common.Address, orderKind uint8, orderType uint8, tokenIn common.Address, tokenOut common.Address, amount *big.Int, targetPrice *big.Int, stopPrice *big.Int, limitPrice *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPecorOrderCreated(&_EventEmitter.TransactOpts, orderId, user, orderKind, orderType, tokenIn, tokenOut, amount, targetPrice, stopPrice, limitPrice)
}

// EmitPecorOrderLifecycle is a paid mutator transaction binding the contract method 0x1a7a34ef.
//
// Solidity: function emitPecorOrderLifecycle(uint256 orderId, address user, uint8 orderKind, uint8 phase, uint256 price, bytes payload) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPecorOrderLifecycle(opts *bind.TransactOpts, orderId *big.Int, user common.Address, orderKind uint8, phase uint8, price *big.Int, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPecorOrderLifecycle", orderId, user, orderKind, phase, price, payload)
}

// EmitPecorOrderLifecycle is a paid mutator transaction binding the contract method 0x1a7a34ef.
//
// Solidity: function emitPecorOrderLifecycle(uint256 orderId, address user, uint8 orderKind, uint8 phase, uint256 price, bytes payload) returns()
func (_EventEmitter *EventEmitterSession) EmitPecorOrderLifecycle(orderId *big.Int, user common.Address, orderKind uint8, phase uint8, price *big.Int, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPecorOrderLifecycle(&_EventEmitter.TransactOpts, orderId, user, orderKind, phase, price, payload)
}

// EmitPecorOrderLifecycle is a paid mutator transaction binding the contract method 0x1a7a34ef.
//
// Solidity: function emitPecorOrderLifecycle(uint256 orderId, address user, uint8 orderKind, uint8 phase, uint256 price, bytes payload) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPecorOrderLifecycle(orderId *big.Int, user common.Address, orderKind uint8, phase uint8, price *big.Int, payload []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPecorOrderLifecycle(&_EventEmitter.TransactOpts, orderId, user, orderKind, phase, price, payload)
}

// EmitPecorSwap is a paid mutator transaction binding the contract method 0xa6df9eea.
//
// Solidity: function emitPecorSwap(address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPecorSwap(opts *bind.TransactOpts, user common.Address, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, priceIn *big.Int, priceOut *big.Int, volumeUSD *big.Int, feeBps *big.Int, feeAmount *big.Int, impactBps *big.Int, swapKind uint8) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPecorSwap", user, tokenIn, tokenOut, amountIn, amountOut, priceIn, priceOut, volumeUSD, feeBps, feeAmount, impactBps, swapKind)
}

// EmitPecorSwap is a paid mutator transaction binding the contract method 0xa6df9eea.
//
// Solidity: function emitPecorSwap(address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind) returns()
func (_EventEmitter *EventEmitterSession) EmitPecorSwap(user common.Address, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, priceIn *big.Int, priceOut *big.Int, volumeUSD *big.Int, feeBps *big.Int, feeAmount *big.Int, impactBps *big.Int, swapKind uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPecorSwap(&_EventEmitter.TransactOpts, user, tokenIn, tokenOut, amountIn, amountOut, priceIn, priceOut, volumeUSD, feeBps, feeAmount, impactBps, swapKind)
}

// EmitPecorSwap is a paid mutator transaction binding the contract method 0xa6df9eea.
//
// Solidity: function emitPecorSwap(address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPecorSwap(user common.Address, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, priceIn *big.Int, priceOut *big.Int, volumeUSD *big.Int, feeBps *big.Int, feeAmount *big.Int, impactBps *big.Int, swapKind uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPecorSwap(&_EventEmitter.TransactOpts, user, tokenIn, tokenOut, amountIn, amountOut, priceIn, priceOut, volumeUSD, feeBps, feeAmount, impactBps, swapKind)
}

// EmitPoolRegistered is a paid mutator transaction binding the contract method 0x25b14c33.
//
// Solidity: function emitPoolRegistered(address pool, address token, address creator, address optical, uint256 nftId) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPoolRegistered(opts *bind.TransactOpts, pool common.Address, token common.Address, creator common.Address, optical common.Address, nftId *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPoolRegistered", pool, token, creator, optical, nftId)
}

// EmitPoolRegistered is a paid mutator transaction binding the contract method 0x25b14c33.
//
// Solidity: function emitPoolRegistered(address pool, address token, address creator, address optical, uint256 nftId) returns()
func (_EventEmitter *EventEmitterSession) EmitPoolRegistered(pool common.Address, token common.Address, creator common.Address, optical common.Address, nftId *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPoolRegistered(&_EventEmitter.TransactOpts, pool, token, creator, optical, nftId)
}

// EmitPoolRegistered is a paid mutator transaction binding the contract method 0x25b14c33.
//
// Solidity: function emitPoolRegistered(address pool, address token, address creator, address optical, uint256 nftId) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPoolRegistered(pool common.Address, token common.Address, creator common.Address, optical common.Address, nftId *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPoolRegistered(&_EventEmitter.TransactOpts, pool, token, creator, optical, nftId)
}

// EmitPoolStateUpdated is a paid mutator transaction binding the contract method 0xfa78f4ca.
//
// Solidity: function emitPoolStateUpdated(bytes32 poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPoolStateUpdated(opts *bind.TransactOpts, poolId [32]byte, virtualReserve *big.Int, realReserve *big.Int, tokenReserve *big.Int, price *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPoolStateUpdated", poolId, virtualReserve, realReserve, tokenReserve, price)
}

// EmitPoolStateUpdated is a paid mutator transaction binding the contract method 0xfa78f4ca.
//
// Solidity: function emitPoolStateUpdated(bytes32 poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price) returns()
func (_EventEmitter *EventEmitterSession) EmitPoolStateUpdated(poolId [32]byte, virtualReserve *big.Int, realReserve *big.Int, tokenReserve *big.Int, price *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPoolStateUpdated(&_EventEmitter.TransactOpts, poolId, virtualReserve, realReserve, tokenReserve, price)
}

// EmitPoolStateUpdated is a paid mutator transaction binding the contract method 0xfa78f4ca.
//
// Solidity: function emitPoolStateUpdated(bytes32 poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPoolStateUpdated(poolId [32]byte, virtualReserve *big.Int, realReserve *big.Int, tokenReserve *big.Int, price *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPoolStateUpdated(&_EventEmitter.TransactOpts, poolId, virtualReserve, realReserve, tokenReserve, price)
}

// EmitPriceUpdated is a paid mutator transaction binding the contract method 0xd08aa55f.
//
// Solidity: function emitPriceUpdated(address token, uint256 roundId, address relayer, uint256 price, uint256 confidence, bytes32 sourceId) returns()
func (_EventEmitter *EventEmitterTransactor) EmitPriceUpdated(opts *bind.TransactOpts, token common.Address, roundId *big.Int, relayer common.Address, price *big.Int, confidence *big.Int, sourceId [32]byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitPriceUpdated", token, roundId, relayer, price, confidence, sourceId)
}

// EmitPriceUpdated is a paid mutator transaction binding the contract method 0xd08aa55f.
//
// Solidity: function emitPriceUpdated(address token, uint256 roundId, address relayer, uint256 price, uint256 confidence, bytes32 sourceId) returns()
func (_EventEmitter *EventEmitterSession) EmitPriceUpdated(token common.Address, roundId *big.Int, relayer common.Address, price *big.Int, confidence *big.Int, sourceId [32]byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPriceUpdated(&_EventEmitter.TransactOpts, token, roundId, relayer, price, confidence, sourceId)
}

// EmitPriceUpdated is a paid mutator transaction binding the contract method 0xd08aa55f.
//
// Solidity: function emitPriceUpdated(address token, uint256 roundId, address relayer, uint256 price, uint256 confidence, bytes32 sourceId) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitPriceUpdated(token common.Address, roundId *big.Int, relayer common.Address, price *big.Int, confidence *big.Int, sourceId [32]byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitPriceUpdated(&_EventEmitter.TransactOpts, token, roundId, relayer, price, confidence, sourceId)
}

// EmitRoleChange is a paid mutator transaction binding the contract method 0xcdc0fc61.
//
// Solidity: function emitRoleChange(uint8 action, bytes32 role, address account, address sender, bytes32 previousAdminRole) returns()
func (_EventEmitter *EventEmitterTransactor) EmitRoleChange(opts *bind.TransactOpts, action uint8, role [32]byte, account common.Address, sender common.Address, previousAdminRole [32]byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitRoleChange", action, role, account, sender, previousAdminRole)
}

// EmitRoleChange is a paid mutator transaction binding the contract method 0xcdc0fc61.
//
// Solidity: function emitRoleChange(uint8 action, bytes32 role, address account, address sender, bytes32 previousAdminRole) returns()
func (_EventEmitter *EventEmitterSession) EmitRoleChange(action uint8, role [32]byte, account common.Address, sender common.Address, previousAdminRole [32]byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitRoleChange(&_EventEmitter.TransactOpts, action, role, account, sender, previousAdminRole)
}

// EmitRoleChange is a paid mutator transaction binding the contract method 0xcdc0fc61.
//
// Solidity: function emitRoleChange(uint8 action, bytes32 role, address account, address sender, bytes32 previousAdminRole) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitRoleChange(action uint8, role [32]byte, account common.Address, sender common.Address, previousAdminRole [32]byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitRoleChange(&_EventEmitter.TransactOpts, action, role, account, sender, previousAdminRole)
}

// EmitRouterTrade is a paid mutator transaction binding the contract method 0xe3cdc125.
//
// Solidity: function emitRouterTrade(uint8 kind, address pool, address sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl) returns()
func (_EventEmitter *EventEmitterTransactor) EmitRouterTrade(opts *bind.TransactOpts, kind uint8, pool common.Address, sender common.Address, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, intermediateUsdl *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitRouterTrade", kind, pool, sender, tokenIn, tokenOut, amountIn, amountOut, intermediateUsdl)
}

// EmitRouterTrade is a paid mutator transaction binding the contract method 0xe3cdc125.
//
// Solidity: function emitRouterTrade(uint8 kind, address pool, address sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl) returns()
func (_EventEmitter *EventEmitterSession) EmitRouterTrade(kind uint8, pool common.Address, sender common.Address, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, intermediateUsdl *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitRouterTrade(&_EventEmitter.TransactOpts, kind, pool, sender, tokenIn, tokenOut, amountIn, amountOut, intermediateUsdl)
}

// EmitRouterTrade is a paid mutator transaction binding the contract method 0xe3cdc125.
//
// Solidity: function emitRouterTrade(uint8 kind, address pool, address sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitRouterTrade(kind uint8, pool common.Address, sender common.Address, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, intermediateUsdl *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitRouterTrade(&_EventEmitter.TransactOpts, kind, pool, sender, tokenIn, tokenOut, amountIn, amountOut, intermediateUsdl)
}

// EmitSwap is a paid mutator transaction binding the contract method 0x100fca4f.
//
// Solidity: function emitSwap(bytes32 poolId, address sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price) returns()
func (_EventEmitter *EventEmitterTransactor) EmitSwap(opts *bind.TransactOpts, poolId [32]byte, sender common.Address, isBuy bool, amountIn *big.Int, amountOut *big.Int, fee *big.Int, price *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitSwap", poolId, sender, isBuy, amountIn, amountOut, fee, price)
}

// EmitSwap is a paid mutator transaction binding the contract method 0x100fca4f.
//
// Solidity: function emitSwap(bytes32 poolId, address sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price) returns()
func (_EventEmitter *EventEmitterSession) EmitSwap(poolId [32]byte, sender common.Address, isBuy bool, amountIn *big.Int, amountOut *big.Int, fee *big.Int, price *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitSwap(&_EventEmitter.TransactOpts, poolId, sender, isBuy, amountIn, amountOut, fee, price)
}

// EmitSwap is a paid mutator transaction binding the contract method 0x100fca4f.
//
// Solidity: function emitSwap(bytes32 poolId, address sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitSwap(poolId [32]byte, sender common.Address, isBuy bool, amountIn *big.Int, amountOut *big.Int, fee *big.Int, price *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitSwap(&_EventEmitter.TransactOpts, poolId, sender, isBuy, amountIn, amountOut, fee, price)
}

// EmitTokenDeployed is a paid mutator transaction binding the contract method 0x3cced653.
//
// Solidity: function emitTokenDeployed(address token, address pool, address creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply) returns()
func (_EventEmitter *EventEmitterTransactor) EmitTokenDeployed(opts *bind.TransactOpts, token common.Address, pool common.Address, creator common.Address, salt [32]byte, name string, symbol string, decimals uint8, totalSupply *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitTokenDeployed", token, pool, creator, salt, name, symbol, decimals, totalSupply)
}

// EmitTokenDeployed is a paid mutator transaction binding the contract method 0x3cced653.
//
// Solidity: function emitTokenDeployed(address token, address pool, address creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply) returns()
func (_EventEmitter *EventEmitterSession) EmitTokenDeployed(token common.Address, pool common.Address, creator common.Address, salt [32]byte, name string, symbol string, decimals uint8, totalSupply *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitTokenDeployed(&_EventEmitter.TransactOpts, token, pool, creator, salt, name, symbol, decimals, totalSupply)
}

// EmitTokenDeployed is a paid mutator transaction binding the contract method 0x3cced653.
//
// Solidity: function emitTokenDeployed(address token, address pool, address creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitTokenDeployed(token common.Address, pool common.Address, creator common.Address, salt [32]byte, name string, symbol string, decimals uint8, totalSupply *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitTokenDeployed(&_EventEmitter.TransactOpts, token, pool, creator, salt, name, symbol, decimals, totalSupply)
}

// EmitTokenTransfer is a paid mutator transaction binding the contract method 0xc6770db6.
//
// Solidity: function emitTokenTransfer(address token, address from, address to, uint256 value) returns()
func (_EventEmitter *EventEmitterTransactor) EmitTokenTransfer(opts *bind.TransactOpts, token common.Address, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitTokenTransfer", token, from, to, value)
}

// EmitTokenTransfer is a paid mutator transaction binding the contract method 0xc6770db6.
//
// Solidity: function emitTokenTransfer(address token, address from, address to, uint256 value) returns()
func (_EventEmitter *EventEmitterSession) EmitTokenTransfer(token common.Address, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitTokenTransfer(&_EventEmitter.TransactOpts, token, from, to, value)
}

// EmitTokenTransfer is a paid mutator transaction binding the contract method 0xc6770db6.
//
// Solidity: function emitTokenTransfer(address token, address from, address to, uint256 value) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitTokenTransfer(token common.Address, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitTokenTransfer(&_EventEmitter.TransactOpts, token, from, to, value)
}

// EmitTreasuryFlow is a paid mutator transaction binding the contract method 0x9de5933b.
//
// Solidity: function emitTreasuryFlow(uint8 direction, address token, address party, uint256 amount) returns()
func (_EventEmitter *EventEmitterTransactor) EmitTreasuryFlow(opts *bind.TransactOpts, direction uint8, token common.Address, party common.Address, amount *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitTreasuryFlow", direction, token, party, amount)
}

// EmitTreasuryFlow is a paid mutator transaction binding the contract method 0x9de5933b.
//
// Solidity: function emitTreasuryFlow(uint8 direction, address token, address party, uint256 amount) returns()
func (_EventEmitter *EventEmitterSession) EmitTreasuryFlow(direction uint8, token common.Address, party common.Address, amount *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitTreasuryFlow(&_EventEmitter.TransactOpts, direction, token, party, amount)
}

// EmitTreasuryFlow is a paid mutator transaction binding the contract method 0x9de5933b.
//
// Solidity: function emitTreasuryFlow(uint8 direction, address token, address party, uint256 amount) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitTreasuryFlow(direction uint8, token common.Address, party common.Address, amount *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitTreasuryFlow(&_EventEmitter.TransactOpts, direction, token, party, amount)
}

// EmitUpgraded is a paid mutator transaction binding the contract method 0x37114247.
//
// Solidity: function emitUpgraded(address proxy, address newImplementation, uint8 kind) returns()
func (_EventEmitter *EventEmitterTransactor) EmitUpgraded(opts *bind.TransactOpts, proxy common.Address, newImplementation common.Address, kind uint8) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitUpgraded", proxy, newImplementation, kind)
}

// EmitUpgraded is a paid mutator transaction binding the contract method 0x37114247.
//
// Solidity: function emitUpgraded(address proxy, address newImplementation, uint8 kind) returns()
func (_EventEmitter *EventEmitterSession) EmitUpgraded(proxy common.Address, newImplementation common.Address, kind uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitUpgraded(&_EventEmitter.TransactOpts, proxy, newImplementation, kind)
}

// EmitUpgraded is a paid mutator transaction binding the contract method 0x37114247.
//
// Solidity: function emitUpgraded(address proxy, address newImplementation, uint8 kind) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitUpgraded(proxy common.Address, newImplementation common.Address, kind uint8) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitUpgraded(&_EventEmitter.TransactOpts, proxy, newImplementation, kind)
}

// EmitVaultFlow is a paid mutator transaction binding the contract method 0x9e4a40ca.
//
// Solidity: function emitVaultFlow(uint8 flowType, address token, address party, uint256 amount, uint256 newReserve) returns()
func (_EventEmitter *EventEmitterTransactor) EmitVaultFlow(opts *bind.TransactOpts, flowType uint8, token common.Address, party common.Address, amount *big.Int, newReserve *big.Int) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "emitVaultFlow", flowType, token, party, amount, newReserve)
}

// EmitVaultFlow is a paid mutator transaction binding the contract method 0x9e4a40ca.
//
// Solidity: function emitVaultFlow(uint8 flowType, address token, address party, uint256 amount, uint256 newReserve) returns()
func (_EventEmitter *EventEmitterSession) EmitVaultFlow(flowType uint8, token common.Address, party common.Address, amount *big.Int, newReserve *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitVaultFlow(&_EventEmitter.TransactOpts, flowType, token, party, amount, newReserve)
}

// EmitVaultFlow is a paid mutator transaction binding the contract method 0x9e4a40ca.
//
// Solidity: function emitVaultFlow(uint8 flowType, address token, address party, uint256 amount, uint256 newReserve) returns()
func (_EventEmitter *EventEmitterTransactorSession) EmitVaultFlow(flowType uint8, token common.Address, party common.Address, amount *big.Int, newReserve *big.Int) (*types.Transaction, error) {
	return _EventEmitter.Contract.EmitVaultFlow(&_EventEmitter.TransactOpts, flowType, token, party, amount, newReserve)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_EventEmitter *EventEmitterTransactor) GrantRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "grantRole", role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_EventEmitter *EventEmitterSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.GrantRole(&_EventEmitter.TransactOpts, role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_EventEmitter *EventEmitterTransactorSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.GrantRole(&_EventEmitter.TransactOpts, role, account)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _admin) returns()
func (_EventEmitter *EventEmitterTransactor) Initialize(opts *bind.TransactOpts, _admin common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "initialize", _admin)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _admin) returns()
func (_EventEmitter *EventEmitterSession) Initialize(_admin common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.Initialize(&_EventEmitter.TransactOpts, _admin)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _admin) returns()
func (_EventEmitter *EventEmitterTransactorSession) Initialize(_admin common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.Initialize(&_EventEmitter.TransactOpts, _admin)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x4739f7e5.
//
// Solidity: function registerToken(address token, address ) returns()
func (_EventEmitter *EventEmitterTransactor) RegisterToken(opts *bind.TransactOpts, token common.Address, arg1 common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "registerToken", token, arg1)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x4739f7e5.
//
// Solidity: function registerToken(address token, address ) returns()
func (_EventEmitter *EventEmitterSession) RegisterToken(token common.Address, arg1 common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.RegisterToken(&_EventEmitter.TransactOpts, token, arg1)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x4739f7e5.
//
// Solidity: function registerToken(address token, address ) returns()
func (_EventEmitter *EventEmitterTransactorSession) RegisterToken(token common.Address, arg1 common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.RegisterToken(&_EventEmitter.TransactOpts, token, arg1)
}

// ReinitializeV2 is a paid mutator transaction binding the contract method 0xbe6b99e3.
//
// Solidity: function reinitializeV2(address adminWithEmitterRole) returns()
func (_EventEmitter *EventEmitterTransactor) ReinitializeV2(opts *bind.TransactOpts, adminWithEmitterRole common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "reinitializeV2", adminWithEmitterRole)
}

// ReinitializeV2 is a paid mutator transaction binding the contract method 0xbe6b99e3.
//
// Solidity: function reinitializeV2(address adminWithEmitterRole) returns()
func (_EventEmitter *EventEmitterSession) ReinitializeV2(adminWithEmitterRole common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.ReinitializeV2(&_EventEmitter.TransactOpts, adminWithEmitterRole)
}

// ReinitializeV2 is a paid mutator transaction binding the contract method 0xbe6b99e3.
//
// Solidity: function reinitializeV2(address adminWithEmitterRole) returns()
func (_EventEmitter *EventEmitterTransactorSession) ReinitializeV2(adminWithEmitterRole common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.ReinitializeV2(&_EventEmitter.TransactOpts, adminWithEmitterRole)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_EventEmitter *EventEmitterTransactor) RenounceRole(opts *bind.TransactOpts, role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "renounceRole", role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_EventEmitter *EventEmitterSession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.RenounceRole(&_EventEmitter.TransactOpts, role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_EventEmitter *EventEmitterTransactorSession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.RenounceRole(&_EventEmitter.TransactOpts, role, callerConfirmation)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_EventEmitter *EventEmitterTransactor) RevokeRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "revokeRole", role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_EventEmitter *EventEmitterSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.RevokeRole(&_EventEmitter.TransactOpts, role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_EventEmitter *EventEmitterTransactorSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.RevokeRole(&_EventEmitter.TransactOpts, role, account)
}

// SetAuthorizedEmitter is a paid mutator transaction binding the contract method 0x27b81718.
//
// Solidity: function setAuthorizedEmitter(address emitter, bool authorized) returns()
func (_EventEmitter *EventEmitterTransactor) SetAuthorizedEmitter(opts *bind.TransactOpts, emitter common.Address, authorized bool) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setAuthorizedEmitter", emitter, authorized)
}

// SetAuthorizedEmitter is a paid mutator transaction binding the contract method 0x27b81718.
//
// Solidity: function setAuthorizedEmitter(address emitter, bool authorized) returns()
func (_EventEmitter *EventEmitterSession) SetAuthorizedEmitter(emitter common.Address, authorized bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetAuthorizedEmitter(&_EventEmitter.TransactOpts, emitter, authorized)
}

// SetAuthorizedEmitter is a paid mutator transaction binding the contract method 0x27b81718.
//
// Solidity: function setAuthorizedEmitter(address emitter, bool authorized) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetAuthorizedEmitter(emitter common.Address, authorized bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetAuthorizedEmitter(&_EventEmitter.TransactOpts, emitter, authorized)
}

// SetMetaAGRouter is a paid mutator transaction binding the contract method 0xf4795b26.
//
// Solidity: function setMetaAGRouter(address router) returns()
func (_EventEmitter *EventEmitterTransactor) SetMetaAGRouter(opts *bind.TransactOpts, router common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setMetaAGRouter", router)
}

// SetMetaAGRouter is a paid mutator transaction binding the contract method 0xf4795b26.
//
// Solidity: function setMetaAGRouter(address router) returns()
func (_EventEmitter *EventEmitterSession) SetMetaAGRouter(router common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetMetaAGRouter(&_EventEmitter.TransactOpts, router)
}

// SetMetaAGRouter is a paid mutator transaction binding the contract method 0xf4795b26.
//
// Solidity: function setMetaAGRouter(address router) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetMetaAGRouter(router common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetMetaAGRouter(&_EventEmitter.TransactOpts, router)
}

// SetOpticalRegistry is a paid mutator transaction binding the contract method 0xde2b7764.
//
// Solidity: function setOpticalRegistry(address registry) returns()
func (_EventEmitter *EventEmitterTransactor) SetOpticalRegistry(opts *bind.TransactOpts, registry common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setOpticalRegistry", registry)
}

// SetOpticalRegistry is a paid mutator transaction binding the contract method 0xde2b7764.
//
// Solidity: function setOpticalRegistry(address registry) returns()
func (_EventEmitter *EventEmitterSession) SetOpticalRegistry(registry common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetOpticalRegistry(&_EventEmitter.TransactOpts, registry)
}

// SetOpticalRegistry is a paid mutator transaction binding the contract method 0xde2b7764.
//
// Solidity: function setOpticalRegistry(address registry) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetOpticalRegistry(registry common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetOpticalRegistry(&_EventEmitter.TransactOpts, registry)
}

// SetPoolRegistry is a paid mutator transaction binding the contract method 0x7b77cd6a.
//
// Solidity: function setPoolRegistry(address _poolRegistry) returns()
func (_EventEmitter *EventEmitterTransactor) SetPoolRegistry(opts *bind.TransactOpts, _poolRegistry common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setPoolRegistry", _poolRegistry)
}

// SetPoolRegistry is a paid mutator transaction binding the contract method 0x7b77cd6a.
//
// Solidity: function setPoolRegistry(address _poolRegistry) returns()
func (_EventEmitter *EventEmitterSession) SetPoolRegistry(_poolRegistry common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetPoolRegistry(&_EventEmitter.TransactOpts, _poolRegistry)
}

// SetPoolRegistry is a paid mutator transaction binding the contract method 0x7b77cd6a.
//
// Solidity: function setPoolRegistry(address _poolRegistry) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetPoolRegistry(_poolRegistry common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetPoolRegistry(&_EventEmitter.TransactOpts, _poolRegistry)
}

// SetSidioraFactory is a paid mutator transaction binding the contract method 0xe53122ec.
//
// Solidity: function setSidioraFactory(address factory) returns()
func (_EventEmitter *EventEmitterTransactor) SetSidioraFactory(opts *bind.TransactOpts, factory common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setSidioraFactory", factory)
}

// SetSidioraFactory is a paid mutator transaction binding the contract method 0xe53122ec.
//
// Solidity: function setSidioraFactory(address factory) returns()
func (_EventEmitter *EventEmitterSession) SetSidioraFactory(factory common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetSidioraFactory(&_EventEmitter.TransactOpts, factory)
}

// SetSidioraFactory is a paid mutator transaction binding the contract method 0xe53122ec.
//
// Solidity: function setSidioraFactory(address factory) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetSidioraFactory(factory common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetSidioraFactory(&_EventEmitter.TransactOpts, factory)
}

// SetTokenRegistry is a paid mutator transaction binding the contract method 0x35a5af92.
//
// Solidity: function setTokenRegistry(address ) returns()
func (_EventEmitter *EventEmitterTransactor) SetTokenRegistry(opts *bind.TransactOpts, arg0 common.Address) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setTokenRegistry", arg0)
}

// SetTokenRegistry is a paid mutator transaction binding the contract method 0x35a5af92.
//
// Solidity: function setTokenRegistry(address ) returns()
func (_EventEmitter *EventEmitterSession) SetTokenRegistry(arg0 common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetTokenRegistry(&_EventEmitter.TransactOpts, arg0)
}

// SetTokenRegistry is a paid mutator transaction binding the contract method 0x35a5af92.
//
// Solidity: function setTokenRegistry(address ) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetTokenRegistry(arg0 common.Address) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetTokenRegistry(&_EventEmitter.TransactOpts, arg0)
}

// SetTypedEmitter is a paid mutator transaction binding the contract method 0xb2473503.
//
// Solidity: function setTypedEmitter(address emitter, bytes4 selector, bool allowed) returns()
func (_EventEmitter *EventEmitterTransactor) SetTypedEmitter(opts *bind.TransactOpts, emitter common.Address, selector [4]byte, allowed bool) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "setTypedEmitter", emitter, selector, allowed)
}

// SetTypedEmitter is a paid mutator transaction binding the contract method 0xb2473503.
//
// Solidity: function setTypedEmitter(address emitter, bytes4 selector, bool allowed) returns()
func (_EventEmitter *EventEmitterSession) SetTypedEmitter(emitter common.Address, selector [4]byte, allowed bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetTypedEmitter(&_EventEmitter.TransactOpts, emitter, selector, allowed)
}

// SetTypedEmitter is a paid mutator transaction binding the contract method 0xb2473503.
//
// Solidity: function setTypedEmitter(address emitter, bytes4 selector, bool allowed) returns()
func (_EventEmitter *EventEmitterTransactorSession) SetTypedEmitter(emitter common.Address, selector [4]byte, allowed bool) (*types.Transaction, error) {
	return _EventEmitter.Contract.SetTypedEmitter(&_EventEmitter.TransactOpts, emitter, selector, allowed)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_EventEmitter *EventEmitterTransactor) UpgradeToAndCall(opts *bind.TransactOpts, newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _EventEmitter.contract.Transact(opts, "upgradeToAndCall", newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_EventEmitter *EventEmitterSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.UpgradeToAndCall(&_EventEmitter.TransactOpts, newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_EventEmitter *EventEmitterTransactorSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _EventEmitter.Contract.UpgradeToAndCall(&_EventEmitter.TransactOpts, newImplementation, data)
}

// EventEmitterAssetApprovalIterator is returned from FilterAssetApproval and is used to iterate over the raw logs and unpacked data for AssetApproval events raised by the EventEmitter contract.
type EventEmitterAssetApprovalIterator struct {
	Event *EventEmitterAssetApproval // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterAssetApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterAssetApproval)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterAssetApproval)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterAssetApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterAssetApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterAssetApproval represents a AssetApproval event raised by the EventEmitter contract.
type EventEmitterAssetApproval struct {
	Asset          common.Address
	Owner          common.Address
	Spender        common.Address
	ValueOrTokenId *big.Int
	IsNft          bool
	Timestamp      *big.Int
	BlockNumber    *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterAssetApproval is a free log retrieval operation binding the contract event 0xa9de58179a7ba626557514c856f27ad734eea61c56c3e9acb937f374ec63afe7.
//
// Solidity: event AssetApproval(address indexed asset, address indexed owner, address indexed spender, uint256 valueOrTokenId, bool isNft, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterAssetApproval(opts *bind.FilterOpts, asset []common.Address, owner []common.Address, spender []common.Address) (*EventEmitterAssetApprovalIterator, error) {

	var assetRule []interface{}
	for _, assetItem := range asset {
		assetRule = append(assetRule, assetItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "AssetApproval", assetRule, ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterAssetApprovalIterator{contract: _EventEmitter.contract, event: "AssetApproval", logs: logs, sub: sub}, nil
}

// WatchAssetApproval is a free log subscription operation binding the contract event 0xa9de58179a7ba626557514c856f27ad734eea61c56c3e9acb937f374ec63afe7.
//
// Solidity: event AssetApproval(address indexed asset, address indexed owner, address indexed spender, uint256 valueOrTokenId, bool isNft, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchAssetApproval(opts *bind.WatchOpts, sink chan<- *EventEmitterAssetApproval, asset []common.Address, owner []common.Address, spender []common.Address) (event.Subscription, error) {

	var assetRule []interface{}
	for _, assetItem := range asset {
		assetRule = append(assetRule, assetItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "AssetApproval", assetRule, ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterAssetApproval)
				if err := _EventEmitter.contract.UnpackLog(event, "AssetApproval", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAssetApproval is a log parse operation binding the contract event 0xa9de58179a7ba626557514c856f27ad734eea61c56c3e9acb937f374ec63afe7.
//
// Solidity: event AssetApproval(address indexed asset, address indexed owner, address indexed spender, uint256 valueOrTokenId, bool isNft, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseAssetApproval(log types.Log) (*EventEmitterAssetApproval, error) {
	event := new(EventEmitterAssetApproval)
	if err := _EventEmitter.contract.UnpackLog(event, "AssetApproval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterBestRouteSwapIterator is returned from FilterBestRouteSwap and is used to iterate over the raw logs and unpacked data for BestRouteSwap events raised by the EventEmitter contract.
type EventEmitterBestRouteSwapIterator struct {
	Event *EventEmitterBestRouteSwap // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterBestRouteSwapIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterBestRouteSwap)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterBestRouteSwap)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterBestRouteSwapIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterBestRouteSwapIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterBestRouteSwap represents a BestRouteSwap event raised by the EventEmitter contract.
type EventEmitterBestRouteSwap struct {
	User           common.Address
	TokenIn        common.Address
	TokenOut       common.Address
	RouteId        [32]byte
	AmountIn       *big.Int
	AmountOut      *big.Int
	Hops           []common.Address
	ProtocolFeeBps *big.Int
	Timestamp      *big.Int
	BlockNumber    *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterBestRouteSwap is a free log retrieval operation binding the contract event 0xf5041a4d1e1f497822428a20e208ea0ad2c90eb99518d0d9a1f598f8af04cd72.
//
// Solidity: event BestRouteSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterBestRouteSwap(opts *bind.FilterOpts, user []common.Address, tokenIn []common.Address, tokenOut []common.Address) (*EventEmitterBestRouteSwapIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var tokenInRule []interface{}
	for _, tokenInItem := range tokenIn {
		tokenInRule = append(tokenInRule, tokenInItem)
	}
	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "BestRouteSwap", userRule, tokenInRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterBestRouteSwapIterator{contract: _EventEmitter.contract, event: "BestRouteSwap", logs: logs, sub: sub}, nil
}

// WatchBestRouteSwap is a free log subscription operation binding the contract event 0xf5041a4d1e1f497822428a20e208ea0ad2c90eb99518d0d9a1f598f8af04cd72.
//
// Solidity: event BestRouteSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchBestRouteSwap(opts *bind.WatchOpts, sink chan<- *EventEmitterBestRouteSwap, user []common.Address, tokenIn []common.Address, tokenOut []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var tokenInRule []interface{}
	for _, tokenInItem := range tokenIn {
		tokenInRule = append(tokenInRule, tokenInItem)
	}
	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "BestRouteSwap", userRule, tokenInRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterBestRouteSwap)
				if err := _EventEmitter.contract.UnpackLog(event, "BestRouteSwap", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseBestRouteSwap is a log parse operation binding the contract event 0xf5041a4d1e1f497822428a20e208ea0ad2c90eb99518d0d9a1f598f8af04cd72.
//
// Solidity: event BestRouteSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseBestRouteSwap(log types.Log) (*EventEmitterBestRouteSwap, error) {
	event := new(EventEmitterBestRouteSwap)
	if err := _EventEmitter.contract.UnpackLog(event, "BestRouteSwap", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterCircuitBreakerTriggeredIterator is returned from FilterCircuitBreakerTriggered and is used to iterate over the raw logs and unpacked data for CircuitBreakerTriggered events raised by the EventEmitter contract.
type EventEmitterCircuitBreakerTriggeredIterator struct {
	Event *EventEmitterCircuitBreakerTriggered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterCircuitBreakerTriggeredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterCircuitBreakerTriggered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterCircuitBreakerTriggered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterCircuitBreakerTriggeredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterCircuitBreakerTriggeredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterCircuitBreakerTriggered represents a CircuitBreakerTriggered event raised by the EventEmitter contract.
type EventEmitterCircuitBreakerTriggered struct {
	Token          common.Address
	SourceId       [32]byte
	ReportedPrice  *big.Int
	ReferencePrice *big.Int
	DeviationBps   *big.Int
	Timestamp      *big.Int
	BlockNumber    *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterCircuitBreakerTriggered is a free log retrieval operation binding the contract event 0xb08f12d79c5828345938b295337ebad6aa44441a33df36e9f766b0b7f253889b.
//
// Solidity: event CircuitBreakerTriggered(address indexed token, bytes32 indexed sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterCircuitBreakerTriggered(opts *bind.FilterOpts, token []common.Address, sourceId [][32]byte) (*EventEmitterCircuitBreakerTriggeredIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var sourceIdRule []interface{}
	for _, sourceIdItem := range sourceId {
		sourceIdRule = append(sourceIdRule, sourceIdItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "CircuitBreakerTriggered", tokenRule, sourceIdRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterCircuitBreakerTriggeredIterator{contract: _EventEmitter.contract, event: "CircuitBreakerTriggered", logs: logs, sub: sub}, nil
}

// WatchCircuitBreakerTriggered is a free log subscription operation binding the contract event 0xb08f12d79c5828345938b295337ebad6aa44441a33df36e9f766b0b7f253889b.
//
// Solidity: event CircuitBreakerTriggered(address indexed token, bytes32 indexed sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchCircuitBreakerTriggered(opts *bind.WatchOpts, sink chan<- *EventEmitterCircuitBreakerTriggered, token []common.Address, sourceId [][32]byte) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var sourceIdRule []interface{}
	for _, sourceIdItem := range sourceId {
		sourceIdRule = append(sourceIdRule, sourceIdItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "CircuitBreakerTriggered", tokenRule, sourceIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterCircuitBreakerTriggered)
				if err := _EventEmitter.contract.UnpackLog(event, "CircuitBreakerTriggered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCircuitBreakerTriggered is a log parse operation binding the contract event 0xb08f12d79c5828345938b295337ebad6aa44441a33df36e9f766b0b7f253889b.
//
// Solidity: event CircuitBreakerTriggered(address indexed token, bytes32 indexed sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseCircuitBreakerTriggered(log types.Log) (*EventEmitterCircuitBreakerTriggered, error) {
	event := new(EventEmitterCircuitBreakerTriggered)
	if err := _EventEmitter.contract.UnpackLog(event, "CircuitBreakerTriggered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterConfigUpdatedIterator is returned from FilterConfigUpdated and is used to iterate over the raw logs and unpacked data for ConfigUpdated events raised by the EventEmitter contract.
type EventEmitterConfigUpdatedIterator struct {
	Event *EventEmitterConfigUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterConfigUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterConfigUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterConfigUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterConfigUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterConfigUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterConfigUpdated represents a ConfigUpdated event raised by the EventEmitter contract.
type EventEmitterConfigUpdated struct {
	Key         [32]byte
	OldValue    *big.Int
	NewValue    *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterConfigUpdated is a free log retrieval operation binding the contract event 0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e.
//
// Solidity: event ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterConfigUpdated(opts *bind.FilterOpts, key [][32]byte) (*EventEmitterConfigUpdatedIterator, error) {

	var keyRule []interface{}
	for _, keyItem := range key {
		keyRule = append(keyRule, keyItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "ConfigUpdated", keyRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterConfigUpdatedIterator{contract: _EventEmitter.contract, event: "ConfigUpdated", logs: logs, sub: sub}, nil
}

// WatchConfigUpdated is a free log subscription operation binding the contract event 0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e.
//
// Solidity: event ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchConfigUpdated(opts *bind.WatchOpts, sink chan<- *EventEmitterConfigUpdated, key [][32]byte) (event.Subscription, error) {

	var keyRule []interface{}
	for _, keyItem := range key {
		keyRule = append(keyRule, keyItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "ConfigUpdated", keyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterConfigUpdated)
				if err := _EventEmitter.contract.UnpackLog(event, "ConfigUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseConfigUpdated is a log parse operation binding the contract event 0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e.
//
// Solidity: event ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseConfigUpdated(log types.Log) (*EventEmitterConfigUpdated, error) {
	event := new(EventEmitterConfigUpdated)
	if err := _EventEmitter.contract.UnpackLog(event, "ConfigUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterContractUpgradedIterator is returned from FilterContractUpgraded and is used to iterate over the raw logs and unpacked data for ContractUpgraded events raised by the EventEmitter contract.
type EventEmitterContractUpgradedIterator struct {
	Event *EventEmitterContractUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterContractUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterContractUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterContractUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterContractUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterContractUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterContractUpgraded represents a ContractUpgraded event raised by the EventEmitter contract.
type EventEmitterContractUpgraded struct {
	Proxy             common.Address
	NewImplementation common.Address
	Kind              uint8
	Timestamp         *big.Int
	BlockNumber       *big.Int
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterContractUpgraded is a free log retrieval operation binding the contract event 0xb059389b6a1d06bc1833ca4b78798c2642d02b2d03d0f8fce5f4b8299737d3b7.
//
// Solidity: event ContractUpgraded(address indexed proxy, address indexed newImplementation, uint8 kind, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterContractUpgraded(opts *bind.FilterOpts, proxy []common.Address, newImplementation []common.Address) (*EventEmitterContractUpgradedIterator, error) {

	var proxyRule []interface{}
	for _, proxyItem := range proxy {
		proxyRule = append(proxyRule, proxyItem)
	}
	var newImplementationRule []interface{}
	for _, newImplementationItem := range newImplementation {
		newImplementationRule = append(newImplementationRule, newImplementationItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "ContractUpgraded", proxyRule, newImplementationRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterContractUpgradedIterator{contract: _EventEmitter.contract, event: "ContractUpgraded", logs: logs, sub: sub}, nil
}

// WatchContractUpgraded is a free log subscription operation binding the contract event 0xb059389b6a1d06bc1833ca4b78798c2642d02b2d03d0f8fce5f4b8299737d3b7.
//
// Solidity: event ContractUpgraded(address indexed proxy, address indexed newImplementation, uint8 kind, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchContractUpgraded(opts *bind.WatchOpts, sink chan<- *EventEmitterContractUpgraded, proxy []common.Address, newImplementation []common.Address) (event.Subscription, error) {

	var proxyRule []interface{}
	for _, proxyItem := range proxy {
		proxyRule = append(proxyRule, proxyItem)
	}
	var newImplementationRule []interface{}
	for _, newImplementationItem := range newImplementation {
		newImplementationRule = append(newImplementationRule, newImplementationItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "ContractUpgraded", proxyRule, newImplementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterContractUpgraded)
				if err := _EventEmitter.contract.UnpackLog(event, "ContractUpgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseContractUpgraded is a log parse operation binding the contract event 0xb059389b6a1d06bc1833ca4b78798c2642d02b2d03d0f8fce5f4b8299737d3b7.
//
// Solidity: event ContractUpgraded(address indexed proxy, address indexed newImplementation, uint8 kind, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseContractUpgraded(log types.Log) (*EventEmitterContractUpgraded, error) {
	event := new(EventEmitterContractUpgraded)
	if err := _EventEmitter.contract.UnpackLog(event, "ContractUpgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterEventLogIterator is returned from FilterEventLog and is used to iterate over the raw logs and unpacked data for EventLog events raised by the EventEmitter contract.
type EventEmitterEventLogIterator struct {
	Event *EventEmitterEventLog // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterEventLogIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterEventLog)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterEventLog)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterEventLogIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterEventLogIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterEventLog represents a EventLog event raised by the EventEmitter contract.
type EventEmitterEventLog struct {
	MsgSender     common.Address
	EventNameHash [32]byte
	EventName     string
	EventData     IEventEmitterEventData
	Timestamp     *big.Int
	BlockNumber   *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterEventLog is a free log retrieval operation binding the contract event 0x3f5c3dc69ea622ef6e129f03f630c1484849db8ae3e8f3d7fa164ca7fae08c27.
//
// Solidity: event EventLog(address indexed msgSender, bytes32 indexed eventNameHash, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterEventLog(opts *bind.FilterOpts, msgSender []common.Address, eventNameHash [][32]byte) (*EventEmitterEventLogIterator, error) {

	var msgSenderRule []interface{}
	for _, msgSenderItem := range msgSender {
		msgSenderRule = append(msgSenderRule, msgSenderItem)
	}
	var eventNameHashRule []interface{}
	for _, eventNameHashItem := range eventNameHash {
		eventNameHashRule = append(eventNameHashRule, eventNameHashItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "EventLog", msgSenderRule, eventNameHashRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterEventLogIterator{contract: _EventEmitter.contract, event: "EventLog", logs: logs, sub: sub}, nil
}

// WatchEventLog is a free log subscription operation binding the contract event 0x3f5c3dc69ea622ef6e129f03f630c1484849db8ae3e8f3d7fa164ca7fae08c27.
//
// Solidity: event EventLog(address indexed msgSender, bytes32 indexed eventNameHash, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchEventLog(opts *bind.WatchOpts, sink chan<- *EventEmitterEventLog, msgSender []common.Address, eventNameHash [][32]byte) (event.Subscription, error) {

	var msgSenderRule []interface{}
	for _, msgSenderItem := range msgSender {
		msgSenderRule = append(msgSenderRule, msgSenderItem)
	}
	var eventNameHashRule []interface{}
	for _, eventNameHashItem := range eventNameHash {
		eventNameHashRule = append(eventNameHashRule, eventNameHashItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "EventLog", msgSenderRule, eventNameHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterEventLog)
				if err := _EventEmitter.contract.UnpackLog(event, "EventLog", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEventLog is a log parse operation binding the contract event 0x3f5c3dc69ea622ef6e129f03f630c1484849db8ae3e8f3d7fa164ca7fae08c27.
//
// Solidity: event EventLog(address indexed msgSender, bytes32 indexed eventNameHash, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseEventLog(log types.Log) (*EventEmitterEventLog, error) {
	event := new(EventEmitterEventLog)
	if err := _EventEmitter.contract.UnpackLog(event, "EventLog", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterEventLog1Iterator is returned from FilterEventLog1 and is used to iterate over the raw logs and unpacked data for EventLog1 events raised by the EventEmitter contract.
type EventEmitterEventLog1Iterator struct {
	Event *EventEmitterEventLog1 // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterEventLog1Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterEventLog1)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterEventLog1)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterEventLog1Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterEventLog1Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterEventLog1 represents a EventLog1 event raised by the EventEmitter contract.
type EventEmitterEventLog1 struct {
	MsgSender     common.Address
	EventNameHash [32]byte
	Topic1        [32]byte
	EventName     string
	EventData     IEventEmitterEventData
	Timestamp     *big.Int
	BlockNumber   *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterEventLog1 is a free log retrieval operation binding the contract event 0xe67d7d5cff44135efdb2aa905b4fcd19c700c05482dd08ccb52a0b02b99f788a.
//
// Solidity: event EventLog1(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterEventLog1(opts *bind.FilterOpts, msgSender []common.Address, eventNameHash [][32]byte, topic1 [][32]byte) (*EventEmitterEventLog1Iterator, error) {

	var msgSenderRule []interface{}
	for _, msgSenderItem := range msgSender {
		msgSenderRule = append(msgSenderRule, msgSenderItem)
	}
	var eventNameHashRule []interface{}
	for _, eventNameHashItem := range eventNameHash {
		eventNameHashRule = append(eventNameHashRule, eventNameHashItem)
	}
	var topic1Rule []interface{}
	for _, topic1Item := range topic1 {
		topic1Rule = append(topic1Rule, topic1Item)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "EventLog1", msgSenderRule, eventNameHashRule, topic1Rule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterEventLog1Iterator{contract: _EventEmitter.contract, event: "EventLog1", logs: logs, sub: sub}, nil
}

// WatchEventLog1 is a free log subscription operation binding the contract event 0xe67d7d5cff44135efdb2aa905b4fcd19c700c05482dd08ccb52a0b02b99f788a.
//
// Solidity: event EventLog1(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchEventLog1(opts *bind.WatchOpts, sink chan<- *EventEmitterEventLog1, msgSender []common.Address, eventNameHash [][32]byte, topic1 [][32]byte) (event.Subscription, error) {

	var msgSenderRule []interface{}
	for _, msgSenderItem := range msgSender {
		msgSenderRule = append(msgSenderRule, msgSenderItem)
	}
	var eventNameHashRule []interface{}
	for _, eventNameHashItem := range eventNameHash {
		eventNameHashRule = append(eventNameHashRule, eventNameHashItem)
	}
	var topic1Rule []interface{}
	for _, topic1Item := range topic1 {
		topic1Rule = append(topic1Rule, topic1Item)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "EventLog1", msgSenderRule, eventNameHashRule, topic1Rule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterEventLog1)
				if err := _EventEmitter.contract.UnpackLog(event, "EventLog1", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEventLog1 is a log parse operation binding the contract event 0xe67d7d5cff44135efdb2aa905b4fcd19c700c05482dd08ccb52a0b02b99f788a.
//
// Solidity: event EventLog1(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseEventLog1(log types.Log) (*EventEmitterEventLog1, error) {
	event := new(EventEmitterEventLog1)
	if err := _EventEmitter.contract.UnpackLog(event, "EventLog1", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterEventLog2Iterator is returned from FilterEventLog2 and is used to iterate over the raw logs and unpacked data for EventLog2 events raised by the EventEmitter contract.
type EventEmitterEventLog2Iterator struct {
	Event *EventEmitterEventLog2 // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterEventLog2Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterEventLog2)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterEventLog2)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterEventLog2Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterEventLog2Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterEventLog2 represents a EventLog2 event raised by the EventEmitter contract.
type EventEmitterEventLog2 struct {
	MsgSender     common.Address
	EventNameHash [32]byte
	Topic1        [32]byte
	Topic2        [32]byte
	EventName     string
	EventData     IEventEmitterEventData
	Timestamp     *big.Int
	BlockNumber   *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterEventLog2 is a free log retrieval operation binding the contract event 0xfb0f08ac39934a2d9d9120c47f7079c97667c3a5495f5a6733b46b2c87cd2502.
//
// Solidity: event EventLog2(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, bytes32 topic2, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterEventLog2(opts *bind.FilterOpts, msgSender []common.Address, eventNameHash [][32]byte, topic1 [][32]byte) (*EventEmitterEventLog2Iterator, error) {

	var msgSenderRule []interface{}
	for _, msgSenderItem := range msgSender {
		msgSenderRule = append(msgSenderRule, msgSenderItem)
	}
	var eventNameHashRule []interface{}
	for _, eventNameHashItem := range eventNameHash {
		eventNameHashRule = append(eventNameHashRule, eventNameHashItem)
	}
	var topic1Rule []interface{}
	for _, topic1Item := range topic1 {
		topic1Rule = append(topic1Rule, topic1Item)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "EventLog2", msgSenderRule, eventNameHashRule, topic1Rule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterEventLog2Iterator{contract: _EventEmitter.contract, event: "EventLog2", logs: logs, sub: sub}, nil
}

// WatchEventLog2 is a free log subscription operation binding the contract event 0xfb0f08ac39934a2d9d9120c47f7079c97667c3a5495f5a6733b46b2c87cd2502.
//
// Solidity: event EventLog2(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, bytes32 topic2, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchEventLog2(opts *bind.WatchOpts, sink chan<- *EventEmitterEventLog2, msgSender []common.Address, eventNameHash [][32]byte, topic1 [][32]byte) (event.Subscription, error) {

	var msgSenderRule []interface{}
	for _, msgSenderItem := range msgSender {
		msgSenderRule = append(msgSenderRule, msgSenderItem)
	}
	var eventNameHashRule []interface{}
	for _, eventNameHashItem := range eventNameHash {
		eventNameHashRule = append(eventNameHashRule, eventNameHashItem)
	}
	var topic1Rule []interface{}
	for _, topic1Item := range topic1 {
		topic1Rule = append(topic1Rule, topic1Item)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "EventLog2", msgSenderRule, eventNameHashRule, topic1Rule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterEventLog2)
				if err := _EventEmitter.contract.UnpackLog(event, "EventLog2", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEventLog2 is a log parse operation binding the contract event 0xfb0f08ac39934a2d9d9120c47f7079c97667c3a5495f5a6733b46b2c87cd2502.
//
// Solidity: event EventLog2(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, bytes32 topic2, string eventName, (((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])) eventData, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseEventLog2(log types.Log) (*EventEmitterEventLog2, error) {
	event := new(EventEmitterEventLog2)
	if err := _EventEmitter.contract.UnpackLog(event, "EventLog2", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterFeeDistributedIterator is returned from FilterFeeDistributed and is used to iterate over the raw logs and unpacked data for FeeDistributed events raised by the EventEmitter contract.
type EventEmitterFeeDistributedIterator struct {
	Event *EventEmitterFeeDistributed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterFeeDistributedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterFeeDistributed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterFeeDistributed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterFeeDistributedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterFeeDistributedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterFeeDistributed represents a FeeDistributed event raised by the EventEmitter contract.
type EventEmitterFeeDistributed struct {
	PoolId      [32]byte
	NftId       *big.Int
	Strategy    uint8
	Amount      *big.Int
	Recipient   common.Address
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterFeeDistributed is a free log retrieval operation binding the contract event 0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc.
//
// Solidity: event FeeDistributed(bytes32 indexed poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterFeeDistributed(opts *bind.FilterOpts, poolId [][32]byte) (*EventEmitterFeeDistributedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "FeeDistributed", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterFeeDistributedIterator{contract: _EventEmitter.contract, event: "FeeDistributed", logs: logs, sub: sub}, nil
}

// WatchFeeDistributed is a free log subscription operation binding the contract event 0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc.
//
// Solidity: event FeeDistributed(bytes32 indexed poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchFeeDistributed(opts *bind.WatchOpts, sink chan<- *EventEmitterFeeDistributed, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "FeeDistributed", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterFeeDistributed)
				if err := _EventEmitter.contract.UnpackLog(event, "FeeDistributed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFeeDistributed is a log parse operation binding the contract event 0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc.
//
// Solidity: event FeeDistributed(bytes32 indexed poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseFeeDistributed(log types.Log) (*EventEmitterFeeDistributed, error) {
	event := new(EventEmitterFeeDistributed)
	if err := _EventEmitter.contract.UnpackLog(event, "FeeDistributed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterFeeFlowIterator is returned from FilterFeeFlow and is used to iterate over the raw logs and unpacked data for FeeFlow events raised by the EventEmitter contract.
type EventEmitterFeeFlowIterator struct {
	Event *EventEmitterFeeFlow // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterFeeFlowIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterFeeFlow)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterFeeFlow)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterFeeFlowIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterFeeFlowIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterFeeFlow represents a FeeFlow event raised by the EventEmitter contract.
type EventEmitterFeeFlow struct {
	Kind        uint8
	Pool        common.Address
	Party       common.Address
	Amount      *big.Int
	ProtocolCut *big.Int
	PoolCut     *big.Int
	Epoch       *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterFeeFlow is a free log retrieval operation binding the contract event 0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4.
//
// Solidity: event FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterFeeFlow(opts *bind.FilterOpts, kind []uint8, pool []common.Address, party []common.Address) (*EventEmitterFeeFlowIterator, error) {

	var kindRule []interface{}
	for _, kindItem := range kind {
		kindRule = append(kindRule, kindItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var partyRule []interface{}
	for _, partyItem := range party {
		partyRule = append(partyRule, partyItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "FeeFlow", kindRule, poolRule, partyRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterFeeFlowIterator{contract: _EventEmitter.contract, event: "FeeFlow", logs: logs, sub: sub}, nil
}

// WatchFeeFlow is a free log subscription operation binding the contract event 0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4.
//
// Solidity: event FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchFeeFlow(opts *bind.WatchOpts, sink chan<- *EventEmitterFeeFlow, kind []uint8, pool []common.Address, party []common.Address) (event.Subscription, error) {

	var kindRule []interface{}
	for _, kindItem := range kind {
		kindRule = append(kindRule, kindItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var partyRule []interface{}
	for _, partyItem := range party {
		partyRule = append(partyRule, partyItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "FeeFlow", kindRule, poolRule, partyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterFeeFlow)
				if err := _EventEmitter.contract.UnpackLog(event, "FeeFlow", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFeeFlow is a log parse operation binding the contract event 0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4.
//
// Solidity: event FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseFeeFlow(log types.Log) (*EventEmitterFeeFlow, error) {
	event := new(EventEmitterFeeFlow)
	if err := _EventEmitter.contract.UnpackLog(event, "FeeFlow", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterFeeRecordedIterator is returned from FilterFeeRecorded and is used to iterate over the raw logs and unpacked data for FeeRecorded events raised by the EventEmitter contract.
type EventEmitterFeeRecordedIterator struct {
	Event *EventEmitterFeeRecorded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterFeeRecordedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterFeeRecorded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterFeeRecorded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterFeeRecordedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterFeeRecordedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterFeeRecorded represents a FeeRecorded event raised by the EventEmitter contract.
type EventEmitterFeeRecorded struct {
	PoolId      [32]byte
	FeeAmount   *big.Int
	ProtocolCut *big.Int
	PoolCut     *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterFeeRecorded is a free log retrieval operation binding the contract event 0xe80e9e3e09460d716260f5ce6979a59f6cf4fbc1096e20f008d78b5cac7b7179.
//
// Solidity: event FeeRecorded(bytes32 indexed poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterFeeRecorded(opts *bind.FilterOpts, poolId [][32]byte) (*EventEmitterFeeRecordedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "FeeRecorded", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterFeeRecordedIterator{contract: _EventEmitter.contract, event: "FeeRecorded", logs: logs, sub: sub}, nil
}

// WatchFeeRecorded is a free log subscription operation binding the contract event 0xe80e9e3e09460d716260f5ce6979a59f6cf4fbc1096e20f008d78b5cac7b7179.
//
// Solidity: event FeeRecorded(bytes32 indexed poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchFeeRecorded(opts *bind.WatchOpts, sink chan<- *EventEmitterFeeRecorded, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "FeeRecorded", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterFeeRecorded)
				if err := _EventEmitter.contract.UnpackLog(event, "FeeRecorded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFeeRecorded is a log parse operation binding the contract event 0xe80e9e3e09460d716260f5ce6979a59f6cf4fbc1096e20f008d78b5cac7b7179.
//
// Solidity: event FeeRecorded(bytes32 indexed poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseFeeRecorded(log types.Log) (*EventEmitterFeeRecorded, error) {
	event := new(EventEmitterFeeRecorded)
	if err := _EventEmitter.contract.UnpackLog(event, "FeeRecorded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterFeeStrategyChangedIterator is returned from FilterFeeStrategyChanged and is used to iterate over the raw logs and unpacked data for FeeStrategyChanged events raised by the EventEmitter contract.
type EventEmitterFeeStrategyChangedIterator struct {
	Event *EventEmitterFeeStrategyChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterFeeStrategyChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterFeeStrategyChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterFeeStrategyChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterFeeStrategyChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterFeeStrategyChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterFeeStrategyChanged represents a FeeStrategyChanged event raised by the EventEmitter contract.
type EventEmitterFeeStrategyChanged struct {
	PoolId      [32]byte
	NftId       *big.Int
	OldStrategy uint8
	NewStrategy uint8
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterFeeStrategyChanged is a free log retrieval operation binding the contract event 0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd.
//
// Solidity: event FeeStrategyChanged(bytes32 indexed poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterFeeStrategyChanged(opts *bind.FilterOpts, poolId [][32]byte) (*EventEmitterFeeStrategyChangedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "FeeStrategyChanged", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterFeeStrategyChangedIterator{contract: _EventEmitter.contract, event: "FeeStrategyChanged", logs: logs, sub: sub}, nil
}

// WatchFeeStrategyChanged is a free log subscription operation binding the contract event 0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd.
//
// Solidity: event FeeStrategyChanged(bytes32 indexed poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchFeeStrategyChanged(opts *bind.WatchOpts, sink chan<- *EventEmitterFeeStrategyChanged, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "FeeStrategyChanged", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterFeeStrategyChanged)
				if err := _EventEmitter.contract.UnpackLog(event, "FeeStrategyChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFeeStrategyChanged is a log parse operation binding the contract event 0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd.
//
// Solidity: event FeeStrategyChanged(bytes32 indexed poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseFeeStrategyChanged(log types.Log) (*EventEmitterFeeStrategyChanged, error) {
	event := new(EventEmitterFeeStrategyChanged)
	if err := _EventEmitter.contract.UnpackLog(event, "FeeStrategyChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterGovernanceIterator is returned from FilterGovernance and is used to iterate over the raw logs and unpacked data for Governance events raised by the EventEmitter contract.
type EventEmitterGovernanceIterator struct {
	Event *EventEmitterGovernance // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterGovernanceIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterGovernance)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterGovernance)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterGovernanceIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterGovernanceIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterGovernance represents a Governance event raised by the EventEmitter contract.
type EventEmitterGovernance struct {
	Action      uint8
	Id          [32]byte
	Actor       common.Address
	Payload     []byte
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterGovernance is a free log retrieval operation binding the contract event 0x637d9c5f725b51336e8338a6c85951604e8ee7bc041e8e2be8dc739abe9d2173.
//
// Solidity: event Governance(uint8 indexed action, bytes32 indexed id, address indexed actor, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterGovernance(opts *bind.FilterOpts, action []uint8, id [][32]byte, actor []common.Address) (*EventEmitterGovernanceIterator, error) {

	var actionRule []interface{}
	for _, actionItem := range action {
		actionRule = append(actionRule, actionItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var actorRule []interface{}
	for _, actorItem := range actor {
		actorRule = append(actorRule, actorItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "Governance", actionRule, idRule, actorRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterGovernanceIterator{contract: _EventEmitter.contract, event: "Governance", logs: logs, sub: sub}, nil
}

// WatchGovernance is a free log subscription operation binding the contract event 0x637d9c5f725b51336e8338a6c85951604e8ee7bc041e8e2be8dc739abe9d2173.
//
// Solidity: event Governance(uint8 indexed action, bytes32 indexed id, address indexed actor, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchGovernance(opts *bind.WatchOpts, sink chan<- *EventEmitterGovernance, action []uint8, id [][32]byte, actor []common.Address) (event.Subscription, error) {

	var actionRule []interface{}
	for _, actionItem := range action {
		actionRule = append(actionRule, actionItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var actorRule []interface{}
	for _, actorItem := range actor {
		actorRule = append(actorRule, actorItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "Governance", actionRule, idRule, actorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterGovernance)
				if err := _EventEmitter.contract.UnpackLog(event, "Governance", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseGovernance is a log parse operation binding the contract event 0x637d9c5f725b51336e8338a6c85951604e8ee7bc041e8e2be8dc739abe9d2173.
//
// Solidity: event Governance(uint8 indexed action, bytes32 indexed id, address indexed actor, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseGovernance(log types.Log) (*EventEmitterGovernance, error) {
	event := new(EventEmitterGovernance)
	if err := _EventEmitter.contract.UnpackLog(event, "Governance", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the EventEmitter contract.
type EventEmitterInitializedIterator struct {
	Event *EventEmitterInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterInitialized represents a Initialized event raised by the EventEmitter contract.
type EventEmitterInitialized struct {
	Version uint64
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_EventEmitter *EventEmitterFilterer) FilterInitialized(opts *bind.FilterOpts) (*EventEmitterInitializedIterator, error) {

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &EventEmitterInitializedIterator{contract: _EventEmitter.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_EventEmitter *EventEmitterFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *EventEmitterInitialized) (event.Subscription, error) {

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterInitialized)
				if err := _EventEmitter.contract.UnpackLog(event, "Initialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInitialized is a log parse operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_EventEmitter *EventEmitterFilterer) ParseInitialized(log types.Log) (*EventEmitterInitialized, error) {
	event := new(EventEmitterInitialized)
	if err := _EventEmitter.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterMarketCreatedIterator is returned from FilterMarketCreated and is used to iterate over the raw logs and unpacked data for MarketCreated events raised by the EventEmitter contract.
type EventEmitterMarketCreatedIterator struct {
	Event *EventEmitterMarketCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterMarketCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterMarketCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterMarketCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterMarketCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterMarketCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterMarketCreated represents a MarketCreated event raised by the EventEmitter contract.
type EventEmitterMarketCreated struct {
	PoolId      [32]byte
	Token       common.Address
	Creator     common.Address
	Pool        common.Address
	Optical     common.Address
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterMarketCreated is a free log retrieval operation binding the contract event 0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b.
//
// Solidity: event MarketCreated(bytes32 indexed poolId, address indexed token, address indexed creator, address pool, address optical, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterMarketCreated(opts *bind.FilterOpts, poolId [][32]byte, token []common.Address, creator []common.Address) (*EventEmitterMarketCreatedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "MarketCreated", poolIdRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterMarketCreatedIterator{contract: _EventEmitter.contract, event: "MarketCreated", logs: logs, sub: sub}, nil
}

// WatchMarketCreated is a free log subscription operation binding the contract event 0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b.
//
// Solidity: event MarketCreated(bytes32 indexed poolId, address indexed token, address indexed creator, address pool, address optical, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchMarketCreated(opts *bind.WatchOpts, sink chan<- *EventEmitterMarketCreated, poolId [][32]byte, token []common.Address, creator []common.Address) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "MarketCreated", poolIdRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterMarketCreated)
				if err := _EventEmitter.contract.UnpackLog(event, "MarketCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseMarketCreated is a log parse operation binding the contract event 0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b.
//
// Solidity: event MarketCreated(bytes32 indexed poolId, address indexed token, address indexed creator, address pool, address optical, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseMarketCreated(log types.Log) (*EventEmitterMarketCreated, error) {
	event := new(EventEmitterMarketCreated)
	if err := _EventEmitter.contract.UnpackLog(event, "MarketCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterNftMintIterator is returned from FilterNftMint and is used to iterate over the raw logs and unpacked data for NftMint events raised by the EventEmitter contract.
type EventEmitterNftMintIterator struct {
	Event *EventEmitterNftMint // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterNftMintIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterNftMint)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterNftMint)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterNftMintIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterNftMintIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterNftMint represents a NftMint event raised by the EventEmitter contract.
type EventEmitterNftMint struct {
	TokenId     *big.Int
	Creator     common.Address
	Pool        common.Address
	Strategy    uint8
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNftMint is a free log retrieval operation binding the contract event 0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5.
//
// Solidity: event NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterNftMint(opts *bind.FilterOpts, tokenId []*big.Int, creator []common.Address, pool []common.Address) (*EventEmitterNftMintIterator, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "NftMint", tokenIdRule, creatorRule, poolRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterNftMintIterator{contract: _EventEmitter.contract, event: "NftMint", logs: logs, sub: sub}, nil
}

// WatchNftMint is a free log subscription operation binding the contract event 0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5.
//
// Solidity: event NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchNftMint(opts *bind.WatchOpts, sink chan<- *EventEmitterNftMint, tokenId []*big.Int, creator []common.Address, pool []common.Address) (event.Subscription, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "NftMint", tokenIdRule, creatorRule, poolRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterNftMint)
				if err := _EventEmitter.contract.UnpackLog(event, "NftMint", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNftMint is a log parse operation binding the contract event 0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5.
//
// Solidity: event NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseNftMint(log types.Log) (*EventEmitterNftMint, error) {
	event := new(EventEmitterNftMint)
	if err := _EventEmitter.contract.UnpackLog(event, "NftMint", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterNftTransferIterator is returned from FilterNftTransfer and is used to iterate over the raw logs and unpacked data for NftTransfer events raised by the EventEmitter contract.
type EventEmitterNftTransferIterator struct {
	Event *EventEmitterNftTransfer // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterNftTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterNftTransfer)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterNftTransfer)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterNftTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterNftTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterNftTransfer represents a NftTransfer event raised by the EventEmitter contract.
type EventEmitterNftTransfer struct {
	Nft         common.Address
	From        common.Address
	To          common.Address
	TokenId     *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNftTransfer is a free log retrieval operation binding the contract event 0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3.
//
// Solidity: event NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterNftTransfer(opts *bind.FilterOpts, nft []common.Address, from []common.Address, to []common.Address) (*EventEmitterNftTransferIterator, error) {

	var nftRule []interface{}
	for _, nftItem := range nft {
		nftRule = append(nftRule, nftItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "NftTransfer", nftRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterNftTransferIterator{contract: _EventEmitter.contract, event: "NftTransfer", logs: logs, sub: sub}, nil
}

// WatchNftTransfer is a free log subscription operation binding the contract event 0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3.
//
// Solidity: event NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchNftTransfer(opts *bind.WatchOpts, sink chan<- *EventEmitterNftTransfer, nft []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

	var nftRule []interface{}
	for _, nftItem := range nft {
		nftRule = append(nftRule, nftItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "NftTransfer", nftRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterNftTransfer)
				if err := _EventEmitter.contract.UnpackLog(event, "NftTransfer", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNftTransfer is a log parse operation binding the contract event 0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3.
//
// Solidity: event NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseNftTransfer(log types.Log) (*EventEmitterNftTransfer, error) {
	event := new(EventEmitterNftTransfer)
	if err := _EventEmitter.contract.UnpackLog(event, "NftTransfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterOpticalExecutedIterator is returned from FilterOpticalExecuted and is used to iterate over the raw logs and unpacked data for OpticalExecuted events raised by the EventEmitter contract.
type EventEmitterOpticalExecutedIterator struct {
	Event *EventEmitterOpticalExecuted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterOpticalExecutedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterOpticalExecuted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterOpticalExecuted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterOpticalExecutedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterOpticalExecutedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterOpticalExecuted represents a OpticalExecuted event raised by the EventEmitter contract.
type EventEmitterOpticalExecuted struct {
	PoolId      [32]byte
	Optical     common.Address
	HookName    string
	Data        []byte
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterOpticalExecuted is a free log retrieval operation binding the contract event 0xfdf5ccbb26a02ea4efd5415e00b1d7782ca04cbb6b33a0d0a0d549ba284f9e57.
//
// Solidity: event OpticalExecuted(bytes32 indexed poolId, address indexed optical, string hookName, bytes data, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterOpticalExecuted(opts *bind.FilterOpts, poolId [][32]byte, optical []common.Address) (*EventEmitterOpticalExecutedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var opticalRule []interface{}
	for _, opticalItem := range optical {
		opticalRule = append(opticalRule, opticalItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "OpticalExecuted", poolIdRule, opticalRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterOpticalExecutedIterator{contract: _EventEmitter.contract, event: "OpticalExecuted", logs: logs, sub: sub}, nil
}

// WatchOpticalExecuted is a free log subscription operation binding the contract event 0xfdf5ccbb26a02ea4efd5415e00b1d7782ca04cbb6b33a0d0a0d549ba284f9e57.
//
// Solidity: event OpticalExecuted(bytes32 indexed poolId, address indexed optical, string hookName, bytes data, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchOpticalExecuted(opts *bind.WatchOpts, sink chan<- *EventEmitterOpticalExecuted, poolId [][32]byte, optical []common.Address) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var opticalRule []interface{}
	for _, opticalItem := range optical {
		opticalRule = append(opticalRule, opticalItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "OpticalExecuted", poolIdRule, opticalRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterOpticalExecuted)
				if err := _EventEmitter.contract.UnpackLog(event, "OpticalExecuted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOpticalExecuted is a log parse operation binding the contract event 0xfdf5ccbb26a02ea4efd5415e00b1d7782ca04cbb6b33a0d0a0d549ba284f9e57.
//
// Solidity: event OpticalExecuted(bytes32 indexed poolId, address indexed optical, string hookName, bytes data, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseOpticalExecuted(log types.Log) (*EventEmitterOpticalExecuted, error) {
	event := new(EventEmitterOpticalExecuted)
	if err := _EventEmitter.contract.UnpackLog(event, "OpticalExecuted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterOpticalLifecycleIterator is returned from FilterOpticalLifecycle and is used to iterate over the raw logs and unpacked data for OpticalLifecycle events raised by the EventEmitter contract.
type EventEmitterOpticalLifecycleIterator struct {
	Event *EventEmitterOpticalLifecycle // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterOpticalLifecycleIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterOpticalLifecycle)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterOpticalLifecycle)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterOpticalLifecycleIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterOpticalLifecycleIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterOpticalLifecycle represents a OpticalLifecycle event raised by the EventEmitter contract.
type EventEmitterOpticalLifecycle struct {
	Action      uint8
	Optical     common.Address
	Pool        common.Address
	Name        [32]byte
	Payload     []byte
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterOpticalLifecycle is a free log retrieval operation binding the contract event 0xa3faac07b1221710e43ca60800fc18acd83007ed48055def89c1e69b79ba44cb.
//
// Solidity: event OpticalLifecycle(uint8 indexed action, address indexed optical, address indexed pool, bytes32 name, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterOpticalLifecycle(opts *bind.FilterOpts, action []uint8, optical []common.Address, pool []common.Address) (*EventEmitterOpticalLifecycleIterator, error) {

	var actionRule []interface{}
	for _, actionItem := range action {
		actionRule = append(actionRule, actionItem)
	}
	var opticalRule []interface{}
	for _, opticalItem := range optical {
		opticalRule = append(opticalRule, opticalItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "OpticalLifecycle", actionRule, opticalRule, poolRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterOpticalLifecycleIterator{contract: _EventEmitter.contract, event: "OpticalLifecycle", logs: logs, sub: sub}, nil
}

// WatchOpticalLifecycle is a free log subscription operation binding the contract event 0xa3faac07b1221710e43ca60800fc18acd83007ed48055def89c1e69b79ba44cb.
//
// Solidity: event OpticalLifecycle(uint8 indexed action, address indexed optical, address indexed pool, bytes32 name, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchOpticalLifecycle(opts *bind.WatchOpts, sink chan<- *EventEmitterOpticalLifecycle, action []uint8, optical []common.Address, pool []common.Address) (event.Subscription, error) {

	var actionRule []interface{}
	for _, actionItem := range action {
		actionRule = append(actionRule, actionItem)
	}
	var opticalRule []interface{}
	for _, opticalItem := range optical {
		opticalRule = append(opticalRule, opticalItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "OpticalLifecycle", actionRule, opticalRule, poolRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterOpticalLifecycle)
				if err := _EventEmitter.contract.UnpackLog(event, "OpticalLifecycle", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOpticalLifecycle is a log parse operation binding the contract event 0xa3faac07b1221710e43ca60800fc18acd83007ed48055def89c1e69b79ba44cb.
//
// Solidity: event OpticalLifecycle(uint8 indexed action, address indexed optical, address indexed pool, bytes32 name, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseOpticalLifecycle(log types.Log) (*EventEmitterOpticalLifecycle, error) {
	event := new(EventEmitterOpticalLifecycle)
	if err := _EventEmitter.contract.UnpackLog(event, "OpticalLifecycle", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterOracleAdapterLifecycleIterator is returned from FilterOracleAdapterLifecycle and is used to iterate over the raw logs and unpacked data for OracleAdapterLifecycle events raised by the EventEmitter contract.
type EventEmitterOracleAdapterLifecycleIterator struct {
	Event *EventEmitterOracleAdapterLifecycle // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterOracleAdapterLifecycleIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterOracleAdapterLifecycle)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterOracleAdapterLifecycle)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterOracleAdapterLifecycleIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterOracleAdapterLifecycleIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterOracleAdapterLifecycle represents a OracleAdapterLifecycle event raised by the EventEmitter contract.
type EventEmitterOracleAdapterLifecycle struct {
	SourceId    [32]byte
	Adapter     common.Address
	Phase       uint8
	Priority    *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterOracleAdapterLifecycle is a free log retrieval operation binding the contract event 0x9f8c04497ebc19f8f7de480e55fdfa3780212516de2d597e6f793d6629017cec.
//
// Solidity: event OracleAdapterLifecycle(bytes32 indexed sourceId, address indexed adapter, uint8 phase, uint256 priority, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterOracleAdapterLifecycle(opts *bind.FilterOpts, sourceId [][32]byte, adapter []common.Address) (*EventEmitterOracleAdapterLifecycleIterator, error) {

	var sourceIdRule []interface{}
	for _, sourceIdItem := range sourceId {
		sourceIdRule = append(sourceIdRule, sourceIdItem)
	}
	var adapterRule []interface{}
	for _, adapterItem := range adapter {
		adapterRule = append(adapterRule, adapterItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "OracleAdapterLifecycle", sourceIdRule, adapterRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterOracleAdapterLifecycleIterator{contract: _EventEmitter.contract, event: "OracleAdapterLifecycle", logs: logs, sub: sub}, nil
}

// WatchOracleAdapterLifecycle is a free log subscription operation binding the contract event 0x9f8c04497ebc19f8f7de480e55fdfa3780212516de2d597e6f793d6629017cec.
//
// Solidity: event OracleAdapterLifecycle(bytes32 indexed sourceId, address indexed adapter, uint8 phase, uint256 priority, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchOracleAdapterLifecycle(opts *bind.WatchOpts, sink chan<- *EventEmitterOracleAdapterLifecycle, sourceId [][32]byte, adapter []common.Address) (event.Subscription, error) {

	var sourceIdRule []interface{}
	for _, sourceIdItem := range sourceId {
		sourceIdRule = append(sourceIdRule, sourceIdItem)
	}
	var adapterRule []interface{}
	for _, adapterItem := range adapter {
		adapterRule = append(adapterRule, adapterItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "OracleAdapterLifecycle", sourceIdRule, adapterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterOracleAdapterLifecycle)
				if err := _EventEmitter.contract.UnpackLog(event, "OracleAdapterLifecycle", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOracleAdapterLifecycle is a log parse operation binding the contract event 0x9f8c04497ebc19f8f7de480e55fdfa3780212516de2d597e6f793d6629017cec.
//
// Solidity: event OracleAdapterLifecycle(bytes32 indexed sourceId, address indexed adapter, uint8 phase, uint256 priority, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseOracleAdapterLifecycle(log types.Log) (*EventEmitterOracleAdapterLifecycle, error) {
	event := new(EventEmitterOracleAdapterLifecycle)
	if err := _EventEmitter.contract.UnpackLog(event, "OracleAdapterLifecycle", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPauseToggleIterator is returned from FilterPauseToggle and is used to iterate over the raw logs and unpacked data for PauseToggle events raised by the EventEmitter contract.
type EventEmitterPauseToggleIterator struct {
	Event *EventEmitterPauseToggle // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPauseToggleIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPauseToggle)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPauseToggle)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPauseToggleIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPauseToggleIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPauseToggle represents a PauseToggle event raised by the EventEmitter contract.
type EventEmitterPauseToggle struct {
	PausedContract common.Address
	Paused         bool
	Timestamp      *big.Int
	BlockNumber    *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterPauseToggle is a free log retrieval operation binding the contract event 0xd079c1af4038467292233cbd5af2e60da5c45ea48670f905aff3da18b483973c.
//
// Solidity: event PauseToggle(address indexed pausedContract, bool paused, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPauseToggle(opts *bind.FilterOpts, pausedContract []common.Address) (*EventEmitterPauseToggleIterator, error) {

	var pausedContractRule []interface{}
	for _, pausedContractItem := range pausedContract {
		pausedContractRule = append(pausedContractRule, pausedContractItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PauseToggle", pausedContractRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPauseToggleIterator{contract: _EventEmitter.contract, event: "PauseToggle", logs: logs, sub: sub}, nil
}

// WatchPauseToggle is a free log subscription operation binding the contract event 0xd079c1af4038467292233cbd5af2e60da5c45ea48670f905aff3da18b483973c.
//
// Solidity: event PauseToggle(address indexed pausedContract, bool paused, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPauseToggle(opts *bind.WatchOpts, sink chan<- *EventEmitterPauseToggle, pausedContract []common.Address) (event.Subscription, error) {

	var pausedContractRule []interface{}
	for _, pausedContractItem := range pausedContract {
		pausedContractRule = append(pausedContractRule, pausedContractItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PauseToggle", pausedContractRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPauseToggle)
				if err := _EventEmitter.contract.UnpackLog(event, "PauseToggle", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePauseToggle is a log parse operation binding the contract event 0xd079c1af4038467292233cbd5af2e60da5c45ea48670f905aff3da18b483973c.
//
// Solidity: event PauseToggle(address indexed pausedContract, bool paused, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePauseToggle(log types.Log) (*EventEmitterPauseToggle, error) {
	event := new(EventEmitterPauseToggle)
	if err := _EventEmitter.contract.UnpackLog(event, "PauseToggle", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPecorOrderCreatedIterator is returned from FilterPecorOrderCreated and is used to iterate over the raw logs and unpacked data for PecorOrderCreated events raised by the EventEmitter contract.
type EventEmitterPecorOrderCreatedIterator struct {
	Event *EventEmitterPecorOrderCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPecorOrderCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPecorOrderCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPecorOrderCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPecorOrderCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPecorOrderCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPecorOrderCreated represents a PecorOrderCreated event raised by the EventEmitter contract.
type EventEmitterPecorOrderCreated struct {
	OrderId     *big.Int
	User        common.Address
	OrderKind   uint8
	OrderType   uint8
	TokenIn     common.Address
	TokenOut    common.Address
	Amount      *big.Int
	TargetPrice *big.Int
	StopPrice   *big.Int
	LimitPrice  *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterPecorOrderCreated is a free log retrieval operation binding the contract event 0xa24904fab3b2e5505845a72e1cd3b798ad4fbd48a7e6cb662de4b0d0d5abb298.
//
// Solidity: event PecorOrderCreated(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPecorOrderCreated(opts *bind.FilterOpts, orderId []*big.Int, user []common.Address) (*EventEmitterPecorOrderCreatedIterator, error) {

	var orderIdRule []interface{}
	for _, orderIdItem := range orderId {
		orderIdRule = append(orderIdRule, orderIdItem)
	}
	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PecorOrderCreated", orderIdRule, userRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPecorOrderCreatedIterator{contract: _EventEmitter.contract, event: "PecorOrderCreated", logs: logs, sub: sub}, nil
}

// WatchPecorOrderCreated is a free log subscription operation binding the contract event 0xa24904fab3b2e5505845a72e1cd3b798ad4fbd48a7e6cb662de4b0d0d5abb298.
//
// Solidity: event PecorOrderCreated(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPecorOrderCreated(opts *bind.WatchOpts, sink chan<- *EventEmitterPecorOrderCreated, orderId []*big.Int, user []common.Address) (event.Subscription, error) {

	var orderIdRule []interface{}
	for _, orderIdItem := range orderId {
		orderIdRule = append(orderIdRule, orderIdItem)
	}
	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PecorOrderCreated", orderIdRule, userRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPecorOrderCreated)
				if err := _EventEmitter.contract.UnpackLog(event, "PecorOrderCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePecorOrderCreated is a log parse operation binding the contract event 0xa24904fab3b2e5505845a72e1cd3b798ad4fbd48a7e6cb662de4b0d0d5abb298.
//
// Solidity: event PecorOrderCreated(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePecorOrderCreated(log types.Log) (*EventEmitterPecorOrderCreated, error) {
	event := new(EventEmitterPecorOrderCreated)
	if err := _EventEmitter.contract.UnpackLog(event, "PecorOrderCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPecorOrderLifecycleIterator is returned from FilterPecorOrderLifecycle and is used to iterate over the raw logs and unpacked data for PecorOrderLifecycle events raised by the EventEmitter contract.
type EventEmitterPecorOrderLifecycleIterator struct {
	Event *EventEmitterPecorOrderLifecycle // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPecorOrderLifecycleIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPecorOrderLifecycle)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPecorOrderLifecycle)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPecorOrderLifecycleIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPecorOrderLifecycleIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPecorOrderLifecycle represents a PecorOrderLifecycle event raised by the EventEmitter contract.
type EventEmitterPecorOrderLifecycle struct {
	OrderId     *big.Int
	User        common.Address
	OrderKind   uint8
	Phase       uint8
	Price       *big.Int
	Payload     []byte
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterPecorOrderLifecycle is a free log retrieval operation binding the contract event 0xdbbb8b49248d8ae078049644637ec9edf159baa1c653f8274ddf3fa26f358d26.
//
// Solidity: event PecorOrderLifecycle(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 phase, uint256 price, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPecorOrderLifecycle(opts *bind.FilterOpts, orderId []*big.Int, user []common.Address) (*EventEmitterPecorOrderLifecycleIterator, error) {

	var orderIdRule []interface{}
	for _, orderIdItem := range orderId {
		orderIdRule = append(orderIdRule, orderIdItem)
	}
	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PecorOrderLifecycle", orderIdRule, userRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPecorOrderLifecycleIterator{contract: _EventEmitter.contract, event: "PecorOrderLifecycle", logs: logs, sub: sub}, nil
}

// WatchPecorOrderLifecycle is a free log subscription operation binding the contract event 0xdbbb8b49248d8ae078049644637ec9edf159baa1c653f8274ddf3fa26f358d26.
//
// Solidity: event PecorOrderLifecycle(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 phase, uint256 price, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPecorOrderLifecycle(opts *bind.WatchOpts, sink chan<- *EventEmitterPecorOrderLifecycle, orderId []*big.Int, user []common.Address) (event.Subscription, error) {

	var orderIdRule []interface{}
	for _, orderIdItem := range orderId {
		orderIdRule = append(orderIdRule, orderIdItem)
	}
	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PecorOrderLifecycle", orderIdRule, userRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPecorOrderLifecycle)
				if err := _EventEmitter.contract.UnpackLog(event, "PecorOrderLifecycle", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePecorOrderLifecycle is a log parse operation binding the contract event 0xdbbb8b49248d8ae078049644637ec9edf159baa1c653f8274ddf3fa26f358d26.
//
// Solidity: event PecorOrderLifecycle(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 phase, uint256 price, bytes payload, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePecorOrderLifecycle(log types.Log) (*EventEmitterPecorOrderLifecycle, error) {
	event := new(EventEmitterPecorOrderLifecycle)
	if err := _EventEmitter.contract.UnpackLog(event, "PecorOrderLifecycle", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPecorSwapIterator is returned from FilterPecorSwap and is used to iterate over the raw logs and unpacked data for PecorSwap events raised by the EventEmitter contract.
type EventEmitterPecorSwapIterator struct {
	Event *EventEmitterPecorSwap // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPecorSwapIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPecorSwap)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPecorSwap)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPecorSwapIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPecorSwapIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPecorSwap represents a PecorSwap event raised by the EventEmitter contract.
type EventEmitterPecorSwap struct {
	User        common.Address
	TokenIn     common.Address
	TokenOut    common.Address
	AmountIn    *big.Int
	AmountOut   *big.Int
	PriceIn     *big.Int
	PriceOut    *big.Int
	VolumeUSD   *big.Int
	FeeBps      *big.Int
	FeeAmount   *big.Int
	ImpactBps   *big.Int
	SwapKind    uint8
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterPecorSwap is a free log retrieval operation binding the contract event 0x9b4f5f387e6b6c21cc8d67c37c41d9d7b8d8b3f16494c84db2dd1fca812a644f.
//
// Solidity: event PecorSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPecorSwap(opts *bind.FilterOpts, user []common.Address, tokenIn []common.Address, tokenOut []common.Address) (*EventEmitterPecorSwapIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var tokenInRule []interface{}
	for _, tokenInItem := range tokenIn {
		tokenInRule = append(tokenInRule, tokenInItem)
	}
	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PecorSwap", userRule, tokenInRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPecorSwapIterator{contract: _EventEmitter.contract, event: "PecorSwap", logs: logs, sub: sub}, nil
}

// WatchPecorSwap is a free log subscription operation binding the contract event 0x9b4f5f387e6b6c21cc8d67c37c41d9d7b8d8b3f16494c84db2dd1fca812a644f.
//
// Solidity: event PecorSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPecorSwap(opts *bind.WatchOpts, sink chan<- *EventEmitterPecorSwap, user []common.Address, tokenIn []common.Address, tokenOut []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var tokenInRule []interface{}
	for _, tokenInItem := range tokenIn {
		tokenInRule = append(tokenInRule, tokenInItem)
	}
	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PecorSwap", userRule, tokenInRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPecorSwap)
				if err := _EventEmitter.contract.UnpackLog(event, "PecorSwap", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePecorSwap is a log parse operation binding the contract event 0x9b4f5f387e6b6c21cc8d67c37c41d9d7b8d8b3f16494c84db2dd1fca812a644f.
//
// Solidity: event PecorSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePecorSwap(log types.Log) (*EventEmitterPecorSwap, error) {
	event := new(EventEmitterPecorSwap)
	if err := _EventEmitter.contract.UnpackLog(event, "PecorSwap", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPoolRegisteredIterator is returned from FilterPoolRegistered and is used to iterate over the raw logs and unpacked data for PoolRegistered events raised by the EventEmitter contract.
type EventEmitterPoolRegisteredIterator struct {
	Event *EventEmitterPoolRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPoolRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPoolRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPoolRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPoolRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPoolRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPoolRegistered represents a PoolRegistered event raised by the EventEmitter contract.
type EventEmitterPoolRegistered struct {
	Pool        common.Address
	Token       common.Address
	Creator     common.Address
	Optical     common.Address
	NftId       *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterPoolRegistered is a free log retrieval operation binding the contract event 0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7.
//
// Solidity: event PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPoolRegistered(opts *bind.FilterOpts, pool []common.Address, token []common.Address, creator []common.Address) (*EventEmitterPoolRegisteredIterator, error) {

	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PoolRegistered", poolRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPoolRegisteredIterator{contract: _EventEmitter.contract, event: "PoolRegistered", logs: logs, sub: sub}, nil
}

// WatchPoolRegistered is a free log subscription operation binding the contract event 0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7.
//
// Solidity: event PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPoolRegistered(opts *bind.WatchOpts, sink chan<- *EventEmitterPoolRegistered, pool []common.Address, token []common.Address, creator []common.Address) (event.Subscription, error) {

	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PoolRegistered", poolRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPoolRegistered)
				if err := _EventEmitter.contract.UnpackLog(event, "PoolRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePoolRegistered is a log parse operation binding the contract event 0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7.
//
// Solidity: event PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePoolRegistered(log types.Log) (*EventEmitterPoolRegistered, error) {
	event := new(EventEmitterPoolRegistered)
	if err := _EventEmitter.contract.UnpackLog(event, "PoolRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPoolStateUpdatedIterator is returned from FilterPoolStateUpdated and is used to iterate over the raw logs and unpacked data for PoolStateUpdated events raised by the EventEmitter contract.
type EventEmitterPoolStateUpdatedIterator struct {
	Event *EventEmitterPoolStateUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPoolStateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPoolStateUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPoolStateUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPoolStateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPoolStateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPoolStateUpdated represents a PoolStateUpdated event raised by the EventEmitter contract.
type EventEmitterPoolStateUpdated struct {
	PoolId         [32]byte
	VirtualReserve *big.Int
	RealReserve    *big.Int
	TokenReserve   *big.Int
	Price          *big.Int
	Timestamp      *big.Int
	BlockNumber    *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterPoolStateUpdated is a free log retrieval operation binding the contract event 0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd.
//
// Solidity: event PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPoolStateUpdated(opts *bind.FilterOpts, poolId [][32]byte) (*EventEmitterPoolStateUpdatedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PoolStateUpdated", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPoolStateUpdatedIterator{contract: _EventEmitter.contract, event: "PoolStateUpdated", logs: logs, sub: sub}, nil
}

// WatchPoolStateUpdated is a free log subscription operation binding the contract event 0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd.
//
// Solidity: event PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPoolStateUpdated(opts *bind.WatchOpts, sink chan<- *EventEmitterPoolStateUpdated, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PoolStateUpdated", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPoolStateUpdated)
				if err := _EventEmitter.contract.UnpackLog(event, "PoolStateUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePoolStateUpdated is a log parse operation binding the contract event 0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd.
//
// Solidity: event PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePoolStateUpdated(log types.Log) (*EventEmitterPoolStateUpdated, error) {
	event := new(EventEmitterPoolStateUpdated)
	if err := _EventEmitter.contract.UnpackLog(event, "PoolStateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterPriceUpdatedIterator is returned from FilterPriceUpdated and is used to iterate over the raw logs and unpacked data for PriceUpdated events raised by the EventEmitter contract.
type EventEmitterPriceUpdatedIterator struct {
	Event *EventEmitterPriceUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterPriceUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterPriceUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterPriceUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterPriceUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterPriceUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterPriceUpdated represents a PriceUpdated event raised by the EventEmitter contract.
type EventEmitterPriceUpdated struct {
	Token       common.Address
	RoundId     *big.Int
	Relayer     common.Address
	Price       *big.Int
	Confidence  *big.Int
	SourceId    [32]byte
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterPriceUpdated is a free log retrieval operation binding the contract event 0x8d99dbd92dc7116e69aa285daa09f2da514d6b2ca202545c3d9a0385982a7a69.
//
// Solidity: event PriceUpdated(address indexed token, uint256 indexed roundId, address indexed relayer, uint256 price, uint256 confidence, bytes32 sourceId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterPriceUpdated(opts *bind.FilterOpts, token []common.Address, roundId []*big.Int, relayer []common.Address) (*EventEmitterPriceUpdatedIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var roundIdRule []interface{}
	for _, roundIdItem := range roundId {
		roundIdRule = append(roundIdRule, roundIdItem)
	}
	var relayerRule []interface{}
	for _, relayerItem := range relayer {
		relayerRule = append(relayerRule, relayerItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "PriceUpdated", tokenRule, roundIdRule, relayerRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterPriceUpdatedIterator{contract: _EventEmitter.contract, event: "PriceUpdated", logs: logs, sub: sub}, nil
}

// WatchPriceUpdated is a free log subscription operation binding the contract event 0x8d99dbd92dc7116e69aa285daa09f2da514d6b2ca202545c3d9a0385982a7a69.
//
// Solidity: event PriceUpdated(address indexed token, uint256 indexed roundId, address indexed relayer, uint256 price, uint256 confidence, bytes32 sourceId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchPriceUpdated(opts *bind.WatchOpts, sink chan<- *EventEmitterPriceUpdated, token []common.Address, roundId []*big.Int, relayer []common.Address) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var roundIdRule []interface{}
	for _, roundIdItem := range roundId {
		roundIdRule = append(roundIdRule, roundIdItem)
	}
	var relayerRule []interface{}
	for _, relayerItem := range relayer {
		relayerRule = append(relayerRule, relayerItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "PriceUpdated", tokenRule, roundIdRule, relayerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterPriceUpdated)
				if err := _EventEmitter.contract.UnpackLog(event, "PriceUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePriceUpdated is a log parse operation binding the contract event 0x8d99dbd92dc7116e69aa285daa09f2da514d6b2ca202545c3d9a0385982a7a69.
//
// Solidity: event PriceUpdated(address indexed token, uint256 indexed roundId, address indexed relayer, uint256 price, uint256 confidence, bytes32 sourceId, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParsePriceUpdated(log types.Log) (*EventEmitterPriceUpdated, error) {
	event := new(EventEmitterPriceUpdated)
	if err := _EventEmitter.contract.UnpackLog(event, "PriceUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterRoleAdminChangedIterator is returned from FilterRoleAdminChanged and is used to iterate over the raw logs and unpacked data for RoleAdminChanged events raised by the EventEmitter contract.
type EventEmitterRoleAdminChangedIterator struct {
	Event *EventEmitterRoleAdminChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterRoleAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterRoleAdminChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterRoleAdminChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterRoleAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterRoleAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterRoleAdminChanged represents a RoleAdminChanged event raised by the EventEmitter contract.
type EventEmitterRoleAdminChanged struct {
	Role              [32]byte
	PreviousAdminRole [32]byte
	NewAdminRole      [32]byte
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRoleAdminChanged is a free log retrieval operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_EventEmitter *EventEmitterFilterer) FilterRoleAdminChanged(opts *bind.FilterOpts, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (*EventEmitterRoleAdminChangedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterRoleAdminChangedIterator{contract: _EventEmitter.contract, event: "RoleAdminChanged", logs: logs, sub: sub}, nil
}

// WatchRoleAdminChanged is a free log subscription operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_EventEmitter *EventEmitterFilterer) WatchRoleAdminChanged(opts *bind.WatchOpts, sink chan<- *EventEmitterRoleAdminChanged, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterRoleAdminChanged)
				if err := _EventEmitter.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleAdminChanged is a log parse operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_EventEmitter *EventEmitterFilterer) ParseRoleAdminChanged(log types.Log) (*EventEmitterRoleAdminChanged, error) {
	event := new(EventEmitterRoleAdminChanged)
	if err := _EventEmitter.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterRoleChangeIterator is returned from FilterRoleChange and is used to iterate over the raw logs and unpacked data for RoleChange events raised by the EventEmitter contract.
type EventEmitterRoleChangeIterator struct {
	Event *EventEmitterRoleChange // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterRoleChangeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterRoleChange)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterRoleChange)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterRoleChangeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterRoleChangeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterRoleChange represents a RoleChange event raised by the EventEmitter contract.
type EventEmitterRoleChange struct {
	Action            uint8
	Role              [32]byte
	Account           common.Address
	Sender            common.Address
	PreviousAdminRole [32]byte
	Timestamp         *big.Int
	BlockNumber       *big.Int
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRoleChange is a free log retrieval operation binding the contract event 0xdcb006502b86d8ebfada5d7606648e10555d5df4a42fe9a8fcd00999c25faaf4.
//
// Solidity: event RoleChange(uint8 indexed action, bytes32 indexed role, address indexed account, address sender, bytes32 previousAdminRole, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterRoleChange(opts *bind.FilterOpts, action []uint8, role [][32]byte, account []common.Address) (*EventEmitterRoleChangeIterator, error) {

	var actionRule []interface{}
	for _, actionItem := range action {
		actionRule = append(actionRule, actionItem)
	}
	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "RoleChange", actionRule, roleRule, accountRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterRoleChangeIterator{contract: _EventEmitter.contract, event: "RoleChange", logs: logs, sub: sub}, nil
}

// WatchRoleChange is a free log subscription operation binding the contract event 0xdcb006502b86d8ebfada5d7606648e10555d5df4a42fe9a8fcd00999c25faaf4.
//
// Solidity: event RoleChange(uint8 indexed action, bytes32 indexed role, address indexed account, address sender, bytes32 previousAdminRole, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchRoleChange(opts *bind.WatchOpts, sink chan<- *EventEmitterRoleChange, action []uint8, role [][32]byte, account []common.Address) (event.Subscription, error) {

	var actionRule []interface{}
	for _, actionItem := range action {
		actionRule = append(actionRule, actionItem)
	}
	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "RoleChange", actionRule, roleRule, accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterRoleChange)
				if err := _EventEmitter.contract.UnpackLog(event, "RoleChange", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleChange is a log parse operation binding the contract event 0xdcb006502b86d8ebfada5d7606648e10555d5df4a42fe9a8fcd00999c25faaf4.
//
// Solidity: event RoleChange(uint8 indexed action, bytes32 indexed role, address indexed account, address sender, bytes32 previousAdminRole, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseRoleChange(log types.Log) (*EventEmitterRoleChange, error) {
	event := new(EventEmitterRoleChange)
	if err := _EventEmitter.contract.UnpackLog(event, "RoleChange", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterRoleGrantedIterator is returned from FilterRoleGranted and is used to iterate over the raw logs and unpacked data for RoleGranted events raised by the EventEmitter contract.
type EventEmitterRoleGrantedIterator struct {
	Event *EventEmitterRoleGranted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterRoleGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterRoleGranted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterRoleGranted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterRoleGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterRoleGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterRoleGranted represents a RoleGranted event raised by the EventEmitter contract.
type EventEmitterRoleGranted struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleGranted is a free log retrieval operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_EventEmitter *EventEmitterFilterer) FilterRoleGranted(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*EventEmitterRoleGrantedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterRoleGrantedIterator{contract: _EventEmitter.contract, event: "RoleGranted", logs: logs, sub: sub}, nil
}

// WatchRoleGranted is a free log subscription operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_EventEmitter *EventEmitterFilterer) WatchRoleGranted(opts *bind.WatchOpts, sink chan<- *EventEmitterRoleGranted, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterRoleGranted)
				if err := _EventEmitter.contract.UnpackLog(event, "RoleGranted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleGranted is a log parse operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_EventEmitter *EventEmitterFilterer) ParseRoleGranted(log types.Log) (*EventEmitterRoleGranted, error) {
	event := new(EventEmitterRoleGranted)
	if err := _EventEmitter.contract.UnpackLog(event, "RoleGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterRoleRevokedIterator is returned from FilterRoleRevoked and is used to iterate over the raw logs and unpacked data for RoleRevoked events raised by the EventEmitter contract.
type EventEmitterRoleRevokedIterator struct {
	Event *EventEmitterRoleRevoked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterRoleRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterRoleRevoked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterRoleRevoked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterRoleRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterRoleRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterRoleRevoked represents a RoleRevoked event raised by the EventEmitter contract.
type EventEmitterRoleRevoked struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleRevoked is a free log retrieval operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_EventEmitter *EventEmitterFilterer) FilterRoleRevoked(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*EventEmitterRoleRevokedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterRoleRevokedIterator{contract: _EventEmitter.contract, event: "RoleRevoked", logs: logs, sub: sub}, nil
}

// WatchRoleRevoked is a free log subscription operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_EventEmitter *EventEmitterFilterer) WatchRoleRevoked(opts *bind.WatchOpts, sink chan<- *EventEmitterRoleRevoked, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterRoleRevoked)
				if err := _EventEmitter.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleRevoked is a log parse operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_EventEmitter *EventEmitterFilterer) ParseRoleRevoked(log types.Log) (*EventEmitterRoleRevoked, error) {
	event := new(EventEmitterRoleRevoked)
	if err := _EventEmitter.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterRouterTradeIterator is returned from FilterRouterTrade and is used to iterate over the raw logs and unpacked data for RouterTrade events raised by the EventEmitter contract.
type EventEmitterRouterTradeIterator struct {
	Event *EventEmitterRouterTrade // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterRouterTradeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterRouterTrade)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterRouterTrade)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterRouterTradeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterRouterTradeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterRouterTrade represents a RouterTrade event raised by the EventEmitter contract.
type EventEmitterRouterTrade struct {
	Kind             uint8
	Pool             common.Address
	Sender           common.Address
	TokenIn          common.Address
	TokenOut         common.Address
	AmountIn         *big.Int
	AmountOut        *big.Int
	IntermediateUsdl *big.Int
	Timestamp        *big.Int
	BlockNumber      *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterRouterTrade is a free log retrieval operation binding the contract event 0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650.
//
// Solidity: event RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterRouterTrade(opts *bind.FilterOpts, kind []uint8, pool []common.Address, sender []common.Address) (*EventEmitterRouterTradeIterator, error) {

	var kindRule []interface{}
	for _, kindItem := range kind {
		kindRule = append(kindRule, kindItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "RouterTrade", kindRule, poolRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterRouterTradeIterator{contract: _EventEmitter.contract, event: "RouterTrade", logs: logs, sub: sub}, nil
}

// WatchRouterTrade is a free log subscription operation binding the contract event 0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650.
//
// Solidity: event RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchRouterTrade(opts *bind.WatchOpts, sink chan<- *EventEmitterRouterTrade, kind []uint8, pool []common.Address, sender []common.Address) (event.Subscription, error) {

	var kindRule []interface{}
	for _, kindItem := range kind {
		kindRule = append(kindRule, kindItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "RouterTrade", kindRule, poolRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterRouterTrade)
				if err := _EventEmitter.contract.UnpackLog(event, "RouterTrade", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRouterTrade is a log parse operation binding the contract event 0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650.
//
// Solidity: event RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseRouterTrade(log types.Log) (*EventEmitterRouterTrade, error) {
	event := new(EventEmitterRouterTrade)
	if err := _EventEmitter.contract.UnpackLog(event, "RouterTrade", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterSwapIterator is returned from FilterSwap and is used to iterate over the raw logs and unpacked data for Swap events raised by the EventEmitter contract.
type EventEmitterSwapIterator struct {
	Event *EventEmitterSwap // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterSwapIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterSwap)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterSwap)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterSwapIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterSwapIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterSwap represents a Swap event raised by the EventEmitter contract.
type EventEmitterSwap struct {
	PoolId      [32]byte
	Sender      common.Address
	IsBuy       bool
	AmountIn    *big.Int
	AmountOut   *big.Int
	Fee         *big.Int
	Price       *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterSwap is a free log retrieval operation binding the contract event 0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc.
//
// Solidity: event Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterSwap(opts *bind.FilterOpts, poolId [][32]byte, sender []common.Address) (*EventEmitterSwapIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "Swap", poolIdRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterSwapIterator{contract: _EventEmitter.contract, event: "Swap", logs: logs, sub: sub}, nil
}

// WatchSwap is a free log subscription operation binding the contract event 0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc.
//
// Solidity: event Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchSwap(opts *bind.WatchOpts, sink chan<- *EventEmitterSwap, poolId [][32]byte, sender []common.Address) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "Swap", poolIdRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterSwap)
				if err := _EventEmitter.contract.UnpackLog(event, "Swap", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSwap is a log parse operation binding the contract event 0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc.
//
// Solidity: event Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseSwap(log types.Log) (*EventEmitterSwap, error) {
	event := new(EventEmitterSwap)
	if err := _EventEmitter.contract.UnpackLog(event, "Swap", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterTokenDeployedIterator is returned from FilterTokenDeployed and is used to iterate over the raw logs and unpacked data for TokenDeployed events raised by the EventEmitter contract.
type EventEmitterTokenDeployedIterator struct {
	Event *EventEmitterTokenDeployed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterTokenDeployedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterTokenDeployed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterTokenDeployed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterTokenDeployedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterTokenDeployedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterTokenDeployed represents a TokenDeployed event raised by the EventEmitter contract.
type EventEmitterTokenDeployed struct {
	Token       common.Address
	Pool        common.Address
	Creator     common.Address
	Salt        [32]byte
	Name        string
	Symbol      string
	Decimals    uint8
	TotalSupply *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterTokenDeployed is a free log retrieval operation binding the contract event 0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237.
//
// Solidity: event TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterTokenDeployed(opts *bind.FilterOpts, token []common.Address, pool []common.Address, creator []common.Address) (*EventEmitterTokenDeployedIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "TokenDeployed", tokenRule, poolRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterTokenDeployedIterator{contract: _EventEmitter.contract, event: "TokenDeployed", logs: logs, sub: sub}, nil
}

// WatchTokenDeployed is a free log subscription operation binding the contract event 0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237.
//
// Solidity: event TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchTokenDeployed(opts *bind.WatchOpts, sink chan<- *EventEmitterTokenDeployed, token []common.Address, pool []common.Address, creator []common.Address) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var poolRule []interface{}
	for _, poolItem := range pool {
		poolRule = append(poolRule, poolItem)
	}
	var creatorRule []interface{}
	for _, creatorItem := range creator {
		creatorRule = append(creatorRule, creatorItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "TokenDeployed", tokenRule, poolRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterTokenDeployed)
				if err := _EventEmitter.contract.UnpackLog(event, "TokenDeployed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTokenDeployed is a log parse operation binding the contract event 0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237.
//
// Solidity: event TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseTokenDeployed(log types.Log) (*EventEmitterTokenDeployed, error) {
	event := new(EventEmitterTokenDeployed)
	if err := _EventEmitter.contract.UnpackLog(event, "TokenDeployed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterTokenTransferIterator is returned from FilterTokenTransfer and is used to iterate over the raw logs and unpacked data for TokenTransfer events raised by the EventEmitter contract.
type EventEmitterTokenTransferIterator struct {
	Event *EventEmitterTokenTransfer // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterTokenTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterTokenTransfer)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterTokenTransfer)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterTokenTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterTokenTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterTokenTransfer represents a TokenTransfer event raised by the EventEmitter contract.
type EventEmitterTokenTransfer struct {
	Token       common.Address
	From        common.Address
	To          common.Address
	Value       *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterTokenTransfer is a free log retrieval operation binding the contract event 0xd1630e8dfeca8f80cedf002590087ccd08ffc4a5a1860fc02892098e7aa92de1.
//
// Solidity: event TokenTransfer(address indexed token, address indexed from, address indexed to, uint256 value, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterTokenTransfer(opts *bind.FilterOpts, token []common.Address, from []common.Address, to []common.Address) (*EventEmitterTokenTransferIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "TokenTransfer", tokenRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterTokenTransferIterator{contract: _EventEmitter.contract, event: "TokenTransfer", logs: logs, sub: sub}, nil
}

// WatchTokenTransfer is a free log subscription operation binding the contract event 0xd1630e8dfeca8f80cedf002590087ccd08ffc4a5a1860fc02892098e7aa92de1.
//
// Solidity: event TokenTransfer(address indexed token, address indexed from, address indexed to, uint256 value, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchTokenTransfer(opts *bind.WatchOpts, sink chan<- *EventEmitterTokenTransfer, token []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "TokenTransfer", tokenRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterTokenTransfer)
				if err := _EventEmitter.contract.UnpackLog(event, "TokenTransfer", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTokenTransfer is a log parse operation binding the contract event 0xd1630e8dfeca8f80cedf002590087ccd08ffc4a5a1860fc02892098e7aa92de1.
//
// Solidity: event TokenTransfer(address indexed token, address indexed from, address indexed to, uint256 value, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseTokenTransfer(log types.Log) (*EventEmitterTokenTransfer, error) {
	event := new(EventEmitterTokenTransfer)
	if err := _EventEmitter.contract.UnpackLog(event, "TokenTransfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterTreasuryFlowIterator is returned from FilterTreasuryFlow and is used to iterate over the raw logs and unpacked data for TreasuryFlow events raised by the EventEmitter contract.
type EventEmitterTreasuryFlowIterator struct {
	Event *EventEmitterTreasuryFlow // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterTreasuryFlowIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterTreasuryFlow)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterTreasuryFlow)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterTreasuryFlowIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterTreasuryFlowIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterTreasuryFlow represents a TreasuryFlow event raised by the EventEmitter contract.
type EventEmitterTreasuryFlow struct {
	Direction   uint8
	Token       common.Address
	Party       common.Address
	Amount      *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterTreasuryFlow is a free log retrieval operation binding the contract event 0x589609a9576cbe900d4afeecbd83764a5db719055826446f88c74bee62af7f81.
//
// Solidity: event TreasuryFlow(uint8 indexed direction, address indexed token, address indexed party, uint256 amount, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterTreasuryFlow(opts *bind.FilterOpts, direction []uint8, token []common.Address, party []common.Address) (*EventEmitterTreasuryFlowIterator, error) {

	var directionRule []interface{}
	for _, directionItem := range direction {
		directionRule = append(directionRule, directionItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var partyRule []interface{}
	for _, partyItem := range party {
		partyRule = append(partyRule, partyItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "TreasuryFlow", directionRule, tokenRule, partyRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterTreasuryFlowIterator{contract: _EventEmitter.contract, event: "TreasuryFlow", logs: logs, sub: sub}, nil
}

// WatchTreasuryFlow is a free log subscription operation binding the contract event 0x589609a9576cbe900d4afeecbd83764a5db719055826446f88c74bee62af7f81.
//
// Solidity: event TreasuryFlow(uint8 indexed direction, address indexed token, address indexed party, uint256 amount, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchTreasuryFlow(opts *bind.WatchOpts, sink chan<- *EventEmitterTreasuryFlow, direction []uint8, token []common.Address, party []common.Address) (event.Subscription, error) {

	var directionRule []interface{}
	for _, directionItem := range direction {
		directionRule = append(directionRule, directionItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var partyRule []interface{}
	for _, partyItem := range party {
		partyRule = append(partyRule, partyItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "TreasuryFlow", directionRule, tokenRule, partyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterTreasuryFlow)
				if err := _EventEmitter.contract.UnpackLog(event, "TreasuryFlow", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTreasuryFlow is a log parse operation binding the contract event 0x589609a9576cbe900d4afeecbd83764a5db719055826446f88c74bee62af7f81.
//
// Solidity: event TreasuryFlow(uint8 indexed direction, address indexed token, address indexed party, uint256 amount, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseTreasuryFlow(log types.Log) (*EventEmitterTreasuryFlow, error) {
	event := new(EventEmitterTreasuryFlow)
	if err := _EventEmitter.contract.UnpackLog(event, "TreasuryFlow", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterTypedEmitterPermissionIterator is returned from FilterTypedEmitterPermission and is used to iterate over the raw logs and unpacked data for TypedEmitterPermission events raised by the EventEmitter contract.
type EventEmitterTypedEmitterPermissionIterator struct {
	Event *EventEmitterTypedEmitterPermission // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterTypedEmitterPermissionIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterTypedEmitterPermission)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterTypedEmitterPermission)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterTypedEmitterPermissionIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterTypedEmitterPermissionIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterTypedEmitterPermission represents a TypedEmitterPermission event raised by the EventEmitter contract.
type EventEmitterTypedEmitterPermission struct {
	Emitter  common.Address
	Selector [4]byte
	Allowed  bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterTypedEmitterPermission is a free log retrieval operation binding the contract event 0xac8cad594581b394b23d152f0d6fe73d7d6611afa3458ccc51e4659ff7dbc844.
//
// Solidity: event TypedEmitterPermission(address indexed emitter, bytes4 indexed selector, bool allowed)
func (_EventEmitter *EventEmitterFilterer) FilterTypedEmitterPermission(opts *bind.FilterOpts, emitter []common.Address, selector [][4]byte) (*EventEmitterTypedEmitterPermissionIterator, error) {

	var emitterRule []interface{}
	for _, emitterItem := range emitter {
		emitterRule = append(emitterRule, emitterItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "TypedEmitterPermission", emitterRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterTypedEmitterPermissionIterator{contract: _EventEmitter.contract, event: "TypedEmitterPermission", logs: logs, sub: sub}, nil
}

// WatchTypedEmitterPermission is a free log subscription operation binding the contract event 0xac8cad594581b394b23d152f0d6fe73d7d6611afa3458ccc51e4659ff7dbc844.
//
// Solidity: event TypedEmitterPermission(address indexed emitter, bytes4 indexed selector, bool allowed)
func (_EventEmitter *EventEmitterFilterer) WatchTypedEmitterPermission(opts *bind.WatchOpts, sink chan<- *EventEmitterTypedEmitterPermission, emitter []common.Address, selector [][4]byte) (event.Subscription, error) {

	var emitterRule []interface{}
	for _, emitterItem := range emitter {
		emitterRule = append(emitterRule, emitterItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "TypedEmitterPermission", emitterRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterTypedEmitterPermission)
				if err := _EventEmitter.contract.UnpackLog(event, "TypedEmitterPermission", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTypedEmitterPermission is a log parse operation binding the contract event 0xac8cad594581b394b23d152f0d6fe73d7d6611afa3458ccc51e4659ff7dbc844.
//
// Solidity: event TypedEmitterPermission(address indexed emitter, bytes4 indexed selector, bool allowed)
func (_EventEmitter *EventEmitterFilterer) ParseTypedEmitterPermission(log types.Log) (*EventEmitterTypedEmitterPermission, error) {
	event := new(EventEmitterTypedEmitterPermission)
	if err := _EventEmitter.contract.UnpackLog(event, "TypedEmitterPermission", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterUpgradedIterator is returned from FilterUpgraded and is used to iterate over the raw logs and unpacked data for Upgraded events raised by the EventEmitter contract.
type EventEmitterUpgradedIterator struct {
	Event *EventEmitterUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterUpgraded represents a Upgraded event raised by the EventEmitter contract.
type EventEmitterUpgraded struct {
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterUpgraded is a free log retrieval operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_EventEmitter *EventEmitterFilterer) FilterUpgraded(opts *bind.FilterOpts, implementation []common.Address) (*EventEmitterUpgradedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterUpgradedIterator{contract: _EventEmitter.contract, event: "Upgraded", logs: logs, sub: sub}, nil
}

// WatchUpgraded is a free log subscription operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_EventEmitter *EventEmitterFilterer) WatchUpgraded(opts *bind.WatchOpts, sink chan<- *EventEmitterUpgraded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterUpgraded)
				if err := _EventEmitter.contract.UnpackLog(event, "Upgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUpgraded is a log parse operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_EventEmitter *EventEmitterFilterer) ParseUpgraded(log types.Log) (*EventEmitterUpgraded, error) {
	event := new(EventEmitterUpgraded)
	if err := _EventEmitter.contract.UnpackLog(event, "Upgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EventEmitterVaultFlowIterator is returned from FilterVaultFlow and is used to iterate over the raw logs and unpacked data for VaultFlow events raised by the EventEmitter contract.
type EventEmitterVaultFlowIterator struct {
	Event *EventEmitterVaultFlow // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EventEmitterVaultFlowIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EventEmitterVaultFlow)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EventEmitterVaultFlow)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EventEmitterVaultFlowIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EventEmitterVaultFlowIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EventEmitterVaultFlow represents a VaultFlow event raised by the EventEmitter contract.
type EventEmitterVaultFlow struct {
	FlowType    uint8
	Token       common.Address
	Party       common.Address
	Amount      *big.Int
	NewReserve  *big.Int
	Timestamp   *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterVaultFlow is a free log retrieval operation binding the contract event 0x4a928ce96ccd98c06c706d2dfc9e3be829d83d7df95db10dfe9d1c359ea54e78.
//
// Solidity: event VaultFlow(uint8 indexed flowType, address indexed token, address indexed party, uint256 amount, uint256 newReserve, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) FilterVaultFlow(opts *bind.FilterOpts, flowType []uint8, token []common.Address, party []common.Address) (*EventEmitterVaultFlowIterator, error) {

	var flowTypeRule []interface{}
	for _, flowTypeItem := range flowType {
		flowTypeRule = append(flowTypeRule, flowTypeItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var partyRule []interface{}
	for _, partyItem := range party {
		partyRule = append(partyRule, partyItem)
	}

	logs, sub, err := _EventEmitter.contract.FilterLogs(opts, "VaultFlow", flowTypeRule, tokenRule, partyRule)
	if err != nil {
		return nil, err
	}
	return &EventEmitterVaultFlowIterator{contract: _EventEmitter.contract, event: "VaultFlow", logs: logs, sub: sub}, nil
}

// WatchVaultFlow is a free log subscription operation binding the contract event 0x4a928ce96ccd98c06c706d2dfc9e3be829d83d7df95db10dfe9d1c359ea54e78.
//
// Solidity: event VaultFlow(uint8 indexed flowType, address indexed token, address indexed party, uint256 amount, uint256 newReserve, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) WatchVaultFlow(opts *bind.WatchOpts, sink chan<- *EventEmitterVaultFlow, flowType []uint8, token []common.Address, party []common.Address) (event.Subscription, error) {

	var flowTypeRule []interface{}
	for _, flowTypeItem := range flowType {
		flowTypeRule = append(flowTypeRule, flowTypeItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var partyRule []interface{}
	for _, partyItem := range party {
		partyRule = append(partyRule, partyItem)
	}

	logs, sub, err := _EventEmitter.contract.WatchLogs(opts, "VaultFlow", flowTypeRule, tokenRule, partyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EventEmitterVaultFlow)
				if err := _EventEmitter.contract.UnpackLog(event, "VaultFlow", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseVaultFlow is a log parse operation binding the contract event 0x4a928ce96ccd98c06c706d2dfc9e3be829d83d7df95db10dfe9d1c359ea54e78.
//
// Solidity: event VaultFlow(uint8 indexed flowType, address indexed token, address indexed party, uint256 amount, uint256 newReserve, uint256 timestamp, uint256 blockNumber)
func (_EventEmitter *EventEmitterFilterer) ParseVaultFlow(log types.Log) (*EventEmitterVaultFlow, error) {
	event := new(EventEmitterVaultFlow)
	if err := _EventEmitter.contract.UnpackLog(event, "VaultFlow", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
