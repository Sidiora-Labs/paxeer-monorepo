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

// NativeLaunchpadMetaData contains all meta data concerning the NativeLaunchpad contract.
var NativeLaunchpadMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"key\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldValue\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newValue\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"ConfigUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"strategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeDistributed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"protocolCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"poolCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"epoch\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeFlow\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"oldStrategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"newStrategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"FeeStrategyChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"MarketCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"strategy\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"NftMint\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nft\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"NftTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"optical\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nftId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PoolRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"virtualReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"realReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tokenReserve\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"PoolStateUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint8\",\"name\":\"kind\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenIn\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenOut\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"intermediateUsdl\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"RouterTrade\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isBuy\",\"type\":\"bool\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amountOut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"Swap\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"salt\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"symbol\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"decimals\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalSupply\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"TokenDeployed\",\"type\":\"event\"}]",
}

// NativeLaunchpadABI is the input ABI used to generate the binding from.
// Deprecated: Use NativeLaunchpadMetaData.ABI instead.
var NativeLaunchpadABI = NativeLaunchpadMetaData.ABI

// NativeLaunchpad is an auto generated Go binding around an Ethereum contract.
type NativeLaunchpad struct {
	NativeLaunchpadCaller     // Read-only binding to the contract
	NativeLaunchpadTransactor // Write-only binding to the contract
	NativeLaunchpadFilterer   // Log filterer for contract events
}

