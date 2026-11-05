package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type frozenMediaOperation struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Service string `json:"service"`
}

func readFrozenMediaCatalog(t *testing.T) map[string]frozenMediaOperation {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate frozen media catalog test source")
	}
	catalogPath := filepath.Join(filepath.Dir(filename), "../../../../../sdk/launchpad/packages/sdk/src/operations.gen.ts")
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read frozen operation catalog: %v", err)
	}
	_, definitions, ok := strings.Cut(string(source), "export const operationDefinitions = ")
	if !ok {
		t.Fatal("frozen operation catalog has no operationDefinitions object")
	}
	var operations map[string]frozenMediaOperation
	decoder := json.NewDecoder(strings.NewReader(definitions))
	if err := decoder.Decode(&operations); err != nil {
		t.Fatalf("decode frozen operationDefinitions: %v", err)
	}
	if suffix := strings.TrimSpace(definitions[decoder.InputOffset():]); suffix != "as const;" {
		t.Fatalf("unexpected operationDefinitions suffix: %q", suffix)
	}
	if len(operations) == 0 {
		t.Fatal("frozen operation catalog is empty")
	}
	for id, operation := range operations {
		if id == "" || operation.Method == "" || operation.Path == "" || operation.Service == "" {
			t.Fatalf("incomplete frozen operation %q: %+v", id, operation)
		}
	}
	return operations
}

type frozenMediaRouteTable struct {
	name    string
	service string
	routes  []ConnectedRoute
}

func frozenMediaConnectedTables() []frozenMediaRouteTable {
	return []frozenMediaRouteTable{
		{name: "UserConnectedRoutes", service: "user", routes: UserConnectedRoutes()},
		{name: "MetadataConnectedRoutes", service: "metadata", routes: MetadataConnectedRoutes()},
		{name: "LivestreamConnectedRoutes", service: "livestream", routes: LivestreamConnectedRoutes()},
		{name: "SocialConnectedRoutes", service: "social", routes: SocialConnectedRoutes()},
	}
}

func TestConnectedTablesCoverFrozenMediaCatalog(t *testing.T) {
	operations := readFrozenMediaCatalog(t)
	tables := append(frozenMediaConnectedTables(), frozenMediaRouteTable{
		name: "RetiredConnectedRoutes", routes: RetiredConnectedRoutes(),
	})
	owners := make(map[string][]string)
	tableRoutes := make(map[string]map[string]bool)
	for _, table := range tables {
		tableRoutes[table.service] = make(map[string]bool)
		for _, route := range table.routes {
			key := route.Method + " " + route.Template
			owners[key] = append(owners[key], table.name)
			tableRoutes[table.service][key] = true
		}
	}
	for service, surface := range FrozenConnectedSurface() {
		for path, methods := range surface {
			for _, method := range strings.Fields(methods) {
				key := method + " " + path
				if !tableRoutes[service][key] {
					owners[key] = append(owners[key], "FrozenConnectedSurface/"+service)
				}
			}
		}
	}
	var mediaIDs []string
	for id, operation := range operations {
		if operation.Service == "media" {
			mediaIDs = append(mediaIDs, id)
		}
	}
	if len(mediaIDs) == 0 {
		t.Fatal("frozen operation catalog has no media operations")
	}
	sort.Strings(mediaIDs)
	for _, id := range mediaIDs {
		t.Run(id, func(t *testing.T) {
			operation := operations[id]
			key := operation.Method + " " + operation.Path
			if matches := owners[key]; len(matches) != 1 {
				t.Errorf("frozen media operation %s (%s) appears %d times in connected or retired tables %v; want exactly once", id, key, len(matches), matches)
			}
		})
	}
}

func TestConnectedTablesForwardOnlyCatalogTemplates(t *testing.T) {
	operations := readFrozenMediaCatalog(t)
	catalogPaths := make(map[string]bool)
	for _, operation := range operations {
		catalogPaths[operation.Path] = true
	}
	connectedTemplates := map[string]bool{
		"/v3/metadata-publication-authority":                            true,
		"/v3/tokens/{tokenAddress}/metadata-publications":               true,
		"/v3/tokens/{tokenAddress}/metadata-publications/{operationId}": true,
		"/v2/voice-rooms/{roomId}/recording/object":                     true,
		"/v2/streams/live":                                          true,
		"/v2/streams/pool/{poolAddress}":                            true,
		"/v2/streams/{streamId}":                                    true,
		"/v2/streams/{streamId}/viewer-session":                     true,
		"/v2/streams/{streamId}/heartbeat":                          true,
		"/v2/streams/{streamId}/go-live":                            true,
		"/v2/streams/{streamId}/end":                                true,
		"/v2/streams/{streamId}/rotate-key":                         true,
		"/v2/streams/{streamId}/restore":                            true,
		"/v2/streams/{streamId}/key-rotation":                       true,
		"/v2/streams/{streamId}/private-playback":                   true,
		"/v2/streams/{streamId}/ingest-key":                         true,
		"/v2/me/exports/{jobId}/download":                           true,
		"/v2/chat/history":                                          true,
		"/v2/social/users/{wallet}/followers":                       true,
		"/v2/social/users/{wallet}/following":                       true,
		"/v2/social/users/{wallet}/follow-stats":                    true,
		"/v2/social/follow/{followee}":                              true,
		"/v2/social/pool/{poolAddress}/comments":                    true,
		"/v2/social/comments/{id}":                                  true,
		"/v2/social/comments/{id}/hide":                             true,
		"/v2/social/comments/{id}/like":                             true,
		"/v2/social/callouts/{id}/references":                       true,
		"/v2/social/me/social/privacy":                              true,
		"/v2/social/me/social/blocks/{wallet}":                      true,
		"/v2/pools/{poolAddress}/comments":                          true,
		"/v2/comments/{commentId}":                                  true,
		"/v2/comments/{commentId}/likes/me":                         true,
		"/v2/reports":                                               true,
		"/v2/callouts":                                              true,
		"/v2/pools/{poolAddress}/callouts":                          true,
		"/v2/bounties":                                              true,
		"/v2/bounties/stats":                                        true,
		"/v2/bounties/{bountyId}":                                   true,
		"/v2/bounties/{bountyId}/submissions":                       true,
		"/v2/bounties/{bountyId}/submissions/{submissionId}:accept": true,
	}
	for _, table := range frozenMediaConnectedTables() {
		t.Run(table.name, func(t *testing.T) {
			versioned := 0
			for _, route := range table.routes {
				if !strings.HasPrefix(route.Template, "/v2/") && !strings.HasPrefix(route.Template, "/v3/") {
					continue
				}
				versioned++
				if !catalogPaths[route.Template] && !connectedTemplates[route.Template] {
					t.Errorf("%s forwards %s %s, which is neither a frozen catalog path nor a named connected template", table.name, route.Method, route.Template)
				}
			}
			if versioned == 0 {
				t.Errorf("%s has no v2 or v3 templates to check", table.name)
			}
		})
	}
}
