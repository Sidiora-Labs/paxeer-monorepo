package types_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	xevm "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	xeth "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	evm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	eth "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestXevmDescriptorsDoNotCollideWithModulesEvm(t *testing.T) {
	type descriptorSource interface {
		Descriptor() ([]byte, []int)
	}
	type descriptorPair struct {
		enginePath string
		modulePath string
		engine     descriptorSource
		module     descriptorSource
	}
	pairs := []descriptorPair{
		{"xevm/config.proto", "evm/config.proto", &xevm.ChainConfig{}, &evm.ChainConfig{}},
		{"xevm/genesis.proto", "evm/genesis.proto", &xevm.GenesisState{}, &evm.GenesisState{}},
		{"xevm/gov.proto", "evm/gov.proto", &xevm.AddERCNativePointerProposal{}, &evm.AddERCNativePointerProposal{}},
		{"xevm/params.proto", "evm/params.proto", &xevm.Params{}, &evm.Params{}},
		{"xevm/query.proto", "evm/query.proto", &xevm.QueryStaticCallRequest{}, &evm.QueryStaticCallRequest{}},
		{"xevm/receipt.proto", "evm/receipt.proto", &xevm.Receipt{}, &evm.Receipt{}},
		{"xevm/tx.proto", "evm/tx.proto", &xevm.MsgEVMTransaction{}, &evm.MsgEVMTransaction{}},
		{"xevm/types.proto", "evm/types.proto", &xevm.Whitelist{}, &evm.Whitelist{}},
		{"xevm/ethtx/tx.proto", "eth/tx.proto", &xeth.AccessTuple{}, &eth.AccessTuple{}},
	}
	decode := func(data []byte) (*descriptorpb.FileDescriptorProto, error) {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		file := new(descriptorpb.FileDescriptorProto)
		if err := proto.Unmarshal(raw, file); err != nil {
			return nil, err
		}
		return file, nil
	}

	// Gogo keeps a separate registry; load its real descriptors into the
	// reflection registry as well so protobuf validates every import and type.
	var registerFile func(string) (protoreflect.FileDescriptor, error)
	registerFile = func(path string) (protoreflect.FileDescriptor, error) {
		if file, err := protoregistry.GlobalFiles.FindFileByPath(path); err == nil {
			return file, nil
		}
		data := gogoproto.FileDescriptor(path)
		if len(data) == 0 {
			return nil, fmt.Errorf("unregistered dependency %q", path)
		}
		file, err := decode(data)
		if err != nil {
			return nil, err
		}
		for _, dependency := range file.Dependency {
			if _, err := registerFile(dependency); err != nil {
				return nil, err
			}
		}
		resolved, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
		if err != nil {
			return nil, err
		}
		if err := protoregistry.GlobalFiles.RegisterFile(resolved); err != nil {
			return nil, err
		}
		return resolved, nil
	}

	engineFiles := make(map[string]*descriptorpb.FileDescriptorProto)
	moduleFiles := make(map[string]bool)
	check := func(path string, data []byte, engine bool) {
		t.Helper()
		file, err := decode(data)
		if err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if file.GetName() != path {
			t.Fatalf("descriptor name = %q, want %q", file.GetName(), path)
		}
		if !bytes.Equal(gogoproto.FileDescriptor(path), data) {
			t.Fatalf("registry entry %s differs from the generated descriptor", path)
		}
		if engine {
			if strings.HasPrefix(path, "evm/") || strings.HasPrefix(path, "eth/") {
				t.Fatalf("engine descriptor uses a module path: %s", path)
			}
			if _, exists := engineFiles[path]; exists {
				t.Fatalf("duplicate engine descriptor path: %s", path)
			}
			engineFiles[path] = file
		} else {
			moduleFiles[path] = true
		}
		for _, message := range file.MessageType {
			name := file.GetPackage() + "." + message.GetName()
			goType := gogoproto.MessageType(name)
			if goType == nil {
				t.Fatalf("%s: descriptor message %s has no registered Go type", path, name)
			}
			value := reflect.New(goType.Elem()).Interface().(gogoproto.Message)
			registered := gogoproto.MessageName(value)
			if registered != name || registered[:strings.LastIndex(registered, ".")] != file.GetPackage() {
				t.Fatalf("%s: registered name %q disagrees with descriptor %q", path, registered, name)
			}
			messageData, _ := value.(descriptorSource).Descriptor()
			if !bytes.Equal(messageData, data) {
				t.Fatalf("%s: message %s belongs to a different descriptor", path, name)
			}
		}
		for _, enum := range file.EnumType {
			name := file.GetPackage() + "." + enum.GetName()
			values := gogoproto.EnumValueMap(name)
			if len(values) != len(enum.Value) {
				t.Fatalf("%s: enum %s disagrees with its registration", path, name)
			}
			for _, value := range enum.Value {
				if number, ok := values[value.GetName()]; !ok || number != value.GetNumber() {
					t.Fatalf("%s: enum value %s.%s disagrees with its registration", path, name, value.GetName())
				}
			}
		}
		if _, err := registerFile(path); err != nil {
			t.Fatalf("register %s for reflection: %v", path, err)
		}
	}

	for _, pair := range pairs {
		engineData, _ := pair.engine.Descriptor()
		moduleData, _ := pair.module.Descriptor()
		check(pair.modulePath, moduleData, false)
		check(pair.enginePath, engineData, true)
	}
	engineEnums, _ := xevm.PointerType_ERC20.EnumDescriptor()
	moduleEnums, _ := evm.PointerType_ERC20.EnumDescriptor()
	check("evm/enums.proto", moduleEnums, false)
	check("xevm/enums.proto", engineEnums, true)

	seen := 0
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if expected, ok := engineFiles[file.Path()]; ok {
			seen++
			if moduleFiles[file.Path()] {
				t.Errorf("engine descriptor collides with a module: %s", file.Path())
			}
			if string(file.Package()) != expected.GetPackage() {
				t.Errorf("%s: reflection package = %q, want %q", file.Path(), file.Package(), expected.GetPackage())
			}
			for i := 0; i < file.Imports().Len(); i++ {
				path := file.Imports().Get(i).Path()
				if moduleFiles[path] {
					t.Errorf("%s imports a module descriptor: %s", file.Path(), path)
				}
			}
		}
		return true
	})
	if len(engineFiles) != 10 || seen != 10 {
		t.Fatalf("checked %d engine files and found %d in reflection, want 10 each", len(engineFiles), seen)
	}
}
