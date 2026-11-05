package bringup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runM795Inspector(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	invalid := errors.New("invalid bringup recovery arguments")
	inventory := func(path string) (PublicInventory, error) {
		f, e := os.Open(path)
		if e != nil {
			return PublicInventory{}, e
		}
		defer f.Close()
		return ReadPublicInventory(f)
	}
	switch args[0] {
	case "archive-name":
		if len(args) != 3 {
			return true, invalid
		}
		i, e := inventory(args[1])
		if e != nil {
			return true, e
		}
		return true, ArchiveName(i, args[2])
	case "archive-reconcile":
		if len(args) != 7 || args[6] != "v1" {
			return true, invalid
		}
		i, e := inventory(args[1])
		if e != nil {
			return true, e
		}
		state, e := (ArchiveState{args[2], args[3], args[4]}).Reconcile(i, args[5], time.Now())
		if e == nil {
			_, e = fmt.Fprintln(output, state)
		}
		return true, e
	case "archive-health":
		if len(args) != 2 {
			return true, invalid
		}
		return true, ProbeArchiveHTTPS(ctx, &http.Client{Timeout: 10 * time.Second}, args[1])
	case "search-selection", "search-render":
		if (args[0] == "search-selection" && len(args) != 4) || (args[0] == "search-render" && len(args) != 5) {
			return true, invalid
		}
		i, e := inventory(args[1])
		if e != nil {
			return true, e
		}
		selection, e := SelectSearchServing(i, input, args[2], args[3])
		if e != nil {
			return true, e
		}
		if args[0] == "search-selection" {
			_, e = fmt.Fprintln(output, selection.Primary, selection.Backup)
			return true, e
		}
		seconds, e := strconv.Atoi(args[4])
		if e != nil || seconds < 1 || seconds > 30 {
			return true, invalid
		}
		return true, RenderSearchFront(ctx, &http.Client{Timeout: time.Duration(seconds) * time.Second}, selection, output)
	case "search-ready":
		if len(args) != 3 {
			return true, invalid
		}
		seconds, e := strconv.Atoi(args[2])
		if e != nil || seconds < 1 || seconds > 30 || !edgeDNS.MatchString(args[1]) {
			return true, invalid
		}
		return true, ProbeSearchReady(ctx, &http.Client{Timeout: time.Duration(seconds) * time.Second}, "https://"+args[1], time.Now())
	case "paxd-init-check":
		if len(args) != 5 {
			return true, invalid
		}
		return true, ValidatePaxdInit(args[1], args[2], args[3], args[4])
	case "paxd-genesis-check":
		if len(args) != 4 {
			return true, invalid
		}
		return true, CheckPaxdGenesis(args[1], args[2], args[3])
	case "paxd-identity":
		if len(args) != 4 || (args[1] != "adopt" && args[1] != "check") {
			return true, invalid
		}
		identity, e := RetainPaxdIdentity(args[2], args[3], args[1] == "adopt")
		if e != nil {
			return true, e
		}
		return true, json.NewEncoder(output).Encode(identity)
	case "resync-journal":
		if len(args) < 3 {
			return true, invalid
		}
		s, e := OpenResyncState(args[1])
		if e != nil {
			return true, e
		}
		defer s.Close()
		switch args[2] {
		case "pending":
			if len(args) != 3 {
				return true, invalid
			}
			r, e := s.Pending()
			if e != nil || r == nil {
				return true, e
			}
			_, e = fmt.Fprintln(output, r.Source, r.Original, r.Phase)
			return true, e
		case "prepare":
			if len(args) != 6 {
				return true, invalid
			}
			return true, s.Prepare(args[3], args[4], args[5])
		case "phase":
			if len(args) != 4 {
				return true, invalid
			}
			return true, s.Phase(args[3])
		case "recovery":
			if len(args) != 4 {
				return true, invalid
			}
			action, e := s.RecoveryAction(args[3])
			if e == nil {
				_, e = fmt.Fprintln(output, action)
			}
			return true, e
		case "restored":
			if len(args) != 4 {
				return true, invalid
			}
			return true, s.Restored(args[3])
		}
		return true, invalid
	case "registry-container-plan", "registry-container-create", "registry-container-clean":
		if (args[0] == "registry-container-plan" && len(args) != 2) || (args[0] == "registry-container-create" && len(args) != 8) || (args[0] == "registry-container-clean" && len(args) != 2 && len(args) != 3) {
			return true, invalid
		}
		s, e := OpenRegistryLifecycle(args[1], LocalDockerPeer())
		if e != nil {
			return true, e
		}
		defer s.Close()
		if args[0] == "registry-container-plan" {
			name, e := s.Plan()
			if e == nil {
				_, e = fmt.Fprintln(output, name)
			}
			return true, e
		}
		if args[0] == "registry-container-clean" {
			name := ""
			if len(args) == 3 {
				name = args[2]
			}
			return true, s.Cleanup(ctx, name)
		}
		b, e := json.Marshal(RegistryCreateConfiguration(args[3], args[4], args[5], args[6]))
		if e != nil {
			return true, e
		}
		if args[7] != "v1" {
			return true, invalid
		}
		id, e := s.Create(ctx, args[2], b)
		if e == nil {
			_, e = fmt.Fprintln(output, id)
		}
		return true, e
	case "mint-token-plan":
		if len(args) != 5 {
			return true, invalid
		}
		rollout, e := os.ReadFile(args[1])
		if e != nil {
			return true, e
		}
		human, e := os.ReadFile(args[2])
		if e != nil {
			return true, e
		}
		module, e := os.ReadFile(args[4])
		if e != nil {
			return true, e
		}
		plan, e := NewSecretPlan(bytes.NewReader(rollout), bytes.NewReader(human), module, "")
		if e != nil {
			return true, e
		}
		return true, plan.Write(args[3], output)
	case "mint-migrate":
		if len(args) != 5 {
			return true, invalid
		}
		return true, MigrateMintSecrets(args[1], args[2], args[3], args[4], output)
	}
	return false, nil
}

