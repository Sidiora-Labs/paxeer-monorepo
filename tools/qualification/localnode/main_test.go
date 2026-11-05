package localnode

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(runLocalNodeTests(m))
}

func runLocalNodeTests(m *testing.M) (status int) {
	if os.Getenv("LOCAL_NODE_RPC") != "" || os.Getenv("PERPS_DATA_TEST_RPC") != "" {
		return m.Run()
	}
	if os.Getenv("PERPS_DATA_TEST_DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "localnode: PERPS_DATA_TEST_DATABASE_URL must name a disposable loopback Postgres database")
		return 1
	}
	state, err := os.MkdirTemp("", "localnode-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "localnode: create private evidence directory:", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "localnode: starting disposable node and gateway; private evidence:", state)
	log, err := os.OpenFile(filepath.Join(state, "setup.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "localnode: open setup log:", err)
		return 1
	}
	defer log.Close()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "localnode: locate fixture scripts")
		return 1
	}
	scripts := filepath.Dir(source)
	envFile := filepath.Join(state, "lx-localnode", "suite.env")
	run := func(script string, args ...string) error {
		cmd := exec.Command("bash", append([]string{filepath.Join(scripts, script)}, args...)...)
		cmd.Env = append(os.Environ(), "TMPDIR="+state)
		cmd.Stdout, cmd.Stderr = log, log
		return cmd.Run()
	}
	if err := run("v6120-node.sh", "--sequencer", "--env-out", envFile); err != nil {
		fmt.Fprintln(os.Stderr, "localnode: start node:", err, "; see", log.Name())
		return 1
	}
	var nodeHome string
	defer func() {
		if nodeHome == "" {
			// Read the home even when gateway startup or environment loading failed.
			cmd := exec.Command("bash", "-c", `. "$1"; printf '%s' "$LOCAL_NODE_HOME"`, "localnode-env", envFile)
			output, err := cmd.Output()
			if err != nil {
				fmt.Fprintln(os.Stderr, "localnode: read node home for cleanup:", err)
				status = 1
				return
			}
			nodeHome = string(output)
		}
		if err := run("v6120-node.sh", "--stop", nodeHome); err != nil {
			fmt.Fprintln(os.Stderr, "localnode: stop node:", err, "; see", log.Name())
			status = 1
		}
	}()
	if err := run("perps-gateway.sh", "--env", envFile); err != nil {
		fmt.Fprintln(os.Stderr, "localnode: start gateway:", err, "; see", log.Name())
		return 1
	}
	defer func() {
		if err := run("perps-gateway.sh", "--env", envFile, "--stop"); err != nil {
			fmt.Fprintln(os.Stderr, "localnode: stop gateway:", err, "; see", log.Name())
			status = 1
		}
	}()
	cmd := exec.Command("bash", "-c", `set -a; . "$1"; env -0`, "localnode-env", envFile)
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "localnode: read fixture environment:", err)
		return 1
	}
	for _, entry := range bytes.Split(output, []byte{0}) {
		key, value, ok := strings.Cut(string(entry), "=")
		if !ok || !(strings.HasPrefix(key, "LOCAL_NODE_") || strings.HasPrefix(key, "PERPS_DATA_TEST_") || key == "NEXT_PUBLIC_PERPS_GATEWAY_URL") {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintln(os.Stderr, "localnode: export", key, ":", err)
			return 1
		}
	}
	nodeHome = os.Getenv("LOCAL_NODE_HOME")
	return m.Run()
}
