package interop

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	seqnode "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

type repair137Env struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
type repair137Container struct {
	Name   string         `yaml:"name"`
	Env    []repair137Env `yaml:"env"`
	Mounts []struct {
		Name     string `yaml:"name"`
		Path     string `yaml:"mountPath"`
		ReadOnly bool   `yaml:"readOnly"`
	} `yaml:"volumeMounts"`
}
type repair137Deployment struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []repair137Container `yaml:"containers"`
				Volumes    []struct {
					Name   string `yaml:"name"`
					Secret struct {
						Name     string `yaml:"secretName"`
						Mode     uint32 `yaml:"defaultMode"`
						Optional bool   `yaml:"optional"`
						Items    []struct {
							Key  string `yaml:"key"`
							Path string `yaml:"path"`
						} `yaml:"items"`
					} `yaml:"secret"`
				} `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
		PodSelector struct {
			Labels map[string]string `yaml:"matchLabels"`
		} `yaml:"podSelector"`
		Egress  []repair137Rule `yaml:"egress"`
		Ingress []repair137Rule `yaml:"ingress"`
	} `yaml:"spec"`
}
type repair137Rule struct {
	To    []repair137Peer `yaml:"to"`
	From  []repair137Peer `yaml:"from"`
	Ports []struct {
		Protocol string `yaml:"protocol"`
		Port     int    `yaml:"port"`
	} `yaml:"ports"`
}
type repair137Peer struct {
	PodSelector struct {
		Labels map[string]string `yaml:"matchLabels"`
	} `yaml:"podSelector"`
}

func TestRepair_137_KubernetesStartupRestart(t *testing.T) {
	decoder := yaml.NewDecoder(bytes.NewReader(KubernetesDeployment))
	var documents []repair137Deployment
	for {
		var document repair137Deployment
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		documents = append(documents, document)
	}
	var deployment *repair137Deployment
	policies := map[string]repair137Deployment{}
	for i := range documents {
		d := &documents[i]
		if d.Kind == "Deployment" {
			deployment = d
		}
		if d.Kind == "NetworkPolicy" {
			policies[d.Metadata.Name] = *d
		}
	}
	require.NotNil(t, deployment)
	require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	container := deployment.Spec.Template.Spec.Containers[0]
	env := map[string]string{}
	for _, entry := range container.Env {
		_, duplicate := env[entry.Name]
		require.False(t, duplicate)
		env[entry.Name] = entry.Value
	}
	repair137Contract(t, env, "https://layerx-node.layerx-testnet.svc.cluster.local:8545")
	require.Equal(t, "eip155-125", env["LAYERX_INTEROP_NETWORK_ID"])
	pinPath := env["LAYERX_INTEROP_RECEIPT_KEYS_FILE"]
	mounted := false
	for _, mount := range container.Mounts {
		if filepath.Dir(pinPath) != mount.Path {
			continue
		}
		require.True(t, mount.ReadOnly)
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Name != mount.Name {
				continue
			}
			require.Equal(t, "layerx-interop-receipt-keys", volume.Secret.Name)
			require.False(t, volume.Secret.Optional)
			require.EqualValues(t, 0440, volume.Secret.Mode)
			require.Len(t, volume.Secret.Items, 1)
			require.Equal(t, "receipt-keys.json", volume.Secret.Items[0].Key)
			require.Equal(t, filepath.Base(pinPath), volume.Secret.Items[0].Path)
			mounted = true
		}
	}
	require.True(t, mounted, "receipt keys must be projected from a required, read-only Secret")
	require.True(t, repair137Allows(policies["layerx-interop-gateway"].Spec.Egress, "layerx-node", true, 8545))
	nodeIngress := policies["layerx-node-interop-ingress"]
	require.Equal(t, "layerx-node", nodeIngress.Spec.PodSelector.Labels["app"])
	require.True(t, repair137Allows(nodeIngress.Spec.Ingress, "layerx-interop-gateway", false, 8545))
	repair137StartupRestart(t, env)
}

func TestRepair_137_FlyStartupRestart(t *testing.T) {
	var contract struct {
		Env   map[string]string `toml:"env"`
		Files []struct {
			Path   string `toml:"guest_path"`
			Secret string `toml:"secret_name"`
		} `toml:"files"`
	}
	_, err := toml.Decode(string(FlyDeployment), &contract)
	require.NoError(t, err)
	repair137Contract(t, contract.Env, "https://paxeer-x-core.internal:8545")
	mounted := 0
	for _, file := range contract.Files {
		if file.Path == contract.Env["LAYERX_INTEROP_RECEIPT_KEYS_FILE"] {
			require.Equal(t, "INTEROP_RECEIPT_KEYS", file.Secret)
			mounted++
		}
	}
	require.Equal(t, 1, mounted, "Fly must remount the durable, independently provisioned app secret on every start")
	// These existing app secrets are intentionally not public Fly env entries.
	contract.Env["LAYERX_INTEROP_NETWORK_ID"] = "eip155-125"
	contract.Env["LAYERX_INTEROP_PROTOCOL_NETWORK_ID"] = "402"
	repair137StartupRestart(t, contract.Env)
}

func repair137Contract(t *testing.T, env map[string]string, endpoint string) {
	t.Helper()
	require.Equal(t, endpoint, env["LAYERX_INTEROP_SEQUENCER_URL"])
	require.Equal(t, "/run/layerx/receipt-keys/receipt-keys.json", env["LAYERX_INTEROP_RECEIPT_KEYS_FILE"])
	require.Equal(t, "125", env["LAYERX_INTEROP_CHAIN_ID"])
	age, err := strconv.ParseUint(env["LAYERX_INTEROP_READINESS_MAX_AGE_MS"], 10, 64)
	require.NoError(t, err)
	require.Positive(t, age)
	require.LessOrEqual(t, age, uint64(30000))
	require.EqualValues(t, 15000, age)
}

func repair137Allows(rules []repair137Rule, app string, egress bool, port int) bool {
	for _, rule := range rules {
		peers := rule.From
		if egress {
			peers = rule.To
		}
		for _, peer := range peers {
			if peer.PodSelector.Labels["app"] == app {
				for _, p := range rule.Ports {
					if p.Protocol == "TCP" && p.Port == port {
						return true
					}
				}
			}
		}
	}
	return false
}

var repair137Build struct {
	sync.Once
	binary string
	err    error
	output []byte
}

func repair137Binary(t *testing.T, root string) string {
	t.Helper()
	repair137Build.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "cargo", "build", "--locked", "--manifest-path", filepath.Join(root, "services/interop/Cargo.toml"), "-p", "layerx-interop-service", "--bin", "layerx-interop-gateway")
		command.Dir = root
		repair137Build.output, repair137Build.err = command.CombinedOutput()
		target := os.Getenv("CARGO_TARGET_DIR")
		if target == "" {
			target = filepath.Join(root, "services/interop/target")
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(root, target)
		}
		repair137Build.binary = filepath.Join(target, "debug/layerx-interop-gateway")
	})
	require.NoError(t, repair137Build.err, "real interop binary build: %s", repair137Build.output)
	return repair137Build.binary
}

func repair137FreeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func repair137StartupRestart(t *testing.T, deploymentEnv map[string]string) {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	binary := repair137Binary(t, root)
	dir := t.TempDir()
	// Independent operator trust input; the RPC response is never used to mint pins.
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	id := common.BytesToHash(bytes.Repeat([]byte{0x37}, 32))
	pins := []evmrpc.ReceiptKey{{KeyID: id, PublicKey: hexutil.Bytes(public), FirstHeight: 1, LastHeight: 1000}}
	pinBytes, err := json.Marshal(pins)
	require.NoError(t, err)
	pinFile := filepath.Join(dir, "receipt-keys.json")
	require.NoError(t, os.WriteFile(pinFile, pinBytes, 0600))

	// Run the production node RPC handler over its real sequencer keeper.
	testApp := app.Setup(t, false, false, false)
	sdkContext := testApp.NewContext(false, tmproto.Header{ChainID: "hyperpax_125-1", Height: 1, Time: time.Now().UTC()})
	require.NoError(t, testApp.SequencerKeeper.SetReceiptKeys(sdkContext, seqkeeper.ReceiptKeys{{KeyId: id[:], PublicKey: public, FirstHeight: 1, LastHeight: 1000}}))
	cfg, err := tmconfig.ResetTestRoot(t.TempDir(), "interop137")
	require.NoError(t, err)
	cfg.Mode = tmconfig.ModeFull
	cfg.RPC.ListenAddress = ""
	cfg.P2P.ListenAddress = "tcp://" + repair137FreeAddress(t)
	consensusApp := kvstore.NewApplication()
	genesis, err := tmtypes.GenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	consensusApp.SetValidators(genesis.ValidatorUpdates())
	tmService, err := tmnode.New(t.Context(), cfg, func() {}, consensusApp, nil, nil, tmnode.DefaultMetricsProvider(cfg.Instrumentation)(cfg.ChainID()), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	require.NoError(t, tmService.Start(t.Context()))
	t.Cleanup(func() { tmService.Stop(); tmService.Wait() })
	localClient, err := local.New(tmService)
	require.NoError(t, err)
	rpcAddress := repair137FreeAddress(t)
	_, rpcPort, err := net.SplitHostPort(rpcAddress)
	require.NoError(t, err)
	rpcConfig := rpcconfig.DefaultConfig
	rpcConfig.HTTPAddress = "127.0.0.1"
	rpcConfig.HTTPPort, err = strconv.Atoi(rpcPort)
	require.NoError(t, err)
	source := seqnode.NewReceiptSource(16)
	server, err := evmrpc.NewEVMHTTPServer(rpcConfig, localClient, &testApp.EvmKeeper, testApp.BeginBlockKeepers, testApp.BaseApp, testApp.TracerAnteHandler,
		func(int64) sdk.Context { return sdkContext }, func(int64) client.TxConfig { return app.MakeEncodingConfig().TxConfig }, "", nil,
		evmrpc.SequencerReceiptOption{Source: source, Keeper: &testApp.SequencerKeeper})
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)
	upstream, err := url.Parse("http://" + rpcAddress)
	require.NoError(t, err)
	var keyReads atomic.Int64
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	tlsNode := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, readErr.Error(), 400)
			return
		}
		var call struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(body, &call) == nil && call.Method == evmrpc.ReceiptKeysMethod {
			keyReads.Add(1)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		proxy.ServeHTTP(w, r)
	}))
	// Match the deployment's DNS-only HTTPS contract with a verified local name.
	nodeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	nodeCAKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	nodeCA := &x509.Certificate{
		SerialNumber: big.NewInt(136),
		Subject:      pkix.Name{CommonName: "interop node test CA"},
		NotBefore:    time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true, IsCA: true,
	}
	nodeCADER, err := x509.CreateCertificate(rand.Reader, nodeCA, nodeCA, &nodeCAKey.PublicKey, nodeCAKey)
	require.NoError(t, err)
	nodeRoot, err := x509.ParseCertificate(nodeCADER)
	require.NoError(t, err)
	nodeCertificate := &x509.Certificate{
		SerialNumber: big.NewInt(137),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	nodeDER, err := x509.CreateCertificate(rand.Reader, nodeCertificate, nodeCA, &nodeKey.PublicKey, nodeCAKey)
	require.NoError(t, err)
	tlsNode.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{{Certificate: [][]byte{nodeDER, nodeCADER}, PrivateKey: nodeKey}},
	}
	tlsNode.StartTLS()
	defer tlsNode.Close()
	nodeURL := strings.Replace(tlsNode.URL, "127.0.0.1", "localhost", 1)
	rpcClient, err := gethrpc.DialHTTPWithClient(nodeURL, tlsNode.Client())
	require.NoError(t, err)
	defer rpcClient.Close()
	var published evmrpc.ReceiptKeysResult
	require.NoError(t, rpcClient.CallContext(t.Context(), &published, evmrpc.ReceiptKeysMethod))
	require.Equal(t, pins, published.Keys)

	env := map[string]string{}
	for key, value := range deploymentEnv {
		env[key] = value
	}
	env["LAYERX_INTEROP_SEQUENCER_URL"] = nodeURL
	env["LAYERX_INTEROP_LISTEN"] = repair137FreeAddress(t)
	env["LAYERX_INTEROP_REDIS_URL"] = "rediss://127.0.0.1:1"
	repair137Material(t, root, dir, env, pinFile, public, nodeRoot)

	start := func() (*exec.Cmd, *bytes.Buffer, <-chan error) {
		command := exec.Command(binary)
		command.Env = []string{}
		for key, value := range env {
			command.Env = append(command.Env, key+"="+value)
		}
		logs := new(bytes.Buffer)
		command.Stdout = logs
		command.Stderr = logs
		require.NoError(t, command.Start())
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		return command, logs, done
	}
	stop := func(command *exec.Cmd, done <-chan error) { require.NoError(t, command.Process.Kill()); <-done }
	for attempt := 0; attempt < 2; attempt++ {
		before := keyReads.Load()
		command, logs, done := start()
		stopped := false
		// Cleanup also runs if an assertion aborts before the explicit restart.
		t.Cleanup(func() {
			if !stopped {
				_ = command.Process.Kill()
				<-done
			}
		})
		transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: repair137Roots(t, dir), MinVersion: tls.VersionTLS12}}
		httpClient := &http.Client{Transport: transport, Timeout: time.Second}
		scheme := "https"
		if env["LAYERX_INTEROP_LISTENER"] == "plain" {
			scheme = "http"
		}
		readyURL := scheme + "://" + env["LAYERX_INTEROP_LISTEN"] + "/livez"
		deadline := time.Now().Add(15 * time.Second)
		for {
			select {
			case exitErr := <-done:
				stopped = true
				t.Fatalf("service refused deployment startup: %v: %s", exitErr, logs)
			default:
			}
			response, requestErr := httpClient.Get(readyURL)
			if requestErr == nil {
				body, readErr := io.ReadAll(response.Body)
				response.Body.Close()
				require.NoError(t, readErr)
				require.Equal(t, 200, response.StatusCode, "%s", body)
				var live map[string]any
				require.NoError(t, json.Unmarshal(body, &live))
				require.Equal(t, "live", live["status"])
				require.Equal(t, "layerx-interop-gateway", live["service"])
				break
			}
			require.True(t, time.Now().Before(deadline), "service did not listen: %v", requestErr)
			time.Sleep(20 * time.Millisecond)
		}
		require.Greater(t, keyReads.Load(), before, "startup must compare pins with the real node RPC")
		response, err := httpClient.Get(scheme + "://" + env["LAYERX_INTEROP_LISTEN"] + "/readyz")
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		var ready struct {
			Capabilities struct {
				Chain struct {
					State   string `json:"state"`
					ChainID uint64 `json:"chain_id"`
				} `json:"chain_serving"`
			} `json:"capabilities"`
		}
		require.NoError(t, json.Unmarshal(body, &ready))
		require.Equal(t, "ready", ready.Capabilities.Chain.State, "%s", body)
		require.EqualValues(t, 125, ready.Capabilities.Chain.ChainID)
		// Redis is not part of this startup contract; overall settlement stays degraded.
		require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
		transport.CloseIdleConnections()
		stop(command, done)
		stopped = true
		recovered, err := os.ReadFile(pinFile)
		require.NoError(t, err)
		require.Equal(t, pinBytes, recovered)
	}
	// Changing an independently provisioned pin must fail closed after a restart.
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pins[0].PublicKey = hexutil.Bytes(otherPublic)
	changedPins, err := json.Marshal(pins)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pinFile, changedPins, 0600))
	before := keyReads.Load()
	command, logs, done := start()
	select {
	case exitErr := <-done:
		require.Error(t, exitErr)
		require.Contains(t, logs.String(), "pin differs from px_getReceiptKeys")
	case <-time.After(15 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Fatal("service accepted mismatched receipt keys")
	}
	require.Greater(t, keyReads.Load(), before)
}

func repair137Roots(t *testing.T, dir string) *x509.CertPool {
	t.Helper()
	der, err := os.ReadFile(filepath.Join(dir, "server.crt.der"))
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return roots
}

func repair137Material(t *testing.T, root, dir string, env map[string]string, pinFile string, public ed25519.PublicKey, rpcCA *x509.Certificate) {
	t.Helper()
	write := func(name string, content []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, content, 0600))
		return path
	}
	run := func(command *exec.Cmd) {
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	keyPath := filepath.Join(dir, "server.key.pem")
	certPath := filepath.Join(dir, "server.crt.pem")
	run(exec.Command("openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-keyout", keyPath, "-out", certPath, "-days", "1", "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1,DNS:localhost"))
	run(exec.Command("openssl", "x509", "-in", certPath, "-outform", "DER", "-out", filepath.Join(dir, "server.crt.der")))
	run(exec.Command("openssl", "pkcs8", "-topk8", "-nocrypt", "-in", keyPath, "-outform", "DER", "-out", filepath.Join(dir, "server.key.der")))
	run(exec.Command("openssl", "pkcs12", "-export", "-out", filepath.Join(dir, "client.p12"), "-inkey", keyPath, "-in", certPath, "-passout", "pass:interop-137-password"))
	files := map[string]string{
		"LAYERX_INTEROP_CONFIG":                        filepath.Join(dir, "config.json"),
		"LAYERX_INTEROP_MODULE_REGISTRY_FILE":          write("registry.json", []byte(`{"modules":[{"module":1,"ordinals":[1]}]}`)),
		"LAYERX_INTEROP_OUTBOUND_CA_DER":               write("rpc-ca.der", rpcCA.Raw),
		"LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12":        filepath.Join(dir, "client.p12"),
		"LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE": write("client-password", []byte("interop-137-password")),
		"LAYERX_INTEROP_REDIS_USERNAME_FILE":           write("redis-username", []byte("interop137")),
		"LAYERX_INTEROP_REDIS_PASSWORD_FILE":           write("redis-password", []byte("interop-137-password")),
		"LAYERX_INTEROP_RECEIPT_KEYS_FILE":             pinFile,
	}
	if env["LAYERX_INTEROP_LISTENER"] != "plain" {
		files["LAYERX_INTEROP_TLS_CERT_DER"] = filepath.Join(dir, "server.crt.der")
		files["LAYERX_INTEROP_TLS_KEY_DER"] = filepath.Join(dir, "server.key.der")
	}
	for key, value := range files {
		require.NotEmpty(t, env[key], "deployment must provide %s", key)
		env[key] = value
	}
	example, err := os.ReadFile(filepath.Join(root, "services/interop/deploy/gateway/config.example.json"))
	require.NoError(t, err)
	var manifest map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(example, &manifest))
	var ap2 []map[string]any
	require.NoError(t, json.Unmarshal(manifest["ap2_keys"], &ap2))
	ap2Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ap2Public := hex.EncodeToString(elliptic.Marshal(elliptic.P256(), ap2Key.X, ap2Key.Y))
	for _, pin := range ap2 {
		pin["public_key_sec1"] = ap2Public
	}
	manifest["ap2_keys"], err = json.Marshal(ap2)
	require.NoError(t, err)
	for _, field := range []string{"visa_agents", "fiat_providers"} {
		var entries []map[string]any
		require.NoError(t, json.Unmarshal(manifest[field], &entries))
		property := "public_key"
		if field == "fiat_providers" {
			property = "public_key_ed25519"
		}
		entries[0][property] = hex.EncodeToString(public)
		manifest[field], err = json.Marshal(entries)
		require.NoError(t, err)
	}
	renderer := exec.Command("python3", filepath.Join(root, "services/interop/deploy/gateway/render.py"), "--network-id", env["LAYERX_INTEROP_NETWORK_ID"], "--receipt-keys-file", pinFile, "--out", files["LAYERX_INTEROP_CONFIG"])
	renderer.Env = os.Environ()
	for _, field := range []string{"ap2_keys", "ap2_assets", "visa_agents", "visa_targets", "fiat_providers"} {
		renderer.Env = append(renderer.Env, "LAYERX_BETA_INTEROP_"+strings.ToUpper(field)+"="+string(manifest[field]))
	}
	run(renderer)
	// Ensure PEM credentials are valid as well as their runtime DER projections.
	certPEM, err := os.ReadFile(certPath)
	require.NoError(t, err)
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	_, err = x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
}
