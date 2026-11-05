package sequencer_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func TestNodeCompletionRetiredSurfacesAbsent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate repository root: %v", err)
	}

	retiredSurfaces := []struct {
		id     string
		path   string
		kind   string
		symbol string
	}{
		{"interface.15.mk-consensus-build-reproducible-j57", "consensus/Makefile", "make", "build-reproducible"},
		{"interface.15.mk-consensus-dist-j57", "consensus/Makefile", "make", "dist"},
		{"interface.15.mk-consensus-draw-deps-j57", "consensus/Makefile", "make", "draw_deps"},
		{"interface.15.mk-consensus-get-deps-bin-size-j57", "consensus/Makefile", "make", "get_deps_bin_size"},
		{"interface.15.mk-consensus-go-mod-cache-j57", "consensus/Makefile", "make", "go-mod-cache"},
		{"interface.15.mk-consensus-go-sum-j57", "consensus/Makefile", "make", "go.sum"},
		{"interface.5.cli-custody-proof-light-credit", "daemon/layerx-custody-proof/main.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-asset", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-current-header-hash", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-deposit-id", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-first-authorized-batch", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-height", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-kernel-socket", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-last-authorized-batch", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-output", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-owner-key", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-profile", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-registry", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-rpc", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-sequencer-id", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-sequencer-key", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-trust-snapshot", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-credit-flag-trusted-validators-height", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile", "daemon/layerx-custody-proof/main.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile-flag-asset", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile-flag-network-id", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile-flag-output", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile-flag-rpc", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile-flag-trusted-height", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-profile-flag-trusting-period-seconds", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry", "daemon/layerx-custody-proof/main.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-evm-rpc", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-network-id", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-output", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-registry", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-rpc", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-trusted-height", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-custody-proof-light-registry-flag-trusting-period-seconds", "daemon/layerx-custody-proof/light.go", "file", ""},
		{"interface.5.cli-layerx-archive-codec", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.cli-layerx-archive-codec-activity", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.cli-layerx-archive-codec-export", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.cli-layerx-archive-codec-genesis", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.cli-layerx-archive-codec-submit", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.cli-layerx-archive-codec-verify", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.cli-layerx-guarantor", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.cli-layerx-guarantor-fetch-only", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.cli-layerx-guarantor-once", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.cli-layerx-handover", "cmd/layerx-handover/main.c", "file", ""},
		{"interface.5.cli-layerx-handover-issue", "cmd/layerx-handover/main.c", "file", ""},
		{"interface.5.cli-layerx-handover-verify-key", "cmd/layerx-handover/main.c", "file", ""},
		{"interface.5.cli-layerx-verify", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-bundle", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-proof", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-receipt", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-receipt-2", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-trust-root", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-trust-root-2", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerx-verify-value", "cmd/layerx-verify/lxp_verify_cli.c", "file", ""},
		{"interface.5.cli-layerxd", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-layerxd-authority-replica", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-layerxd-check-config", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-layerxd-config", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-layerxd-relay-archive", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-layerxd-replica", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-layerxd-serve", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.cli-main-daemon-layerx-custody-proof", "daemon/layerx-custody-proof/main.go", "file", ""},
		{"interface.5.env-layerx-arbiter-admission-prestate", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-arbiter-prestate", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-asset-execution-prestate", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-authority-address", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-availability-log", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-bearer-token", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-first-batch", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-genesis-manifest", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-last-batch", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-port", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-ready-fd", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-replica-id", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-replica-log", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-sequencer-id", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-sequencer-public-key", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-authority-status-genesis-manifest", "cmd/layerxd/lxp_daemon_authority_replica.c", "file", ""},
		{"interface.5.env-layerx-execution-prestate", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-guarantor-anchor-abi", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-guarantor-checkpoint-pause-at", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-checkpoint-timeout-ms", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-id", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-key-file", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-listen-port", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-lni-socket", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-node-config", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-peer-url", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-publication-authorization-file", "cmd/layerx-guarantor/publication.py", "file", ""},
		{"interface.5.env-layerx-guarantor-publication-inputs-dir", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-python", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-settlement-domain", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-settlement-file", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-settlement-helper", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-state-dir", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-submitter-key-file", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-submitter-lock-file", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-guarantor-timing", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-guarantor-tls-ca-file", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-tls-cert-file", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-guarantor-tls-key-file", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-node-anchor-precompile", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-node-authority-replica-address", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-authority-replica-bearer-token", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-authority-replica-id", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-authority-replica-port", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-batch-log", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-canonical-log", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-checkpoint-directory", "cmd/layerxd/lni.env.example", "file", ""},
		{"interface.5.env-layerx-node-checkpoint-registry", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-node-custody-precompile", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-node-da-retain-batches", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-evidence-log", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-first-batch", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-node-genesis-manifest", "cmd/layerx-guarantor/runtime.c", "file", ""},
		{"interface.5.env-layerx-node-genesis-registration", "cmd/layerx-guarantor/runtime.c", "file", ""},
		{"interface.5.env-layerx-node-history-database", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-history-migrations", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-identities", "cmd/layerx-guarantor/runtime.c", "file", ""},
		{"interface.5.env-layerx-node-last-batch", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-node-lni-allowed-gid", "cmd/layerxd/lni.env.example", "file", ""},
		{"interface.5.env-layerx-node-lni-allowed-uid", "cmd/layerxd/lni.env.example", "file", ""},
		{"interface.5.env-layerx-node-lni-deadline-ms", "cmd/layerxd/lni.env.example", "file", ""},
		{"interface.5.env-layerx-node-lni-frame-bytes", "cmd/layerxd/lni.env.example", "file", ""},
		{"interface.5.env-layerx-node-lni-socket", "cmd/layerxd/lni.env.example", "file", ""},
		{"interface.5.env-layerx-node-network-id", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-node-paxeer-chain-id", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-node-paxeer-rpc-address", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-node-paxeer-rpc-port", "cmd/layerx-guarantor/settlement.c", "file", ""},
		{"interface.5.env-layerx-node-paxeer-rpc-url", "cmd/layerxd/lxp_daemon_finality_authority.c", "file", ""},
		{"interface.5.env-layerx-node-program-address", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-program-bearer-token", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-program-feed-log", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-program-port", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-receipt-authority-log", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-node-registry-precompile", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-node-sequencer-id", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-node-sequencer-private-key", "cmd/layerxd/lxp_daemon_lni.c", "file", ""},
		{"interface.5.env-layerx-node-sequencer-public-key", "cmd/layerx-guarantor/main.c", "file", ""},
		{"interface.5.env-layerx-node-settlement-contract", "cmd/layerx-guarantor/settlement.py", "file", ""},
		{"interface.5.env-layerx-node-snapshot", "cmd/layerx-guarantor/runtime.c", "file", ""},
		{"interface.5.env-layerx-pay-timing", "cmd/layerxd/lxp_daemon_batch_wal.c", "file", ""},
		{"interface.5.env-layerx-relay-archive-runtime", "cmd/layerxd/lxp_daemon_cli.c", "file", ""},
		{"interface.5.env-layerx-replay-catalogue", "cmd/layerxd/lxp_daemon_process.c", "file", ""},
		{"interface.5.env-layerx-replica-ack-threshold", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-availability-log", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-batch-limit", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-first-batch", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-id", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-log", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-poll-ms", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-set", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.env-layerx-replica-state-directory", "cmd/layerxd/lxp_daemon_replica.c", "file", ""},
		{"interface.5.fmt-layerx-archive", "cmd/layerx-archive-codec/main.c", "file", ""},
		{"interface.5.fmt-layerxctl-lxct-output", "cmd/layerxctl/lxp_ctl_main.c", "file", ""},
	}
	if len(retiredSurfaces) != 159 {
		t.Fatalf("retired interface inventory has %d entries, want 159", len(retiredSurfaces))
	}
	seen := make(map[string]bool, len(retiredSurfaces))
	for _, retired := range retiredSurfaces {
		if seen[retired.id] {
			t.Fatalf("duplicate retired interface %s", retired.id)
		}
		seen[retired.id] = true
		t.Run("retired/"+retired.id, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(retired.path))
			if retired.kind == "file" {
				_, err := os.Lstat(path)
				if err == nil {
					t.Fatalf("retired interface %s remains at %s", retired.id, retired.path)
				}
				if !os.IsNotExist(err) {
					t.Fatalf("check retired interface %s at %s: %v", retired.id, retired.path, err)
				}
				return
			}
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read retired interface %s source %s: %v", retired.id, retired.path, err)
			}
			switch retired.kind {
			case "go":
				file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors)
				if err != nil {
					t.Fatalf("parse retired interface %s source %s: %v", retired.id, retired.path, err)
				}
				ast.Inspect(file, func(n ast.Node) bool {
					if ident, ok := n.(*ast.Ident); ok && ident.Name == retired.symbol {
						t.Errorf("retired interface %s symbol %s remains at %s", retired.id, retired.symbol, retired.path)
					}
					return true
				})
			case "make":
				sourceText := strings.ReplaceAll(string(source), "\\\n", " ")
				for number, line := range strings.Split(sourceText, "\n") {
					if strings.HasPrefix(line, "\t") {
						continue
					}
					line, _, _ = strings.Cut(line, "#")
					targets, prerequisites, ok := strings.Cut(line, ":")
					if !ok || strings.Contains(targets, "=") || strings.HasPrefix(prerequisites, "=") {
						continue
					}
					names := strings.Fields(targets)
					if strings.TrimSpace(targets) == ".PHONY" {
						names = strings.Fields(prerequisites)
					}
					for _, name := range names {
						if name == retired.symbol {
							t.Errorf("retired interface %s target %s remains at %s:%d", retired.id, retired.symbol, retired.path, number+1)
						}
					}
				}
			default:
				t.Fatalf("unknown retired interface kind %q for %s", retired.kind, retired.id)
			}
		})
	}

	plannedPaths := []struct {
		task string
		path string
	}{
		{"5.7.1", "node/store_delete_test.go"},
		{"5.42.3", "storage/state_db/sc/memiavl/anchor_delete_test.go"},
		{"5.50.1", "cmd/retirement/test_layerxd_lni.py"},
		{"5.50.2", "cmd/retirement/test_layerxd_batch_wal.py"},
		{"5.50.3", "cmd/retirement/test_layerxd_replicas.py"},
		{"5.50.4", "cmd/retirement/test_layerxd_finality.py"},
		{"5.50.5", "cmd/retirement/test_layerxd_evidence.py"},
		{"5.50.6", "cmd/retirement/test_layerxd_artifacts.py"},
		{"5.50.7", "cmd/retirement/test_layerxd_core.py"},
		{"5.51.1", "cmd/retirement/test_guarantor_core.py"},
		{"5.51.2", "cmd/retirement/test_guarantor_exchange.py"},
		{"5.51.3", "cmd/retirement/test_guarantor_settlement.py"},
		{"5.51.4", "cmd/retirement/test_guarantor_python.py"},
		{"5.52.1", "cmd/retirement/test_layerx_handover.py"},
		{"5.52.2", "cmd/retirement/test_layerx_archive_codec.py"},
		{"5.52.3", "cmd/retirement/test_layerx_verify.py"},
		{"5.52.4", "cmd/retirement/test_layerxctl_c_tool.py"},
		{"5.53.1", "cmd/retirement/test_custody_proof_daemon.py"},
		{"5.54.1", "cmd/retirement/test_daemon_tests_guarantor_c.py"},
		{"5.54.2", "cmd/retirement/test_daemon_tests_guarantor_python.py"},
		{"5.54.3", "cmd/retirement/test_daemon_tests_handover_finality.py"},
		{"5.54.7", "cmd/retirement/test_daemon_tests_maintenance.py"},
		{"5.54.8", "cmd/retirement/test_daemon_tests_program_admission.py"},
		{"5.54.9", "cmd/retirement/test_daemon_tests_custody.py"},
		{"5.54.10", "cmd/retirement/test_daemon_tests_runtime_fixture.py"},
		{"5.54.11", "cmd/retirement/test_daemon_tests_clients.py"},
		{"5.55.5", "daemon/committed-fixture/vector_v4.go"},
		{"5.55.5", "daemon/committed-fixture/vectors/vector-v4-committed.json"},
		{"5.55.5", "daemon/committed-fixture/vector_v4_test.go"},
		{"5.62.6", "consensus/test/maketargets/consensus_makefile_test.go"},
		{"5.64.16", "consensus/internal/rpc/core/blocks_autobahn_search_test.go"},
		{"5.69.1", "consensus/internal/rpc/core/autobahn_routes_conformance_test.go"},
		{"5.97.1", "cmd/retirement/test_kernel_symbol_consumers_retired.py"},
		{"5.97.3", "cmd/retirement/test_movement_proof_retired.py"},
	}
	if len(plannedPaths) != 34 {
		t.Fatalf("planned path inventory has %d entries, want 34", len(plannedPaths))
	}
	seenPaths := make(map[string]bool, len(plannedPaths))
	for _, planned := range plannedPaths {
		if seenPaths[planned.path] {
			t.Fatalf("duplicate planned path %s", planned.path)
		}
		seenPaths[planned.path] = true
		t.Run("planned/"+planned.path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(planned.path)))
			if err != nil {
				t.Fatalf("task %s required planned path %s: %v", planned.task, planned.path, err)
			}
			if !info.Mode().IsRegular() {
				t.Errorf("task %s required planned path %s is not a regular file", planned.task, planned.path)
			}
		})
	}
}

func TestVectorV4CommittedVerifies(t *testing.T) {
	data, err := os.ReadFile("../../daemon/committed-fixture/vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Authorization verify.SequencerAuthorization `json:"authorization"`
		Positive      struct {
			Receipt         hexutil.Bytes     `json:"receipt"`
			Header          hexutil.Bytes     `json:"header"`
			HeaderSignature hexutil.Bytes     `json:"header_signature"`
			Proof           codec.MerkleProof `json:"proof"`
		} `json:"positive"`
		Settlement struct {
			RawTransaction   hexutil.Bytes                  `json:"raw_transaction"`
			Receipt          *evmrpc.SequencerReceiptResult `json:"receipt"`
			Batch            seqtypes.MsgSequenceBatch      `json:"sequence_batch"`
			BatchProof       tmtypes.TxProof                `json:"sequence_batch_proof"`
			TransactionProof tmtypes.TxProof                `json:"transaction_proof"`
			ExecutionHeader  *tmtypes.Header                `json:"execution_header"`
			ExecutionCommit  *tmtypes.Commit                `json:"execution_commit"`
			Validators       []*tmtypes.Validator           `json:"validators"`
		} `json:"settlement"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	s := vector.Settlement
	if s.Receipt == nil || s.ExecutionHeader == nil || s.ExecutionCommit == nil || len(s.Validators) == 0 {
		t.Fatal("incomplete committed recording")
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(s.RawTransaction); err != nil {
		t.Fatal(err)
	}
	if tx.Type() != ethtypes.DynamicFeeTxType || tx.ChainId().Cmp(big.NewInt(125)) != 0 || tx.Hash() != s.Receipt.TxHash {
		t.Fatal("committed recording must bind a type-2 chain-125 transaction")
	}
	keyID := sha256.Sum256(vector.Authorization.PublicKey[:])
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: vector.Authorization.PublicKey[:], FirstHeight: vector.Authorization.FirstBatchNumber, LastHeight: vector.Authorization.LastBatchNumber}}
	if err := s.Receipt.Verify(pins); err != nil {
		t.Fatal(err)
	}
	if s.Receipt.Rung != evmrpc.RungCommitted || s.Receipt.BlockNumber == nil || s.Receipt.BlockHash == nil || int64(*s.Receipt.BlockNumber) != s.ExecutionHeader.Height || !bytes.Equal(s.Receipt.BlockHash.Bytes(), s.ExecutionHeader.Hash()) {
		t.Fatal("committed RPC coordinates differ from execution header")
	}
	if !bytes.Equal(vector.Positive.Receipt, s.Receipt.Receipt) {
		t.Fatal("inclusion envelope differs from RPC receipt")
	}
	header, err := codec.DecodeBatchHeader(vector.Positive.Header)
	if err != nil {
		t.Fatal(err)
	}
	if header.NetworkID != 125 {
		t.Fatalf("inclusion network = %d, want 125", header.NetworkID)
	}
	if _, err := verify.ReceiptAtRoot(vector.Positive.Receipt, &vector.Positive.Proof, header.ReceiptMerkleRoot, vector.Authorization.PublicKey); err != nil {
		t.Fatal(err)
	}
	if len(vector.Positive.HeaderSignature) != 64 {
		t.Fatal("invalid header signature length")
	}
	if err := verify.Ed25519Digest(vector.Authorization.PublicKey, [64]byte(vector.Positive.HeaderSignature), codec.BatchHeaderDigest(vector.Positive.Header)); err != nil {
		t.Fatal(err)
	}
	if err := s.BatchProof.Validate(s.ExecutionHeader.DataHash); err != nil {
		t.Fatal(err)
	}
	if err := s.TransactionProof.Validate(s.ExecutionHeader.DataHash); err != nil {
		t.Fatal(err)
	}
	batch, err := seqtypes.DecodeSequenceBatchTx(s.BatchProof.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.VerifySignature("committed-fixture", vector.Authorization.PublicKey); err != nil {
		t.Fatal(err)
	}
	if batch.Height != uint64(s.ExecutionHeader.Height) || batch.FromSeq > uint64(s.Receipt.Seq) || batch.ToSeq < uint64(s.Receipt.Seq) {
		t.Fatal("committed sequence batch does not cover receipt")
	}
	set := tmtypes.NewValidatorSet(s.Validators)
	if !bytes.Equal(set.Hash(), s.ExecutionHeader.ValidatorsHash) {
		t.Fatal("validator set differs from execution header")
	}
	if err := set.VerifyCommit("committed-fixture", s.ExecutionCommit.BlockID, s.ExecutionHeader.Height, s.ExecutionCommit); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s.ExecutionCommit.BlockID.Hash, s.ExecutionHeader.Hash()) {
		t.Fatal("commit does not authenticate execution header")
	}
}
