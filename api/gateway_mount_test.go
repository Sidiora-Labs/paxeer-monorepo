package api_test

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
)

type ModuleBasics = module.BasicManager

func RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux, basics ModuleBasics) {
	authtx.RegisterGRPCGatewayRoutes(clientCtx, mux)
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, mux)
	basics.RegisterGRPCGatewayRoutes(clientCtx, mux)
}

func TestEveryHTTPRuleIsMounted(t *testing.T) {
	mux := runtime.NewServeMux()
	RegisterGRPCGatewayRoutes(client.Context{}, mux, node.ModuleBasics)

	excludedServices := map[string]struct{}{
		"cosmos.base.reflection.v1beta1.ReflectionService":  {},
		"cosmos.base.reflection.v2alpha1.ReflectionService": {},
	}
	sources := make(map[string]string)
	for _, root := range []string{"api", "sdk/proto"} {
		dir := filepath.Join("..", filepath.FromSlash(root))
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".proto" {
				return nil
			}
			name, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			name = filepath.ToSlash(name)
			if previous, exists := sources[name]; exists {
				t.Fatalf("duplicate proto source %s in %s and %s", name, previous, root)
			}
			sources[name] = root + "/" + name
			return nil
		}); err != nil {
			t.Fatalf("collect %s proto sources: %v", root, err)
		}
	}

	variable := regexp.MustCompile(`\{[^{}]+\}`)
	expandPath := func(template string) string {
		path := variable.ReplaceAllStringFunc(template, func(match string) string {
			_, pattern, hasPattern := strings.Cut(match[1:len(match)-1], "=")
			if !hasPattern {
				return "1"
			}
			return strings.ReplaceAll(strings.ReplaceAll(pattern, "**", "1"), "*", "1")
		})
		path = strings.ReplaceAll(strings.ReplaceAll(path, "**", "1"), "*", "1")
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "{}") {
			t.Fatalf("invalid HTTP path template %q", template)
		}
		return path
	}

	checked := 0
	var checkRule func(string, string, *annotations.HttpRule)
	checkRule = func(file, rpc string, rule *annotations.HttpRule) {
		var method, template string
		switch pattern := rule.GetPattern().(type) {
		case *annotations.HttpRule_Get:
			method, template = http.MethodGet, pattern.Get
		case *annotations.HttpRule_Post:
			method, template = http.MethodPost, pattern.Post
		case *annotations.HttpRule_Put:
			method, template = http.MethodPut, pattern.Put
		case *annotations.HttpRule_Delete:
			method, template = http.MethodDelete, pattern.Delete
		case *annotations.HttpRule_Patch:
			method, template = http.MethodPatch, pattern.Patch
		case *annotations.HttpRule_Custom:
			method, template = pattern.Custom.GetKind(), pattern.Custom.GetPath()
		default:
			t.Fatalf("%s:%s: HTTP rule has no method/path", file, rpc)
		}
		path := expandPath(template)
		var body io.Reader
		if rule.GetBody() == "*" {
			body = strings.NewReader("{}")
		}
		req := httptest.NewRequest(method, path, body).WithContext(t.Context())
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		checked++
		if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
			t.Errorf("%s:%s %s %s: unmounted (status %d, template %s): %s", file, rpc, method, path, response.Code, template, response.Body.String())
		}
		for _, binding := range rule.GetAdditionalBindings() {
			checkRule(file, rpc, binding)
		}
	}

	for _, file := range bufImage(t).GetFile() {
		source, owned := sources[file.GetName()]
		if !owned {
			continue
		}
		delete(sources, file.GetName())
		for _, service := range file.GetService() {
			name := file.GetPackage() + "." + service.GetName()
			if _, excluded := excludedServices[name]; excluded {
				continue
			}
			for _, rpc := range service.GetMethod() {
				if !proto.HasExtension(rpc.GetOptions(), annotations.E_Http) {
					continue
				}
				rule, ok := proto.GetExtension(rpc.GetOptions(), annotations.E_Http).(*annotations.HttpRule)
				if !ok || rule == nil {
					t.Fatalf("%s:%s.%s: google.api.http is not an HttpRule", source, name, rpc.GetName())
				}
				checkRule(source, name+"."+rpc.GetName(), rule)
			}
		}
	}
	for _, source := range sources {
		t.Errorf("%s: proto source missing from buf workspace image", source)
	}
	if checked == 0 {
		t.Fatal("no HTTP rules checked in api/ or sdk/proto/")
	}
	t.Logf("checked %d HTTP rules and additional bindings", checked)
}
