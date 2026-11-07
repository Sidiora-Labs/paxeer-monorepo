package bringup

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type SearchSelection struct{ Primary, Backup string }

func SelectSearchServing(inventory PublicInventory, listing io.Reader, primary, backup string) (SearchSelection, error) {
	roles := map[string]string{}
	scanner := bufio.NewScanner(listing)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || (fields[0] != "serve" && fields[0] != "validator") {
			return SearchSelection{}, errors.New("invalid serving discovery")
		}
		if err := ArchiveName(inventory, fields[1]); err != nil {
			return SearchSelection{}, err
		}
		if _, ok := roles[fields[1]]; ok {
			return SearchSelection{}, errors.New("duplicate serving discovery")
		}
		roles[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return SearchSelection{}, err
	}
	serving := []string{}
	for _, endpoint := range inventory.Nodes {
		name := strings.TrimPrefix(endpoint, "https://")
		if roles[name] == "serve" {
			serving = append(serving, name)
		}
	}
	if primary == "" {
		for _, name := range serving {
			if name != backup {
				primary = name
				break
			}
		}
	}
	if backup == "" {
		for _, name := range serving {
			if name != primary {
				backup = name
				break
			}
		}
	}
	if primary == "" || backup == "" || primary == backup || roles[primary] != "serve" || roles[backup] != "serve" {
		return SearchSelection{}, errors.New("primary and backup must be distinct discovered serving nodes")
	}
	return SearchSelection{primary, backup}, nil
}

type searchDependency struct {
	Dependency  string  `json:"dependency"`
	State       string  `json:"state"`
	Critical    bool    `json:"critical"`
	LastSuccess *uint64 `json:"last_success_unix_ms"`
}
type searchRole struct {
	Role         string             `json:"role"`
	State        string             `json:"state"`
	Dependencies []searchDependency `json:"dependencies"`
	Budget       uint64             `json:"freshness_budget_ms"`
	Deadline     uint64             `json:"first_use_deadline_unix_ms"`
}
type searchReadiness struct {
	Version  int          `json:"version"`
	Ready    bool         `json:"ready"`
	Roles    []searchRole `json:"roles"`
	Checked  uint64       `json:"checked_at_unix_ms"`
	Network  uint64       `json:"network_id"`
	Protocol int          `json:"protocol_version"`
}

func ProbeSearchReady(ctx context.Context, client *http.Client, endpoint string, now time.Time) error {
	if !strings.HasPrefix(endpoint, "https://") {
		return errors.New("search readiness requires HTTPS")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint+"/readyz", nil)
	if e != nil {
		return e
	}
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, e := c.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil {
		return e
	}
	if len(raw) > 65536 {
		return errors.New("search readiness exceeds bound")
	}
	var d searchReadiness
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil || dec.Decode(new(any)) != io.EOF || d.Version != 1 || !d.Ready || res.StatusCode != 200 || d.Network == 0 || d.Protocol != 3 || len(d.Roles) < 1 || len(d.Roles) > 3 {
		return errors.New("search readiness refused")
	}
	// Reject duplicate keys before typed decoding; json.Unmarshal alone keeps the last value.
	if e = uniqueJSON(raw); e != nil {
		return e
	}
	exact := func(raw json.RawMessage, keys ...string) (map[string]json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || len(fields) != len(keys) {
			return nil, errors.New("readiness object shape differs")
		}
		for _, key := range keys {
			if _, ok := fields[key]; !ok {
				return nil, errors.New("required readiness field absent")
			}
		}
		return fields, nil
	}
	root, e := exact(raw, "version", "ready", "roles", "checked_at_unix_ms", "network_id", "protocol_version")
	if e != nil {
		return e
	}
	var roles []json.RawMessage
	if json.Unmarshal(root["roles"], &roles) != nil {
		return errors.New("invalid readiness roles")
	}
	for _, role := range roles {
		fields, e := exact(role, "role", "state", "dependencies", "freshness_budget_ms", "first_use_deadline_unix_ms")
		if e != nil {
			return e
		}
		var deps []json.RawMessage
		if json.Unmarshal(fields["dependencies"], &deps) != nil {
			return errors.New("invalid readiness dependencies")
		}
		for _, dep := range deps {
			if _, e = exact(dep, "dependency", "state", "critical", "last_success_unix_ms"); e != nil {
				return e
			}
		}
	}
	expected := map[string][]string{"paid_delivery": {"index", "content_storage", "payment_journal", "settlement_authority"}, "evm_attestor": {"evm_chain", "registered_peer_quorum", "attestor_progress", "attestation_journal"}, "program_relay": {"evm_chain", "registered_peer_quorum", "relay_progress", "program_journal"}}
	seen := map[string]bool{}
	millis := now.UnixMilli()
	if millis < 0 || d.Checked > uint64(millis)+5000 {
		return errors.New("search readiness clock")
	}
	for _, role := range d.Roles {
		deps, ok := expected[role.Role]
		if !ok || seen[role.Role] || role.State != "ready" || role.Budget < 1 || role.Budget > 300000 || (uint64(millis) > d.Checked && uint64(millis)-d.Checked > role.Budget) || len(role.Dependencies) != len(deps) {
			return errors.New("search role not ready")
		}
		seen[role.Role] = true
		remaining := map[string]bool{}
		for _, dep := range deps {
			remaining[dep] = true
		}
		for _, dep := range role.Dependencies {
			if !remaining[dep.Dependency] || dep.State != "ready" || !dep.Critical || dep.LastSuccess == nil || *dep.LastSuccess > d.Checked || d.Checked-*dep.LastSuccess > role.Budget {
				return errors.New("search dependency not ready")
			}
			delete(remaining, dep.Dependency)
		}
	}
	if !seen["paid_delivery"] {
		return errors.New("search serving role absent")
	}
	return nil
}

func uniqueJSON(raw []byte) error {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	var value func() error
	value = func() error {
		tok, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := tok.(json.Delim); ok {
			if delim == '{' {
				keys := map[string]bool{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return e
					}
					k, ok := key.(string)
					if !ok || keys[k] {
						return errors.New("duplicate JSON field")
					}
					keys[k] = true
					if e = value(); e != nil {
						return e
					}
				}
			} else if delim == '[' {
				for d.More() {
					if e = value(); e != nil {
						return e
					}
				}
			} else {
				return errors.New("invalid JSON delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if e := value(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("JSON trailing input")
	}
	return nil
}

func RenderSearchFront(ctx context.Context, client *http.Client, selection SearchSelection, output io.Writer) error {
	pin, admitted := selection.Primary, 0
	if ProbeSearchReady(ctx, client, "https://"+pin, time.Now()) == nil {
		admitted = 1
	} else if ProbeSearchReady(ctx, client, "https://"+selection.Backup, time.Now()) == nil {
		pin = selection.Backup
		admitted = 1
		fmt.Fprintf(os.Stderr, "search-front: %s has no valid readiness contract, %s is pinned\n", selection.Primary, pin)
	} else {
		fmt.Fprintf(os.Stderr, "search-front: neither %s nor %s has a valid readiness contract\n", selection.Primary, selection.Backup)
	}
	_, e := fmt.Fprintf(output, "# The serving name of search.paxeer.network, rendered by tools/bringup/search-front.sh.\nsplit_clients \"${xweb_client}\" $xweb_node {\n    * %s;\n}\nmap $xweb_node $xweb_eligible {\n    default 0;\n    %s %d;\n}\n", pin, pin, admitted)
	return e
}
