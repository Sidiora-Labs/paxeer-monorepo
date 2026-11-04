package cosmovisor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func repair189Config(t *testing.T) *Config {
	t.Helper()
	cfg := &Config{Home: t.TempDir(), Name: "dummyd"}
	for _, version := range []string{genesisDir, filepath.Join(upgradesDir, "chain2"), filepath.Join(upgradesDir, "chain3")} {
		binary := filepath.Join(version, "bin", cfg.Name)
		contents, err := os.ReadFile(filepath.Join("testdata", "validate", rootName, binary))
		require.NoError(t, err)
		target := filepath.Join(cfg.Root(), binary)
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
		require.NoError(t, os.WriteFile(target, contents, 0755))
	}
	return cfg
}

func repair189Reopen(t *testing.T, cfg *Config, upgrade string) {
	t.Helper()
	reopened := &Config{Home: cfg.Home, Name: cfg.Name}
	bin, err := reopened.CurrentBin()
	require.NoError(t, err)
	require.Equal(t, reopened.UpgradeBin(upgrade), bin)
	output, err := exec.Command(bin, "recovered").CombinedOutput()
	require.NoError(t, err, string(output))
	want := "Chain 2 is live!\nArgs: recovered\nFinished successfully\n"
	if upgrade == "chain3" {
		want = "Chain 3 finally!\nArgs: recovered\nFinished successfully\n"
	}
	require.Equal(t, want, string(output))
}

func TestRepair_189_InterruptedPublication(t *testing.T) {
	// The subprocess stops at the real staging boundary, or immediately after
	// DoUpgrade has durably published the replacement. No cleanup runs on kill.
	if phase := os.Getenv("COSMOVISOR_REPAIR_189_PHASE"); phase != "" {
		cfg := &Config{Home: os.Getenv("COSMOVISOR_REPAIR_189_HOME"), Name: "dummyd"}
		if phase == "staged" {
			_, err := cfg.stageCurrentUpgrade("chain3")
			require.NoError(t, err)
		} else {
			require.NoError(t, DoUpgrade(cfg, &UpgradeInfo{Name: "chain3"}))
		}
		fmt.Println("ready")
		_, err := bufio.NewReader(os.Stdin).ReadByte()
		t.Fatalf("interruption helper unexpectedly resumed: %v", err)
	}

	for _, phase := range []string{"staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			cfg := repair189Config(t)
			require.NoError(t, DoUpgrade(cfg, &UpgradeInfo{Name: "chain2"}))
			executable, err := os.Executable()
			require.NoError(t, err)
			child := exec.Command(executable, "-test.run=^TestRepair_189_InterruptedPublication$")
			child.Env = append(os.Environ(), "COSMOVISOR_REPAIR_189_PHASE="+phase, "COSMOVISOR_REPAIR_189_HOME="+cfg.Home)
			stdin, err := child.StdinPipe()
			require.NoError(t, err)
			defer stdin.Close()
			stdout, err := child.StdoutPipe()
			require.NoError(t, err)
			child.Stderr = os.Stderr
			require.NoError(t, child.Start())
			t.Cleanup(func() {
				if child.ProcessState == nil {
					_ = child.Process.Kill()
					_ = child.Wait()
				}
			})
			ready := make(chan string, 1)
			go func() {
				line, err := bufio.NewReader(stdout).ReadString('\n')
				if err != nil {
					ready <- fmt.Sprintf("%s: %v", line, err)
					return
				}
				ready <- line
			}()
			select {
			case line := <-ready:
				require.Equal(t, "ready\n", line)
			case <-time.After(10 * time.Second):
				t.Fatal("upgrade subprocess did not reach the interruption boundary")
			}
			require.NoError(t, child.Process.Kill())
			require.Error(t, child.Wait())
			if phase == "staged" {
				staged, err := filepath.Glob(filepath.Join(cfg.Root(), ".current-*", currentLink))
				require.NoError(t, err)
				require.Len(t, staged, 1)
				target, err := os.Readlink(staged[0])
				require.NoError(t, err)
				require.Equal(t, cfg.UpgradeDir("chain3"), target)
				repair189Reopen(t, cfg, "chain2")
				// An abandoned staging directory must not block the next upgrade.
				require.NoError(t, DoUpgrade(cfg, &UpgradeInfo{Name: "chain3"}))
			}
			repair189Reopen(t, cfg, "chain3")
		})
	}
}

func TestRepair_189_AtomicSelection(t *testing.T) {
	cfg := repair189Config(t)
	require.NoError(t, DoUpgrade(cfg, &UpgradeInfo{Name: "chain2"}))
	stop := make(chan struct{})
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		first := true
		for {
			bin, err := cfg.CurrentBin()
			if err != nil {
				result <- err
				return
			}
			if bin != cfg.UpgradeBin("chain2") && bin != cfg.UpgradeBin("chain3") {
				result <- fmt.Errorf("observed unselected binary %q", bin)
				return
			}
			if first {
				close(ready)
				first = false
			}
			select {
			case <-stop:
				result <- nil
				return
			default:
			}
		}
	}()
	select {
	case <-ready:
	case err := <-result:
		t.Fatalf("initial read failed: %v", err)
	}
	var upgradeErr error
	for i := 0; i < 100; i++ {
		name := "chain2"
		if i%2 == 1 {
			name = "chain3"
		}
		if upgradeErr = DoUpgrade(cfg, &UpgradeInfo{Name: name}); upgradeErr != nil {
			break
		}
	}
	close(stop)
	require.NoError(t, <-result)
	require.NoError(t, upgradeErr)
	repair189Reopen(t, cfg, "chain3")
}

