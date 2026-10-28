package main

import (
	"strings"
	"testing"
)

func TestValidateMessageTypesRejectsRetiredWasmOcc(t *testing.T) {
	if err := (Config{MessageTypes: []string{Bank, EVM, WasmInstantiate}}).ValidateMessageTypes(); err != nil {
		t.Fatalf("supported message types rejected: %v", err)
	}

	retired := []string{
		"wasm_occ_iterator_write",
		"wasm_occ_iterator_range",
		"wasm_occ_parallel_write",
	}
	for _, messageType := range retired {
		t.Run(messageType, func(t *testing.T) {
			for _, messageTypes := range [][]string{{messageType}, {Bank, messageType, WasmInstantiate, EVM}} {
				err := (Config{MessageTypes: messageTypes}).ValidateMessageTypes()
				if err == nil {
					t.Fatalf("retired message type %q accepted in %q", messageType, messageTypes)
				}
				if !strings.Contains(err.Error(), `"`+messageType+`"`) {
					t.Errorf("error %q does not name retired message type %q", err, messageType)
				}
			}
		})
	}

	err := (Config{MessageTypes: retired}).ValidateMessageTypes()
	if err == nil {
		t.Fatal("all retired wasm OCC message types accepted")
	}
	for _, messageType := range retired {
		if !strings.Contains(err.Error(), `"`+messageType+`"`) {
			t.Errorf("error %q does not name retired message type %q", err, messageType)
		}
	}
}
