package decode

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func chainLaunchSourceABI(t *testing.T) abi.ABI {
	t.Helper()
	data, err := os.ReadFile("../../../../../precompiles/launchpad/abi.json")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestChainLaunchEventsDecodeWithTheNamesOfTheAbi(t *testing.T) {
	source := chainLaunchSourceABI(t)
	precompile, _ := deployment(t, "EventEmitter", LaunchpadPrecompileAddress)
	precompile.Role = LaunchpadPrecompileRole
	r := registry(t, precompile)
	legacy := NewDecoder()
	for _, name := range []string{"ChainLaunched", "ChainRevenueEpochOpened", "ChainRevenueClaimed", "LaunchedChainRetired"} {
		t.Run(name, func(t *testing.T) {
			event, ok := source.Events[name]
			if !ok {
				t.Fatal("event absent from launchpad ABI")
			}
			compiled, ok := chainLaunchEvents[event.ID]
			if !ok || len(compiled.Inputs) != len(event.Inputs) {
				t.Fatal("event layout differs from launchpad ABI")
			}
			for i, input := range event.Inputs {
				if compiled.Inputs[i].Name != input.Name || compiled.Inputs[i].Type.String() != input.Type.String() || compiled.Inputs[i].Indexed != input.Indexed {
					t.Fatalf("input %d differs from launchpad ABI", i)
				}
			}
			log, want := syntheticLog(t, event, LaunchpadPrecompileAddress)
			got, err := r.Decode(log)
			if err != nil || got.Event == nil || got.EventName != name || got.EmitterRole != LaunchpadPrecompileRole || !reflect.DeepEqual(got.Event.Args, want) {
				t.Fatalf("registry decode: %#v, %v", got, err)
			}
			if len(got.Event.Args) != len(event.Inputs) {
				t.Fatalf("decoded %d fields, ABI has %d", len(got.Event.Args), len(event.Inputs))
			}
			for _, input := range event.Inputs {
				if _, ok := got.Event.Args[input.Name]; !ok {
					t.Fatalf("missing ABI field %s", input.Name)
				}
			}
			row, err := legacy.Decode(log)
			if err != nil || row == nil || row.EventName != name || len(row.Args) != len(want) {
				t.Fatalf("processor decode: %#v, %v", row, err)
			}
			for key, value := range want {
				if fmt.Sprint(row.Args[key]) != fmt.Sprint(value) {
					t.Fatalf("processor field %s: %v, want %v", key, row.Args[key], value)
				}
			}
		})
	}
}

func TestChainLaunchEventWithShortDataIsRefused(t *testing.T) {
	event := chainLaunchSourceABI(t).Events["ChainLaunched"]
	log, _ := syntheticLog(t, event, LaunchpadPrecompileAddress)
	log.Data = log.Data[:len(log.Data)-1]
	precompile, _ := deployment(t, "EventEmitter", LaunchpadPrecompileAddress)
	precompile.Role = LaunchpadPrecompileRole
	r := registry(t, precompile)
	if row, err := r.Decode(log); err == nil || row.Event != nil {
		t.Fatalf("short event accepted by registry: %#v, %v", row, err)
	}
	if row, err := NewDecoder().Decode(log); err == nil || row != nil {
		t.Fatalf("short event accepted by processor decoder: %#v, %v", row, err)
	}
}