// NativeLaunchpadCaller is an auto generated read-only Go binding around an Ethereum contract.
type NativeLaunchpadCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NativeLaunchpadTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NativeLaunchpadTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NativeLaunchpadFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NativeLaunchpadFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NativeLaunchpadSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NativeLaunchpadSession struct {
	Contract     *NativeLaunchpad  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NativeLaunchpadCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NativeLaunchpadCallerSession struct {
	Contract *NativeLaunchpadCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// NativeLaunchpadTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NativeLaunchpadTransactorSession struct {
	Contract     *NativeLaunchpadTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// NativeLaunchpadRaw is an auto generated low-level Go binding around an Ethereum contract.
type NativeLaunchpadRaw struct {
	Contract *NativeLaunchpad // Generic contract binding to access the raw methods on
}

// NativeLaunchpadCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NativeLaunchpadCallerRaw struct {
	Contract *NativeLaunchpadCaller // Generic read-only contract binding to access the raw methods on
}

// NativeLaunchpadTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NativeLaunchpadTransactorRaw struct {
	Contract *NativeLaunchpadTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNativeLaunchpad creates a new instance of NativeLaunchpad, bound to a specific deployed contract.
func NewNativeLaunchpad(address common.Address, backend bind.ContractBackend) (*NativeLaunchpad, error) {
	contract, err := bindNativeLaunchpad(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpad{NativeLaunchpadCaller: NativeLaunchpadCaller{contract: contract}, NativeLaunchpadTransactor: NativeLaunchpadTransactor{contract: contract}, NativeLaunchpadFilterer: NativeLaunchpadFilterer{contract: contract}}, nil
}

// NewNativeLaunchpadCaller creates a new read-only instance of NativeLaunchpad, bound to a specific deployed contract.
func NewNativeLaunchpadCaller(address common.Address, caller bind.ContractCaller) (*NativeLaunchpadCaller, error) {
	contract, err := bindNativeLaunchpad(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadCaller{contract: contract}, nil
}

// NewNativeLaunchpadTransactor creates a new write-only instance of NativeLaunchpad, bound to a specific deployed contract.
func NewNativeLaunchpadTransactor(address common.Address, transactor bind.ContractTransactor) (*NativeLaunchpadTransactor, error) {
	contract, err := bindNativeLaunchpad(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadTransactor{contract: contract}, nil
}

// NewNativeLaunchpadFilterer creates a new log filterer instance of NativeLaunchpad, bound to a specific deployed contract.
func NewNativeLaunchpadFilterer(address common.Address, filterer bind.ContractFilterer) (*NativeLaunchpadFilterer, error) {
	contract, err := bindNativeLaunchpad(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadFilterer{contract: contract}, nil
}

// bindNativeLaunchpad binds a generic wrapper to an already deployed contract.
func bindNativeLaunchpad(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NativeLaunchpadMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NativeLaunchpad *NativeLaunchpadRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NativeLaunchpad.Contract.NativeLaunchpadCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NativeLaunchpad *NativeLaunchpadRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NativeLaunchpad.Contract.NativeLaunchpadTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NativeLaunchpad *NativeLaunchpadRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NativeLaunchpad.Contract.NativeLaunchpadTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NativeLaunchpad *NativeLaunchpadCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NativeLaunchpad.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NativeLaunchpad *NativeLaunchpadTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NativeLaunchpad.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NativeLaunchpad *NativeLaunchpadTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NativeLaunchpad.Contract.contract.Transact(opts, method, params...)
}

// NativeLaunchpadConfigUpdatedIterator is returned from FilterConfigUpdated and is used to iterate over the raw logs and unpacked data for ConfigUpdated events raised by the NativeLaunchpad contract.
type NativeLaunchpadConfigUpdatedIterator struct {
	Event *NativeLaunchpadConfigUpdated // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadConfigUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadConfigUpdated)
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
		it.Event = new(NativeLaunchpadConfigUpdated)
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
func (it *NativeLaunchpadConfigUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadConfigUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadConfigUpdated represents a ConfigUpdated event raised by the NativeLaunchpad contract.
type NativeLaunchpadConfigUpdated struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterConfigUpdated(opts *bind.FilterOpts, key [][32]byte) (*NativeLaunchpadConfigUpdatedIterator, error) {

	var keyRule []interface{}
	for _, keyItem := range key {
		keyRule = append(keyRule, keyItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "ConfigUpdated", keyRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadConfigUpdatedIterator{contract: _NativeLaunchpad.contract, event: "ConfigUpdated", logs: logs, sub: sub}, nil
}

// WatchConfigUpdated is a free log subscription operation binding the contract event 0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e.
//
// Solidity: event ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchConfigUpdated(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadConfigUpdated, key [][32]byte) (event.Subscription, error) {

	var keyRule []interface{}
	for _, keyItem := range key {
		keyRule = append(keyRule, keyItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "ConfigUpdated", keyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadConfigUpdated)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "ConfigUpdated", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseConfigUpdated(log types.Log) (*NativeLaunchpadConfigUpdated, error) {
	event := new(NativeLaunchpadConfigUpdated)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "ConfigUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadFeeDistributedIterator is returned from FilterFeeDistributed and is used to iterate over the raw logs and unpacked data for FeeDistributed events raised by the NativeLaunchpad contract.
type NativeLaunchpadFeeDistributedIterator struct {
	Event *NativeLaunchpadFeeDistributed // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadFeeDistributedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadFeeDistributed)
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
		it.Event = new(NativeLaunchpadFeeDistributed)
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
func (it *NativeLaunchpadFeeDistributedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadFeeDistributedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadFeeDistributed represents a FeeDistributed event raised by the NativeLaunchpad contract.
type NativeLaunchpadFeeDistributed struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterFeeDistributed(opts *bind.FilterOpts, poolId [][32]byte) (*NativeLaunchpadFeeDistributedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "FeeDistributed", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadFeeDistributedIterator{contract: _NativeLaunchpad.contract, event: "FeeDistributed", logs: logs, sub: sub}, nil
}

// WatchFeeDistributed is a free log subscription operation binding the contract event 0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc.
//
// Solidity: event FeeDistributed(bytes32 indexed poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchFeeDistributed(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadFeeDistributed, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "FeeDistributed", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadFeeDistributed)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "FeeDistributed", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseFeeDistributed(log types.Log) (*NativeLaunchpadFeeDistributed, error) {
	event := new(NativeLaunchpadFeeDistributed)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "FeeDistributed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadFeeFlowIterator is returned from FilterFeeFlow and is used to iterate over the raw logs and unpacked data for FeeFlow events raised by the NativeLaunchpad contract.
type NativeLaunchpadFeeFlowIterator struct {
	Event *NativeLaunchpadFeeFlow // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadFeeFlowIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadFeeFlow)
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
		it.Event = new(NativeLaunchpadFeeFlow)
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
func (it *NativeLaunchpadFeeFlowIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadFeeFlowIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadFeeFlow represents a FeeFlow event raised by the NativeLaunchpad contract.
type NativeLaunchpadFeeFlow struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterFeeFlow(opts *bind.FilterOpts, kind []uint8, pool []common.Address, party []common.Address) (*NativeLaunchpadFeeFlowIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "FeeFlow", kindRule, poolRule, partyRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadFeeFlowIterator{contract: _NativeLaunchpad.contract, event: "FeeFlow", logs: logs, sub: sub}, nil
}

// WatchFeeFlow is a free log subscription operation binding the contract event 0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4.
//
// Solidity: event FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchFeeFlow(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadFeeFlow, kind []uint8, pool []common.Address, party []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "FeeFlow", kindRule, poolRule, partyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadFeeFlow)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "FeeFlow", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseFeeFlow(log types.Log) (*NativeLaunchpadFeeFlow, error) {
	event := new(NativeLaunchpadFeeFlow)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "FeeFlow", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadFeeStrategyChangedIterator is returned from FilterFeeStrategyChanged and is used to iterate over the raw logs and unpacked data for FeeStrategyChanged events raised by the NativeLaunchpad contract.
type NativeLaunchpadFeeStrategyChangedIterator struct {
	Event *NativeLaunchpadFeeStrategyChanged // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadFeeStrategyChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadFeeStrategyChanged)
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
		it.Event = new(NativeLaunchpadFeeStrategyChanged)
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
func (it *NativeLaunchpadFeeStrategyChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadFeeStrategyChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadFeeStrategyChanged represents a FeeStrategyChanged event raised by the NativeLaunchpad contract.
type NativeLaunchpadFeeStrategyChanged struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterFeeStrategyChanged(opts *bind.FilterOpts, poolId [][32]byte) (*NativeLaunchpadFeeStrategyChangedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "FeeStrategyChanged", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadFeeStrategyChangedIterator{contract: _NativeLaunchpad.contract, event: "FeeStrategyChanged", logs: logs, sub: sub}, nil
}

// WatchFeeStrategyChanged is a free log subscription operation binding the contract event 0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd.
//
// Solidity: event FeeStrategyChanged(bytes32 indexed poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchFeeStrategyChanged(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadFeeStrategyChanged, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "FeeStrategyChanged", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadFeeStrategyChanged)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "FeeStrategyChanged", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseFeeStrategyChanged(log types.Log) (*NativeLaunchpadFeeStrategyChanged, error) {
	event := new(NativeLaunchpadFeeStrategyChanged)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "FeeStrategyChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadMarketCreatedIterator is returned from FilterMarketCreated and is used to iterate over the raw logs and unpacked data for MarketCreated events raised by the NativeLaunchpad contract.
type NativeLaunchpadMarketCreatedIterator struct {
	Event *NativeLaunchpadMarketCreated // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadMarketCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadMarketCreated)
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
		it.Event = new(NativeLaunchpadMarketCreated)
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
func (it *NativeLaunchpadMarketCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadMarketCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadMarketCreated represents a MarketCreated event raised by the NativeLaunchpad contract.
type NativeLaunchpadMarketCreated struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterMarketCreated(opts *bind.FilterOpts, poolId [][32]byte, token []common.Address, creator []common.Address) (*NativeLaunchpadMarketCreatedIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "MarketCreated", poolIdRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadMarketCreatedIterator{contract: _NativeLaunchpad.contract, event: "MarketCreated", logs: logs, sub: sub}, nil
}

// WatchMarketCreated is a free log subscription operation binding the contract event 0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b.
//
// Solidity: event MarketCreated(bytes32 indexed poolId, address indexed token, address indexed creator, address pool, address optical, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchMarketCreated(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadMarketCreated, poolId [][32]byte, token []common.Address, creator []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "MarketCreated", poolIdRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadMarketCreated)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "MarketCreated", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseMarketCreated(log types.Log) (*NativeLaunchpadMarketCreated, error) {
	event := new(NativeLaunchpadMarketCreated)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "MarketCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadNftMintIterator is returned from FilterNftMint and is used to iterate over the raw logs and unpacked data for NftMint events raised by the NativeLaunchpad contract.
type NativeLaunchpadNftMintIterator struct {
	Event *NativeLaunchpadNftMint // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadNftMintIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadNftMint)
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
		it.Event = new(NativeLaunchpadNftMint)
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
func (it *NativeLaunchpadNftMintIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadNftMintIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadNftMint represents a NftMint event raised by the NativeLaunchpad contract.
type NativeLaunchpadNftMint struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterNftMint(opts *bind.FilterOpts, tokenId []*big.Int, creator []common.Address, pool []common.Address) (*NativeLaunchpadNftMintIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "NftMint", tokenIdRule, creatorRule, poolRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadNftMintIterator{contract: _NativeLaunchpad.contract, event: "NftMint", logs: logs, sub: sub}, nil
}

// WatchNftMint is a free log subscription operation binding the contract event 0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5.
//
// Solidity: event NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchNftMint(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadNftMint, tokenId []*big.Int, creator []common.Address, pool []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "NftMint", tokenIdRule, creatorRule, poolRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadNftMint)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "NftMint", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseNftMint(log types.Log) (*NativeLaunchpadNftMint, error) {
	event := new(NativeLaunchpadNftMint)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "NftMint", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadNftTransferIterator is returned from FilterNftTransfer and is used to iterate over the raw logs and unpacked data for NftTransfer events raised by the NativeLaunchpad contract.
type NativeLaunchpadNftTransferIterator struct {
	Event *NativeLaunchpadNftTransfer // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadNftTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadNftTransfer)
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
		it.Event = new(NativeLaunchpadNftTransfer)
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
func (it *NativeLaunchpadNftTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadNftTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadNftTransfer represents a NftTransfer event raised by the NativeLaunchpad contract.
type NativeLaunchpadNftTransfer struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterNftTransfer(opts *bind.FilterOpts, nft []common.Address, from []common.Address, to []common.Address) (*NativeLaunchpadNftTransferIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "NftTransfer", nftRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadNftTransferIterator{contract: _NativeLaunchpad.contract, event: "NftTransfer", logs: logs, sub: sub}, nil
}

// WatchNftTransfer is a free log subscription operation binding the contract event 0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3.
//
// Solidity: event NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchNftTransfer(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadNftTransfer, nft []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "NftTransfer", nftRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadNftTransfer)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "NftTransfer", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseNftTransfer(log types.Log) (*NativeLaunchpadNftTransfer, error) {
	event := new(NativeLaunchpadNftTransfer)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "NftTransfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadPoolRegisteredIterator is returned from FilterPoolRegistered and is used to iterate over the raw logs and unpacked data for PoolRegistered events raised by the NativeLaunchpad contract.
type NativeLaunchpadPoolRegisteredIterator struct {
	Event *NativeLaunchpadPoolRegistered // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadPoolRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadPoolRegistered)
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
		it.Event = new(NativeLaunchpadPoolRegistered)
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
func (it *NativeLaunchpadPoolRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadPoolRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadPoolRegistered represents a PoolRegistered event raised by the NativeLaunchpad contract.
type NativeLaunchpadPoolRegistered struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterPoolRegistered(opts *bind.FilterOpts, pool []common.Address, token []common.Address, creator []common.Address) (*NativeLaunchpadPoolRegisteredIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "PoolRegistered", poolRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadPoolRegisteredIterator{contract: _NativeLaunchpad.contract, event: "PoolRegistered", logs: logs, sub: sub}, nil
}

// WatchPoolRegistered is a free log subscription operation binding the contract event 0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7.
//
// Solidity: event PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchPoolRegistered(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadPoolRegistered, pool []common.Address, token []common.Address, creator []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "PoolRegistered", poolRule, tokenRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadPoolRegistered)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "PoolRegistered", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParsePoolRegistered(log types.Log) (*NativeLaunchpadPoolRegistered, error) {
	event := new(NativeLaunchpadPoolRegistered)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "PoolRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadPoolStateUpdatedIterator is returned from FilterPoolStateUpdated and is used to iterate over the raw logs and unpacked data for PoolStateUpdated events raised by the NativeLaunchpad contract.
type NativeLaunchpadPoolStateUpdatedIterator struct {
	Event *NativeLaunchpadPoolStateUpdated // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadPoolStateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadPoolStateUpdated)
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
		it.Event = new(NativeLaunchpadPoolStateUpdated)
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
func (it *NativeLaunchpadPoolStateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadPoolStateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadPoolStateUpdated represents a PoolStateUpdated event raised by the NativeLaunchpad contract.
type NativeLaunchpadPoolStateUpdated struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterPoolStateUpdated(opts *bind.FilterOpts, poolId [][32]byte) (*NativeLaunchpadPoolStateUpdatedIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "PoolStateUpdated", poolIdRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadPoolStateUpdatedIterator{contract: _NativeLaunchpad.contract, event: "PoolStateUpdated", logs: logs, sub: sub}, nil
}

// WatchPoolStateUpdated is a free log subscription operation binding the contract event 0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd.
//
// Solidity: event PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchPoolStateUpdated(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadPoolStateUpdated, poolId [][32]byte) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "PoolStateUpdated", poolIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadPoolStateUpdated)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "PoolStateUpdated", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParsePoolStateUpdated(log types.Log) (*NativeLaunchpadPoolStateUpdated, error) {
	event := new(NativeLaunchpadPoolStateUpdated)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "PoolStateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadRouterTradeIterator is returned from FilterRouterTrade and is used to iterate over the raw logs and unpacked data for RouterTrade events raised by the NativeLaunchpad contract.
type NativeLaunchpadRouterTradeIterator struct {
	Event *NativeLaunchpadRouterTrade // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadRouterTradeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadRouterTrade)
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
		it.Event = new(NativeLaunchpadRouterTrade)
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
func (it *NativeLaunchpadRouterTradeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadRouterTradeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadRouterTrade represents a RouterTrade event raised by the NativeLaunchpad contract.
type NativeLaunchpadRouterTrade struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterRouterTrade(opts *bind.FilterOpts, kind []uint8, pool []common.Address, sender []common.Address) (*NativeLaunchpadRouterTradeIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "RouterTrade", kindRule, poolRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadRouterTradeIterator{contract: _NativeLaunchpad.contract, event: "RouterTrade", logs: logs, sub: sub}, nil
}

// WatchRouterTrade is a free log subscription operation binding the contract event 0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650.
//
// Solidity: event RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchRouterTrade(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadRouterTrade, kind []uint8, pool []common.Address, sender []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "RouterTrade", kindRule, poolRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadRouterTrade)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "RouterTrade", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseRouterTrade(log types.Log) (*NativeLaunchpadRouterTrade, error) {
	event := new(NativeLaunchpadRouterTrade)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "RouterTrade", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadSwapIterator is returned from FilterSwap and is used to iterate over the raw logs and unpacked data for Swap events raised by the NativeLaunchpad contract.
type NativeLaunchpadSwapIterator struct {
	Event *NativeLaunchpadSwap // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadSwapIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadSwap)
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
		it.Event = new(NativeLaunchpadSwap)
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
func (it *NativeLaunchpadSwapIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadSwapIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadSwap represents a Swap event raised by the NativeLaunchpad contract.
type NativeLaunchpadSwap struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterSwap(opts *bind.FilterOpts, poolId [][32]byte, sender []common.Address) (*NativeLaunchpadSwapIterator, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "Swap", poolIdRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadSwapIterator{contract: _NativeLaunchpad.contract, event: "Swap", logs: logs, sub: sub}, nil
}

// WatchSwap is a free log subscription operation binding the contract event 0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc.
//
// Solidity: event Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchSwap(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadSwap, poolId [][32]byte, sender []common.Address) (event.Subscription, error) {

	var poolIdRule []interface{}
	for _, poolIdItem := range poolId {
		poolIdRule = append(poolIdRule, poolIdItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "Swap", poolIdRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadSwap)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "Swap", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseSwap(log types.Log) (*NativeLaunchpadSwap, error) {
	event := new(NativeLaunchpadSwap)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "Swap", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NativeLaunchpadTokenDeployedIterator is returned from FilterTokenDeployed and is used to iterate over the raw logs and unpacked data for TokenDeployed events raised by the NativeLaunchpad contract.
type NativeLaunchpadTokenDeployedIterator struct {
	Event *NativeLaunchpadTokenDeployed // Event containing the contract specifics and raw log

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
func (it *NativeLaunchpadTokenDeployedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NativeLaunchpadTokenDeployed)
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
		it.Event = new(NativeLaunchpadTokenDeployed)
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
func (it *NativeLaunchpadTokenDeployedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NativeLaunchpadTokenDeployedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NativeLaunchpadTokenDeployed represents a TokenDeployed event raised by the NativeLaunchpad contract.
type NativeLaunchpadTokenDeployed struct {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) FilterTokenDeployed(opts *bind.FilterOpts, token []common.Address, pool []common.Address, creator []common.Address) (*NativeLaunchpadTokenDeployedIterator, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.FilterLogs(opts, "TokenDeployed", tokenRule, poolRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return &NativeLaunchpadTokenDeployedIterator{contract: _NativeLaunchpad.contract, event: "TokenDeployed", logs: logs, sub: sub}, nil
}

// WatchTokenDeployed is a free log subscription operation binding the contract event 0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237.
//
// Solidity: event TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)
func (_NativeLaunchpad *NativeLaunchpadFilterer) WatchTokenDeployed(opts *bind.WatchOpts, sink chan<- *NativeLaunchpadTokenDeployed, token []common.Address, pool []common.Address, creator []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _NativeLaunchpad.contract.WatchLogs(opts, "TokenDeployed", tokenRule, poolRule, creatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NativeLaunchpadTokenDeployed)
				if err := _NativeLaunchpad.contract.UnpackLog(event, "TokenDeployed", log); err != nil {
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
func (_NativeLaunchpad *NativeLaunchpadFilterer) ParseTokenDeployed(log types.Log) (*NativeLaunchpadTokenDeployed, error) {
	event := new(NativeLaunchpadTokenDeployed)
	if err := _NativeLaunchpad.contract.UnpackLog(event, "TokenDeployed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
