package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	anchormodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	anchor "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_811_SourceHome(t *testing.T) {
	for _, suppliedHome := range []bool{false, true} {
		t.Run(map[bool]string{false: "data-directory", true: "node-home"}[suppliedHome], func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "original")
			data := filepath.Join(source, "data")
			if err := os.MkdirAll(data, 0o750); err != nil {
				t.Fatal(err)
			}
			// The copied state is a database written by the real backend.
			db, err := dbm.NewDB("committer", dbm.GoLevelDBBackend, data)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SetSync([]byte("original-state"), []byte("committed")); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{
				"config/genesis.json":  []byte(`{"chain_id":"hyperpax_125-1","app_state":{}}`),
				"config/app.toml":      []byte("[state-commit]\nenable = true\n"),
				"wasm/wasm/empty.wasm": {0, 97, 115, 109, 1, 0, 0, 0},
			}
			for name, value := range files {
				path := filepath.Join(source, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, value, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			input := data
			if suppliedHome {
				input = source
			}
			staged := filepath.Join(root, "staged")
			if err := run("stage", input, "", "", "", staged, 0, false, 0, ""); err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(root, "replay-home")
			if err := prepareHome(home, filepath.Join(staged, "data")); err != nil {
				t.Fatal(err)
			}
			for name, want := range files {
				path := filepath.Join(home, name)
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("staged sibling %s: %q, %v", name, got, err)
				}
				if err := os.WriteFile(path, []byte("replay changed its copy"), 0o600); err != nil {
					t.Fatal(err)
				}
				original, err := os.ReadFile(filepath.Join(source, name))
				if err != nil || !bytes.Equal(original, want) {
					t.Fatalf("original sibling changed: %s, %v", name, err)
				}
			}
			copyDB, err := dbm.NewDB("committer", dbm.GoLevelDBBackend, filepath.Join(home, "data"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := copyDB.Get([]byte("original-state"))
			if err != nil || string(got) != "committed" {
				t.Fatalf("copied database did not recover committed state: %q, %v", got, err)
			}
			if err := copyDB.SetSync([]byte("original-state"), []byte("replayed")); err != nil {
				t.Fatal(err)
			}
			if err := copyDB.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = dbm.NewDB("committer", dbm.GoLevelDBBackend, data)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			got, err = db.Get([]byte("original-state"))
			if err != nil || string(got) != "committed" {
				t.Fatalf("original database was changed: %q, %v", got, err)
			}
			if err := stageSourceHome(input, staged); err == nil {
				t.Fatal("staging must refuse an existing destination")
			}
			if err := stageSourceHome(input, filepath.Join(data, "nested-copy")); err == nil {
				t.Fatal("staging must refuse a destination inside the source")
			}
		})
	}
	// The shell delegates staging to the same handler exercised above and
	// passes its coherent data directory to replay/prepareHome.
	script, err := os.ReadFile("run.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, wiring := range []string{`"$binary" -mode stage -data "$data" -out "$sourcehome"`, `state="$sourcehome/data"`, `args=(-mode replay -data "$state" -home "$home")`} {
		if !strings.Contains(string(script), wiring) {
			t.Fatalf("run.sh does not wire the staged source home: %s", wiring)
		}
	}
}

