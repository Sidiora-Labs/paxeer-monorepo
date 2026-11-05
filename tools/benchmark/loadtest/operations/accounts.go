package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

type Account struct {
	Address  string `json:"address"`
	Mnemonic string `json:"mnemonic"`
}

type GenesisOptions struct {
	Count                int
	Home, Accounts, Paxd string
	Local                bool
}

// privateDirectory pins the directory inode so subsequent opens cannot be
// redirected by replacing its pathname or inserting a symlink.
func privateDirectory(path string) (*os.File, error) {
	if err := os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm() != 0700 {
		file.Close()
		return nil, errors.New("account directory must be owned by the current user with mode 0700")
	}
	return file, nil
}

func SaveAccount(directory, name string, account Account) error {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return errors.New("invalid account filename")
	}
	dir, err := privateDirectory(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return saveAccountAt(dir, name, account)
}

func saveAccountAt(dir *os.File, name string, account Account) (err error) {
	fd, err := syscall.Openat(int(dir.Fd()), name+".json", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	if err = file.Chmod(0600); err != nil {
		return err
	}
	if err = json.NewEncoder(file).Encode(account); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return dir.Sync()
}

func GenerateGenesis(ctx context.Context, runner Commands, options GenesisOptions) error {
	if options.Count < 0 {
		return errors.New("account count must be nonnegative")
	}
	dir, err := privateDirectory(options.Accounts)
	if err != nil {
		return err
	}
	defer dir.Close()
	path := filepath.Join(options.Home, "config", "genesis.json")
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var genesis map[string]any
	if err = decodeJSON(original, &genesis); err != nil {
		return err
	}
	// Bound concurrency to 64 workers and clamp every final range.
	size := (options.Count + 63) / 64
	if size == 0 {
		size = 1
	}
	accounts := make(map[int]Account)
	var mutex sync.Mutex
	var workers sync.WaitGroup
	var failures []error
	for start := 0; start < options.Count; start += size {
		end := min(start+size, options.Count)
		workers.Add(1)
		go func(start, end int) {
			defer workers.Done()
			for index := start; index < end; index++ {
				name := "ta" + strconv.Itoa(index)
				args := []string{options.Paxd, "keys", "add", name, "--home", options.Home, "--output", "json"}
				if options.Local {
					args = append(args, "--keyring-backend", "test")
				}
				raw, err := runner.Run(ctx, Command{Args: args})
				var account Account
				if err == nil {
					err = decodeJSON(raw, &account)
				}
				if err == nil && (account.Address == "" || account.Mnemonic == "") {
					err = errors.New("key output is missing address or mnemonic")
				}
				if err == nil {
					err = saveAccountAt(dir, name, account)
				}
				mutex.Lock()
				if err != nil {
					failures = append(failures, fmt.Errorf("account %d: %w", index, err))
				} else {
					accounts[index] = account
				}
				mutex.Unlock()
			}
		}(start, end)
	}
	workers.Wait()
	if len(failures) != 0 {
		return errors.Join(failures...)
	}
	if len(accounts) != options.Count {
		return fmt.Errorf("created %d accounts, expected exactly %d", len(accounts), options.Count)
	}
	balances := make([]any, 0, options.Count)
	auth := make([]any, 0, options.Count)
	seen := make(map[string]bool)
	for index := 0; index < options.Count; index++ {
		account, ok := accounts[index]
		if !ok || seen[account.Address] {
			return fmt.Errorf("noncontiguous or duplicate account inventory at %d", index)
		}
		seen[account.Address] = true
		balances = append(balances, map[string]any{"address": account.Address, "coins": []any{map[string]string{"denom": "uhpx", "amount": "1000000000000000000000000"}}})
		auth = append(auth, map[string]any{"@type": "/cosmos.auth.v1beta1.BaseAccount", "address": account.Address, "pub_key": nil, "account_number": strconv.Itoa(index), "sequence": "0"})
	}
	app, ok := genesis["app_state"].(map[string]any)
	if !ok {
		return errors.New("missing app_state")
	}
	bank, ok := app["bank"].(map[string]any)
	if !ok {
		return errors.New("missing bank state")
	}
	accountsState, ok := app["auth"].(map[string]any)
	if !ok {
		return errors.New("missing auth state")
	}
	oldBalances, ok := bank["balances"].([]any)
	if !ok {
		return errors.New("invalid bank balances")
	}
	oldAccounts, ok := accountsState["accounts"].([]any)
	if !ok {
		return errors.New("invalid auth accounts")
	}
	for _, old := range oldAccounts {
		if data, ok := old.(map[string]any); ok {
			if address, _ := data["address"].(string); seen[address] {
				return errors.New("generated account already exists in genesis")
			}
		}
	}
	bank["balances"] = append(oldBalances, balances...)
	accountsState["accounts"] = append(oldAccounts, auth...)
	// Do not overwrite an operator's concurrent genesis edit.
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(current) != string(original) {
		return errors.New("genesis changed during account generation")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return writeJSONAtomic(path, genesis, info.Mode().Perm())
}
