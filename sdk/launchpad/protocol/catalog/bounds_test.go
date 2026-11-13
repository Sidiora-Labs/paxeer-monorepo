package catalog

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"testing"
)

func TestBoundedDynamicArrayBeforeUnpack(t *testing.T) {
	typ, e := abi.NewType("uint256[]", "", nil)
	if e != nil {
		t.Fatal(e)
	}
	args := abi.Arguments{{Type: typ}}
	data := make([]byte, 64)
	data[31] = 32
	binary.BigEndian.PutUint64(data[56:], 1<<60)
	if bounded(args, data) {
		t.Fatal("hostile allocation accepted")
	}
	binary.BigEndian.PutUint64(data[56:], 4097)
	if bounded(args, data) {
		t.Fatal("array bound accepted")
	}
	binary.BigEndian.PutUint64(data[56:], 0)
	if !bounded(args, data) {
		t.Fatal("empty array rejected")
	}
	data[31] = 33
	if bounded(args, data) {
		t.Fatal("unaligned offset accepted")
	}
}

func TestDynamicOffsetsCannotAliasTheirHead(t *testing.T) {
	typ, err := abi.NewType("bytes", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Type: typ}}
	data := make([]byte, 64)
	if bounded(args, data) {
		t.Fatal("dynamic head alias accepted")
	}
	data[31] = 32
	if !bounded(args, data) {
		t.Fatal("canonical empty bytes rejected")
	}
}

func TestArgumentsBoundedFixedAndNestedArrayWidths(t *testing.T) {
	fixed, err := abi.NewType("uint256[2]", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ArgumentsBounded(abi.Arguments{{Type: fixed}}, make([]byte, 32)) {
		t.Fatal("truncated fixed array accepted")
	}
	if !ArgumentsBounded(abi.Arguments{{Type: fixed}}, make([]byte, 64)) {
		t.Fatal("complete fixed array rejected")
	}

	nested, err := abi.NewType("uint256[2][]", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 128)
	data[31] = 32
	data[63] = 1
	if !ArgumentsBounded(abi.Arguments{{Type: nested}}, data) {
		t.Fatal("complete dynamic array of fixed arrays rejected")
	}
	if ArgumentsBounded(abi.Arguments{{Type: nested}}, data[:96]) {
		t.Fatal("truncated nested array accepted")
	}
}
