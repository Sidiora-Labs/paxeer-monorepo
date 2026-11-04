package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/contracts/evm/bindings/erc20"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/contracts/evm/bindings/erc721"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/operations"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func m879JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func m879Read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func m879Response(t *testing.T, code uint32, event, key, value string, rawLog bool) []byte {
	t.Helper()
	logs := sdk.ABCIMessageLogs{{Events: []sdk.StringEvent{{Type: event, Attributes: []sdk.Attribute{{Key: key, Value: value}, {Key: "note", Value: "quoted \"value\" and C:\\path with spaces"}}}}}}
	response := &sdk.TxResponse{Code: code, Height: 42, TxHash: strings.Repeat("a", 64), Logs: logs}
	if rawLog {
		response.RawLog = string(m879JSON(t, logs))
		response.Logs = nil
	}
	raw, err := codec.ProtoMarshalJSON(response, nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRepairM_879_TransactionParser(t *testing.T) {
	address := sdk.AccAddress(bytes.Repeat([]byte{7}, 20)).String()
	for _, tc := range []struct{ kind, event, key, value string }{
		{"code_id", "store_code", "code_id", "47"},
		{"proposal_id", "submit_proposal", "proposal_id", "18446744073709551615"},
		{"contract_address", "instantiate", "_contract_address", address},
	} {
		for _, rawLog := range []bool{false, true} {
			raw := m879Response(t, 0, tc.event, tc.key, tc.value, rawLog)
			var output bytes.Buffer
			if err := operations.RunCLI(context.Background(), []string{"parse-transaction", tc.kind}, bytes.NewReader(raw), &output, nil); err != nil {
				t.Fatal(err)
			}
			if output.String() != tc.value+"\n" {
				t.Fatalf("identifier = %q", output.String())
			}
			for _, bad := range [][]byte{
				m879Response(t, 9, tc.event, tc.key, tc.value, rawLog),
				m879Response(t, 0, "other", tc.key, tc.value, rawLog),
				m879Response(t, 0, tc.event, tc.key, "", rawLog),
				m879Response(t, 0, tc.event, tc.key, "-1", rawLog),
				m879Response(t, 0, tc.event, tc.key, "not an identifier", rawLog),
				[]byte(`{"code":0,"raw_log":"not JSON"}`), []byte(`{`), []byte(`{"logs":[]}`),
			} {
				output.Reset()
				err := operations.RunCLI(context.Background(), []string{"parse-transaction", tc.kind}, bytes.NewReader(bad), &output, nil)
				if err == nil || output.Len() != 0 {
					t.Fatalf("invalid response accepted: %s", bad)
				}
			}
		}
	}
	for _, invalid := range []string{"0", "01", "1.1", "18446744073709551616"} {
		if _, err := operations.TransactionIdentifier(m879Response(t, 0, "store_code", "code_id", invalid, true), "code_id"); err == nil {
			t.Fatalf("accepted id %q", invalid)
		}
	}
}

type m879CommandStep struct {
	args  []string
	raw   []byte
	err   error
	check func(operations.Command)
}
type m879Transcript struct {
	t     *testing.T
	steps []m879CommandStep
	used  int
}

func (p *m879Transcript) Run(ctx context.Context, cmd operations.Command) ([]byte, error) {
	p.t.Helper()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.used >= len(p.steps) {
		p.t.Errorf("unexpected command %v", cmd.Args)
		return nil, errors.New("end of transcript")
	}
	step := p.steps[p.used]
	p.used++
	if step.check != nil {
		step.check(cmd)
	} else if !reflect.DeepEqual(step.args, cmd.Args) {
		p.t.Errorf("argv = %q; want %q", cmd.Args, step.args)
	}
	return step.raw, step.err
}
func (p *m879Transcript) done() {
	p.t.Helper()
	if p.used != len(p.steps) {
		p.t.Fatalf("consumed %d of %d transcript commands", p.used, len(p.steps))
	}
}

func m879WasmSteps(t *testing.T, options operations.WasmDeployment) []m879CommandStep {
	t.Helper()
	steps := []m879CommandStep{}
	names := []string{"jupiter", "mars", "saturn", "venus"}
	ids, addresses := map[string]string{}, map[string]string{}
	for i, name := range names {
		root := filepath.Join(options.Root, name)
		steps = append(steps, m879CommandStep{args: []string{"cargo", "build", "--manifest-path", filepath.Join(root, "Cargo.toml")}}, m879CommandStep{args: []string{"docker", "run", "--rm", "-v", root + ":/code", "--mount", "type=volume,source=" + name + "_cache,target=/code/target", "--mount", "type=volume,source=registry_cache,target=/usr/local/cargo/registry", "cosmwasm/rust-optimizer:0.12.5"}})
		ids[name] = fmt.Sprint(i + 1)
		addresses[name] = sdk.AccAddress(bytes.Repeat([]byte{byte(i + 1)}, 20)).String()
	}
	tx := func(raw []byte, args ...string) {
		argv := append([]string{options.Paxd, "tx"}, args...)
		argv = append(argv, "-y", "--from="+options.Key, "--chain-id="+options.Chain)
		if args[0] == "wasm" {
			argv = append(argv, "--gas=5000000", "--fees=1000000uhpx")
		}
		if args[0] == "dex" {
			argv = append(argv, "--gas=500000", "--fees=10000000uhpx")
		}
		argv = append(argv, "--broadcast-mode=block", "--output=json")
		steps = append(steps, m879CommandStep{args: argv, raw: raw})
	}
	success := m879Response(t, 0, "message", "action", "ok", false)
	for _, name := range names {
		tx(m879Response(t, 0, "store_code", "code_id", ids[name], true), "wasm", "store", filepath.Join(options.Root, name, "artifacts", name+".wasm"))
	}
	validator := sdk.ValAddress(bytes.Repeat([]byte{9}, 20)).String()
	steps = append(steps, m879CommandStep{args: []string{options.Paxd, "keys", "show", "node_admin", "--bech=val", "--output=json"}, raw: m879JSON(t, map[string]string{"address": validator})})
	tx(success, "staking", "delegate", validator, "1000000000uhpx")
	for _, name := range []string{"mars", "saturn", "venus", "jupiter"} {
		msg := "{}"
		if name == "jupiter" {
			msg = string(m879JSON(t, map[string]string{"mars_address": addresses["mars"]}))
		}
		tx(m879Response(t, 0, "instantiate", "_contract_address", addresses[name], false), "wasm", "instantiate", ids[name], msg, "--no-admin", "--label=dex")
	}
	for _, name := range []string{"mars", "saturn", "venus", "jupiter"} {
		args := []string{"dex", "register-contract", addresses[name], ids[name], "false", "true"}
		if name == "saturn" || name == "jupiter" {
			args = append(args, addresses["mars"])
		}
		tx(success, args...)
	}
	for _, name := range names {
		name := name
		steps = append(steps, m879CommandStep{raw: success, check: func(cmd operations.Command) {
			if len(cmd.Args) != 12 || cmd.Args[0] != options.Paxd || !reflect.DeepEqual(cmd.Args[1:4], []string{"tx", "dex", "register-pairs"}) {
				t.Fatalf("bad pair argv: %q", cmd.Args)
			}
			var pair struct {
				Items []struct {
					Address string `json:"contract_addr"`
				} `json:"batch_contract_pair"`
			}
			if err := json.Unmarshal(m879Read(t, cmd.Args[4]), &pair); err != nil {
				t.Fatal(err)
			}
			if len(pair.Items) != 1 || pair.Items[0].Address != addresses[name] {
				t.Fatal("wrong pair payload")
			}
		}})
	}
	tx(success, "staking", "unbond", validator, "1000000000uhpx")
	return steps
}

func TestRepairM_879_WasmDeployment(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout with spaces", "contracts")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	options := operations.WasmDeployment{Root: root, Paxd: "/binary with spaces/paxd", Key: "admin key", Chain: "benchmark-chain"}
	steps := m879WasmSteps(t, options)
	output := new(bytes.Buffer)
	peer := &m879Transcript{t: t, steps: steps}
	if err := operations.DeployWasm(context.Background(), peer, options, output); err != nil {
		t.Fatal(err)
	}
	peer.done()
	want := ""
	for i := 1; i <= 4; i++ {
		want += sdk.AccAddress(bytes.Repeat([]byte{byte(i)}, 20)).String() + "\n"
	}
	if output.String() != want {
		t.Fatalf("deployment output = %q", output.String())
	}
	for index := range steps {
		t.Run(fmt.Sprintf("failed-command-%d", index), func(t *testing.T) {
			prefix := append([]m879CommandStep(nil), steps[:index+1]...)
			prefix[index].err = errors.New("recorded command failure")
			peer := &m879Transcript{t: t, steps: prefix}
			var output bytes.Buffer
			if operations.DeployWasm(context.Background(), peer, options, &output) == nil || output.Len() != 0 {
				t.Fatal("failed deployment published output")
			}
			peer.done()
		})
		if len(steps[index].args) > 1 && steps[index].args[1] == "tx" || steps[index].check != nil {
			t.Run(fmt.Sprintf("refused-transaction-%d", index), func(t *testing.T) {
				prefix := append([]m879CommandStep(nil), steps[:index+1]...)
				prefix[index].raw = m879Response(t, 7, "message", "action", "refused", false)
				peer := &m879Transcript{t: t, steps: prefix}
				if operations.DeployWasm(context.Background(), peer, options, io.Discard) == nil {
					t.Fatal("refused transaction accepted")
				}
				peer.done()
			})
		}
	}
	for _, index := range []int{8, 14} {
		prefix := append([]m879CommandStep(nil), steps[:index+1]...)
		prefix[index].raw = m879Response(t, 0, "store_code", "code_id", "-1", true)
		peer := &m879Transcript{t: t, steps: prefix}
		if operations.DeployWasm(context.Background(), peer, options, io.Discard) == nil {
			t.Fatal("invalid identifier used")
		}
		peer.done()
	}
}

func m879EVMDeployment(t *testing.T, kind string) {
	t.Helper()
	options := operations.EVMDeployment{Root: "/checkout with spaces/contracts/evm", Endpoint: "http://127.0.0.1:8545", Key: "fixture key", Contract: kind, Paxd: "/binary with spaces/paxd"}
	fixture := m879Read(t, "testdata/m879/forge-create.json")
	account := "0x1234567890123456789012345678901234567890"
	source := "src/NoopToken.sol:NoopToken"
	if kind == "erc721" {
		source = "src/ERC721.sol:MyNFT"
	}
	forge := []string{"forge", "create", "--root", options.Root, "--broadcast", "-r", options.Endpoint, "--private-key", options.Key, source, "--json"}
	if kind == "erc20" {
		forge = append(forge, "--constructor-args", "NoopToken", "NT")
	}
	steps := []m879CommandStep{
		{args: []string{filepath.Join(options.Root, "setup.sh")}},
		{args: []string{"cast", "wallet", "address", "--private-key", options.Key}, raw: []byte(account + "\n")},
		{args: []string{"cast", "balance", account, "--rpc-url", options.Endpoint}, raw: []byte("100000000000000000000\n")},
		{args: forge, raw: fixture},
	}
	peer := &m879Transcript{t: t, steps: steps}
	address, err := operations.DeployEVM(context.Background(), peer, options)
	if err != nil || address != "0x9876543210987654321098765432109876543210" {
		t.Fatalf("address %q, err %v", address, err)
	}
	peer.done()
	for index := range steps {
		t.Run(fmt.Sprintf("command-%d", index), func(t *testing.T) {
			prefix := append([]m879CommandStep(nil), steps[:index+1]...)
			prefix[index].err = errors.New("recorded subprocess failure")
			peer := &m879Transcript{t: t, steps: prefix}
			if address, err := operations.DeployEVM(context.Background(), peer, options); err == nil || address != "" {
				t.Fatal("failed command admitted deployment")
			}
			peer.done()
		})
	}
	for _, bad := range []string{`{}`, `{"deployedTo":null}`, `{"deployedTo":"0x0000000000000000000000000000000000000000"}`, `{"deployedTo":"garbage"}`, `{"deployedTo":"0x1234"}`, `{`, string(fixture) + `{}`} {
		changed := append([]m879CommandStep(nil), steps...)
		changed[3].raw = []byte(bad)
		peer := &m879Transcript{t: t, steps: changed}
		if address, err := operations.DeployEVM(context.Background(), peer, options); err == nil || address != "" {
			t.Fatalf("invalid forge response admitted: %s", bad)
		}
		peer.done()
	}
	for _, badBalance := range []string{"not a balance", "-1"} {
		prefix := append([]m879CommandStep(nil), steps[:3]...)
		prefix[2].raw = []byte(badBalance)
		peer := &m879Transcript{t: t, steps: prefix}
		if _, err := operations.DeployEVM(context.Background(), peer, options); err == nil {
			t.Fatal("malformed balance accepted")
		}
		peer.done()
	}
	prefix := append([]m879CommandStep(nil), steps[:3]...)
	prefix[2].raw = []byte("0")
	prefix = append(prefix, m879CommandStep{args: []string{options.Paxd, "tx", "evm", "send", account, "100000000000000000000", "--from", "admin", "--evm-rpc", options.Endpoint, "-y"}, err: errors.New("funding refused")})
	peer = &m879Transcript{t: t, steps: prefix}
	if _, err := operations.DeployEVM(context.Background(), peer, options); err == nil {
		t.Fatal("refused funding admitted forge")
	}
	peer.done()
}
func TestRepairM_879_ERC20Deployment(t *testing.T)  { m879EVMDeployment(t, "erc20") }
func TestRepairM_879_ERC721Deployment(t *testing.T) { m879EVMDeployment(t, "erc721") }

func TestRepairM_879_CompleteABI(t *testing.T) {
	for _, pair := range []struct{ path, binding string }{
		{"contracts/evm/bindings/erc20/abi.json", erc20.Erc20MetaData.ABI},
		{"contracts/evm/bindings/erc721/abi.json", erc721.Erc721MetaData.ABI},
	} {
		original := m879Read(t, pair.path)
		expected, err := operations.CanonicalABI(original)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := operations.CanonicalABI([]byte(pair.binding))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("binding ABI differs: %s", pair.path)
		}
		var entries []map[string]any
		if err := json.Unmarshal(original, &entries); err != nil {
			t.Fatal(err)
		}
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
		reordered, err := operations.CanonicalABI(m879JSON(t, entries))
		if err != nil || !reflect.DeepEqual(expected, reordered) {
			t.Fatal("top-level ordering changed ABI")
		}
		for _, change := range []string{"parameter", "output", "mutability", "indexed", "anonymous", "overload", "constructor"} {
			t.Run(filepath.Base(filepath.Dir(pair.path))+"-"+change, func(t *testing.T) {
				var entries []map[string]any
				if err := json.Unmarshal(original, &entries); err != nil {
					t.Fatal(err)
				}
				modified := false
				for _, entry := range entries {
					if modified {
						break
					}
					inputs, _ := entry["inputs"].([]any)
					switch change {
					case "parameter":
						if entry["type"] == "function" && len(inputs) > 0 {
							inputs[0].(map[string]any)["type"] = "bytes32"
							modified = true
						}
					case "output":
						outputs, _ := entry["outputs"].([]any)
						if len(outputs) > 0 {
							outputs[0].(map[string]any)["type"] = "bytes32"
							modified = true
						}
					case "mutability":
						if entry["type"] == "function" {
							entry["stateMutability"] = "payable"
							modified = true
						}
					case "indexed":
						if entry["type"] == "event" && len(inputs) > 0 {
							v, _ := inputs[0].(map[string]any)["indexed"].(bool)
							inputs[0].(map[string]any)["indexed"] = !v
							modified = true
						}
					case "anonymous":
						if entry["type"] == "event" {
							entry["anonymous"] = true
							modified = true
						}
					case "constructor":
						if entry["type"] == "constructor" {
							entry["stateMutability"] = "payable"
							modified = true
						}
					case "overload":
						if entry["type"] == "function" {
							clone := map[string]any{}
							for k, v := range entry {
								clone[k] = v
							}
							clone["inputs"] = []any{}
							entries = append(entries, clone)
							modified = true
						}
					}
				}
				if !modified && change == "constructor" {
					entries = append(entries, map[string]any{"type": "constructor", "inputs": []any{}, "stateMutability": "payable"})
					modified = true
				}
				if !modified {
					t.Fatalf("no %s fixture changed", change)
				}
				var result bytes.Buffer
				if err := operations.RunCLI(context.Background(), []string{"canonical-abi"}, bytes.NewReader(m879JSON(t, entries)), &result, nil); err != nil {
					t.Fatal(err)
				}
				var actual []string
				if err := json.Unmarshal(result.Bytes(), &actual); err != nil {
					t.Fatal(err)
				}
				if reflect.DeepEqual(expected, actual) {
					t.Fatalf("%s drift accepted", change)
				}
			})
		}
	}
}

