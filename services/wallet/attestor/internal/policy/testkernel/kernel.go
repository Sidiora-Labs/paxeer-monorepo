// Package testkernel drives the canonical native modules and their proof exporter
// in process for the policy repair tests. It has no network or service dependency.
package testkernel

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../../include -I${SRCDIR}/../../../../../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../../../../../build -L${SRCDIR}/../../../../../../programs/target/release -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include "kernel.h"
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type Account struct {
	ID, Asset, Authority [32]byte
	Balance              uint64
}

func ptr(b []byte) *C.uint8_t { return (*C.uint8_t)(unsafe.Pointer(&b[0])) }
func check(rc C.int) error {
	if rc != 0 {
		return fmt.Errorf("native kernel result %d", int(rc))
	}
	return nil
}

var fixtureMu sync.Mutex
var nativeCalls chan func()

// The state store binds its writer to the creating OS thread. Go test subtests
// and their callers may use different goroutines, so the entire native fixture
// lives on one worker, including proof reads and destruction.
func Open() error       { return open(false) }
func OpenNative() error { return open(true) }

func open(native bool) error {
	fixtureMu.Lock()
	nativeCalls = make(chan func())
	go func(calls <-chan func()) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		for call := range calls {
			call()
		}
	}(nativeCalls)
	mode := C.uint(0)
	if native {
		mode = 1
	}
	if err := nativeResult(func() C.int { return C.repair244_init(mode) }); err != nil {
		close(nativeCalls)
		nativeCalls = nil
		fixtureMu.Unlock()
		return err
	}
	return nil
}

func onKernel(call func()) {
	done := make(chan struct{})
	nativeCalls <- func() { call(); close(done) }
	<-done
}

func nativeResult(call func() C.int) error {
	var rc C.int
	onKernel(func() { rc = call() })
	return check(rc)
}

func Close() {
	onKernel(func() { C.repair244_close() })
	close(nativeCalls)
	nativeCalls = nil
	fixtureMu.Unlock()
}
func Info(which uint) (Account, error) {
	var a Account
	var balance C.uint64_t
	err := nativeResult(func() C.int {
		return C.repair244_info(C.uint(which), ptr(a.ID[:]), ptr(a.Asset[:]), ptr(a.Authority[:]), &balance)
	})
	a.Balance = uint64(balance)
	return a, err
}
func Root() ([32]byte, error) {
	var root [32]byte
	err := nativeResult(func() C.int { return C.repair244_root(ptr(root[:])) })
	return root, err
}
func Proof(module uint, key []byte) ([]byte, error) {
	out := make([]byte, 1<<21)
	var length C.size_t
	if err := nativeResult(func() C.int {
		return C.repair244_proof(C.uint(module), ptr(key), C.size_t(len(key)), ptr(out), C.size_t(len(out)), &length)
	}); err != nil {
		return nil, err
	}
	return out[:int(length)], nil
}

// Execute accepts the same canonical ABI bytes inspected by monetary policy,
// transports every bytes32/uint128 field into the native module payload, and
// executes the actual module/ledger. Policy results never drive the execution.
func Execute(domain string, data []byte) error {
	to, ok := evm.PrecompileAddress(domain)
	if !ok {
		return fmt.Errorf("unknown domain %s", domain)
	}
	call, err := evm.DecodeCalldata(to, data)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	word := func(name string) { id := call.Args[name].([32]byte); b.Write(id[:]) }
	u128 := func(name string) { raw := make([]byte, 16); call.Args[name].(*big.Int).FillBytes(raw); b.Write(raw) }
	u64 := func(name string) { _ = binary.Write(&b, binary.BigEndian, call.Args[name].(uint64)) }
	native := C.uint(0)
	switch call.Method {
	case "budgetCreate", "budgetCreateNative":
		version := uint16(1)
		if call.Method == "budgetCreateNative" {
			version = 2
			native = 1
		}
		_ = binary.Write(&b, binary.BigEndian, version)
		for _, name := range []string{"budgetId", "budgetAccount", "assetId", "purposeHash"} {
			word(name)
		}
		for _, name := range []string{"perPeriodLimit", "carryCap", "amount"} {
			u128(name)
		}
		for _, name := range []string{"periodLengthMs", "periodStartMs", "expiryMs", "revocationSequence"} {
			u64(name)
		}
		b.WriteByte(call.Args["rolloverPolicy"].(uint8))
		if native != 0 {
			word("sourceAccount")
			u64("sourceSequence")
		}
	case "budgetFund", "budgetFundNative":
		version := uint16(1)
		if call.Method == "budgetFundNative" {
			version = 2
			native = 1
		}
		_ = binary.Write(&b, binary.BigEndian, version)
		word("budgetId")
		u128("amount")
		if native != 0 {
			u64("sourceSequence")
		}
	case "escrowOpen":
		for _, name := range []string{"escrowId", "owner", "escrowAccount", "beneficiary", "arbiter", "assetId"} {
			word(name)
		}
		u128("amount")
		u64("expiryMs")
		u64("disputeWindowMs")
		word("termsHash")
		word("agreementReference")
		payload := b.Bytes()
		return nativeResult(func() C.int { return C.repair244_escrow(ptr(payload), C.size_t(len(payload))) })
	case "transfer", "sandboxFund":
		type leg struct {
			From, Asset, To [32]byte
			Amount          *big.Int
		}
		name := "legs"
		if call.Method == "sandboxFund" {
			name = "fundingLegs"
		}
		legs := *abi.ConvertType(call.Args[name], new([]leg)).(*[]leg)
		for _, l := range legs {
			raw := make([]byte, 16)
			l.Amount.FillBytes(raw)
			if err := nativeResult(func() C.int {
				return C.repair244_transfer(ptr(l.From[:]), ptr(l.Asset[:]), ptr(l.To[:]), ptr(raw))
			}); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported native test execution %s", call.Method)
	}
	payload := b.Bytes()
	return nativeResult(func() C.int { return C.repair244_budget(native, ptr(payload), C.size_t(len(payload))) })
}
