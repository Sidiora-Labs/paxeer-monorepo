package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestRepairM_1361_ModuleIdentity(t *testing.T) {
	const root = "github.com/Sidiora-Labs/Paxeer-X-Network"
	cases := []struct {
		name     string
		metadata string
		want     string
	}{
		{
			name:     "native main module",
			metadata: "path\t" + root + "/cmd/paxd\nmod\t" + root + "\t(devel)\t\n",
			want:     root + "@(devel)",
		},
		{
			name:     "versioned main module",
			metadata: "mod\t" + root + "\tv7.0.0\th1:root\n",
			want:     root + "@v7.0.0",
		},
		{
			name:     "main takes precedence over obsolete sdk dependency",
			metadata: "mod\t" + root + "\t(devel)\t\ndep\t" + root + "/sdk\tv0.47.0\th1:sdk\n",
			want:     root + "@(devel)",
		},
		{
			name:     "external consumer root dependency",
			metadata: "mod\texample.org/app\t(devel)\t\ndep\t" + root + "\tv7.0.0\th1:root\n",
			want:     root + "@v7.0.0",
		},
		{
			name:     "versioned replacement",
			metadata: "mod\texample.org/app\t(devel)\t\ndep\t" + root + "\tv7.0.0\th1:root\n=>\texample.org/fork\tv7.0.1\th1:fork\n",
			want:     root + "@v7.0.0 => example.org/fork@v7.0.1",
		},
		{
			name:     "local replacement provenance",
			metadata: "mod\texample.org/app\t(devel)\t\ndep\t" + root + "\tv7.0.0\th1:root\n=>\t../paxeer\t(devel)\t\n",
			want:     root + "@v7.0.0 => ../paxeer@(devel)",
		},
		{
			name:     "main replacement provenance",
			metadata: "mod\t" + root + "\tv7.0.0\th1:root\n=>\texample.org/fork\tv7.0.1\th1:fork\n",
			want:     root + "@v7.0.0 => example.org/fork@v7.0.1",
		},
		{
			name:     "missing version is explicit",
			metadata: "mod\t" + root + "\t\t\n",
			want:     root + "@unavailable",
		},
		{
			name:     "missing replacement version is explicit",
			metadata: "mod\texample.org/app\t(devel)\t\ndep\t" + root + "\tv7.0.0\th1:root\n=>\t../paxeer\t\t\n",
			want:     root + "@v7.0.0 => ../paxeer@unavailable",
		},
		{
			name:     "obsolete sdk module cannot identify root provenance",
			metadata: "mod\texample.org/app\t(devel)\t\ndep\t" + root + "/sdk\tv0.47.0\th1:sdk\n",
			want:     "unavailable",
		},
		{
			name:     "absent module metadata",
			metadata: "path\t" + root + "/sdk/version.test\n",
			want:     "unavailable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := debug.ParseBuildInfo("go\t" + runtime.Version() + "\n" + tc.metadata)
			if err != nil {
				t.Fatal(err)
			}
			before := info.String()
			if got := sdkVersionFromBuildInfo(info); got != tc.want {
				t.Fatalf("module provenance = %q, want %q", got, tc.want)
			}
			if info.String() != before {
				t.Fatal("version lookup mutated the build metadata")
			}
		})
	}
	if got := sdkVersionFromBuildInfo(nil); got != "unavailable" {
		t.Fatalf("missing build info = %q, want unavailable", got)
	}
}

func TestRepairM_1361_RuntimeMetadata(t *testing.T) {
	const root = "github.com/Sidiora-Labs/Paxeer-X-Network"
	want := "unavailable"
	if info, ok := debug.ReadBuildInfo(); ok {
		modules := append([]*debug.Module{&info.Main}, info.Deps...)
		for _, module := range modules {
			if module.Path != root {
				continue
			}
			version := module.Version
			if version == "" {
				version = "unavailable"
			}
			want = root + "@" + version
			if module.Replace != nil {
				version = module.Replace.Version
				if version == "" {
					version = "unavailable"
				}
				want += " => " + module.Replace.Path + "@" + version
			}
			break
		}
	}
	if got := NewInfo().CosmosSdkVersion; got != want {
		t.Fatalf("runtime SDK provenance = %q, want %q", got, want)
	}
}

func TestRepairM_1361_NativeMetadataCommand(t *testing.T) {
	previous := NewInfo()
	t.Cleanup(func() {
		Name, AppName, Version, Commit, BuildTags, NativeArchiveDigest = previous.Name, previous.AppName, previous.Version, previous.GitCommit, previous.BuildTags, previous.NativeArchiveDigest
	})
	Name = "Paxeer X Network"
	AppName = "paxd"
	Version = "7.0.0"
	Commit = "72c91077cb966f1c4c640ded61b4b259f41cad90"
	BuildTags = "netgo,ledger"
	NativeArchiveDigest = strings.Repeat("ab", 32)
	want := NewInfo()
	if want.Name != Name || want.AppName != AppName || want.Version != Version || want.GitCommit != Commit || want.BuildTags != BuildTags || want.NativeArchiveDigest != NativeArchiveDigest {
		t.Fatalf("native build metadata was not preserved: %+v", want)
	}
	goVersion := fmt.Sprintf("go version %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if want.GoVersion != goVersion || want.CosmosSdkVersion == "" {
		t.Fatalf("runtime metadata is incomplete: %+v", want)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var wantFields map[string]interface{}
	if err := json.Unmarshal(wantJSON, &wantFields); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"json", "text"} {
		t.Run(output, func(t *testing.T) {
			expectedFields := make(map[string]interface{}, len(wantFields))
			for key, value := range wantFields {
				expectedFields[key] = value
			}
			if output == "text" && want.BuildDeps == nil {
				// The YAML codec encodes a nil dependency slice as an empty sequence.
				expectedFields["build_deps"] = []interface{}{}
			}
			var buffer bytes.Buffer
			command := NewVersionCommand()
			command.SetOut(&buffer)
			command.SetErr(&buffer)
			command.SetArgs([]string{"--long", "--output=" + output})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			var fields map[string]interface{}
			if output == "json" {
				if err := json.Unmarshal(buffer.Bytes(), &fields); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := yaml.Unmarshal(buffer.Bytes(), &fields); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(fields, expectedFields) {
				t.Fatalf("command metadata = %v, want %v", fields, expectedFields)
			}
			if fields["native_archive_digest"] != NativeArchiveDigest || fields["cosmos_sdk_version"] != want.CosmosSdkVersion {
				t.Fatalf("wire metadata lost provenance: %v", fields)
			}
		})
	}
}
