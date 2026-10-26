package types

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"testing"

	gogoprotoopts "github.com/gogo/protobuf/gogoproto"
	gogoproto "github.com/gogo/protobuf/proto"
	gogodescriptor "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/api/annotations"
)

func TestPayQueryParamsResponseRoundTrip(t *testing.T) {
	request := &QueryParamsRequest{}
	encoded, err := gogoproto.Marshal(request)
	require.NoError(t, err)
	require.Empty(t, encoded)
	var decodedRequest QueryParamsRequest
	require.NoError(t, gogoproto.Unmarshal(encoded, &decodedRequest))
	require.Equal(t, request, &decodedRequest)

	for flags := 0; flags < 32; flags++ {
		t.Run(fmt.Sprintf("flags_%02d", flags), func(t *testing.T) {
			original := QueryParamsResponse{Params: Params{
				Paused: flags&1 != 0, EscrowEnabled: flags&2 != 0,
				BudgetEnabled: flags&4 != 0, StreamEnabled: flags&8 != 0,
				ServiceEnabled: flags&16 != 0,
			}}
			encoded, err := gogoproto.Marshal(&original)
			require.NoError(t, err)
			require.Len(t, encoded, original.Size())
			require.GreaterOrEqual(t, len(encoded), 2)
			require.Equal(t, byte(0x0a), encoded[0])
			var decoded QueryParamsResponse
			require.NoError(t, gogoproto.Unmarshal(encoded, &decoded))
			require.Equal(t, original, decoded)
			again, err := gogoproto.Marshal(&decoded)
			require.NoError(t, err)
			require.Equal(t, encoded, again)
		})
	}

	reader, err := gzip.NewReader(bytes.NewReader(gogoproto.FileDescriptor("layerxpay/query.proto")))
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var file descriptor.FileDescriptorProto
	require.NoError(t, proto.Unmarshal(data, &file))
	require.Equal(t, "paxprotocol.paxchain.layerxpay", file.GetPackage())
	require.Contains(t, file.Dependency, "layerxpay/tx.proto")
	require.Len(t, file.Service, 1)
	service := file.Service[0]
	require.Equal(t, "Query", service.GetName())
	require.Len(t, service.Method, 1)
	method := service.Method[0]
	require.Equal(t, "Params", method.GetName())
	require.Equal(t, ".paxprotocol.paxchain.layerxpay.QueryParamsRequest", method.GetInputType())
	require.Equal(t, ".paxprotocol.paxchain.layerxpay.QueryParamsResponse", method.GetOutputType())
	require.False(t, method.GetClientStreaming())
	require.False(t, method.GetServerStreaming())
	extension, err := proto.GetExtension(method.GetOptions(), annotations.E_Http)
	require.NoError(t, err)
	rule, ok := extension.(*annotations.HttpRule)
	require.True(t, ok)
	require.Equal(t, "/paxprotocol/paxchain/layerxpay/params", rule.GetGet())

	require.Len(t, file.MessageType, 2)
	require.Equal(t, "QueryParamsRequest", file.MessageType[0].GetName())
	require.Empty(t, file.MessageType[0].Field)
	response := file.MessageType[1]
	require.Equal(t, "QueryParamsResponse", response.GetName())
	require.Len(t, response.Field, 1)
	field := response.Field[0]
	require.Equal(t, "params", field.GetName())
	require.Equal(t, int32(1), field.GetNumber())
	require.Equal(t, descriptor.FieldDescriptorProto_TYPE_MESSAGE, field.GetType())
	require.Equal(t, ".paxprotocol.paxchain.layerxpay.Params", field.GetTypeName())
	options := &gogodescriptor.FieldOptions{}
	optionsBytes, err := proto.Marshal(field.GetOptions())
	require.NoError(t, err)
	require.NoError(t, gogoproto.Unmarshal(optionsBytes, options))
	nullable, err := gogoproto.GetExtension(options, gogoprotoopts.E_Nullable)
	require.NoError(t, err)
	require.Equal(t, false, *nullable.(*bool))

	require.Equal(t, "paxprotocol.paxchain.layerxpay.Query", _Query_serviceDesc.ServiceName)
	require.Equal(t, "layerxpay/query.proto", _Query_serviceDesc.Metadata)
	require.Len(t, _Query_serviceDesc.Methods, 1)
	require.Equal(t, "Params", _Query_serviceDesc.Methods[0].MethodName)
	require.NotNil(t, _Query_serviceDesc.Methods[0].Handler)
}
