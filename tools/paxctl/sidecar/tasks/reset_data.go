package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	tmtypes "github.com/sei-protocol/sei-chain/sei-tendermint/types"
	"github.com/sei-protocol/seictl/internal/patch"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	"github.com/sei-protocol/seilog"
)

var resetDataLog = seilog.NewLogger("seictl", "task", "reset-data")

// privValidatorStateFile is CometBFT's last-sign-state file. It lives inside
// data/. Reset retains its exact bytes and also keeps a durable copy outside
// chain data so interrupted wipes cannot reset a validator's signing history.
const privValidatorStateFile = "priv_validator_state.json"

const preservedSignStateFile = ".pax-sidecar-last-sign-state.json"

// ResetDataResult is the reset-data task's structured result. WipedBytes is the
// pre-wipe on-disk size of data/ (regular files only), surfaced so the
// workflow's hold event can report how much was cleared. It is -1 when the
// measurement failed: the count is observability, never a gate, so a failed
// measurement does not block the reset. Symlinked entries are counted by their
// own (link) size rather than their target, so a data dir that uses symlinks
// may undercount — acceptable for an observability figure.
type ResetDataResult struct {
	WipedBytes int64 `json:"wipedBytes"`
}

// ResetDataer clears the chain data directory for a state-sync re-bootstrap.
// The wipe is scoped to <homeDir>/data/ and nothing else: the home root holds
// config/ (node identity), the sidecar's task ledger (sidecar.db — the database
// that resumes this very wipe after a crash), and the hold sentinel/markers.
// Wiping the home root would destroy the machinery mid-flight; this is the
// design's most important correctness rule.
//
// The reset needs no atomicity of its own. A partially deleted data directory
// is only dangerous if seid starts on it, and the node hold guarantees it does
// not. As defense-in-depth the handler refuses to run while seid's local RPC is
// serving (i.e. the node is not actually held). It is content-idempotent: an
// already-wiped directory is success.
type ResetDataer struct {
	homeDir string
	probeUp func(ctx context.Context) bool
	// measure returns the pre-wipe size of data/. A test seam; defaults to
	// dirSize when nil.
	measure   func(dir string) (int64, error)
	witnesses *StateSyncConfigurer
}

// NewResetDataer builds a ResetDataer rooted at homeDir with the real local-RPC
// serving probe.
func NewResetDataer(homeDir string) *ResetDataer {
	statusClient := rpc.NewStatusClient("", nil)
	return &ResetDataer{
		homeDir: homeDir,
		probeUp: func(ctx context.Context) bool { return seidRPCUp(ctx, statusClient) },
		measure: dirSize,
	}
}

// Handler returns an engine.TaskHandler for the reset-data task type. Params
// carry optional explicit RPC witnesses; the result carries the pre-wipe byte count.
func (d *ResetDataer) Handler() engine.TaskHandler {
	return engine.TypedHandlerWithResult(func(ctx context.Context, p StateSyncRequest) (ResetDataResult, error) {
		return d.resetWithWitnesses(ctx, p)
	})
}

func (d *ResetDataer) reset(ctx context.Context) (ResetDataResult, error) {
	return d.resetWithWitnesses(ctx, StateSyncRequest{})
}

func (d *ResetDataer) resetWithWitnesses(ctx context.Context, p StateSyncRequest) (ResetDataResult, error) {
	if err := d.confirmResetAuthority(ctx); err != nil {
		return ResetDataResult{}, err
	}
	witnesses := d.witnesses
	if witnesses == nil {
		witnesses = NewStateSyncConfigurer(d.homeDir, nil)
	}
	if _, err := witnesses.Preflight(ctx, p); err != nil {
		return ResetDataResult{}, fmt.Errorf("reset-data: witness preflight: %w", err)
	}
	// Defense-in-depth: a serving RPC means seid is running, so the node is not
	// held and a wipe would race a live process. Refuse rather than wipe under
	// it (mirrors restart-seid's refusal to report a stop that did not happen).
	if d.probeUp(ctx) {
		return ResetDataResult{}, fmt.Errorf("reset-data: paxd RPC is serving; node is not held — refusing to wipe a live data directory")
	}

	dataDir := filepath.Join(d.homeDir, "data")
	home, err := openConfinedDir(d.homeDir, false)
	if err != nil {
		return ResetDataResult{}, err
	}
	defer home.Close()
	data, err := openConfinedDir(dataDir, true)
	if err != nil {
		return ResetDataResult{}, err
	}
	defer data.Close()
	state, err := confinedRead(data, privValidatorStateFile, 1<<20)
	if err != nil && !os.IsNotExist(err) {
		return ResetDataResult{}, err
	}
	if len(state) > 0 {
		if !json.Valid(state) {
			return ResetDataResult{}, fmt.Errorf("invalid validator last-sign state")
		}
		if err := confinedWrite(home, preservedSignStateFile, state, 0600); err != nil {
			return ResetDataResult{}, err
		}
	} else {
		saved, err := confinedRead(home, preservedSignStateFile, 1<<20)
		if err != nil && !os.IsNotExist(err) {
			return ResetDataResult{}, err
		}
		if len(saved) > 0 {
			if err := confinedWrite(data, privValidatorStateFile, saved, 0600); err != nil {
				return ResetDataResult{}, err
			}
		}
	}

	// Measurement is observability only — never let it gate the wipe. On any
	// non-ENOENT failure, log and proceed with an unknown (-1) size.
	measure := d.measure
	if measure == nil {
		measure = dirSize
	}
	size, err := measure(dataDir)
	if err != nil {
		resetDataLog.Warn("measuring data dir failed; proceeding with unknown size", "dir", dataDir, "err", err)
		size = -1
	}

	if err := wipeDirContents(dataDir); err != nil {
		return ResetDataResult{}, fmt.Errorf("reset-data: wiping %s: %w", dataDir, err)
	}

	// Clear the state-sync completion marker (home root, outside data/) so the
	// downstream configure-state-sync reruns instead of short-circuiting.
	markerPath := filepath.Join(d.homeDir, stateSyncMarkerFile)
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		return ResetDataResult{}, fmt.Errorf("reset-data: removing marker %s: %w", markerPath, err)
	}

	resetDataLog.Info("data directory reset", "dir", dataDir, "wipedBytes", size)
	return ResetDataResult{WipedBytes: size}, nil
}