func MigrateMintSecrets(rollout, human, oldDir, newDir string, output io.Writer) error {
	oldDir, e := filepath.Abs(oldDir)
	if e != nil {
		return e
	}
	newDir, e = filepath.Abs(newDir)
	if e != nil {
		return e
	}
	if oldDir == newDir || strings.HasPrefix(newDir, oldDir+string(filepath.Separator)) {
		return errors.New("migration requires a separate new directory")
	}
	info, e := os.Stat(oldDir)
	if e != nil {
		return e
	}
	if info.Mode().Perm() != 0700 {
		return errors.New("old mint is not protected")
	}
	readProtected := func(path string) ([]byte, error) {
		info, e := os.Lstat(path)
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return nil, errors.New("retained mint material is not protected and regular")
		}
		return os.ReadFile(path)
	}
	oldGateway, e := readProtected(filepath.Join(oldDir, "router.railway.env"))
	if e != nil {
		return e
	}
	module := ""
	for _, line := range strings.Split(string(oldGateway), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && key == "ENDPOINT_MODULE_REGISTRY" {
			if module != "" {
				return errors.New("duplicate module registry")
			}
			module = value
		}
	}
	decoded, e := base64.StdEncoding.Strict().DecodeString(module)
	if e != nil || !json.Valid(decoded) {
		return errors.New("retained module registry invalid")
	}
	rolloutBytes, e := os.ReadFile(rollout)
	if e != nil {
		return e
	}
	humanBytes, e := os.ReadFile(human)
	if e != nil {
		return e
	}
	plan, e := NewSecretPlan(bytes.NewReader(rolloutBytes), bytes.NewReader(humanBytes), decoded, oldDir)
	if e != nil {
		return e
	}
	// Preserve all non-token wallet identity material, but never copy obsolete
	// target envs. The old directory remains the explicit revocation inventory.
	copies := map[string][]byte{}
	for _, name := range []string{"wallet-gateway.railway.env", "wallet-gateway.railway.sh", "paxeer-attestor-1.box.env", "paxeer-attestor-2.box.env", "paxeer-attestor-3.box.env", "paxeer-attestor-4.box.env", "paxeer-attestor-5.box.env"} {
		b, e := readProtected(filepath.Join(oldDir, name))
		if e != nil {
			return e
		}
		copies[name] = b
	}
	entries, e := os.ReadDir(filepath.Join(oldDir, "keys"))
	if e != nil {
		return e
	}
	for _, entry := range entries {
		b, e := readProtected(filepath.Join(oldDir, "keys", entry.Name()))
		if e != nil {
			return e
		}
		copies[filepath.Join("keys", entry.Name())] = b
	}
	if e = os.Mkdir(newDir, 0700); e != nil {
		return e
	}
	if e = plan.Write(newDir, output); e != nil {
		return e
	}
	for path, b := range copies {
		if e = edgeAtomic(filepath.Join(newDir, path), b, 0600); e != nil {
			return e
		}
		fmt.Fprintln(output, filepath.Join(newDir, path))
	}
	report := []byte("Retired authority, replica, node-component and interop-authority credentials remain only in the preserved old mint. Revoke them at the old consumers after deploying the surviving role bundles. gateway.railway.sh resolves the actual gateway service from rollout; no router-named bundle is emitted. No signing identity was rotated.\n")
	return edgeAtomic(filepath.Join(newDir, "retired-migration.txt"), report, 0600)
}
