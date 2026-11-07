package types_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParamsDescriptorReservesRemovedFields(t *testing.T) {
	files := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto),
	}}
	seen := map[string]bool{"google/protobuf/descriptor.proto": true}
	var load func(string)
	load = func(path string) {
		t.Helper()
		if seen[path] {
			return
		}
		seen[path] = true
		data := gogoproto.FileDescriptor(path)
		if len(data) == 0 {
			t.Fatalf("missing registered descriptor %q", path)
		}
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("open descriptor %q: %v", path, err)
		}
		raw, err := io.ReadAll(r)
		closeErr := r.Close()
		if err != nil {
			t.Fatalf("read descriptor %q: %v", path, err)
		}
		if closeErr != nil {
			t.Fatalf("close descriptor %q: %v", path, closeErr)
		}
		file := new(descriptorpb.FileDescriptorProto)
		if err := proto.Unmarshal(raw, file); err != nil {
			t.Fatalf("decode descriptor %q: %v", path, err)
		}
		files.File = append(files.File, file)
		for _, dependency := range file.GetDependency() {
			load(dependency)
		}
	}
	load("evm/params.proto")
	registry, err := protodesc.NewFiles(files)
	if err != nil {
		t.Fatalf("create descriptor registry: %v", err)
	}
	descriptor, err := registry.FindDescriptorByName("paxprotocol.paxchain.evm.Params")
	if err != nil {
		t.Fatalf("find Params descriptor: %v", err)
	}
	params, ok := descriptor.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("Params descriptor has type %T, want message", descriptor)
	}
	for _, number := range []protoreflect.FieldNumber{1, 5, 6, 7} {
		if !params.ReservedRanges().Has(number) {
			t.Errorf("Params does not reserve field number %d", number)
		}
		if field := params.Fields().ByNumber(number); field != nil {
			t.Errorf("Params field %s reuses reserved number %d", field.Name(), number)
		}
	}
	for _, name := range []protoreflect.Name{"base_denom", "chain_config", "chain_id", "whitelisted_codehashes_bank_send"} {
		if !params.ReservedNames().Has(name) {
			t.Errorf("Params does not reserve field name %q", name)
		}
		if field := params.Fields().ByName(name); field != nil {
			t.Errorf("Params field %s reuses a reserved name", field.Name())
		}
	}
}
