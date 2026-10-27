package types_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/gogo/protobuf/gogoproto"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	goproto "github.com/golang/protobuf/proto"
	godescriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/api/annotations"
)

func TestQueryServiceDeclaresTheXWebReads(t *testing.T) {
	compressed := proto.FileDescriptor("xweb/query.proto")
	require.NotEmpty(t, compressed)
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	raw, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var file descriptor.FileDescriptorProto
	require.NoError(t, proto.Unmarshal(raw, &file))
	require.Equal(t, "paxprotocol.paxchain.xweb", file.GetPackage())
	require.Equal(t, "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types", file.GetOptions().GetGoPackage())
	require.Len(t, file.Service, 1)
	service := file.Service[0]
	require.Equal(t, "Query", service.GetName())
	want := []struct{ name, path string }{
		{"Params", "/pax-protocol/paxchain/xweb/params"},
		{"Attestors", "/pax-protocol/paxchain/xweb/attestors"},
		{"Request", "/pax-protocol/paxchain/xweb/request/{request_id}"},
		{"Result", "/pax-protocol/paxchain/xweb/result/{request_id}"},
		{"ProgramRequest", "/pax-protocol/paxchain/xweb/program_request/{program_id}/{request_id}"},
		{"ProgramResult", "/pax-protocol/paxchain/xweb/program_result/{program_id}/{request_id}"},
		{"ProgramCounters", "/pax-protocol/paxchain/xweb/program_counters"},
	}
	require.Len(t, service.Method, len(want))
	for i, expected := range want {
		t.Run(expected.name, func(t *testing.T) {
			method := service.Method[i]
			require.Equal(t, expected.name, method.GetName())
			require.Equal(t, ".paxprotocol.paxchain.xweb.Query"+expected.name+"Request", method.GetInputType())
			require.Equal(t, ".paxprotocol.paxchain.xweb.Query"+expected.name+"Response", method.GetOutputType())
			require.False(t, method.GetClientStreaming())
			require.False(t, method.GetServerStreaming())
			options, err := proto.Marshal(method.GetOptions())
			require.NoError(t, err)
			var httpOptions godescriptor.MethodOptions
			require.NoError(t, goproto.Unmarshal(options, &httpOptions))
			extension, err := goproto.GetExtension(&httpOptions, annotations.E_Http)
			require.NoError(t, err)
			rule, ok := extension.(*annotations.HttpRule)
			require.True(t, ok)
			require.Equal(t, expected.path, rule.GetGet())
			require.Empty(t, rule.GetBody())
			require.Empty(t, rule.GetAdditionalBindings())
		})
	}

	messages := make(map[string]*descriptor.DescriptorProto)
	for _, message := range file.MessageType {
		messages[message.GetName()] = message
	}
	for _, name := range []string{"Params", "Request", "Result"} {
		extension, err := proto.GetExtension(messages[name].GetOptions(), gogoproto.E_Typedecl)
		require.NoError(t, err)
		require.Equal(t, proto.Bool(false), extension)
	}
	for _, name := range []string{"QueryProgramRequestRequest", "QueryProgramResultRequest"} {
		fields := messages[name].GetField()
		require.Len(t, fields, 2)
		require.Equal(t, "program_id", fields[0].GetName())
		require.Equal(t, descriptor.FieldDescriptorProto_TYPE_BYTES, fields[0].GetType())
		require.Equal(t, "request_id", fields[1].GetName())
		require.Equal(t, descriptor.FieldDescriptorProto_TYPE_UINT64, fields[1].GetType())
	}

	// Exercise the codecs on the existing state types, including every cast
	// and fixed-width custom field, rather than only checking descriptor text.
	params := types.DefaultParams(authority)
	params.KernelNetworkId = 125
	params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	request := types.Request{ID: 7, Requester: signer, Kind: types.KindApi,
		PayloadHash: types.Hash32{1, 2, 3}, CallbackGas: 500, Fee: sdk.NewInt(17),
		Height: 8, TimeoutHeight: 18, Status: types.StatusFulfilled,
		Level: types.LevelSingle, Attestor: signer}
	result := types.Result{RequestID: 7, Response: []byte("answer"),
		ContentDigest: types.Hash32{4, 5, 6}, FullLength: 6,
		Signers: []types.Address20{signer}, Height: 9,
		Callback: types.CallbackDelivered, CallbackGasUsed: 100, Level: types.LevelSingle}
	for _, pair := range []struct{ source, decoded proto.Message }{
		{&types.QueryParamsResponse{Params: params}, &types.QueryParamsResponse{}},
		{&types.QueryAttestorsResponse{Attestors: []types.Attestor{{Signer: signer, Payout: payout}}, Threshold: 1}, &types.QueryAttestorsResponse{}},
		{&types.QueryRequestResponse{Request: request}, &types.QueryRequestResponse{}},
		{&types.QueryResultResponse{Result: result}, &types.QueryResultResponse{}},
		{&types.QueryProgramRequestRequest{ProgramId: bytes.Repeat([]byte{1}, 32), RequestId: 7}, &types.QueryProgramRequestRequest{}},
		{&types.QueryProgramResultRequest{ProgramId: bytes.Repeat([]byte{2}, 32), RequestId: 7}, &types.QueryProgramResultRequest{}},
		{&types.QueryProgramRequestResponse{Record: bytes.Repeat([]byte{1}, 123), FeeDenom: types.ProgramFeeDenomPAX}, &types.QueryProgramRequestResponse{}},
		{&types.QueryProgramResultResponse{ContentDigest: result.ContentDigest, FullLength: 6, ResponseLength: 6, Response: result.Response}, &types.QueryProgramResultResponse{}},
		{&types.QueryProgramCountersResponse{Pending: 3, Committed: 2, PendingCapacity: 256, CommittedCapacity: 64}, &types.QueryProgramCountersResponse{}},
	} {
		encoded, err := proto.Marshal(pair.source)
		require.NoError(t, err)
		require.NoError(t, proto.Unmarshal(encoded, pair.decoded))
		require.Equal(t, pair.source, pair.decoded)
	}
	for _, pair := range []struct{ source, decoded proto.Message }{
		{&params, &types.Params{}},
		{&request, &types.Request{}},
		{&result, &types.Result{}},
	} {
		encoded, err := (&jsonpb.Marshaler{}).MarshalToString(pair.source)
		require.NoError(t, err)
		require.NoError(t, jsonpb.UnmarshalString(encoded, pair.decoded))
		require.Equal(t, pair.source, pair.decoded)
	}
	var hash types.Hash32
	require.Error(t, hash.Unmarshal(make([]byte, 31)))
	require.Error(t, hash.Unmarshal(make([]byte, 33)))
	_, err = hash.MarshalTo(make([]byte, 31))
	require.Error(t, err)
}
