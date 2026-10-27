package main

import (
	"strings"
	"testing"
)

func TestValidateMessageTypesRejectsUnknownType(t *testing.T) {
	valid := []string{
		Bank, EVM, ERC20, ERC721, CollectRewards, DistributeRewards,
		FailureBankMalformed, FailureBankInvalid, FailureDexMalformed, FailureDexInvalid,
		Dex, Staking, Tokenfactory, Limit, Market, WasmMintNft, Vortex, WasmInstantiate,
		WasmOccIteratorWrite, WasmOccIteratorRange, WasmOccParallelWrite,
	}
	for _, messageTypes := range [][]string{nil, {}, valid} {
		if err := (Config{MessageTypes: messageTypes}).ValidateMessageTypes(); err != nil {
			t.Fatalf("valid message types %q rejected: %v", messageTypes, err)
		}
	}

	for _, unknown := range [][]string{{"nosuchtype"}, {"nosuchtype", "another_unknown", "", "EVM"}} {
		config := Config{MessageTypes: append([]string{Bank}, unknown...)}
		err := config.ValidateMessageTypes()
		if err == nil {
			t.Fatalf("expected error for unknown message types %q", unknown)
		}
		for _, messageType := range unknown {
			if !strings.Contains(err.Error(), `"`+messageType+`"`) {
				t.Errorf("error %q does not name unknown message type %q", err, messageType)
			}
		}
	}

	t.Run("run validates before startup", func(t *testing.T) {
		config := Config{MessageTypes: []string{ERC20, "nosuchtype"}}
		defer func() {
			failure := recover()
			err, ok := failure.(error)
			if !ok || !strings.Contains(err.Error(), "nosuchtype") {
				t.Fatalf("expected validation panic naming nosuchtype, got %v", failure)
			}
			if config.EVMAddresses != nil {
				t.Fatal("run initialized EVM addresses before rejecting invalid message types")
			}
		}()
		run(&config)
		t.Fatal("run accepted an unknown message type")
	})
}

func TestValidateMessageTypesRejectsRetiredUniv2(t *testing.T) {
	for _, messageTypes := range [][]string{{"univ2"}, {Bank, "univ2", EVM}} {
		err := (Config{MessageTypes: messageTypes}).ValidateMessageTypes()
		if err == nil {
			t.Fatalf("expected retired univ2 message type to be rejected in %q", messageTypes)
		}
		if !strings.Contains(err.Error(), "univ2") {
			t.Fatalf("error %q does not name retired message type univ2", err)
		}
	}
}
