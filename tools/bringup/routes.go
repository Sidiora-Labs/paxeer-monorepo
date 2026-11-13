package bringup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ProbeRoutes uses the operator's exact hosted JSON-RPC URL. Only node
// methods are sent to the canonical root, never to a synthesized /rpc route.
func ProbeRoutes(ctx context.Context, client *http.Client, inventory PublicInventory, gateway, account, token string, output io.Writer) error {
	u, err := url.Parse(gateway)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return errors.New("hosted gateway URL must be explicit and protected")
	}
	selector := regexp.MustCompile(`^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$`)
	if !selector.MatchString(account) {
		return errors.New("account must be an EVM address or explicit DID")
	}
	if gateway == inventory.Canonical {
		return errors.New("node and hosted gateway must have separate routes")
	}
	calls := []struct {
		endpoint, method string
		params           []string
	}{
		{inventory.Canonical, "eth_chainId", []string{}},
		{gateway, "px_getNetwork", []string{}},
		{gateway, "px_getAccount", []string{account}},
	}
	failures := 0
	for _, call := range calls {
		result, err := callRPC(ctx, client, call.endpoint, call.method, call.params, token, call.endpoint == gateway)
		if err == nil {
			err = validateRouteResult(call.method, result, account)
		}
		if err != nil {
			fmt.Fprintf(output, "fail %s answer=invalid\n", call.method)
			failures++
		} else {
			fmt.Fprintf(output, "pass %s route=%s\n", call.method, map[bool]string{true: "node", false: "hosted-gateway"}[call.endpoint == inventory.Canonical])
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d routed checks failed", failures)
	}
	return nil
}

func callRPC(ctx context.Context, client *http.Client, endpoint, method string, params any, token string, hosted bool) (json.RawMessage, error) {
	body, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", 1, method, params})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if hosted && token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	// Redirects must not move a credential to a different owner or route.
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := bounded.Do(request)
	if err != nil {
		return nil, errors.New("RPC transport failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("RPC HTTP status failed")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, errors.New("RPC body unreadable")
	}
	var doc struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := uniqueJSON(data); err != nil {
		return nil, errors.New("ambiguous RPC envelope")
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.JSONRPC != "2.0" || string(doc.ID) != "1" || len(doc.Result) == 0 || string(doc.Result) == "null" || (len(doc.Error) > 0 && string(doc.Error) != "null") {
		return nil, errors.New("invalid RPC envelope")
	}
	return doc.Result, nil
}

func validateRouteResult(method string, data json.RawMessage, selector string) error {
	switch method {
	case "eth_chainId":
		if string(data) != `"0x7d"` {
			return errors.New("wrong chain")
		}
	case "px_getNetwork":
		var value struct {
			NetworkID string `json:"network_id"`
			Paxeer    struct {
				ChainID     string `json:"chain_id"`
				LatestBlock string `json:"latest_block"`
			} `json:"paxeer"`
			Consensus *struct {
				LatestHeight    string `json:"latest_height"`
				LatestBlockHash string `json:"latest_block_hash"`
				LatestAppHash   string `json:"latest_app_hash"`
				CatchingUp      *bool  `json:"catching_up"`
			} `json:"consensus"`
		}
		hex := regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
		height := regexp.MustCompile(`^[0-9]+$`)
		hash := regexp.MustCompile(`^([0-9a-fA-F]{2})+$`)
		if json.Unmarshal(data, &value) != nil || value.NetworkID == "" || value.Paxeer.ChainID != "0x7d" || !hex.MatchString(value.Paxeer.LatestBlock) || value.Consensus == nil || value.Consensus.CatchingUp == nil || *value.Consensus.CatchingUp || !height.MatchString(value.Consensus.LatestHeight) || !hash.MatchString(value.Consensus.LatestBlockHash) || !hash.MatchString(value.Consensus.LatestAppHash) {
			return errors.New("hosted consensus unavailable or malformed")
		}
	case "px_getAccount":
		var value struct {
			Account *struct {
				EVM    *string `json:"evm_address"`
				Pax    *string `json:"pax_address"`
				DID    *string `json:"layerx_did"`
				Native *string `json:"layerx_account"`
				Bound  *bool   `json:"bound"`
			} `json:"account"`
			Paxeer *struct {
				Address string  `json:"address"`
				Balance string  `json:"balance"`
				Nonce   string  `json:"nonce"`
				Height  *uint64 `json:"height"`
			} `json:"paxeer"`
		}
		hex := regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
		address := regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
		if json.Unmarshal(data, &value) != nil || value.Account == nil || value.Account.Bound == nil || value.Paxeer == nil || value.Paxeer.Height == nil || !address.MatchString(value.Paxeer.Address) || !hex.MatchString(value.Paxeer.Balance) || !hex.MatchString(value.Paxeer.Nonce) {
			return errors.New("account evidence unavailable")
		}
		if strings.HasPrefix(selector, "0x") && !strings.EqualFold(selector, value.Paxeer.Address) {
			return errors.New("account evidence belongs to another address")
		}
		if value.Account.EVM == nil || !strings.EqualFold(*value.Account.EVM, value.Paxeer.Address) {
			return errors.New("account binding disagrees with chain evidence")
		}
		if strings.HasPrefix(selector, "did:") && (value.Account.DID == nil || *value.Account.DID != selector) {
			return errors.New("account evidence belongs to another DID")
		}
	default:
		return errors.New("unsupported probe")
	}
	return nil
}

func ValidateGatewayReadiness(r io.Reader, now time.Time) error {
	var doc struct {
		Version    int    `json:"readiness_version"`
		Service    string `json:"service"`
		Status     string `json:"status"`
		Observed   int64  `json:"observed_at_ms"`
		ValidUntil int64  `json:"valid_until_ms"`
		Backends   map[string]struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		} `json:"backends"`
	}
	if json.NewDecoder(r).Decode(&doc) != nil || doc.Version != 1 || doc.Service != "layerx-gateway" || doc.Status != "ready" || doc.Observed <= 0 || doc.Observed > now.UnixMilli() || doc.ValidUntil < now.UnixMilli() || doc.ValidUntil <= doc.Observed || doc.ValidUntil-doc.Observed > 30_000 {
		return errors.New("hosted gateway readiness is invalid or stale")
	}
	for _, required := range []string{"durable_store", "paxeer_chain", "consensus"} {
		backend, ok := doc.Backends[required]
		if !ok || backend.State != "ready" {
			return errors.New("hosted gateway required backend unavailable")
		}
	}
	for _, backend := range doc.Backends {
		if backend.Reason != "not_configured" && backend.State != "ready" {
			return errors.New("hosted gateway configured backend unavailable")
		}
	}
	return nil
}