func TestRepair_189_LostSelection(t *testing.T) {
	for _, state := range []string{"genesis", "upgrade", "legacy-upgrade", "legacy-current"} {
		t.Run(state, func(t *testing.T) {
			cfg := repair189Config(t)
			link := filepath.Join(cfg.Root(), currentLink)
			switch state {
			case "genesis":
				bin, err := cfg.CurrentBin()
				require.NoError(t, err)
				require.Equal(t, cfg.GenesisBin(), bin)
				require.NoError(t, os.Remove(link))
			case "upgrade":
				require.NoError(t, DoUpgrade(cfg, &UpgradeInfo{Name: "chain2"}))
				require.NoError(t, os.Remove(link))
			case "legacy-upgrade":
				require.NoError(t, os.MkdirAll(filepath.Join(cfg.Home, "data"), 0755))
				// The upgrade keeper's legacy halt record is itself sufficient
				// evidence; even malformed state must never permit genesis.
				require.NoError(t, os.WriteFile(filepath.Join(cfg.Home, "data", "upgrade-info.json"), nil, 0600))
			case "legacy-current":
				require.NoError(t, os.Symlink(cfg.UpgradeDir("chain2"), link))
				bin, err := cfg.CurrentBin()
				require.NoError(t, err)
				require.Equal(t, cfg.UpgradeBin("chain2"), bin)
				require.NoError(t, os.Remove(link))
			}
			reopened := &Config{Home: cfg.Home, Name: cfg.Name}
			bin, err := reopened.CurrentBin()
			require.ErrorContains(t, err, "restore the selected binary")
			require.Empty(t, bin)
			_, err = os.Lstat(link)
			require.True(t, os.IsNotExist(err), "startup must not recreate current: %v", err)
			// The launch consumer must stop before executing a binary as well.
			var stdout, stderr strings.Builder
			upgraded, err := LaunchProcess(reopened, nil, &stdout, &stderr)
			require.ErrorContains(t, err, "restore the selected binary")
			require.False(t, upgraded)
			require.Empty(t, stdout.String())
			require.Empty(t, stderr.String())
		})
	}
}

func TestRepair_189_InvalidSelection(t *testing.T) {
	for _, state := range []string{"file", "dangling", "parent-file"} {
		t.Run(state, func(t *testing.T) {
			cfg := repair189Config(t)
			link := filepath.Join(cfg.Root(), currentLink)
			switch state {
			case "file":
				require.NoError(t, os.WriteFile(link, []byte("not a symlink"), 0600))
			case "dangling":
				require.NoError(t, os.Symlink(cfg.UpgradeDir("missing"), link))
			case "parent-file":
				parent := filepath.Join(cfg.Home, "not-a-directory")
				require.NoError(t, os.WriteFile(parent, nil, 0600))
				cfg.Home = parent
			}
			reopened := &Config{Home: cfg.Home, Name: cfg.Name}
			bin, err := reopened.CurrentBin()
			require.Error(t, err)
			require.Empty(t, bin)
			if state == "file" {
				contents, err := os.ReadFile(link)
				require.NoError(t, err)
				require.Equal(t, "not a symlink", string(contents))
			}
		})
	}
}

func TestRepair_189_FirstInitialization(t *testing.T) {
	cfg := repair189Config(t)
	// Pre-staged upgrade binaries must not prevent the first genesis launch.
	bin, err := cfg.CurrentBin()
	require.NoError(t, err)
	require.Equal(t, cfg.GenesisBin(), bin)
	reopened := &Config{Home: cfg.Home, Name: cfg.Name}
	bin, err = reopened.CurrentBin()
	require.NoError(t, err)
	require.Equal(t, reopened.GenesisBin(), bin)

	// Drive the real log producer, scanner, upgrade publisher and launch consumer.
	var stdout, stderr strings.Builder
	upgraded, err := LaunchProcess(reopened, []string{"initial"}, &stdout, &stderr)
	require.NoError(t, err)
	require.True(t, upgraded)
	require.Equal(t, "Genesis initial\nUPGRADE \"chain2\" NEEDED at height: 49: {}\n", stdout.String())
	require.Empty(t, stderr.String())
	repair189Reopen(t, reopened, "chain2")
}

func TestRepair_189_PublicationFailure(t *testing.T) {
	t.Run("invalid-upgrade", func(t *testing.T) {
		cfg := repair189Config(t)
		require.NoError(t, DoUpgrade(cfg, &UpgradeInfo{Name: "chain2"}))
		require.Error(t, cfg.SetCurrentUpgrade("missing"))
		repair189Reopen(t, cfg, "chain2")
	})
	t.Run("current-directory", func(t *testing.T) {
		cfg := repair189Config(t)
		link := filepath.Join(cfg.Root(), currentLink)
		require.NoError(t, os.Mkdir(link, 0755))
		state := filepath.Join(link, "state")
		require.NoError(t, os.WriteFile(state, []byte("retain selection state"), 0600))
		require.ErrorContains(t, cfg.SetCurrentUpgrade("chain3"), "publishing current symlink")
		contents, err := os.ReadFile(state)
		require.NoError(t, err)
		require.Equal(t, "retain selection state", string(contents))
		staged, err := filepath.Glob(filepath.Join(cfg.Root(), ".current-*"))
		require.NoError(t, err)
		require.Empty(t, staged)
		reopened := &Config{Home: cfg.Home, Name: cfg.Name}
		bin, err := reopened.CurrentBin()
		require.ErrorContains(t, err, "is not a symlink")
		require.Empty(t, bin)
	})
}
