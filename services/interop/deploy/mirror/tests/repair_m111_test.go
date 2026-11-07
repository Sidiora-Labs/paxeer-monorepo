package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

type repairM111Endpoint struct {
	URL         string `json:"url"`
	Backend     string `json:"independent_backend"`
	Certificate string `json:"ca_certificate_der"`
	Token       string `json:"bearer_token_file"`
}

type repairM111RPC struct {
	Endpoints []repairM111Endpoint `json:"endpoints"`
	Quorum    int                  `json:"quorum"`
}

type repairM111Target struct {
	RPC repairM111RPC `json:"rpc"`
}

type repairM111Config struct {
	Paxeer   repairM111Target  `json:"paxeer"`
	Ethereum repairM111Target  `json:"ethereum"`
	Solana   *repairM111Target `json:"solana"`
}

type repairM111Source struct {
	SHA256     string              `json:"sha256"`
	Arguments  map[string][]string `json:"endpoint_arguments"`
	Renderings []repairM111Config  `json:"renderings"`
}

func repairM111Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func repairM111Digest(t *testing.T, path, want string) {
	t.Helper()
	digest := sha256.Sum256(repairM111Read(t, path))
	if got := hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("%s changed since the production rendering was recorded: got %s, want %s; regenerate testdata/repair_m111 with its recorder", path, got, want)
	}
}

func repairM111Record(t *testing.T, source string) repairM111Source {
	t.Helper()
	var records struct {
		RendererSHA256 string                      `json:"renderer_sha256"`
		Sources        map[string]repairM111Source `json:"sources"`
	}
	if err := json.Unmarshal(repairM111Read(t, filepath.Join("testdata", "repair_m111", "rendered.json")), &records); err != nil {
		t.Fatal(err)
	}
	record, ok := records.Sources[source]
	if !ok {
		t.Fatalf("no production rendering for %s", source)
	}
	repairM111Digest(t, source, record.SHA256)
	repairM111Digest(t, "../render-config.py", records.RendererSHA256)
	if len(record.Renderings) != 2 {
		t.Fatalf("want both Ethereum-only and Solana renderings, got %d", len(record.Renderings))
	}
	return record
}

func repairM111Path(t *testing.T, endpoint, want string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Path != want || parsed.RawQuery != "" || parsed.Fragment != "" {
		t.Fatalf("endpoint %q must be HTTPS with path %q and no credentials, query or fragment", endpoint, want)
	}
	return parsed
}

func repairM111RenderedPaths(t *testing.T, record repairM111Source) {
	t.Helper()
	for index, config := range record.Renderings {
		targets := map[string]repairM111Target{"paxeer": config.Paxeer, "ethereum": config.Ethereum}
		if (config.Solana != nil) != (index == 1) {
			t.Fatalf("rendering %d has wrong Solana presence", index)
		}
		if config.Solana != nil {
			targets["solana"] = *config.Solana
		}
		for chain, target := range targets {
			arguments := record.Arguments[chain]
			if len(arguments) != 2 || len(target.RPC.Endpoints) != 2 || target.RPC.Quorum != 2 {
				t.Fatalf("%s must preserve two independent endpoints and quorum 2: %+v", chain, target)
			}
			for i, endpoint := range target.RPC.Endpoints {
				parts := strings.Split(arguments[i], ",")
				if len(parts) != 4 {
					t.Fatalf("invalid recorded endpoint argument %q", arguments[i])
				}
				want := repairM111Endpoint{URL: parts[0], Backend: parts[1], Certificate: parts[2], Token: parts[3]}
				if !reflect.DeepEqual(endpoint, want) {
					t.Fatalf("%s rendering changed endpoint: got %+v, want %+v", chain, endpoint, want)
				}
				path := "/rpc"
				if chain == "paxeer" {
					path = "/"
				}
				repairM111Path(t, endpoint.URL, path)
				if i > 0 && (endpoint.Backend == target.RPC.Endpoints[0].Backend || endpoint.URL == target.RPC.Endpoints[0].URL) {
					t.Fatalf("%s backends are not independent", chain)
				}
			}
		}
	}
}

func repairM111RootRPC(t *testing.T, record repairM111Source) {
	t.Helper()
	// This is the same production handler and built-in rpc_modules service
	// used by the node's HTTP stack, without the keeper/native runtime.
	handler := rpc.NewServer()
	defer handler.Stop()
	type request struct{ Host, Path, Method string }
	requests := make(chan request, 4)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- request{r.Host, r.URL.RequestURI(), r.Method}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	loopback, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	// Redirect only the TCP connection. Preserve the rendered HTTPS URL,
	// HTTP Host and request target, and verify the loopback TLS certificate.
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, loopback.Host)
	}
	transport.TLSClientConfig.ServerName = loopback.Hostname()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, config := range record.Renderings {
		for _, endpoint := range config.Paxeer.RPC.Endpoints {
			parsed := repairM111Path(t, endpoint.URL, "/")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			connection, err := rpc.DialOptions(ctx, endpoint.URL, rpc.WithHTTPClient(client))
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			var modules map[string]string
			err = connection.CallContext(ctx, &modules, "rpc_modules")
			connection.Close()
			cancel()
			if err != nil {
				t.Fatalf("rendered endpoint %s failed real root JSON-RPC: %v", endpoint.URL, err)
			}
			if !reflect.DeepEqual(modules, map[string]string{"rpc": "1.0"}) {
				t.Fatalf("unexpected production rpc_modules response: %v", modules)
			}
			select {
			case got := <-requests:
				if got != (request{parsed.Host, "/", http.MethodPost}) {
					t.Fatalf("rendered endpoint changed over transport: %+v", got)
				}
			default:
				t.Fatal("no HTTP request reached the production handler")
			}
		}
	}
}

func TestRepairM_111_MirrorShellRPC(t *testing.T) {
	record := repairM111Record(t, "render-config-check.sh")
	source := string(repairM111Read(t, "render-config-check.sh"))
	matches := regexp.MustCompile(`--paxeer-endpoint\s+"([^"]+)"`).FindAllStringSubmatch(source, -1)
	if len(matches) != 4 {
		t.Fatalf("want the normal and refusal-scenario Paxeer endpoint pairs, got %d", len(matches))
	}
	for _, match := range matches {
		repairM111Path(t, strings.Split(match[1], ",")[0], "/")
	}
	repairM111RenderedPaths(t, record)
	repairM111RootRPC(t, record)
}

func TestRepairM_111_MirrorPythonRPC(t *testing.T) {
	record := repairM111Record(t, "test_render_check_arguments.py")
	repairM111RenderedPaths(t, record)
	repairM111RootRPC(t, record)
}
