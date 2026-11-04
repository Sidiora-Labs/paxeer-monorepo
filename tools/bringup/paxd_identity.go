package bringup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type PaxdIdentity struct {
	Version         int
	ChainID, NodeID string
	Files           map[string]string
}

func CheckPaxdGenesis(path, chainID, digest string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	hash := sha256.Sum256(b)
	if len(digest) != 64 || hex.EncodeToString(hash[:]) != digest {
		return errors.New("approved paxd genesis digest differs")
	}
	doc, e := tmtypes.GenesisDocFromJSON(b)
	if e != nil {
		return errors.New("invalid native paxd genesis")
	}
	if doc.ChainID != chainID || len(doc.AppState) == 0 {
		return errors.New("paxd genesis chain identity differs")
	}
	var modules map[string]json.RawMessage
	if json.Unmarshal(doc.AppState, &modules) != nil || len(modules) == 0 {
		return errors.New("paxd genesis app state missing")
	}
	return nil
}

// RetainPaxdIdentity records the native initialization outputs once, then only
// compares them. It never replaces a key, genesis, receipt key or signing state.
func RetainPaxdIdentity(home, chainID string, adopt bool) (PaxdIdentity, error) {
	var result PaxdIdentity
	home, e := filepath.Abs(home)
	if e != nil {
		return result, e
	}
	guard, e := OpenEdgeState(home)
	if e != nil {
		return result, e
	}
	e = guard.withLock(func() error {
		var cfg struct {
			NodeKey       string `toml:"node-key-file"`
			Genesis       string `toml:"genesis-file"`
			PrivValidator struct {
				Key   string `toml:"key-file"`
				State string `toml:"state-file"`
			} `toml:"priv-validator"`
			Sequencer struct {
				Enabled bool   `toml:"enable"`
				Receipt string `toml:"receipt-key-file"`
			} `toml:"sequencer"`
		}
		if _, e := toml.DecodeFile(filepath.Join(home, "config/config.toml"), &cfg); e != nil {
			return e
		}
		node, genesis := cfg.NodeKey, cfg.Genesis
		if node == "" {
			node = "config/node_key.json"
		}
		if genesis == "" {
			genesis = "config/genesis.json"
		}
		resolve := func(path string) string {
			if filepath.IsAbs(path) {
				return path
			}
			return filepath.Join(home, path)
		}
		nodePath, genesisPath := resolve(node), resolve(genesis)
		nk, e := tmtypes.LoadNodeKey(nodePath)
		if e != nil {
			return errors.New("retained native node identity unreadable")
		}
		b, e := os.ReadFile(genesisPath)
		if e != nil {
			return e
		}
		hash := sha256.Sum256(b)
		if e = CheckPaxdGenesis(genesisPath, chainID, hex.EncodeToString(hash[:])); e != nil {
			return e
		}
		paths := []string{nodePath, genesisPath}
		if cfg.PrivValidator.Key != "" {
			keyPath, statePath := resolve(cfg.PrivValidator.Key), resolve(cfg.PrivValidator.State)
			if cfg.PrivValidator.State == "" {
				return errors.New("native validator signing state path absent")
			}
			if _, e := privval.LoadFilePV(keyPath, statePath); e != nil {
				return errors.New("retained validator identity or signing state invalid")
			}
			paths = append(paths, keyPath)
		}
		// paxd stores sequencer settings in app.toml, independently of Comet config.
		if _, e := toml.DecodeFile(filepath.Join(home, "config/app.toml"), &cfg); e != nil {
			return e
		}
		if cfg.Sequencer.Enabled {
			if cfg.Sequencer.Receipt == "" {
				return errors.New("enabled sequencer receipt key absent")
			}
			paths = append(paths, resolve(cfg.Sequencer.Receipt))
		}
		result = PaxdIdentity{1, chainID, string(nk.ID()), map[string]string{}}
		for _, path := range paths {
			info, e := os.Lstat(path)
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return errors.New("native identity path is not a regular file")
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if len(b) == 0 {
				return errors.New("native identity file is empty")
			}
			sum := sha256.Sum256(b)
			result.Files[path] = hex.EncodeToString(sum[:])
		}
		record := filepath.Join(home, "identity", "paxd.json")
		old, e := os.ReadFile(record)
		if e == nil {
			var retained PaxdIdentity
			if json.Unmarshal(old, &retained) != nil || !reflect.DeepEqual(result, retained) {
				return errors.New("retained paxd identity changed; restore its approved genesis and keys")
			}
			return nil
		}
		if !os.IsNotExist(e) {
			return e
		}
		if !adopt {
			return errors.New("native identity is not yet adopted")
		}
		encoded, e := json.Marshal(result)
		if e != nil {
			return e
		}
		return edgeAtomic(record, append(encoded, '\n'), 0600)
	})
	guard.Close()
	return result, e
}

func ValidatePaxdInit(home, moniker, chainID, mode string) error {
	if !filepath.IsAbs(home) || strings.TrimSpace(moniker) == "" || strings.ContainsAny(chainID, " \n\r\t") || chainID == "" {
		return errors.New("explicit native home, moniker and chain ID required")
	}
	if mode != "full" && mode != "archive" && mode != "validator" && mode != "seed" {
		return errors.New("invalid paxd mode")
	}
	entries, e := os.ReadDir(home)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if len(entries) > 0 {
		return errors.New("paxd init requires a new empty home; retain an existing identity instead")
	}
	return nil
}
