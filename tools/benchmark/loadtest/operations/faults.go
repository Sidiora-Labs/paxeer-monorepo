package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.:-]*$`)

const faultHandle = "1f00:"

type FaultOptions struct {
	Paxd       string        `json:"paxd"`
	SSHKey     string        `json:"ssh_key"`
	ChainID    string        `json:"chain_id"`
	Validators []string      `json:"validators"`
	Concurrent int           `json:"concurrent"`
	Recovery   string        `json:"-"`
	Hold       time.Duration `json:"-"`
}

type RecoveryAction struct {
	Kind      string `json:"kind"`
	Host      string `json:"host,omitempty"`
	Validator string `json:"validator,omitempty"`
	Admin     string `json:"admin,omitempty"`
	Before    string `json:"before,omitempty"`
	Added     string `json:"added,omitempty"`
	Applied   bool   `json:"applied"`
}

type RecoveryRecord struct {
	Version int              `json:"version"`
	Options FaultOptions     `json:"options"`
	Actions []RecoveryAction `json:"actions"`
}

type FaultController struct {
	Commands Commands
	Options  FaultOptions
	Wait     func(context.Context, time.Duration) error
	record   RecoveryRecord
}

func NewFaultController(commands Commands, options FaultOptions) *FaultController {
	return &FaultController{Commands: commands, Options: options, Wait: Sleep, record: RecoveryRecord{Version: 1, Options: options, Actions: []RecoveryAction{}}}
}

func (c *FaultController) ssh(ctx context.Context, host string, args ...string) ([]byte, error) {
	if !hostPattern.MatchString(host) {
		return nil, errors.New("invalid validator host")
	}
	if c.Options.SSHKey == "" {
		return nil, errors.New("SSH key is required")
	}
	argv := []string{"ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-i", c.Options.SSHKey, "--", "ubuntu@" + host}
	argv = append(argv, args...)
	return c.Commands.Run(ctx, Command{Args: argv})
}

func (c *FaultController) Restarts(ctx context.Context) error {
	if c.Options.Concurrent <= 0 {
		return errors.New("concurrency must be positive")
	}
	for start := 0; start < len(c.Options.Validators); start += c.Options.Concurrent {
		for _, host := range c.Options.Validators[start:min(start+c.Options.Concurrent, len(c.Options.Validators))] {
			before, err := c.ssh(ctx, host, "systemctl", "show", "paxd", "--property=InvocationID", "--value")
			if err != nil {
				return err
			}
			if len(strings.TrimSpace(string(before))) == 0 {
				return errors.New("paxd has no active invocation")
			}
			if _, err = c.ssh(ctx, host, "sudo", "-n", "systemctl", "restart", "paxd"); err != nil {
				return err
			}
			if _, err = c.ssh(ctx, host, "systemctl", "is-active", "--quiet", "paxd"); err != nil {
				return err
			}
			after, err := c.ssh(ctx, host, "systemctl", "show", "paxd", "--property=InvocationID", "--value")
			if err != nil {
				return err
			}
			if len(strings.TrimSpace(string(after))) == 0 || strings.TrimSpace(string(after)) == strings.TrimSpace(string(before)) {
				return errors.New("restart did not produce a new active paxd invocation")
			}
		}
		if err := c.Wait(ctx, c.Options.Hold); err != nil {
			return err
		}
	}
	return nil
}

func (c *FaultController) createRecord() error {
	dir, err := privateDirectory(filepath.Dir(c.Options.Recovery))
	if err != nil {
		return err
	}
	defer dir.Close()
	// Exclusive creation refuses an unresolved earlier run or a symlink.
	if err = saveAccountRecord(dir, filepath.Base(c.Options.Recovery), c.record); err != nil {
		return fmt.Errorf("recovery record unavailable; recover the previous run first: %w", err)
	}
	return nil
}

func saveAccountRecord(dir *os.File, name string, record RecoveryRecord) error {
	// Use the same exclusive, no-follow persistence policy as account secrets.
	fd, err := openExclusiveAt(dir, name)
	if err != nil {
		return err
	}
	defer fd.Close()
	if err = json.NewEncoder(fd).Encode(record); err != nil {
		return err
	}
	if err = fd.Sync(); err != nil {
		return err
	}
	if err = fd.Close(); err != nil {
		return err
	}
	return dir.Sync()
}

func (c *FaultController) persist() error { return writeJSONAtomic(c.Options.Recovery, c.record, 0600) }

func (c *FaultController) qdisc(ctx context.Context, host string) (bool, error) {
	raw, err := c.ssh(ctx, host, "sudo", "-n", "tc", "-j", "qdisc", "show", "dev", "ens5")
	if err != nil {
		return false, err
	}
	var entries []struct {
		Kind   string `json:"kind"`
		Handle string `json:"handle"`
		Root   bool   `json:"root"`
	}
	if err = decodeJSON(raw, &entries); err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Handle == faultHandle {
			if entry.Kind != "netem" || !entry.Root {
				return false, errors.New("recovery qdisc handle belongs to an unexpected rule")
			}
			return true, nil
		}
	}
	return false, nil
}

func (c *FaultController) SlowNetwork(ctx context.Context) (err error) {
	if c.Options.Concurrent <= 0 {
		return errors.New("concurrency must be positive")
	}
	if err = c.createRecord(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.Cleanup()) }()
	for start := 0; start < len(c.Options.Validators); start += c.Options.Concurrent {
		for _, host := range c.Options.Validators[start:min(start+c.Options.Concurrent, len(c.Options.Validators))] {
			exists, e := c.qdisc(ctx, host)
			if e != nil {
				return e
			}
			if exists {
				return errors.New("fault qdisc already exists; recover it before starting")
			}
			c.record.Actions = append(c.record.Actions, RecoveryAction{Kind: "network", Host: host})
			if e = c.persist(); e != nil {
				return e
			}
			if _, e = c.ssh(ctx, host, "sudo", "-n", "tc", "qdisc", "add", "dev", "ens5", "root", "handle", faultHandle, "netem", "delay", "200ms", "10ms", "25%"); e != nil {
				return e
			}
			c.record.Actions[len(c.record.Actions)-1].Applied = true
			if e = c.persist(); e != nil {
				return e
			}
			exists, e = c.qdisc(ctx, host)
			if e != nil {
				return e
			}
			if !exists {
				return errors.New("requested network fault was not applied")
			}
		}
		if e := c.Wait(ctx, c.Options.Hold); e != nil {
			return e
		}
		if e := c.restoreActions(ctx); e != nil {
			return e
		}
	}
	return nil
}

func (c *FaultController) paxd(ctx context.Context, args ...string) ([]byte, error) {
	if c.Options.Paxd == "" {
		return nil, errors.New("paxd path is required")
	}
	return c.Commands.Run(ctx, Command{Args: append([]string{c.Options.Paxd}, args...)})
}

func (c *FaultController) admin(ctx context.Context) (string, error) {
	raw, err := c.paxd(ctx, "keys", "list", "--output", "json")
	if err != nil {
		return "", err
	}
	var accounts []struct{ Name, Address string }
	if err = decodeJSON(raw, &accounts); err != nil {
		return "", err
	}
	address := ""
	for _, account := range accounts {
		if account.Name == "admin" {
			if address != "" {
				return "", errors.New("multiple admin accounts")
			}
			address = account.Address
		}
	}
	if address == "" {
		return "", errors.New("admin account is missing")
	}
	return address, nil
}

func parseStake(value string) (*big.Int, error) {
	if value == "" {
		return nil, errors.New("missing delegation amount")
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return nil, errors.New("invalid delegation amount")
		}
	}
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok || amount.Sign() < 0 || amount.BitLen() > 256 {
		return nil, errors.New("invalid delegation amount")
	}
	return amount, nil
}

func stakeTarget(action RecoveryAction) (*big.Int, error) {
	before, err := parseStake(action.Before)
	if err != nil {
		return nil, err
	}
	added, err := parseStake(action.Added)
	if err != nil {
		return nil, err
	}
	if added.Sign() <= 0 {
		return nil, errors.New("recovery addition must be positive")
	}
	target := new(big.Int).Add(before, added)
	if target.BitLen() > 256 {
		return nil, errors.New("delegation balance exceeds uint256")
	}
	return target, nil
}

func (c *FaultController) delegationAmount(ctx context.Context, admin, validator string) (*big.Int, error) {
	raw, err := c.paxd(ctx, "q", "staking", "delegation", admin, validator, "--chain-id", c.Options.ChainID, "--output", "json")
	if err != nil {
		return nil, err
	}
	var value struct {
		Balance struct {
			Amount string `json:"amount"`
		} `json:"balance"`
	}
	if err = decodeJSON(raw, &value); err != nil {
		return nil, err
	}
	return parseStake(value.Balance.Amount)
}

func (c *FaultController) stakingTx(ctx context.Context, operation, validator, amount string) error {
	raw, err := c.paxd(ctx, "tx", "staking", operation, validator, amount+"uhpx", "--from", "admin", "--chain-id", c.Options.ChainID, "--broadcast-mode", "block", "--output", "json", "-y")
	if err != nil {
		return err
	}
	var response struct {
		Code   *uint32 `json:"code"`
		Height string  `json:"height"`
		Hash   string  `json:"txhash"`
	}
	if err = decodeJSON(raw, &response); err != nil {
		return err
	}
	height, e := strconv.ParseInt(response.Height, 10, 64)
	if response.Code == nil || *response.Code != 0 || e != nil || height <= 0 || response.Hash == "" {
		return errors.New("staking transaction was not committed successfully")
	}
	return nil
}

type delegationRow struct {
	Balance    struct{ Amount, Denom string } `json:"balance"`
	Delegation struct {
		Validator string `json:"validator_address"`
	} `json:"delegation"`
}

func (c *FaultController) delegations(ctx context.Context, admin string) ([]delegationRow, error) {
	var rows []delegationRow
	pageKey := ""
	seen := map[string]bool{}
	for {
		args := []string{"q", "staking", "delegations", admin, "--chain-id", c.Options.ChainID, "--output", "json"}
		if pageKey != "" {
			args = append(args, "--page-key", pageKey)
		}
		raw, err := c.paxd(ctx, args...)
		if err != nil {
			return nil, err
		}
		var page struct {
			Responses  []delegationRow `json:"delegation_responses"`
			Pagination struct {
				NextKey string `json:"next_key"`
			} `json:"pagination"`
		}
		if err = decodeJSON(raw, &page); err != nil {
			return nil, err
		}
		rows = append(rows, page.Responses...)
		pageKey = page.Pagination.NextKey
		if pageKey == "" {
			return rows, nil
		}
		if seen[pageKey] {
			return nil, errors.New("delegation pagination repeated a key")
		}
		seen[pageKey] = true
	}
}

func (c *FaultController) DelegationChange(ctx context.Context) (err error) {
	admin, err := c.admin(ctx)
	if err != nil {
		return err
	}
	rows, err := c.delegations(ctx, admin)
	if err != nil {
		return err
	}
	total := new(big.Int)
	validators := []string{}
	seen := map[string]bool{}
	balances := map[string]string{}
	for _, response := range rows {
		amount, e := parseStake(response.Balance.Amount)
		if e != nil || response.Balance.Denom != "uhpx" || response.Delegation.Validator == "" || seen[response.Delegation.Validator] {
			return errors.New("invalid delegation inventory")
		}
		total.Add(total, amount)
		if new(big.Int).Mul(total, big.NewInt(10)).BitLen() > 256 {
			return errors.New("delegation amount overflow")
		}
		validators = append(validators, response.Delegation.Validator)
		seen[response.Delegation.Validator] = true
		balances[response.Delegation.Validator] = amount.String()
	}
	if len(validators) < 4 || total.Sign() <= 0 {
		return errors.New("at least four delegated validators with positive total balance are required")
	}
	if err = c.createRecord(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.Cleanup()) }()
	for _, validator := range validators[:len(validators)/4] {
		action := RecoveryAction{Kind: "delegation", Validator: validator, Admin: admin, Before: balances[validator], Added: new(big.Int).Mul(total, big.NewInt(10)).String()}
		target, e := stakeTarget(action)
		if e != nil {
			return e
		}
		c.record.Actions = append(c.record.Actions, action)
		if err = c.persist(); err != nil {
			return err
		}
		if err = c.stakingTx(ctx, "delegate", validator, action.Added); err != nil {
			return err
		}
		c.record.Actions[len(c.record.Actions)-1].Applied = true
		if err = c.persist(); err != nil {
			return err
		}
		amount, e := c.delegationAmount(ctx, admin, validator)
		if e != nil {
			return e
		}
		if amount.Cmp(target) != 0 {
			return errors.New("delegation did not reach the requested balance")
		}
	}
	return c.Wait(ctx, c.Options.Hold)
}

func (c *FaultController) restoreAction(ctx context.Context, action RecoveryAction) error {
	switch action.Kind {
	case "network":
		exists, err := c.qdisc(ctx, action.Host)
		if err != nil {
			return err
		}
		if exists {
			if _, err = c.ssh(ctx, action.Host, "sudo", "-n", "tc", "qdisc", "del", "dev", "ens5", "root", "handle", faultHandle); err != nil {
				return err
			}
		}
		exists, err = c.qdisc(ctx, action.Host)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("network fault remains after cleanup")
		}
	case "delegation":
		amount, err := c.delegationAmount(ctx, action.Admin, action.Validator)
		if err != nil {
			return err
		}
		if amount.String() != action.Before {
			target, e := stakeTarget(action)
			if e != nil {
				return e
			}
			if amount.Cmp(target) != 0 {
				return errors.New("delegation changed externally; recovery requires operator reconciliation")
			}
			if err = c.stakingTx(ctx, "unbond", action.Validator, action.Added); err != nil {
				return err
			}
			amount, err = c.delegationAmount(ctx, action.Admin, action.Validator)
			if err != nil {
				return err
			}
			if amount.String() != action.Before {
				return errors.New("delegation was not restored")
			}
		}
	default:
		return errors.New("unsupported recovery action")
	}
	return nil
}

func (c *FaultController) restoreActions(ctx context.Context) error {
	// Attempt every restoration even when one host is unreachable. Remove only
	// confirmed restorations, persisting each one before touching the next action.
	var failures []error
	for index := len(c.record.Actions) - 1; index >= 0; index-- {
		if err := c.restoreAction(ctx, c.record.Actions[index]); err != nil {
			failures = append(failures, err)
			continue
		}
		c.record.Actions = append(c.record.Actions[:index], c.record.Actions[index+1:]...)
		if err := c.persist(); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (c *FaultController) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := c.restoreActions(ctx); err != nil {
		return fmt.Errorf("cleanup incomplete: %w; retain %s and run validator_failures.py --recover %s", err, c.Options.Recovery, c.Options.Recovery)
	}
	if err := os.Remove(c.Options.Recovery); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func OpenRecovery(commands Commands, path string) (*FaultController, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("recovery record must be a regular mode-0600 file")
	}
	file, err := openPrivateFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var record RecoveryRecord
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&record); err != nil {
		return nil, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing recovery record data")
	}
	if record.Version != 1 {
		return nil, errors.New("unsupported recovery record version")
	}
	for _, action := range record.Actions {
		if action.Kind != "network" && action.Kind != "delegation" {
			return nil, errors.New("unsupported recovery action")
		}
		if action.Kind == "delegation" {
			if _, e := stakeTarget(action); e != nil {
				return nil, e
			}
		}
	}
	record.Options.Recovery = path
	controller := NewFaultController(commands, record.Options)
	controller.record = record
	return controller, nil
}
