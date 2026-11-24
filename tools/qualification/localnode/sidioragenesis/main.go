package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
)

func install(path, proposalPath, exportPath string) error {
	proposalData, err := os.ReadFile(proposalPath)
	if err != nil {
		return err
	}
	var proposal struct {
		Messages []struct {
			Token, Pointer string
			Version        uint16
		}
	}
	if err := json.Unmarshal(proposalData, &proposal); err != nil {
		return err
	}
	if len(proposal.Messages) != 1 {
		return fmt.Errorf("Sidiora requires one native pointer binding")
	}
	binding := proposal.Messages[0]
	address := common.HexToAddress(binding.Pointer)
	args, err := native.GetParsedABI().Pack("", binding.Token, "Sidiora", "SID", uint8(6))
	if err != nil {
		return err
	}
	db, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		return err
	}
	storage := map[common.Hash]common.Hash{}
	hooked := state.NewHookedState(db, &tracing.Hooks{OnStorageChange: func(_ common.Address, slot, _, value common.Hash) { storage[slot] = value }})
	code, _, _, err := runtime.Create(append(native.GetBin(), args...), &runtime.Config{State: hooked, GasLimit: 10_000_000})
	if err != nil {
		return fmt.Errorf("create Sidiora native pointer: %w", err)
	}
	if len(code) == 0 {
		return fmt.Errorf("Sidiora native pointer has empty runtime")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	var app map[string]json.RawMessage
	if err := json.Unmarshal(doc["app_state"], &app); err != nil {
		return err
	}
	var rawEVM map[string]json.RawMessage
	if err := json.Unmarshal(app["evm"], &rawEVM); err != nil {
		return err
	}
	var evm struct {
		Codes      []*evmtypes.Code          `json:"codes"`
		States     []*evmtypes.ContractState `json:"states"`
		Serialized []*evmtypes.Serialized    `json:"serialized"`
	}
	if err := json.Unmarshal(app["evm"], &evm); err != nil {
		return err
	}
	baselineData, err := os.ReadFile(exportPath)
	if err != nil {
		return err
	}
	var baseline struct {
		AppState struct {
			EVM struct {
				Codes []*evmtypes.Code `json:"codes"`
			} `json:"evm"`
		} `json:"app_state"`
	}
	if err := json.Unmarshal(baselineData, &baseline); err != nil {
		return err
	}
	var exportedCode []byte
	for _, entry := range baseline.AppState.EVM.Codes {
		if common.HexToAddress(entry.Address) == address {
			exportedCode = entry.Code
		}
	}
	codes := evm.Codes[:0]
	for _, entry := range evm.Codes {
		if common.HexToAddress(entry.Address) == address {
			if !bytes.Equal(entry.Code, exportedCode) {
				return fmt.Errorf("Sidiora code differs from the exported initial state")
			}
			continue
		}
		codes = append(codes, entry)
	}
	evm.Codes = append(codes, &evmtypes.Code{Address: address.Hex(), Code: code})
	states := evm.States[:0]
	for _, entry := range evm.States {
		_, initialized := storage[common.BytesToHash(entry.Key)]
		if common.HexToAddress(entry.Address) != address || !initialized {
			states = append(states, entry)
		}
	}
	evm.States = states
	var slots []common.Hash
	for slot := range storage {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return bytes.Compare(slots[i][:], slots[j][:]) < 0 })
	for _, slot := range slots {
		value := storage[slot]
		evm.States = append(evm.States, &evmtypes.ContractState{Address: address.Hex(), Key: slot.Bytes(), Value: value.Bytes()})
	}
	forward := evmtypes.PointerERC20NativeKey(binding.Token)
	reverse := evmtypes.PointerReverseRegistryKey(address)
	entries := evm.Serialized[:0]
	for _, entry := range evm.Serialized {
		key := append(append([]byte(nil), entry.Prefix...), entry.Key...)
		if bytes.HasPrefix(key, forward) || bytes.HasPrefix(key, reverse) || (bytes.HasPrefix(key, evmtypes.PointerReverseRegistryPrefix) && bytes.Equal(entry.Value, []byte(binding.Token))) {
			continue
		}
		entries = append(entries, entry)
	}
	version := []byte{byte(binding.Version >> 8), byte(binding.Version)}
	evm.Serialized = append(entries,
		&evmtypes.Serialized{Prefix: forward, Key: version, Value: address.Bytes()},
		&evmtypes.Serialized{Prefix: reverse, Key: version, Value: []byte(binding.Token)})
	rawEVM["codes"], err = json.Marshal(evm.Codes)
	if err != nil {
		return err
	}
	rawEVM["states"], err = json.Marshal(evm.States)
	if err != nil {
		return err
	}
	rawEVM["serialized"], err = json.Marshal(evm.Serialized)
	if err != nil {
		return err
	}
	app["evm"], err = json.Marshal(rawEVM)
	if err != nil {
		return err
	}
	doc["app_state"], err = json.Marshal(app)
	if err != nil {
		return err
	}
	data, err = json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: sidioragenesis genesis.json pointer-proposal.json export.json")
		os.Exit(1)
	}
	if err := install(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
