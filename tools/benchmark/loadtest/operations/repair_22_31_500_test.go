package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type cw20StoreRecordingRunner struct {
	t        *testing.T
	commands []Command
	stop     error
	failAt   int
	response []byte
}

func (r *cw20StoreRecordingRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	r.t.Helper()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.commands = append(r.commands, command)
	n := len(r.commands)
	if n == r.failAt || n > 2 {
		return nil, r.stop
	}
	if r.response != nil {
		return r.response, nil
	}
	logs := sdk.ABCIMessageLogs{{Events: []sdk.StringEvent{{
		Type: "store_code", Attributes: []sdk.Attribute{{Key: "code_id", Value: fmt.Sprint(40 + n)}},
	}}}}
	rawLog, err := json.Marshal(logs)
	if err != nil {
		r.t.Fatal(err)
	}
	raw, err := codec.ProtoMarshalJSON(&sdk.TxResponse{Code: 0, Height: 42, RawLog: string(rawLog)}, nil)
	if err != nil {
		r.t.Fatal(err)
	}
	return raw, nil
}

func TestRepair22_31_500_CW20StorePaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repository with spaces")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, optionRoot := range []string{root, relative} {
		t.Run(optionRoot, func(t *testing.T) {
			stop := errors.New("end of store recording")
			runner := &cw20StoreRecordingRunner{t: t, stop: stop}
			options := WasmDeployment{Root: optionRoot, Paxd: "/binary with spaces/paxd", Key: "admin key", Chain: "loadtest-chain"}
			var output bytes.Buffer
			if err := DeployWasm(context.Background(), runner, options, &output); !errors.Is(err, stop) {
				t.Fatalf("deployment did not reach the end of the store step: %v", err)
			}
			if len(runner.commands) != 3 || output.Len() != 0 {
				t.Fatalf("commands = %v, output = %q", runner.commands, output.String())
			}
			want := Command{Args: []string{
				options.Paxd, "tx", "wasm", "store", filepath.Join(root, "contracts", "wasm", "cw20_base.wasm"),
				"-y", "--from=" + options.Key, "--chain-id=" + options.Chain,
				"--gas=5000000", "--fees=1000000uhpx", "--broadcast-mode=block", "--output=json",
			}}
			for i := 0; i < 2; i++ {
				if !reflect.DeepEqual(runner.commands[i], want) {
					t.Fatalf("store %d = %#v; want %#v", i+1, runner.commands[i], want)
				}
			}
			if want := (Command{Args: []string{options.Paxd, "keys", "show", "node_admin", "--bech=val", "--output=json"}}); !reflect.DeepEqual(runner.commands[2], want) {
				t.Fatalf("unexpected command after the two stores: %#v", runner.commands[2])
			}
		})
	}
}

func TestRepair22_31_500_CW20StoreFailureStopsDeployment(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			stop := errors.New("store command failed")
			runner := &cw20StoreRecordingRunner{t: t, stop: stop, failAt: failAt}
			var output bytes.Buffer
			err := DeployWasm(context.Background(), runner, WasmDeployment{Root: t.TempDir(), Paxd: "paxd", Key: "admin", Chain: "loadtest-chain"}, &output)
			if !errors.Is(err, stop) || len(runner.commands) != failAt || output.Len() != 0 {
				t.Fatalf("error = %v, commands = %v, output = %q", err, runner.commands, output.String())
			}
		})
	}
	for _, raw := range [][]byte{[]byte(`{"code":7}`), []byte(`{"code":0,"logs":[]}`), []byte(`{`)} {
		runner := &cw20StoreRecordingRunner{t: t, response: raw}
		var output bytes.Buffer
		err := DeployWasm(context.Background(), runner, WasmDeployment{Root: t.TempDir(), Paxd: "paxd", Key: "admin", Chain: "loadtest-chain"}, &output)
		if err == nil || len(runner.commands) != 1 || output.Len() != 0 {
			t.Fatalf("response %s: error = %v, commands = %v, output = %q", raw, err, runner.commands, output.String())
		}
	}
}
