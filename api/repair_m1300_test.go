package api_test

import (
	"bytes"
	"compress/gzip"
	"slices"
	"strings"
	"testing"

	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func repairM1300Parse(t *testing.T, source string) *descriptorpb.FileDescriptorProto {
	t.Helper()
	parser := protoparse.Parser{Accessor: protoparse.FileContentsFromMap(map[string]string{"repair_m1300.proto": source})}
	files, err := parser.ParseFiles("repair_m1300.proto")
	if err != nil {
		t.Fatalf("compile descriptor fixture: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("compiled %d descriptors, want 1", len(files))
	}
	return files[0].AsFileDescriptorProto()
}

type repairM1300Pair struct {
	name, source, generated string
	keys                    []string
}

func repairM1300CheckPairs(t *testing.T, pairs []repairM1300Pair) {
	t.Helper()
	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			source := repairM1300Parse(t, pair.source)
			generated := repairM1300Parse(t, pair.generated)
			wire, err := proto.Marshal(generated)
			if err != nil {
				t.Fatal(err)
			}
			var compressed bytes.Buffer
			writer := gzip.NewWriter(&compressed)
			if _, err := writer.Write(wire); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			transported := decodeFile(t, compressed.Bytes())
			if !proto.Equal(generated, transported) {
				t.Fatal("descriptor codec changed the generated descriptor")
			}
			for _, direction := range []struct {
				name        string
				left, right *descriptorpb.FileDescriptorProto
			}{
				{"source-to-generated", source, transported},
				{"generated-to-source", transported, source},
			} {
				differences := compareShapes(source.GetName(), shapeOf(direction.left), shapeOf(direction.right))
				want := slices.Clone(pair.keys)
				slices.Sort(want)
				if len(differences) != len(want) {
					t.Fatalf("%s: got %d differences, want %d (%v): %v", direction.name, len(differences), len(want), want, differences)
				}
				for index, key := range want {
					prefix := source.GetName() + ": " + key + ": source "
					if !strings.HasPrefix(differences[index], prefix) || !strings.Contains(differences[index], " generated ") {
						t.Errorf("%s: difference = %q, want diagnostic for %q", direction.name, differences[index], key)
					}
				}
			}
		})
	}
}

func TestRepairM_1300_JSONNames(t *testing.T) {
	repairM1300CheckPairs(t, []repairM1300Pair{
		{"explicit-name", `syntax="proto3"; message M { string request_id=1 [json_name="requestID"]; }`, `syntax="proto3"; message M { string request_id=1 [json_name="requestIdentifier"]; }`, []string{"field M.request_id json_name"}},
		{"default-vs-explicit", `syntax="proto3"; message M { string request_id=1; }`, `syntax="proto3"; message M { string request_id=1 [json_name="requestID"]; }`, []string{"field M.request_id json_name"}},
	})
	t.Run("default-json-names", TestDescriptorShapeJSONNameDefaults)
}

func TestRepairM_1300_OneofStructure(t *testing.T) {
	repairM1300CheckPairs(t, []repairM1300Pair{
		{"declaration-name", `syntax="proto3"; message M { oneof choice { string text=1; } }`, `syntax="proto3"; message M { oneof selection { string text=1; } }`, []string{"oneof M[0] name"}},
		{"absent-vs-index-zero", `syntax="proto3"; message M { string text=1; oneof choice { int32 number=2; } }`, `syntax="proto3"; message M { oneof choice { string text=1; int32 number=2; } }`, []string{"field M.text oneof_index"}},
		{"membership", `syntax="proto3"; message M { oneof first { string text=1; int32 number=2; } oneof second { bool flag=3; } }`, `syntax="proto3"; message M { oneof first { int32 number=2; } oneof second { bool flag=3; string text=1; } }`, []string{"field M.text oneof_index"}},
		{"declaration-removed", `syntax="proto3"; message M { oneof choice { string text=1; } }`, `syntax="proto3"; message M { string text=1; }`, []string{"oneof M[0] name", "field M.text oneof_index"}},
	})
}

func TestRepairM_1300_OptionalPresence(t *testing.T) {
	repairM1300CheckPairs(t, []repairM1300Pair{
		{"proto3-optional", `syntax="proto3"; message M { string note=1; }`, `syntax="proto3"; message M { optional string note=1; }`, []string{"field M.note proto3_optional", "field M.note oneof_index", "oneof M[0] name"}},
		{"optional-vs-real-oneof", `syntax="proto3"; message M { optional string note=1; }`, `syntax="proto3"; message M { oneof _note { string note=1; } }`, []string{"field M.note proto3_optional"}},
		{"proto2-vs-proto3", `syntax="proto2"; message M { optional string note=1; }`, `syntax="proto3"; message M { string note=1; }`, []string{"syntax"}},
	})
}

