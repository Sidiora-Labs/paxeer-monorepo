package bringup

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const PublicNamesSection = "contract_context.108.decision.public_names"

type PublicInventory struct {
	Canonical string   `json:"canonical"`
	Nodes     []string `json:"nodes"`
}

// ReadPublicInventory consumes the maintained public endpoint table, not a
// second hardcoded fleet. Numbered nodes must be unique and contiguous.
func ReadPublicInventory(r io.Reader) (PublicInventory, error) {
	var inventory PublicInventory
	row := regexp.MustCompile("^\\| ([0-9]+|Canonical alias) \\| `([^`]+)` \\|")
	seen := map[string]bool{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		match := row.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}
		u, err := url.Parse(match[2])
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" || seen[match[2]] {
			return inventory, errors.New("invalid or duplicate public node endpoint")
		}
		seen[match[2]] = true
		if match[1] == "Canonical alias" {
			if inventory.Canonical != "" {
				return inventory, errors.New("duplicate canonical endpoint")
			}
			inventory.Canonical = match[2]
		} else {
			n, _ := strconv.Atoi(match[1])
			if n != len(inventory.Nodes)+1 || !strings.HasPrefix(u.Hostname(), fmt.Sprintf("api%d.", n)) {
				return inventory, errors.New("noncontiguous public node inventory")
			}
			inventory.Nodes = append(inventory.Nodes, match[2])
		}
	}
	if err := scanner.Err(); err != nil {
		return inventory, err
	}
	if inventory.Canonical == "" || len(inventory.Nodes) == 0 {
		return inventory, errors.New("public node inventory is incomplete")
	}
	domain := strings.SplitN(strings.TrimPrefix(inventory.Nodes[0], "https://"), ".", 2)[1]
	for n, endpoint := range inventory.Nodes {
		if endpoint != fmt.Sprintf("https://api%d.%s", n+1, domain) {
			return inventory, errors.New("public node domains disagree")
		}
	}
	return inventory, nil
}

func (i PublicInventory) Endpoints() []string {
	return append([]string{i.Canonical}, i.Nodes...)
}

func (i PublicInventory) ValidatePool(endpoints []string) error {
	allowed := map[string]bool{}
	for _, endpoint := range i.Endpoints() {
		allowed[endpoint] = true
	}
	seen := map[string]bool{}
	if len(endpoints) == 0 {
		return errors.New("empty node pool")
	}
	for _, endpoint := range endpoints {
		if !allowed[endpoint] || seen[endpoint] {
			return errors.New("pool contains an undeclared, routed or duplicate node endpoint")
		}
		seen[endpoint] = true
	}
	if endpoints[0] != i.Canonical {
		return errors.New("canonical node must be first in pool")
	}
	return nil
}

func WalletName(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	section, value := "", ""
	host := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		if section != PublicNamesSection {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "wallet_endpoint" {
			continue
		}
		name, err := strconv.Unquote(strings.TrimSpace(raw))
		if err != nil || !host.MatchString(name) || value != "" {
			return "", errors.New("invalid or duplicate wallet_endpoint declaration")
		}
		value = name
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("wallet_endpoint missing from [%s]", PublicNamesSection)
	}
	return value, nil
}

func ValidateGasConfig(inventory PublicInventory, r io.Reader) (string, error) {
	var config struct {
		ChainID   uint64   `json:"chain_id"`
		Token     string   `json:"token"`
		Endpoints []string `json:"endpoints"`
	}
	if err := json.NewDecoder(r).Decode(&config); err != nil {
		return "", errors.New("gas config unreadable")
	}
	if config.ChainID != 125 || !strings.EqualFold(config.Token, "0x21f7b20a555199fa73A238B1a91FD0f549068fEe") {
		return "", errors.New("gas chain or token mismatch")
	}
	if err := inventory.ValidatePool(config.Endpoints); err != nil {
		return "", err
	}
	return config.Endpoints[0], nil
}

func ValidateWalletPool(inventory PublicInventory, r io.Reader) error {
	var doc struct {
		Components struct {
			RPCPool struct {
				State     string `json:"state"`
				Healthy   int    `json:"healthy"`
				Endpoints []struct {
					URL string `json:"url"`
				} `json:"endpoints"`
			} `json:"rpc_pool"`
		} `json:"components"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return errors.New("wallet rpc_pool unreadable")
	}
	pool := doc.Components.RPCPool
	if pool.State != "up" || pool.Healthy <= 0 || pool.Healthy > len(pool.Endpoints) {
		return errors.New("wallet rpc_pool is not healthy")
	}
	endpoints := make([]string, len(pool.Endpoints))
	for n, endpoint := range pool.Endpoints {
		endpoints[n] = endpoint.URL
	}
	return inventory.ValidatePool(endpoints)
}
