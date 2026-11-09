package lib

import (
	"reflect"
	"testing"
)

func TestInitEvmoneVMLoadsThePinnedLibrary(t *testing.T) {
	vm, err := InitEvmoneVM()
	if err != nil {
		t.Fatalf("InitEvmoneVM(): %v (build the library with make evmone-lib)", err)
	}
	if vm == nil {
		t.Fatal("InitEvmoneVM() returned a nil VM")
	}
	defer vm.Destroy()

	if got := vm.Name(); got != "evmone" {
		t.Fatalf("VM name = %q, want evmone", got)
	}
	if got := vm.Version(); got != "0.12.0" {
		t.Fatalf("VM version = %q, want 0.12.0", got)
	}

	// The v12.1.0 Go binding's EVMC_ABI_VERSION is 12. Its loader checks
	// compatibility, but exposes no ABI accessor; inspect the real C VM field.
	const bindingABIVersion = 12
	abi := reflect.ValueOf(vm).Elem().FieldByName("handle").Elem().FieldByName("abi_version").Int()
	if abi != bindingABIVersion {
		t.Fatalf("VM ABI version = %d, want binding ABI %d", abi, bindingABIVersion)
	}
}
