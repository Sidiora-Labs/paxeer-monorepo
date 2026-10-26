package api_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	bufImageOnce sync.Once
	bufImageSet  *descriptorpb.FileDescriptorSet
	bufImageErr  error
)

func bufImage(t *testing.T) *descriptorpb.FileDescriptorSet {
	t.Helper()
	bufImageOnce.Do(func() {
		bufImageSet, bufImageErr = func() (*descriptorpb.FileDescriptorSet, error) {
			root, err := filepath.Abs("..")
			if err != nil {
				return nil, fmt.Errorf("locate workspace root: %w", err)
			}
			output := filepath.Join(t.TempDir(), "image.binpb")
			cmd := exec.CommandContext(t.Context(), "go", "run", "github.com/bufbuild/buf/cmd/buf@v1.58.0", "build", "--exclude-source-info", "-o", output)
			cmd.Dir = root
			if log, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("build buf workspace image: %w\n%s", err, log)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				return nil, fmt.Errorf("read buf workspace image: %w", err)
			}
			image := new(descriptorpb.FileDescriptorSet)
			if err := proto.Unmarshal(data, image); err != nil {
				return nil, fmt.Errorf("decode buf workspace image: %w", err)
			}
			return image, nil
		}()
	})
	if bufImageErr != nil {
		t.Fatal(bufImageErr)
	}
	return bufImageSet
}

func bufFile(t *testing.T, name string) *descriptorpb.FileDescriptorProto {
	t.Helper()
	var found *descriptorpb.FileDescriptorProto
	for _, file := range bufImage(t).GetFile() {
		if file.GetName() == name {
			if found != nil {
				t.Fatalf("%s: duplicate file in buf workspace image", name)
			}
			found = file
		}
	}
	if found == nil {
		t.Fatalf("%s: file missing from buf workspace image", name)
	}
	return found
}

func generatedFile(t *testing.T, desc interface{ Descriptor() ([]byte, []int) }) *descriptorpb.FileDescriptorProto {
	t.Helper()
	data, _ := desc.Descriptor()
	return decodeFile(t, data)
}

func registeredFile(t *testing.T, name string) *descriptorpb.FileDescriptorProto {
	t.Helper()
	data := gogoproto.FileDescriptor(name)
	if len(data) == 0 {
		t.Fatalf("%s: file missing from gogo descriptor registry", name)
	}
	file := decodeFile(t, data)
	if file.GetName() != name {
		t.Fatalf("%s: registered descriptor names %q", name, file.GetName())
	}
	return file
}

func decodeFile(t *testing.T, data []byte) *descriptorpb.FileDescriptorProto {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open generated descriptor gzip: %v", err)
	}
	raw, err := io.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		t.Fatalf("read generated descriptor gzip: %v", err)
	}
	if closeErr != nil {
		t.Fatalf("close generated descriptor gzip: %v", closeErr)
	}
	file := new(descriptorpb.FileDescriptorProto)
	if err := proto.Unmarshal(raw, file); err != nil {
		t.Fatalf("decode generated file descriptor: %v", err)
	}
	return file
}

type descriptorShape map[string]string

func shapeOf(file *descriptorpb.FileDescriptorProto) descriptorShape {
	shape := descriptorShape{"package": file.GetPackage()}
	dependencies, err := json.Marshal(file.GetDependency())
	if err != nil {
		panic(err)
	}
	shape["dependencies"] = string(dependencies)
	var addEnum func(string, *descriptorpb.EnumDescriptorProto)
	addEnum = func(prefix string, enum *descriptorpb.EnumDescriptorProto) {
		name := prefix + enum.GetName()
		shape["enum "+name] = "present"
		for _, value := range enum.GetValue() {
			shape["enum value "+name+"."+value.GetName()] = fmt.Sprint(value.GetNumber())
		}
	}
	var addMessage func(string, *descriptorpb.DescriptorProto)
	addMessage = func(prefix string, message *descriptorpb.DescriptorProto) {
		name := prefix + message.GetName()
		shape["message "+name] = "present"
		for _, field := range message.GetField() {
			key := "field " + name + "." + field.GetName()
			shape[key+" name"] = field.GetName()
			shape[key+" number"] = fmt.Sprint(field.GetNumber())
			shape[key+" label"] = field.GetLabel().String()
			shape[key+" type"] = field.GetType().String()
			shape[key+" type_name"] = field.GetTypeName()
		}
		for _, nested := range message.GetNestedType() {
			addMessage(name+".", nested)
		}
		for _, enum := range message.GetEnumType() {
			addEnum(name+".", enum)
		}
	}
	for _, message := range file.GetMessageType() {
		addMessage("", message)
	}
	for _, enum := range file.GetEnumType() {
		addEnum("", enum)
	}
	for _, service := range file.GetService() {
		name := service.GetName()
		shape["service "+name] = "present"
		for _, method := range service.GetMethod() {
			key := "rpc " + name + "." + method.GetName()
			shape[key+" input"] = method.GetInputType()
			shape[key+" output"] = method.GetOutputType()
			shape[key+" client_streaming"] = fmt.Sprint(method.GetClientStreaming())
			shape[key+" server_streaming"] = fmt.Sprint(method.GetServerStreaming())
			if proto.HasExtension(method.GetOptions(), annotations.E_Http) {
				rule := proto.GetExtension(method.GetOptions(), annotations.E_Http).(*annotations.HttpRule)
				shape[key+" google.api.http"] = rule.String()
			}
		}
	}
	return shape
}

func compareShapes(name string, source, generated descriptorShape) []string {
	keys := make(map[string]struct{}, len(source)+len(generated))
	for key := range source {
		keys[key] = struct{}{}
	}
	for key := range generated {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	var differences []string
	for _, key := range ordered {
		sourceValue, sourceOK := source[key]
		generatedValue, generatedOK := generated[key]
		if sourceOK && generatedOK && sourceValue == generatedValue {
			continue
		}
		sourceText, generatedText := fmt.Sprintf("%q", sourceValue), fmt.Sprintf("%q", generatedValue)
		if !sourceOK {
			sourceText = "<missing>"
		}
		if !generatedOK {
			generatedText = "<missing>"
		}
		differences = append(differences, fmt.Sprintf("%s: %s: source %s generated %s", name, key, sourceText, generatedText))
	}
	return differences
}

func appQueryHelper(t *testing.T) (*node.App, sdk.Context, *baseapp.QueryServiceTestHelper) {
	t.Helper()
	app := node.SetupWithDefaultHome(false, true, false)
	if app == nil {
		t.Fatal("app setup returned nil")
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Errorf("close query app: %v", err)
		}
	})
	ctx := app.NewContext(false, tmproto.Header{Height: 1, ChainID: "pax-test", Time: time.Now().UTC()})
	return app, ctx, baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
}

func serveGateway(t *testing.T, register func(context.Context, *runtime.ServeMux) error) *httptest.Server {
	t.Helper()
	mux := runtime.NewServeMux()
	if err := register(t.Context(), mux); err != nil {
		t.Fatalf("register gateway: %v", err)
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
