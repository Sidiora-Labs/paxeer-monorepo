package layerxprograms_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

func TestLayerXProgramsDescriptorsLinked(t *testing.T) {
	for _, tc := range []struct {
		file    string
		service string
		methods []string
	}{
		{"tx.proto", "Msg", []string{"UpdateFeeSchedule", "UpdateMeteringSchedule", "StageRetirementProfile", "UpdateParams"}},
		{"query.proto", "Query", []string{"ProgramState", "ProgramBalances", "ProgramEvents"}},
	} {
		t.Run(tc.service, func(t *testing.T) {
			name := "layerxprograms/" + tc.file
			registered := proto.FileDescriptor(name)
			if len(registered) == 0 {
				t.Fatalf("%s is absent from the global proto registry", name)
			}
			reader, err := gzip.NewReader(bytes.NewReader(registered))
			if err != nil {
				t.Fatal(err)
			}
			raw, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			var file descriptor.FileDescriptorProto
			if err := proto.Unmarshal(raw, &file); err != nil {
				t.Fatal(err)
			}
			if file.GetName() != name || file.GetPackage() != "paxprotocol.paxchain.layerxprograms" {
				t.Fatalf("unexpected registered file: %s, package %s", file.GetName(), file.GetPackage())
			}
			if got := file.GetOptions().GetGoPackage(); got != "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms/types" {
				t.Fatalf("unexpected Go package: %s", got)
			}
			if len(file.GetService()) != 1 || file.GetService()[0].GetName() != tc.service {
				t.Fatalf("%s does not register the %s service", name, tc.service)
			}
			methods := file.GetService()[0].GetMethod()
			if len(methods) != len(tc.methods) {
				t.Fatalf("%s has %d methods, want %d", tc.service, len(methods), len(tc.methods))
			}
			for i, want := range tc.methods {
				method := methods[i]
				if method.GetName() != want {
					t.Fatalf("method %d is %s, want %s", i, method.GetName(), want)
				}
				for _, message := range []string{method.GetInputType(), method.GetOutputType()} {
					if len(message) == 0 || message[0] != '.' || proto.MessageType(message[1:]) == nil {
						t.Fatalf("%s.%s message %q is absent from the global proto registry", tc.service, want, message)
					}
				}
			}
		})
	}
}
