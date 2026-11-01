package core

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIPathsMatchRoutesMap(t *testing.T) {
	data, err := os.ReadFile("../../../rpc/openapi/openapi.yaml")
	require.NoError(t, err)

	var document struct {
		Paths map[string]yaml.Node `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(data, &document))
	require.NotEmpty(t, document.Paths)
	require.Contains(t, document.Paths, "/unsafe_flush_mempool")

	documented := make(map[string]struct{}, len(document.Paths))
	for path := range document.Paths {
		require.True(t, strings.HasPrefix(path, "/"), "OpenAPI path %q must start with /", path)
		documented[strings.TrimPrefix(path, "/")] = struct{}{}
	}

	routes := NewRoutesMap(&Environment{}, &RouteOptions{Unsafe: true})
	registered := make(map[string]struct{}, len(routes))
	for name := range routes {
		registered[name] = struct{}{}
	}
	require.Equal(t, registered, documented, "OpenAPI paths must match registered JSON-RPC routes")
}
