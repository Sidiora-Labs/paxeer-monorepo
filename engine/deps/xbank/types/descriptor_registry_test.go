package types_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/stretchr/testify/require"

	xbank "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	bank "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestXbankDescriptorsLeaveCosmosBank(t *testing.T) {
	const protoPackage = "paxprotocol.paxchain.gigabank"
	const protoPath = "paxprotocol/paxchain/gigabank/"
	type message interface {
		proto.Message
		Descriptor() ([]byte, []int)
	}
	decode := func(t *testing.T, data []byte) *descriptor.FileDescriptorProto {
		t.Helper()
		require.NotEmpty(t, data)
		reader, err := gzip.NewReader(bytes.NewReader(data))
		require.NoError(t, err)
		decoded, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		file := new(descriptor.FileDescriptorProto)
		require.NoError(t, proto.Unmarshal(decoded, file))
		return file
	}

	files := []struct {
		name      string
		canonical message
		messages  []message
		fields    []string
	}{
		{
			name: "authz.proto", canonical: &bank.SendAuthorization{},
			messages: []message{&xbank.SendAuthorization{}},
			fields:   []string{"spend_limit:1"},
		},
		{
			name: "bank.proto", canonical: &bank.Params{},
			messages: []message{
				&xbank.Params{}, &xbank.SendEnabled{}, &xbank.Input{}, &xbank.Output{},
				&xbank.Supply{}, &xbank.DenomUnit{}, &xbank.Metadata{}, &xbank.AllowList{},
			},
			fields: []string{
				"send_enabled:1 default_send_enabled:2", "denom:1 enabled:2", "address:1 coins:2", "address:1 coins:2",
				"total:1", "denom:1 exponent:2 aliases:3", "description:1 denom_units:2 base:3 display:4 name:5 symbol:6", "addresses:1",
			},
		},
		{
			name: "genesis.proto", canonical: &bank.GenesisState{},
			messages: []message{&xbank.GenesisState{}, &xbank.Balance{}, &xbank.WeiBalance{}},
			fields: []string{
				"params:1 balances:2 supply:3 denom_metadata:4 wei_balances:5", "address:1 coins:2", "address:1 amount:2",
			},
		},
	}
	for _, file := range files {
		t.Run(file.name, func(t *testing.T) {
			registered := proto.FileDescriptor(protoPath + file.name)
			fd := decode(t, registered)
			require.Equal(t, protoPath+file.name, fd.GetName())
			require.False(t, strings.HasPrefix(fd.GetName(), "cosmos/bank/"))
			require.Equal(t, protoPackage, fd.GetPackage())
			require.NotContains(t, fd.GetPackage(), "-")
			require.Equal(t, "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types", fd.GetOptions().GetGoPackage())
			require.Len(t, fd.GetMessageType(), len(file.messages))
			for _, dependency := range fd.GetDependency() {
				require.False(t, strings.HasPrefix(dependency, "cosmos/bank/"))
				require.NotEmpty(t, proto.FileDescriptor(dependency), "unregistered dependency %s", dependency)
			}
			if file.name == "genesis.proto" {
				require.Contains(t, fd.GetDependency(), protoPath+"bank.proto")
			}
			for i, msg := range file.messages {
				goType := reflect.TypeOf(msg)
				name := goType.Elem().Name()
				t.Run(name, func(t *testing.T) {
					data, indexes := msg.Descriptor()
					require.Equal(t, registered, data)
					require.Equal(t, []int{i}, indexes)
					md := fd.GetMessageType()[i]
					require.Equal(t, name, md.GetName())
					fullName := protoPackage + "." + name
					require.Equal(t, fullName, proto.MessageName(msg))
					require.Equal(t, goType, proto.MessageType(fullName))
					var fields []string
					for j, field := range md.GetField() {
						number := strconv.Itoa(int(field.GetNumber()))
						fields = append(fields, field.GetName()+":"+number)
						goTag := strings.Split(goType.Elem().Field(j).Tag.Get("protobuf"), ",")
						require.GreaterOrEqual(t, len(goTag), 4)
						require.Equal(t, number, goTag[1])
						require.Contains(t, goTag, "name="+field.GetName())
						if field.GetTypeName() != "" {
							require.NotContains(t, field.GetTypeName(), ".cosmos.bank.")
							require.NotContains(t, field.GetTypeName(), "-")
							require.NotNil(t, proto.MessageType(strings.TrimPrefix(field.GetTypeName(), ".")))
						}
					}
					require.Equal(t, file.fields[i], strings.Join(fields, " "))
				})
			}
			canonicalData, _ := file.canonical.Descriptor()
			require.Equal(t, canonicalData, proto.FileDescriptor("cosmos/bank/v1beta1/"+file.name))
			require.Equal(t, "cosmos.bank.v1beta1", decode(t, canonicalData).GetPackage())
		})
	}
}