type m879CA struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	pem         []byte
}

func m879CertificateAuthority(t *testing.T) m879CA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "benchmark fixture CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(crand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m879CA{certificate: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})}
}
func m879TLSLeaf(t *testing.T, ca m879CA, matching bool) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "benchmark endpoint"}, DNSNames: []string{"unrelated.invalid"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if matching {
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		template.DNSNames = []string{"localhost"}
	}
	raw, err := x509.CreateCertificate(crand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw, ca.certificate.Raw}, PrivateKey: key}
}
func m879GRPCServer(t *testing.T, certificate *tls.Certificate) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var options []grpc.ServerOption
	if certificate != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*certificate}})))
	}
	server := grpc.NewServer(options...)
	service := health.NewServer()
	service.SetServingStatus("benchmark", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, service)
	joined := make(chan struct{})
	go func() { defer close(joined); _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close(); <-joined })
	return listener.Addr().String()
}
func m879Health(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: "benchmark"})
	if err != nil || result.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("real gRPC health response: %v %v", result, err)
	}
}
func TestRepairM_879_TLSAuthenticatesEndpoint(t *testing.T) {
	trusted := m879CertificateAuthority(t)
	unrelated := m879CertificateAuthority(t)
	caFile := filepath.Join(t.TempDir(), "trust.pem")
	if err := os.WriteFile(caFile, trusted.pem, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		ca       m879CA
		matching bool
		file     string
		want     bool
	}{
		{"explicit-roots", trusted, true, caFile, true},
		{"unrelated-issuer", unrelated, true, caFile, false},
		{"unrelated-server-name", trusted, false, caFile, false},
		{"system-roots-reject-private-CA", trusted, true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			leaf := m879TLSLeaf(t, tc.ca, tc.matching)
			endpoint := m879GRPCServer(t, &leaf)
			ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
			defer cancel()
			clients, conns, err := buildGrpcClients(ctx, &Config{GrpcEndpoints: endpoint, TLS: true, TLSCAFile: tc.file})
			if tc.want {
				if err != nil || len(clients) != 1 || len(conns) != 1 {
					t.Fatalf("trusted endpoint failed: %v", err)
				}
				defer conns[0].Close()
				m879Health(t, conns[0])
			} else {
				if err == nil || len(clients) != 0 || len(conns) != 0 {
					t.Fatal("unauthenticated endpoint admitted")
				}
				if !strings.Contains(err.Error(), "certificate") {
					t.Fatalf("rejection did not report certificate failure: %v", err)
				}
			}
		})
	}
	badFile := filepath.Join(t.TempDir(), "not-a-ca.pem")
	if err := os.WriteFile(badFile, []byte("garbage"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuildGrpcClients(&Config{GrpcEndpoints: "127.0.0.1:1", TLS: true, TLSCAFile: badFile}); err == nil {
		t.Fatal("invalid trust roots accepted")
	}
}

func TestRepairM_879_GRPCDeadlineAndClose(t *testing.T) {
	// This listener never speaks HTTP/2. The public constructor must fail before
	// it starts signer/account discovery, rather than blocking forever in Dial.
	blackhole, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blackhole.Close()
	start := time.Now()
	client, err := NewLoadTestClient(Config{GrpcEndpoints: blackhole.Addr().String()})
	if err == nil || client != nil {
		t.Fatal("unresponsive endpoint admitted")
	}
	if elapsed := time.Since(start); elapsed > grpcStartupTimeout+time.Second {
		t.Fatalf("startup was not bounded: %s", elapsed)
	}
	endpoint := m879GRPCServer(t, nil)
	clients, conns, err := BuildGrpcClients(&Config{GrpcEndpoints: endpoint + "," + endpoint})
	if err != nil || len(clients) != 2 || len(conns) != 2 {
		t.Fatalf("gRPC startup: %v", err)
	}
	for _, conn := range conns {
		m879Health(t, conn)
	}
	owner := &LoadTestClient{GrpcConns: conns}
	owner.startGrpcMonitors()
	joined := make(chan struct{})
	go func() { owner.Close(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel and join monitors")
	}
	for _, conn := range conns {
		if conn.GetState() != connectivity.Shutdown {
			t.Fatal("connection survived Close")
		}
	}
	owner.Close()
	// Closing a connection externally must also terminate a monitor that observes Shutdown.
	_, conns, err = BuildGrpcClients(&Config{GrpcEndpoints: endpoint})
	if err != nil {
		t.Fatal(err)
	}
	owner = &LoadTestClient{GrpcConns: conns}
	owner.startGrpcMonitors()
	_ = conns[0].Close()
	joined = make(chan struct{})
	go func() { owner.monitors.wg.Wait(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("Shutdown monitor leaked")
	}
	owner.Close()
	// Startup is all-or-nothing when a later endpoint fails.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	clients, conns, err = buildGrpcClients(ctx, &Config{GrpcEndpoints: endpoint + "," + blackhole.Addr().String()})
	if err == nil || clients != nil || conns != nil {
		t.Fatal("partial startup was returned")
	}
}

type m879RPCUncertain struct{}

func (m879RPCUncertain) Error() string  { return "server request timed out" }
func (m879RPCUncertain) ErrorCode() int { return -32002 }

type m879RPCRefusal struct{}

func (m879RPCRefusal) Error() string  { return "transaction refused by fee policy" }
func (m879RPCRefusal) ErrorCode() int { return -32000 }

type m879EthNode struct {
	mutex        sync.Mutex
	account      common.Address
	chain        *big.Int
	nonce        uint64
	transactions map[common.Hash]*ethtypes.Transaction
	attempts     []*ethtypes.Transaction
	mode         string
	failNonce    bool
	failLookup   bool
}

func (n *m879EthNode) GetTransactionCount(ctx context.Context, address common.Address, block string) (hexutil.Uint64, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.failNonce {
		return 0, errors.New("pending nonce unavailable")
	}
	if address != n.account || block != "pending" {
		return 0, errors.New("unexpected nonce query")
	}
	return hexutil.Uint64(n.nonce), nil
}
func (n *m879EthNode) GetTransactionByHash(ctx context.Context, hash common.Hash) (*ethtypes.Transaction, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.failLookup {
		return nil, errors.New("lookup unavailable")
	}
	return n.transactions[hash], nil
}
func (n *m879EthNode) SendRawTransaction(ctx context.Context, raw hexutil.Bytes) (common.Hash, error) {
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	sender, err := ethtypes.Sender(ethtypes.NewLondonSigner(n.chain), &tx)
	if err != nil || sender != n.account {
		return common.Hash{}, errors.New("invalid signed producer transaction")
	}
	n.mutex.Lock()
	n.attempts = append(n.attempts, &tx)
	mode := n.mode
	if mode == "rpc-uncertain" {
		n.mode = "accepted"
		n.mutex.Unlock()
		return common.Hash{}, m879RPCUncertain{}
	}
	if mode == "refused" {
		n.mode = "accepted"
		n.mutex.Unlock()
		return common.Hash{}, m879RPCRefusal{}
	}
	if mode == "uncertain-absent" || mode == "uncertain-accepted" {
		if mode == "uncertain-accepted" {
			n.transactions[tx.Hash()] = &tx
			n.nonce = tx.Nonce() + 1
		}
		n.mode = "accepted"
		n.mutex.Unlock()
		// JSON-RPC HTTP contexts are cancelled when the bounded client disconnects.
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		return tx.Hash(), nil
	}
	if tx.Nonce() != n.nonce {
		n.mutex.Unlock()
		return common.Hash{}, m879RPCRefusal{}
	}
	n.transactions[tx.Hash()] = &tx
	n.nonce++
	n.mutex.Unlock()
	return tx.Hash(), nil
}
func m879EVMClient(t *testing.T, mode string) (*EvmTxClient, *m879EthNode) {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	account := crypto.PubkeyToAddress(key.PublicKey)
	node := &m879EthNode{account: account, chain: big.NewInt(125), nonce: 7, mode: mode, transactions: make(map[common.Hash]*ethtypes.Transaction)}
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName("eth", node); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(rpcServer)
	t.Cleanup(server.Close)
	peer, err := ethclient.Dial(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(peer.Close)
	client := &EvmTxClient{accountAddress: account, chainId: big.NewInt(125), privateKey: key, ethClients: []*ethclient.Client{peer}, evmAddresses: &EVMAddresses{}, rpcTimeout: 100 * time.Millisecond}
	client.nonce.Store(7)
	return client, node
}

func TestRepairM_879_EVMRefusalReconcilesQueue(t *testing.T) {
	client, node := m879EVMClient(t, "refused")
	first, queued := client.GenerateSendFundsTx(), client.GenerateSendFundsTx()
	if first.Nonce() != 7 || queued.Nonce() != 8 {
		t.Fatal("producer did not reserve sequential nonces")
	}
	accepted := 0
	if err := client.SendEvmTx(first, func() { accepted++ }); err == nil {
		t.Fatal("refused send returned success")
	}
	if accepted != 0 || client.nonce.Load() != 7 {
		t.Fatal("refusal counted or nonce not reconciled")
	}
	if err := client.SendEvmTx(queued, func() { accepted++ }); err == nil {
		t.Fatal("stale queued transaction was sent")
	}
	replacement := client.GenerateSendFundsTx()
	if replacement.Nonce() != 7 {
		t.Fatalf("replacement nonce=%d", replacement.Nonce())
	}
	if err := client.SendEvmTx(replacement, func() { accepted++ }); err != nil {
		t.Fatal(err)
	}
	if accepted != 1 {
		t.Fatalf("accepted count=%d", accepted)
	}
	node.mutex.Lock()
	defer node.mutex.Unlock()
	if len(node.attempts) != 2 || node.nonce != 8 || len(node.transactions) != 1 {
		t.Fatal("stale work reached the real RPC transport")
	}
}
func TestRepairM_879_EVMUncertainSend(t *testing.T) {
	for _, mode := range []string{"uncertain-accepted", "uncertain-absent", "rpc-uncertain"} {
		t.Run(mode, func(t *testing.T) {
			client, node := m879EVMClient(t, mode)
			first, queued := client.GenerateSendFundsTx(), client.GenerateSendFundsTx()
			accepted := 0
			start := time.Now()
			err := client.SendEvmTx(first, func() { accepted++ })
			if time.Since(start) > time.Second {
				t.Fatal("send exceeded RPC bound")
			}
			if mode == "uncertain-accepted" {
				if err != nil || accepted != 1 || client.nonce.Load() != 8 {
					t.Fatalf("accepted transaction did not reconcile: %v count=%d nonce=%d", err, accepted, client.nonce.Load())
				}
			} else {
				if err == nil || accepted != 0 {
					t.Fatal("unknown acceptance reported success")
				}
				if err := client.PrepareNext(); err == nil {
					t.Fatal("generation admitted with unresolved submission")
				}
				if err := client.SendEvmTx(queued, func() { accepted++ }); err == nil {
					t.Fatal("queued work sent during uncertainty")
				}
				node.mutex.Lock()
				if len(node.attempts) != 1 {
					t.Fatal("work submitted behind uncertain nonce")
				}
				node.transactions[first.Hash()] = first
				node.nonce = 8
				node.mutex.Unlock()
				if err := client.PrepareNext(); err != nil {
					t.Fatal(err)
				}
				if accepted != 1 {
					t.Fatal("recovered acceptance was not counted exactly once")
				}
			}
			if err := client.PrepareNext(); err != nil {
				t.Fatal(err)
			}
			if accepted != 1 {
				t.Fatal("acceptance counted twice")
			}
			if err := client.SendEvmTx(queued, func() { accepted++ }); err == nil {
				t.Fatal("old queue retained after reconciliation")
			}
			next := client.GenerateSendFundsTx()
			if next.Nonce() != 8 {
				t.Fatalf("new work nonce=%d", next.Nonce())
			}
			if err := client.SendEvmTx(next, func() { accepted++ }); err != nil {
				t.Fatal(err)
			}
			if accepted != 2 {
				t.Fatalf("acceptance=%d", accepted)
			}
		})
	}
}
func TestRepairM_879_EVMReconciliationFailure(t *testing.T) {
	client, node := m879EVMClient(t, "refused")
	tx := client.GenerateSendFundsTx()
	node.mutex.Lock()
	node.failNonce = true
	node.mutex.Unlock()
	if err := client.SendEvmTx(tx, nil); err == nil {
		t.Fatal("missing nonce authority accepted")
	}
	if err := client.PrepareNext(); err == nil {
		t.Fatal("generation proceeded without pending nonce")
	}
	node.mutex.Lock()
	node.failNonce = false
	node.mutex.Unlock()
	if err := client.PrepareNext(); err != nil {
		t.Fatal(err)
	}
	if tx := client.GenerateSendFundsTx(); tx.Nonce() != 7 {
		t.Fatal("nonce did not recover")
	}
}

func TestRepairM_879_EVMAcceptedThroughput(t *testing.T) {
	client, node := m879EVMClient(t, "refused")
	first, stale := client.GenerateSendFundsTx(), client.GenerateSendFundsTx()
	queue := make(chan SignedTx, 3)
	queue <- SignedTx{EvmTx: first, MsgType: EVM}
	queue <- SignedTx{EvmTx: stale, MsgType: EVM}
	close(queue)
	count := new(int64)
	owner := &LoadTestClient{EvmTxClients: []*EvmTxClient{client}}
	var wg sync.WaitGroup
	owner.SendTxs(queue, 0, make(chan struct{}), map[string]*int64{EVM: count}, semaphore.NewWeighted(1), &wg)
	if atomic.LoadInt64(count) != 0 {
		t.Fatal("refused or stale transactions inflated accepted throughput")
	}
	next := client.GenerateSendFundsTx()
	queue = make(chan SignedTx, 1)
	queue <- SignedTx{EvmTx: next, MsgType: EVM}
	close(queue)
	owner.SendTxs(queue, 0, make(chan struct{}), map[string]*int64{EVM: count}, semaphore.NewWeighted(1), &wg)
	if atomic.LoadInt64(count) != 1 {
		t.Fatal("accepted transaction missing from throughput")
	}
	node.mutex.Lock()
	defer node.mutex.Unlock()
	if len(node.attempts) != 2 {
		t.Fatal("stale queue reached transport")
	}
}

func TestRepairM_879_EVMProducerResumesAfterReconciliation(t *testing.T) {
	client, node := m879EVMClient(t, "uncertain-absent")
	tx := client.GenerateSendFundsTx()
	if err := client.SendEvmTx(tx, nil); err == nil {
		t.Fatal("uncertain send accepted")
	}
	owner := &LoadTestClient{EvmTxClients: []*EvmTxClient{client}, LoadTestConfig: Config{MessageTypes: []string{EVM}}}
	queue := make(chan SignedTx, 1)
	done := make(chan struct{})
	joined := make(chan struct{})
	var wg sync.WaitGroup
	go func() {
		defer close(joined)
		owner.BuildTxs(queue, 0, &wg, done, rate.NewLimiter(rate.Inf, 1), map[string]*int64{EVM: new(int64)})
	}()
	select {
	case <-queue:
		t.Fatal("producer generated behind uncertain nonce")
	case <-time.After(150 * time.Millisecond):
	}
	node.mutex.Lock()
	node.transactions[tx.Hash()] = tx
	node.nonce = 8
	node.mutex.Unlock()
	select {
	case next := <-queue:
		if next.EvmTx.Nonce() != 8 {
			t.Fatalf("resumed nonce=%d", next.EvmTx.Nonce())
		}
	case <-time.After(time.Second):
		t.Fatal("producer did not resume after acceptance was observed")
	}
	close(done)
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("producer did not stop")
	}
}

func TestRepairM_879_EVMMessageNoncePaths(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		client, _ := m879EVMClient(t, "accepted")
		client.useEip1559 = dynamic
		client.evmAddresses = &EVMAddresses{ERC20: common.HexToAddress("0x1234567890123456789012345678901234567890"), ERC721: common.HexToAddress("0x9876543210987654321098765432109876543210")}
		for i, kind := range []string{EVM, ERC20, ERC721} {
			tx, err := client.GetTxForMsgType(kind)
			if err != nil {
				t.Fatal(err)
			}
			if tx.Nonce() != uint64(7+i) {
				t.Fatalf("%s nonce=%d", kind, tx.Nonce())
			}
			if dynamic && tx.Type() != ethtypes.DynamicFeeTxType {
				t.Fatal("lost EIP-1559 transaction type")
			}
			if err := client.SendEvmTx(tx, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestRepairM_879_EVMTransientLookupFailure(t *testing.T) {
	client, node := m879EVMClient(t, "refused")
	tx := client.GenerateSendFundsTx()
	node.mutex.Lock()
	node.failLookup = true
	node.mutex.Unlock()
	accepted := 0
	if err := client.SendEvmTx(tx, func() { accepted++ }); err == nil {
		t.Fatal("unavailable reconciliation accepted")
	}
	if _, err := client.GetTxForMsgType(EVM); err == nil {
		t.Fatal("generation admitted without reconciliation")
	}
	node.mutex.Lock()
	node.failLookup = false
	node.mutex.Unlock()
	next, err := client.GetTxForMsgType(EVM)
	if err != nil || next.Nonce() != 7 {
		t.Fatalf("refused nonce did not recover: %v", err)
	}
	if accepted != 0 {
		t.Fatal("refused transaction counted during recovery")
	}
	if err := client.SendEvmTx(next, func() { accepted++ }); err != nil {
		t.Fatal(err)
	}
	if accepted != 1 {
		t.Fatal("recovered account could not send")
	}
}

type m879FundingNode struct{ receipt *ethtypes.Receipt }

func (n *m879FundingNode) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*ethtypes.Receipt, error) {
	if hash != n.receipt.TxHash {
		return nil, errors.New("unexpected funding hash")
	}
	return n.receipt, nil
}
func TestRepairM_879_DeploymentFundingReceipt(t *testing.T) {
	for _, status := range []uint64{0, 1} {
		t.Run(fmt.Sprintf("receipt-%d", status), func(t *testing.T) {
			producer, _ := m879EVMClient(t, "accepted")
			tx := producer.GenerateSendFundsTx()
			receipt := &ethtypes.Receipt{Type: tx.Type(), Status: status, TxHash: tx.Hash(), BlockHash: common.HexToHash("0x1234"), BlockNumber: big.NewInt(10), Logs: []*ethtypes.Log{}, GasUsed: 21000, CumulativeGasUsed: 21000}
			rpcServer := rpc.NewServer()
			if err := rpcServer.RegisterName("eth", &m879FundingNode{receipt: receipt}); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(rpcServer)
			defer server.Close()
			options := operations.EVMDeployment{Root: "/contract root/evm", Endpoint: server.URL, Key: "fixture key", Contract: "erc20", Paxd: "paxd"}
			account := producer.accountAddress.Hex()
			steps := []m879CommandStep{
				{args: []string{filepath.Join(options.Root, "setup.sh")}},
				{args: []string{"cast", "wallet", "address", "--private-key", options.Key}, raw: []byte(account)},
				{args: []string{"cast", "balance", account, "--rpc-url", options.Endpoint}, raw: []byte("0")},
				{args: []string{"paxd", "tx", "evm", "send", account, "100000000000000000000", "--from", "admin", "--evm-rpc", options.Endpoint, "-y"}, raw: []byte("Transaction hash: " + tx.Hash().Hex() + "\n")},
			}
			if status == 1 {
				steps = append(steps, m879CommandStep{args: []string{"forge", "create", "--root", options.Root, "--broadcast", "-r", options.Endpoint, "--private-key", options.Key, "src/NoopToken.sol:NoopToken", "--json", "--constructor-args", "NoopToken", "NT"}, raw: m879Read(t, "testdata/m879/forge-create.json")})
			}
			peer := &m879Transcript{t: t, steps: steps}
			address, err := operations.DeployEVM(context.Background(), peer, options)
			if status == 1 {
				if err != nil || address == "" {
					t.Fatalf("confirmed funding failed: %v", err)
				}
			} else {
				if err == nil || address != "" {
					t.Fatal("reverted funding admitted deployment")
				}
			}
			peer.done()
		})
	}
}

func TestRepairM_879_DeploymentConsumer(t *testing.T) {
	// Drive the same address printer and command-output consumer used by the
	// scripts and benchmark startup, with the committed forge response as input.
	address, err := operations.DeploymentAddress(m879Read(t, "testdata/m879/forge-create.json"))
	if err != nil {
		t.Fatal(err)
	}
	config := &Config{EvmRpcEndpoints: "http://127.0.0.1:8545"}
	for _, output := range []string{address + "\n", "", "null", "0x0000000000000000000000000000000000000000", "0x1234", address + "\nextra output"} {
		peer := &m879Transcript{t: t, steps: []m879CommandStep{{args: []string{"/deployment script with spaces", config.EVMRpcEndpoint()}, raw: []byte(output)}}}
		got, err := deployEvmContractWithCommands(context.Background(), peer, "/deployment script with spaces", config)
		if output == address+"\n" {
			if err != nil || got != common.HexToAddress(address) {
				t.Fatalf("valid deployment consumer failed: %v", err)
			}
		} else {
			if err == nil || got != (common.Address{}) {
				t.Fatalf("invalid deployment admitted startup: %q", output)
			}
		}
		peer.done()
	}
	peer := &m879Transcript{t: t, steps: []m879CommandStep{{args: []string{"deploy", config.EVMRpcEndpoint()}, raw: []byte(address), err: errors.New("deployment failed despite stdout")}}}
	if address, err := deployEvmContractWithCommands(context.Background(), peer, "deploy", config); err == nil || address != (common.Address{}) {
		t.Fatal("failed deployment admitted load generation")
	}
	peer.done()
}