func TestRepairM_1300_MessageReservations(t *testing.T) {
	repairM1300CheckPairs(t, []repairM1300Pair{
		{"range-removed", `syntax="proto3"; message M { reserved 10 to 12; }`, `syntax="proto3"; message M {}`, []string{"message M reserved range [10,13)"}},
		{"range-start", `syntax="proto3"; message M { reserved 10 to 12; }`, `syntax="proto3"; message M { reserved 11 to 12; }`, []string{"message M reserved range [10,13)", "message M reserved range [11,13)"}},
		{"range-end", `syntax="proto3"; message M { reserved 10 to 12; }`, `syntax="proto3"; message M { reserved 10 to 13; }`, []string{"message M reserved range [10,13)", "message M reserved range [10,14)"}},
		{"name-removed", `syntax="proto3"; message M { reserved "retired"; }`, `syntax="proto3"; message M {}`, []string{"message M reserved name retired"}},
		{"name-changed", `syntax="proto3"; message M { reserved "retired"; }`, `syntax="proto3"; message M { reserved "legacy"; }`, []string{"message M reserved name retired", "message M reserved name legacy"}},
	})
	t.Run("reservation-order", TestDescriptorShapeReservationOrder)
}

func TestRepairM_1300_EnumReservations(t *testing.T) {
	repairM1300CheckPairs(t, []repairM1300Pair{
		{"range-removed", `syntax="proto3"; enum E { ZERO=0; reserved 10 to 12; }`, `syntax="proto3"; enum E { ZERO=0; }`, []string{"enum E reserved range [10,12]"}},
		{"range-start", `syntax="proto3"; enum E { ZERO=0; reserved 10 to 12; }`, `syntax="proto3"; enum E { ZERO=0; reserved 11 to 12; }`, []string{"enum E reserved range [10,12]", "enum E reserved range [11,12]"}},
		{"range-end", `syntax="proto3"; enum E { ZERO=0; reserved 10 to 12; }`, `syntax="proto3"; enum E { ZERO=0; reserved 10 to 13; }`, []string{"enum E reserved range [10,12]", "enum E reserved range [10,13]"}},
		{"name-removed", `syntax="proto3"; enum E { ZERO=0; reserved "RETIRED"; }`, `syntax="proto3"; enum E { ZERO=0; }`, []string{"enum E reserved name RETIRED"}},
		{"name-changed", `syntax="proto3"; enum E { ZERO=0; reserved "RETIRED"; }`, `syntax="proto3"; enum E { ZERO=0; reserved "LEGACY"; }`, []string{"enum E reserved name RETIRED", "enum E reserved name LEGACY"}},
	})
}

func TestRepairM_1300_NestedDescriptors(t *testing.T) {
	repairM1300CheckPairs(t, []repairM1300Pair{
		{"nested-json-name", `syntax="proto3"; message Outer { message Inner { string request_id=1; } }`, `syntax="proto3"; message Outer { message Inner { string request_id=1 [json_name="id"]; } }`, []string{"field Outer.Inner.request_id json_name"}},
		{"nested-reservations", `syntax="proto3"; message Outer { message Inner { reserved 10; reserved "old"; } enum E { ZERO=0; reserved 10; reserved "OLD"; } }`, `syntax="proto3"; message Outer { message Inner {} enum E { ZERO=0; } }`, []string{"message Outer.Inner reserved range [10,11)", "message Outer.Inner reserved name old", "enum Outer.E reserved range [10,10]", "enum Outer.E reserved name OLD"}},
		{"nested-oneof", `syntax="proto3"; message Outer { message Inner { oneof choice { string text=1; } } }`, `syntax="proto3"; message Outer { message Inner { oneof selection { string text=1; } } }`, []string{"oneof Outer.Inner[0] name"}},
		{"nested-optional", `syntax="proto3"; message Outer { message Inner { string note=1; } }`, `syntax="proto3"; message Outer { message Inner { optional string note=1; } }`, []string{"field Outer.Inner.note proto3_optional", "field Outer.Inner.note oneof_index", "oneof Outer.Inner[0] name"}},
	})
}

func TestRepairM_1300_RegisteredDescriptors(t *testing.T) {
	parser := protoparse.Parser{ImportPaths: []string{".", "../sdk/third_party/proto"}}
	for _, name := range []string{"oracle/oracle.proto", "oracle/tx.proto", "oracle/genesis.proto"} {
		t.Run(name, func(t *testing.T) {
			files, err := parser.ParseFiles(name)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 1 {
				t.Fatalf("parsed %d descriptors, want 1", len(files))
			}
			registered := registeredFile(t, name)
			if differences := compareShapes(name, shapeOf(files[0].AsFileDescriptorProto()), shapeOf(registered)); len(differences) != 0 {
				t.Fatalf("real source/registry parity failed: %v", differences)
			}
		})
	}
}
