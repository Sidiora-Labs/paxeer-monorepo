package api_test

import (
	"bytes"
	"reflect"
	"regexp"
	"strings"
	"testing"

	xbank "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	bank "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type xbankDescriptorMessage interface {
	gogoproto.Message
	Descriptor() ([]byte, []int)
}

func FileDescriptor(t *testing.T, name string, message xbankDescriptorMessage) *descriptorpb.FileDescriptorProto {
	t.Helper()
	want, _ := message.Descriptor()
	got := gogoproto.FileDescriptor(name)
	if len(want) == 0 || len(got) == 0 {
		t.Fatalf("%s: missing representative or registered descriptor", name)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: registered descriptor differs from %T.Descriptor", name, message)
	}
	file := decodeFile(t, got)
	if file.GetName() != name {
		t.Fatalf("%s: registered descriptor names %q", name, file.GetName())
	}
	return file
}

func RegisterType(t *testing.T, packageName, messageName string, message gogoproto.Message) {
	t.Helper()
	want := packageName + "." + messageName
	if got := gogoproto.MessageName(message); got != want {
		t.Errorf("%T: RegisterType name = %q, descriptor requires %q", message, got, want)
	}
	if got := gogoproto.MessageType(want); got != reflect.TypeOf(message) {
		t.Errorf("%s: registered Go type = %v, want %T", want, got, message)
	}
}

func TestXbankDescriptorsDoNotShadowBankFiles(t *testing.T) {
	cases := []struct {
		name  string
		bank  xbankDescriptorMessage
		xbank []xbankDescriptorMessage
	}{
		{
			name: "cosmos/bank/v1beta1/authz.proto",
			bank: &bank.SendAuthorization{},
			xbank: []xbankDescriptorMessage{
				&xbank.SendAuthorization{},
			},
		},
		{
			name: "cosmos/bank/v1beta1/bank.proto",
			bank: &bank.Params{},
			xbank: []xbankDescriptorMessage{
				&xbank.Params{},
				&xbank.SendEnabled{},
				&xbank.Input{},
				&xbank.Output{},
				&xbank.Supply{},
				&xbank.DenomUnit{},
				&xbank.Metadata{},
				&xbank.AllowList{},
			},
		},
		{
			name: "cosmos/bank/v1beta1/genesis.proto",
			bank: &bank.GenesisState{},
			xbank: []xbankDescriptorMessage{
				&xbank.GenesisState{},
				&xbank.Balance{},
				&xbank.WeiBalance{},
			},
		},
	}

	validPackage := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	xbankFiles := make(map[string]string)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			FileDescriptor(t, tc.name, tc.bank)

			file := generatedFile(t, tc.xbank[0])
			name := file.GetName()
			if name == "" || strings.HasPrefix(name, "cosmos/bank/") {
				t.Fatalf("%T: xbank descriptor must name a file outside cosmos/bank/, got %q", tc.xbank[0], name)
			}
			if previous, exists := xbankFiles[name]; exists {
				t.Fatalf("%s and %s share xbank file name %q", previous, tc.name, name)
			}
			xbankFiles[name] = tc.name
			FileDescriptor(t, name, tc.xbank[0])

			packageName := file.GetPackage()
			if !validPackage.MatchString(packageName) {
				t.Errorf("%s: invalid proto package %q", name, packageName)
			}
			messages := file.GetMessageType()
			if len(messages) != len(tc.xbank) {
				t.Fatalf("%s: descriptor declares %d messages, checked Go types cover %d", name, len(messages), len(tc.xbank))
			}
			fileBytes, _ := tc.xbank[0].Descriptor()
			for i, message := range tc.xbank {
				data, indexes := message.Descriptor()
				if !bytes.Equal(data, fileBytes) || len(indexes) != 1 || indexes[0] != i {
					t.Fatalf("%T: descriptor does not identify message %d in %s", message, i, name)
				}
				RegisterType(t, packageName, messages[i].GetName(), message)
			}
		})
	}

	t.Run("no interface implementations or Any type URLs", func(t *testing.T) {
		registry := codectypes.NewInterfaceRegistry()
		xbank.RegisterInterfaces(registry)
		if !reflect.DeepEqual(registry, codectypes.NewInterfaceRegistry()) {
			t.Error("xbank.RegisterInterfaces changed an empty interface registry")
		}
		for _, tc := range cases {
			for _, message := range tc.xbank {
				typeURL := "/" + gogoproto.MessageName(message)
				if resolved, err := registry.Resolve(typeURL); err == nil || resolved != nil {
					t.Errorf("xbank codec resolves Any type URL %q to %T (error %v)", typeURL, resolved, err)
				}
			}
		}
	})
}