// wipeDirContents removes every entry under dir, leaving dir itself. A missing
// dir is success (content-idempotent: already-empty is the goal state), and a
// concurrent peer removing an entry first (ENOENT) is tolerated so a rehydrated
// re-run cannot fail on a half-wiped tree.
func wipeDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.Name() == privValidatorStateFile {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", p, err)
		}
	}
	return nil
}

// dirSize sums the on-disk size of regular files under dir. A missing dir is
// zero. Errors from entries that vanish mid-walk (ENOENT) are ignored — the
// count is observability, not a correctness signal.
func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		total += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}

func (d *ResetDataer) confirmResetAuthority(ctx context.Context) error {
	if d.probeUp == nil || d.probeUp(ctx) {
		return fmt.Errorf("reset-data: node RPC is serving or unobservable")
	}
	configDir, err := openConfinedDir(filepath.Join(d.homeDir, "config"), false)
	if err != nil {
		return err
	}
	defer configDir.Close()
	raw, err := confinedRead(configDir, "config.toml", 8<<20)
	if err != nil {
		return err
	}
	cfg, err := patch.UnmarshalTOML(raw)
	if err != nil {
		return err
	}
	if cfg["mode"] != "full" {
		return fmt.Errorf("reset-data: verified full-node role required")
	}
	if addr, _ := cfg["priv-validator-laddr"].(string); addr != "" {
		return fmt.Errorf("reset-data: remote validator signer configured")
	}
	if pv, ok := cfg["priv-validator"].(map[string]any); ok {
		if addr, _ := pv["laddr"].(string); addr != "" {
			return fmt.Errorf("reset-data: remote validator signer configured")
		}
		for name, expected := range map[string]string{"key-file": "config/priv_validator_key.json", "state-file": "data/priv_validator_state.json"} {
			if path, _ := pv[name].(string); path != "" && filepath.Clean(path) != expected && filepath.Clean(path) != filepath.Join(d.homeDir, expected) {
				return fmt.Errorf("reset-data: noncanonical validator %s requires explicit preservation", name)
			}
		}
	}

	doc, err := tmtypes.GenesisDocFromFile(filepath.Join(d.homeDir, "config", "genesis.json"))
	if err != nil {
		return fmt.Errorf("reset-data: verify genesis validators: %w", err)
	}
	key, err := confinedRead(configDir, "priv_validator_key.json", 1<<20)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(key) > 0 {
		var identity struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(key, &identity); err != nil || identity.Address == "" {
			return fmt.Errorf("reset-data: invalid validator identity")
		}
		for _, val := range doc.Validators {
			if strings.EqualFold(identity.Address, val.Address.String()) {
				return fmt.Errorf("reset-data: active validator identity cannot reset chain data")
			}
		}
	}
	if _, err := os.Lstat(filepath.Join(d.homeDir, "sidecar.db")); err != nil {
		return fmt.Errorf("reset-data: persisted workflow hold required: %w", err)
	}
	store, err := engine.NewSQLiteStore(filepath.Join(d.homeDir, "sidecar.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	hold, err := store.LatestByType(string(engine.TaskMarkNotReady))
	if err != nil {
		return err
	}
	if hold == nil || hold.Status != engine.TaskStatusCompleted {
		return fmt.Errorf("reset-data: completed durable hold required")
	}
	release, err := store.LatestByType(string(engine.TaskMarkReady))
	if err != nil {
		return err
	}
	if release != nil && !release.SubmittedAt.Before(hold.SubmittedAt) {
		return fmt.Errorf("reset-data: hold has been superseded by release")
	}
	stop, err := store.LatestByType(string(engine.TaskStopSeid))
	if err != nil {
		return err
	}
	if stop == nil || stop.Status != engine.TaskStatusCompleted || stop.SubmittedAt.Before(hold.SubmittedAt) {
		return fmt.Errorf("reset-data: confirmed process termination under hold required")
	}
	if pid, err := (seidStartFinder{}).FindPID(restartSeidProcess); !errors.Is(err, errNodeProcessAbsent) {
		return fmt.Errorf("reset-data: process may still be running (pid %d): %v", pid, err)
	}
	return nil
}
