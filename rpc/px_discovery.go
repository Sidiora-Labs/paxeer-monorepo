package evmrpc

import (
	"context"
	_ "embed"
	"encoding/json"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type pxProbe struct {
	name, address, method, contract string
	args                            []interface{}
}

var pxProbes = []pxProbe{
	{"exchange", "0x0000000000000000000000000000000000001015", "totalMarkets", `[{"type":"function","name":"totalMarkets","inputs":[],"outputs":[{"type":"uint256"}]}]`, nil},
	{"bridge", "0x0000000000000000000000000000000000001016", "isPaused", `[{"type":"function","name":"isPaused","inputs":[],"outputs":[{"type":"bool"}]}]`, nil},
	{"launchpad", "0x0000000000000000000000000000000000001017", "getPoolCount", `[{"type":"function","name":"getPoolCount","inputs":[],"outputs":[{"type":"uint256"}]}]`, nil},
	{"ledger", "0x000000000000000000000000000000000000101a", "accountId", `[{"type":"function","name":"accountId","inputs":[{"type":"bytes32"},{"type":"bytes32"}],"outputs":[{"type":"bytes32"}]}]`, []interface{}{[32]byte{}, [32]byte{}}},
	{"pay", "0x000000000000000000000000000000000000101b", "budgetRemaining", `[{"type":"function","name":"budgetRemaining","inputs":[{"type":"bytes32"}],"outputs":[{"type":"uint128"}]}]`, []interface{}{[32]byte{}}},
	{"auth", "0x000000000000000000000000000000000000101c", "isLive", `[{"type":"function","name":"isLive","inputs":[{"type":"bytes32"},{"type":"uint64"},{"type":"uint64"}],"outputs":[{"type":"bool"},{"type":"string"}]}]`, []interface{}{[32]byte{}, uint64(0), uint64(0)}},
	{"programs", "0x000000000000000000000000000000000000101d", "getCurrentFeeSchedule", `[{"type":"function","name":"getCurrentFeeSchedule","inputs":[],"outputs":[{"type":"uint32"},{"type":"uint64[7]"},{"type":"bytes32"}]}]`, nil},
}

func (a *pxAccountAPI) GetCapabilities(ctx context.Context) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getCapabilities", a.state.connectionType, start, returnErr, recover())
	}()
	a.capabilitiesMu.Lock()
	defer a.capabilitiesMu.Unlock()
	if a.capabilities != nil && time.Since(a.probedAt) < 15*time.Second {
		return a.capabilities, nil
	}
	height, err := a.height(ctx)
	if err != nil {
		return nil, err
	}
	addresses := map[string]string{}
	result = map[string]interface{}{"addresses": addresses, "rpc_height": strconv.FormatInt(height, 10), "probed_at": start.Unix()}
	for _, probe := range pxProbes {
		addresses[probe.name] = probe.address
		_, err := a.call(ctx, height, common.HexToAddress(probe.address), mustPxABI(probe.contract), probe.method, probe.args...)
		if ctx.Err() != nil {
			return nil, pxUnavailable("paxeer_state_unavailable")
		}
		result[probe.name] = err == nil
	}
	a.probedAt = start
	a.capabilities = result
	return result, nil
}

//go:generate python3 generate_px_catalogue.py
//go:embed px_route_catalogue.json
var pxRouteDefinitions []byte

func (a *pxAccountAPI) GetRouteCatalogue(ctx context.Context) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getRouteCatalogue", a.state.connectionType, start, returnErr, recover())
	}()
	// Preserve the generated contract metadata, including canonical_origin.
	if err := json.Unmarshal(pxRouteDefinitions, &result); err != nil {
		return nil, pxUnavailable("route_catalogue_unavailable")
	}
	result["node_rpc"] = map[string]interface{}{
		"origin": result["canonical_origin"], "rpc_path": "/", "chain_id": result["chain_id"],
	}
	result["hosted_gateway"] = map[string]interface{}{
		"location_source": "operator-provided", "rpc_path": "/", "catalogue_path": "/v1/routes",
	}
	readiness := map[string]interface{}{}
	for _, service := range result["services"].([]interface{}) {
		definition := service.(map[string]interface{})
		readiness[definition["id"].(string)] = map[string]interface{}{"ready": false, "status": "unknown", "reason": "hosted_dependency_not_measured"}
	}
	readiness["node_rpc"] = map[string]interface{}{"ready": true, "scope": "node_registration", "methods": a.methods()}
	result["readiness"] = readiness
	result["rpc_paths"] = []string{"/"}
	return result, nil
}
