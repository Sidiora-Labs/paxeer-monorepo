package types

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"reflect"
	"testing"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
)

func TestQueryDenomPausedWireShape(t *testing.T) {
	request := &QueryDenomPausedRequest{Denom: "factory/issuer/asset"}
	requestWire := append([]byte{0x0a, byte(len(request.Denom))}, []byte(request.Denom)...)
	encoded, err := gogoproto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, requestWire) {
		t.Fatalf("request wire = %x, want %x", encoded, requestWire)
	}
	var decodedRequest QueryDenomPausedRequest
	if err := gogoproto.Unmarshal(requestWire, &decodedRequest); err != nil {
		t.Fatal(err)
	}
	if decodedRequest.GetDenom() != request.Denom {
		t.Fatalf("decoded denom = %q, want %q", decodedRequest.GetDenom(), request.Denom)
	}

	for _, paused := range []bool{false, true} {
		response := &QueryDenomPausedResponse{Paused: paused, IssuerDid: "did:layerx:issuer"}
		responseWire := []byte{}
		if paused {
			responseWire = append(responseWire, 0x08, 0x01)
		}
		responseWire = append(responseWire, 0x12, byte(len(response.IssuerDid)))
		responseWire = append(responseWire, []byte(response.IssuerDid)...)
		encoded, err := gogoproto.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, responseWire) {
			t.Fatalf("response paused=%t wire = %x, want %x", paused, encoded, responseWire)
		}
		var decodedResponse QueryDenomPausedResponse
		if err := gogoproto.Unmarshal(responseWire, &decodedResponse); err != nil {
			t.Fatal(err)
		}
		if decodedResponse.GetPaused() != paused || decodedResponse.GetIssuerDid() != response.IssuerDid {
			t.Fatalf("decoded response = %+v, want %+v", decodedResponse, response)
		}
	}
	encoded, err = gogoproto.Marshal(&QueryDenomPausedResponse{})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 0 {
		t.Fatalf("zero response wire = %x, want empty proto3 encoding", encoded)
	}

	compressed, _ := request.Descriptor()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	var file descriptor.FileDescriptorProto
	if err := proto.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if file.GetPackage() != "paxprotocol.paxchain.tokenfactory" {
		t.Fatalf("proto package = %q", file.GetPackage())
	}
	for _, want := range []struct {
		name   string
		fields []*descriptor.FieldDescriptorProto
	}{
		{"QueryDenomPausedRequest", []*descriptor.FieldDescriptorProto{
			{Name: proto.String("denom"), Number: proto.Int32(1), Type: descriptor.FieldDescriptorProto_TYPE_STRING.Enum()},
		}},
		{"QueryDenomPausedResponse", []*descriptor.FieldDescriptorProto{
			{Name: proto.String("paused"), Number: proto.Int32(1), Type: descriptor.FieldDescriptorProto_TYPE_BOOL.Enum()},
			{Name: proto.String("issuer_did"), Number: proto.Int32(2), Type: descriptor.FieldDescriptorProto_TYPE_STRING.Enum()},
		}},
	} {
		var message *descriptor.DescriptorProto
		for _, candidate := range file.GetMessageType() {
			if candidate.GetName() == want.name {
				message = candidate
				break
			}
		}
		if message == nil || len(message.GetField()) != len(want.fields) {
			t.Fatalf("%s descriptor missing or wrong field count: %v", want.name, message)
		}
		for i, field := range message.GetField() {
			expected := want.fields[i]
			if field.GetName() != expected.GetName() || field.GetNumber() != expected.GetNumber() || field.GetType() != expected.GetType() || field.GetLabel() != descriptor.FieldDescriptorProto_LABEL_OPTIONAL {
				t.Fatalf("%s field %d = %v, want name=%s number=%d type=%s optional", want.name, i, field, expected.GetName(), expected.GetNumber(), expected.GetType())
			}
		}
	}

	var method *descriptor.MethodDescriptorProto
	for _, service := range file.GetService() {
		if service.GetName() == "Query" {
			for _, candidate := range service.GetMethod() {
				if candidate.GetName() == "DenomPaused" {
					method = candidate
				}
			}
		}
	}
	if method == nil {
		t.Fatal("Query.DenomPaused missing from generated descriptor")
	}
	if method.GetInputType() != ".paxprotocol.paxchain.tokenfactory.QueryDenomPausedRequest" || method.GetOutputType() != ".paxprotocol.paxchain.tokenfactory.QueryDenomPausedResponse" || method.GetClientStreaming() || method.GetServerStreaming() {
		t.Fatalf("unexpected DenomPaused signature: %v", method)
	}
	extension, err := proto.GetExtension(method.GetOptions(), annotations.E_Http)
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := extension.(*annotations.HttpRule)
	if !ok || rule.GetGet() != "/pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused" {
		t.Fatalf("unexpected DenomPaused HTTP rule: %v", extension)
	}

	serverMethod, ok := reflect.TypeOf((*QueryServer)(nil)).Elem().MethodByName("DenomPaused")
	serverSignature := reflect.TypeOf((func(context.Context, *QueryDenomPausedRequest) (*QueryDenomPausedResponse, error))(nil))
	if !ok || serverMethod.Type != serverSignature {
		t.Fatalf("QueryServer.DenomPaused signature = %v, want %v", serverMethod.Type, serverSignature)
	}
	clientMethod, ok := reflect.TypeOf((*QueryClient)(nil)).Elem().MethodByName("DenomPaused")
	clientSignature := reflect.TypeOf((func(context.Context, *QueryDenomPausedRequest, ...grpc.CallOption) (*QueryDenomPausedResponse, error))(nil))
	if !ok || clientMethod.Type != clientSignature {
		t.Fatalf("QueryClient.DenomPaused signature = %v, want %v", clientMethod.Type, clientSignature)
	}
	service := grpc.NewServer()
	defer service.Stop()
	RegisterQueryServer(service, &UnimplementedQueryServer{})
	registered := false
	for _, method := range service.GetServiceInfo()["paxprotocol.paxchain.tokenfactory.Query"].Methods {
		if method.Name == "DenomPaused" && !method.IsClientStream && !method.IsServerStream {
			registered = true
		}
	}
	if !registered {
		t.Fatal("DenomPaused missing from registered unary gRPC methods")
	}

	pathParams, err := pattern_Query_DenomPaused_0.Match([]string{"pax-protocol", "paxchain", "tokenfactory", "denoms", "asset", "paused"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if pathParams["denom"] != "asset" {
		t.Fatalf("gateway denom = %q, want asset", pathParams["denom"])
	}
}