func TestRepairM_811_PrecompileCallsSurviveReopen(t *testing.T) {
	home := t.TempDir()
	if _, err := generateFixture(home, defaultChainID, 1, true); err != nil {
		t.Fatal(err)
	}
	opts := newAppOptions(defaultChainID, false)
	// The current module manager no longer initialises the historical anchor
	// module. Supply its real committed genesis as a retained legacy snapshot;
	// the replay's existing genesis and version-map assertions remain intact.
	legacy := openApp(home, opts)
	legacyCtx, writeLegacy := blockContext(legacy, legacy.LastBlockHeight()+1, defaultChainID, time.Unix(1_800_000_000, 0).UTC()).CacheContext()
	legacy.LayerXAnchorKeeper.InitGenesis(legacyCtx, *anchortypes.DefaultGenesis())
	versions := legacy.UpgradeKeeper.GetModuleVersionMap(legacyCtx)
	versions[anchortypes.ModuleName] = (anchormodule.AppModule{}).ConsensusVersion()
	legacy.UpgradeKeeper.SetModuleVersionMap(legacyCtx, versions)
	writeLegacy()
	legacy.CommitMultiStore().Commit(true)
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	pre, err := probeState(home, opts, allStoreKeys())
	if err != nil {
		t.Fatal(err)
	}
	// Drive the production replay, including its real upgrade handler, commit,
	// close and reopen, before adding the later merge-height expectations.
	if failures := replay(home, opts, app.ActivationUpgrade, defaultChainID, pre); failures != 0 {
		t.Fatalf("historical activation replay: %d assertions failed", failures)
	}
	a := openApp(home, opts)
	opened := true
	defer func() {
		if opened {
			_ = a.Close()
		}
	}()
	height := a.LastBlockHeight() + 1
	when := time.Unix(1_800_000_000, 0).UTC()
	ctx, write := blockContext(a, height, defaultChainID, when).CacheContext()
	c := &checks{}
	// Read historical behavior from the real pre-merge state. These contracts
	// carry only the current binary's version, so rewinding the height after
	// recording that version's merge marker does not reconstruct older code.
	historical := assertPrecompilesServed(c, a, ctx, height-1)
	if c.failures != 0 {
		t.Fatalf("historical calls before merge: %d failures", c.failures)
	}
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	before := assertPrecompiles(c, a, ctx, height)
	if c.failures != 0 {
		t.Fatalf("merged calls before commit: %d failures", c.failures)
	}
	for _, name := range []string{"layerxledger.accountId", "layerxpay.progressHighWater", "layerxverify.verifyReceipt", "layerxcustody.deposit", "layerxanchor.submitCheckpoint", "layerxanchor.threshold"} {
		if len(before[name]) == 0 {
			t.Fatalf("real call %s was not executed", name)
		}
	}
	// Every verify/custody method and every anchor write must be called,
	// rather than accepting one representative refusal for an entire ABI.
	for _, entry := range forkPrecompiles {
		if entry.Name != "layerxverify" && entry.Name != "layerxcustody" && entry.Name != "layerxanchor" {
			continue
		}
		p := a.EvmKeeper.CustomPrecompiles(ctx)[common.HexToAddress(entry.Address)].(precompiles.IPrecompile)
		for name, method := range p.GetABI().Methods {
			if entry.Name == "layerxanchor" && method.IsConstant() {
				continue
			}
			output, ok := before[entry.Name+"."+name]
			reason, err := abi.UnpackRevert(output)
			if !ok || err != nil || reason != "LXP_MODULE_DISABLED: module disabled; "+p.GetName()+" retired" {
				t.Fatalf("missing exact refusal for %s.%s: %q, %v", entry.Name, name, reason, err)
			}
		}
	}
	if c.failures != 0 || bytes.Equal(historical["layerxverify.verifyEd25519"], before["layerxverify.verifyEd25519"]) {
		t.Fatal("historical raw verification must still succeed below merge height")
	}
	belowMerge := a.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height - 1))
	if _, ok := belowMerge[common.HexToAddress("0x101B")]; ok {
		t.Fatal("pay must not execute below its own merge activation")
	}
	if _, ok := belowMerge[common.HexToAddress("0x101A")]; ok {
		t.Fatal("ledger must not execute below its own merge activation")
	}
	write()
	if committed := a.CommitMultiStore().Commit(true).Version; committed != height {
		t.Fatalf("commit height %d, want %d", committed, height)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	opened = false
	b := openApp(home, opts)
	defer b.Close()
	if b.LastBlockHeight() != height {
		t.Fatalf("reopened height %d, want %d", b.LastBlockHeight(), height)
	}
	reopened := blockContext(b, height, defaultChainID, when)
	if got := b.UpgradeKeeper.GetDoneHeight(reopened, app.MergeUpgrade); got != height {
		t.Fatalf("recovered merge activation height %d, want %d", got, height)
	}
	after := assertPrecompilesServed(c, b, reopened, height)
	if c.failures != 0 || !reflect.DeepEqual(before, after) {
		t.Fatalf("precompile call behavior changed after reopen: %d failures", c.failures)
	}
	// The second retirement step must also refuse the retained anchor views.
	deleted, _ := reopened.CacheContext()
	b.UpgradeKeeper.SetDone(deleted, anchor.StoreDeleteUpgrade)
	allRetired := assertPrecompilesServed(c, b, deleted, height)
	reason, err := abi.UnpackRevert(allRetired["layerxanchor.threshold"])
	if c.failures != 0 || err != nil || reason != "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired" {
		t.Fatalf("anchor views did not retire at the separate plan: %q, %v", reason, err)
	}
	// Exercise the assertion's failure path with real, incorrectly wired
	// production contracts: membership alone must not qualify a wrong ABI.
	custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, b.GetPrecompileKeepers())
	if err != nil {
		t.Fatal(err)
	}
	custom[common.HexToAddress("0x101B")] = utils.VersionedPrecompiles{app.MergeUpgrade: custom[common.HexToAddress("0x101A")][app.MergeUpgrade]}
	b.EvmKeeper.SetCustomPrecompiles(custom, app.LatestUpgrade)
	rejected := &checks{}
	assertPrecompilesServed(rejected, b, reopened, height)
	if rejected.failures == 0 {
		t.Fatal("registration with the wrong production ABI qualified")
	}
}
