package bringup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runRepairInspector(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	recognized := map[string]bool{"edge-export": true, "edge-import": true, "edge-rpc": true, "edge-ramp": true, "edge-map": true, "edge-http": true, "edge-stream": true, "edge-contract": true, "edge-reload": true, "edge-state": true, "rollout-service": true, "role-instances": true, "internal-readiness": true, "event-role": true, "event-trigger": true, "event-handoff": true, "registry-observe": true, "registry-health": true, "docs-names": true}
	if !recognized[args[0]] {
		return false, nil
	}
	invalid := errors.New("invalid bringup repair arguments")
	switch args[0] {
	case "edge-export", "edge-import":
		if len(args) != 2 {
			return true, invalid
		}
		s, e := OpenEdgeState(args[1])
		if e != nil {
			return true, e
		}
		defer s.Close()
		if args[0] == "edge-export" {
			return true, s.Export(output)
		}
		changed, e := s.Import(input)
		if e != nil {
			return true, e
		}
		if changed {
			fmt.Fprintln(output, "changed")
		} else {
			fmt.Fprintln(output, "unchanged")
		}
		return true, nil
	case "edge-map":
		if len(args) != 5 {
			return true, invalid
		}
		r, e := EdgeMapLine(args[1], args[2], args[3], args[4])
		if e != nil {
			return true, e
		}
		_, e = fmt.Fprintln(output, r.String())
		return true, e
	case "edge-http":
		if len(args) != 10 {
			return true, invalid
		}
		r := EdgeRoute{Name: args[1], Mode: "http", Upstream: args[3], Port: "443", Kind: args[2], SNI: args[4]}
		if args[2] == "-" && args[5] == "plain" {
			r.Mode = "rpc"
		}
		return true, RenderEdgeHTTP(output, r, args[6], args[7], args[8], args[5] == "tls", args[9] == "1")
	case "edge-rpc":
		if len(args) != 9 {
			return true, invalid
		}
		pool, e := os.ReadFile(args[7])
		if e != nil {
			return true, e
		}
		down, e := os.ReadFile(args[8])
		if e != nil {
			return true, e
		}
		return true, RenderEdgeRPC(output, args[1], strings.Fields(string(pool)), strings.Fields(string(down)), args[3], args[4], args[5], args[2] == "tls", args[6] == "1")
	case "edge-ramp":
		if len(args) != 2 {
			return true, invalid
		}
		rows, _, e := ReadEdgeContract(input)
		if e != nil {
			return true, e
		}
		r, e := SelectRampRoute(rows, args[1])
		if e != nil {
			return true, e
		}
		_, e = fmt.Fprintln(output, r.String())
		return true, e
	case "edge-stream":
		if len(args) != 3 {
			return true, invalid
		}
		rows, e := ReadEdgeManifest(input, false)
		if e != nil {
			return true, e
		}
		return true, RenderEdgeStream(output, rows, args[1], args[2])
	case "edge-contract":
		if len(args) != 1 && len(args) != 3 {
			return true, invalid
		}
		rows, sites, e := ReadEdgeContract(input)
		if e != nil {
			return true, e
		}
		if len(args) == 1 {
			for _, r := range rows {
				fmt.Fprintln(output, r.String())
			}
			return true, nil
		}
		for _, r := range rows {
			if r.Name == args[1] && r.Port == args[2] {
				members, e := EdgeRouteContract(r, sites)
				if e != nil {
					return true, e
				}
				for _, member := range members {
					fmt.Fprintln(output, member)
				}
				return true, nil
			}
		}
		return true, errors.New("edge route unregistered")
	case "edge-reload":
		if len(args) != 4 {
			return true, invalid
		}
		validation, e := strconv.Atoi(args[2])
		if e != nil {
			return true, invalid
		}
		reload, e := strconv.Atoi(args[3])
		if e != nil {
			return true, invalid
		}
		state, e := OpenEdgeState(args[1])
		if e != nil {
			return true, e
		}
		defer state.Close()
		return true, state.ReloadResult(validation, reload)
	case "edge-state":
		if len(args) != 3 {
			return true, invalid
		}
		s, e := OpenEdgeState(args[2])
		if e != nil {
			return true, e
		}
		defer s.Close()
		switch args[1] {
		case "begin":
			e = s.Begin()
		case "commit":
			e = s.Finish(true)
		case "rollback":
			e = s.Finish(false)
		case "recover":
			e = s.Recover()
		case "reconcile":
			e = s.Reconcile()
		default:
			e = invalid
		}
		return true, e
	case "rollout-service":
		if len(args) != 3 {
			return true, invalid
		}
		f, e := os.Open(args[1])
		if e != nil {
			return true, e
		}
		defer f.Close()
		r, e := ReadRolloutService(f, args[2])
		if e != nil {
			return true, e
		}
		service, unit, host := r.Service, r.Unit, r.HostKey()
		if service == "" {
			service = "-"
		}
		if unit == "" {
			unit = "-"
		}
		if r.Target != "box" {
			host = "-"
		}
		_, e = fmt.Fprintln(output, r.Target, service, host, unit, r.Health)
		return true, e
	case "role-instances":
		if len(args) != 1 {
			return true, invalid
		}
		ids, e := ReadRoleInstances(input)
		if e != nil {
			return true, e
		}
		for _, id := range ids {
			fmt.Fprintln(output, id)
		}
		return true, nil
	case "internal-readiness":
		if len(args) != 3 && len(args) != 4 {
			return true, invalid
		}
		bundle, e := ReadProtectedBundle(args[2])
		if e != nil {
			return true, e
		}
		client, e := BundleTLSClient(bundle)
		if e != nil {
			return true, e
		}
		defer client.CloseIdleConnections()
		if len(args) == 4 {
			if e = SelectInternalOrigin(client, args[1], args[3]); e != nil {
				return true, e
			}
		}
		if e = ProbeInternalReady(ctx, client, args[1]); e != nil {
			return true, e
		}
		_, e = fmt.Fprintln(output, "pass readiness http=200 ready=true identity=human-event-client")
		return true, e
	case "event-role":
		if len(args) != 2 {
			return true, invalid
		}
		env, e := readProcessEnvironment("/proc", "layerx-event-source", map[string]string{"journey": "journeys", "approval": "approvals", "payment": "payments", "program": "programs"}[args[1]])
		if e != nil {
			return true, e
		}
		if e = InspectEventRole(env, args[1]); e != nil {
			return true, e
		}
		_, e = fmt.Fprintf(output, "pass event-role kind=%s material=present role=matched\n", args[1])
		return true, e
	case "event-trigger":
		if len(args) != 1 {
			return true, invalid
		}
		env, e := ReadProcessEnvironment("/proc", "layerx-webhooks")
		if e != nil {
			return true, e
		}
		if env["LAYERX_WEBHOOKS_ROLE"] != "ingress" {
			return true, errors.New("webhook trigger role mismatch")
		}
		token, e := os.ReadFile(env["LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE"])
		if e != nil || strings.TrimSpace(string(token)) == "" {
			return true, errors.New("webhook trigger credential absent")
		}
		_, e = fmt.Fprintln(output, "pass event-trigger role=ingress material=present")
		return true, e
	case "event-handoff":
		if len(args) != 3 {
			return true, invalid
		}
		env, e := ReadProcessEnvironment("/proc", args[2])
		if e != nil {
			return true, e
		}
		return true, ProbeEventHandoff(ctx, env, args[1], output)
	case "registry-observe", "registry-health":
		if (args[0] == "registry-observe" && len(args) != 4) || (args[0] == "registry-health" && len(args) != 2) {
			return true, invalid
		}
		env, e := ReadProcessEnvironment("/proc", "layerx-program-registry")
		if e != nil {
			return true, e
		}
		listen := env["LAYERX_REGISTRY_LISTEN"]
		if listen == "" {
			listen = "127.0.0.1:9420"
		}
		_, port, e := net.SplitHostPort(listen)
		if e != nil {
			return true, errors.New("registry listener invalid")
		}
		bundle, e := ReadProtectedBundle(args[1])
		if e != nil {
			return true, e
		}
		client, e := registryLoopbackClient(bundle, port)
		if e != nil {
			return true, e
		}
		defer client.CloseIdleConnections()
		endpoint := "https://layerx-program-registry:" + port
		status, b, _, e := jsonRequest(ctx, client, "GET", endpoint+"/healthz", "", "", nil)
		var readiness struct{ Status string }
		if e != nil || status != 200 || json.Unmarshal(b, &readiness) != nil || readiness.Status != "ready" {
			return true, errors.New("registry readiness refused")
		}
		// The same trust roots without a client leaf must be refused by TLS.
		anonymous, e := liveHTTPClient(filepath.Join(args[1], "ca.pem"))
		if e != nil {
			return true, e
		}
		anonymous.Transport = client.Transport.(*http.Transport).Clone()
		anonymous.Transport.(*http.Transport).TLSClientConfig = anonymous.Transport.(*http.Transport).TLSClientConfig.Clone()
		anonymous.Transport.(*http.Transport).TLSClientConfig.Certificates = nil
		_, _, _, e = jsonRequest(ctx, anonymous, "GET", endpoint+"/healthz", "", "", nil)
		anonymous.CloseIdleConnections()
		if e == nil {
			return true, errors.New("registry admitted absent client identity")
		}
		if args[0] == "registry-health" {
			_, e = fmt.Fprintln(output, "pass registry healthz=ready anonymous=refused")
			return true, e
		}
		token, e := os.ReadFile(env["LAYERX_REGISTRY_REQUEST_TOKEN_FILE"])
		if e != nil || len(strings.TrimSpace(string(token))) == 0 {
			return true, errors.New("registry request authority missing")
		}
		log, e := ObserveRegistryEvents(ctx, &http.Client{Timeout: 10 * time.Second}, env["LAYERX_REGISTRY_CHAIN_RPC_URL"], args[2], args[3])
		if e != nil {
			return true, e
		}
		if e = ProbeRegistryPublication(ctx, client, endpoint, "Bearer "+strings.TrimSpace(string(token)), args[2], log, time.Now()); e != nil {
			return true, e
		}
		_, e = fmt.Fprintf(output, "pass registry publication=verified program=%s committed-block=%d cursor=durable\n", args[2], log.BlockNumber)
		return true, e
	case "docs-names":
		if len(args) < 4 {
			return true, invalid
		}
		f, e := os.Open(args[1])
		if e != nil {
			return true, e
		}
		inventory, e := ReadPublicInventory(f)
		f.Close()
		if e != nil {
			return true, e
		}
		f, e = os.Open(args[2])
		if e != nil {
			return true, e
		}
		allowed, e := DocumentationNames(inventory, f)
		f.Close()
		if e != nil {
			return true, e
		}
		for _, path := range args[3:] {
			b, e := os.ReadFile(path)
			if e != nil {
				return true, e
			}
			if e = ValidateDocumentNames(inventory, allowed, string(b)); e != nil {
				return true, fmt.Errorf("docs-names: %s: %w", path, e)
			}
		}
		return true, nil
	}
	return true, invalid
}
