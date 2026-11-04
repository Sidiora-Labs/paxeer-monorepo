package configuration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"google.golang.org/protobuf/encoding/protowire"
)

func repairFixture(t *testing.T) Fixture {
	t.Helper()
	f, err := LoadFixture(".")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func repairArchive(t *testing.T) []byte {
	t.Helper()
	b, err := readArchive("../node/data.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRepairM_1356_BootstrapState(t *testing.T) {
	f := repairFixture(t)
	got, err := SnapshotMetadata(repairArchive(t), f)
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != "cosmos1w537ue6nmewqqdcqfhcn0jsntg8qjpcs4wf75w" || got.Currency != (Currency{"stake", 0}) || got.SnapshotHeight != 4 || got.Balance != "999989999900" {
		t.Fatalf("wrong committed snapshot metadata: %+v", got)
	}
	if !reflect.DeepEqual(got, f) {
		t.Fatalf("manifest disagrees with recovered bank store: got %+v want %+v", got, f)
	}
	generated, err := Metadata(got)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range generated {
		disk, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(disk, want) {
			t.Fatalf("%s differs from real snapshot producer\n%s", name, want)
		}
	}
	var balances []BootstrapBalance
	if err := json.Unmarshal(generated["bootstrap.json"], &balances); err != nil {
		t.Fatal(err)
	}
	if len(balances) != 1 || balances[0].AccountIdentifier.Address != got.Address || balances[0].Currency != got.Currency || balances[0].Value != got.Balance {
		t.Fatal("reconciliation bootstrap lost recovered bank state")
	}
}

func TestRepairM_1356_RegenerationContract(t *testing.T) {
	f := repairFixture(t)
	root := t.TempDir()
	config := filepath.Join(root, "config")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := WriteMetadata(config, f); err != nil {
		t.Fatal(err)
	}
	archive, err := filepath.Abs("../node/data.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	var first []byte
	for i := 0; i < 2; i++ {
		home := filepath.Join(root, fmt.Sprintf("node-%d", i))
		out := filepath.Join(root, fmt.Sprintf("data-%d.tar.gz", i))
		args := []string{"restore", "--config", config, "--archive", archive, "--home", home, "--output", out}
		if err := Run(context.Background(), args, io.Discard); err != nil {
			t.Fatal(err)
		}
		reproduced, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reproduced, repairArchive(t)) {
			t.Fatal("restoration changed pinned snapshot identity")
		}
		// Restore's snapshot reader is closed; reopen its persisted LevelDB bank root.
		reopened, err := metadataFromHome(home, f)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(reopened, f) {
			t.Fatalf("restored state differs after reopen: %+v", reopened)
		}
		loaded, err := LoadFixture(config)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(loaded, reopened) {
			t.Fatal("metadata consumer differs from reopened state")
		}
		current, err := os.ReadFile(filepath.Join(config, "bootstrap.json"))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = current
		} else if !bytes.Equal(first, current) {
			t.Fatal("metadata regeneration is nondeterministic")
		}
		if err := Run(context.Background(), args, io.Discard); err == nil {
			t.Fatal("restoration overwrote a populated node home")
		}
		still, err := metadataFromHome(home, f)
		if err != nil || !reflect.DeepEqual(still, f) {
			t.Fatalf("refused restore altered existing state: %+v %v", still, err)
		}
	}
	corrupted := append([]byte(nil), repairArchive(t)...)
	corrupted[len(corrupted)/2] ^= 1
	if _, err := SnapshotMetadata(corrupted, f); err == nil {
		t.Fatal("accepted archive outside explicit fixture identity")
	}
	wrong := f
	wrong.Address = "cosmos19g9cm8ymzchq2qkcdv3zgqtwayj9asv3hjv5u5"
	if _, err := SnapshotMetadata(repairArchive(t), wrong); err == nil {
		t.Fatal("accepted a different genesis identity")
	}
}

func repairBank(t *testing.T, mode, marker string) BankProcess {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return BankProcess{Binary: binary, Prefix: []string{"-test.run=^TestRepairM_1356_ChildPeer$", "--", mode, marker}}
}

func repairServer(t *testing.T, bank BankProcess) string {
	t.Helper()
	faucet, err := NewFaucet(repairFixture(t), bank)
	if err != nil {
		t.Fatal(err)
	}
	server := NewFaucetServer("127.0.0.1:0", faucet)
	if server.ReadTimeout != RequestTimeout || server.ReadHeaderTimeout != RequestTimeout || server.WriteTimeout > 9*time.Second || server.MaxHeaderBytes > 4096 {
		t.Fatal("unbounded production HTTP deadlines")
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		if err := <-done; err != http.ErrServerClosed {
			t.Errorf("server: %v", err)
		}
	})
	return "http://" + listener.Addr().String()
}

func repairPost(t *testing.T, url, body string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 9 * time.Second}
	response, err := client.Post(url, "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	b, err := io.ReadAll(io.LimitReader(response.Body, 1025))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 128 || response.ContentLength != int64(len(b)) {
		t.Fatal("unbounded or incorrectly framed faucet response")
	}
	return response.StatusCode, string(b)
}

func repairRaw(t *testing.T, url, request string, closeWrite bool) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(url, "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatal(err)
	}
	if closeWrite {
		if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
			t.Fatal(err)
		}
	}
	b, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRepairM_1356_FaucetInputAndRequestBounds(t *testing.T) {
	url := repairServer(t, repairBank(t, "success", "-"))
	valid := repairFixture(t).Address
	for _, body := range []string{"", strings.Repeat("x", MaxAddressBody+1), "--help", strings.ToUpper(valid), valid[:44] + "q", valid + "\n", "cosmosvaloper1w537ue6nmewqqdcqfhcn0jsntg8qjpcss6atca", string([]byte{255, 254})} {
		status, _ := repairPost(t, url, body)
		expected := http.StatusBadRequest
		if len(body) > MaxAddressBody {
			expected = http.StatusRequestEntityTooLarge
		}
		if status != expected {
			t.Fatalf("body %q: got %d want %d", body, status, expected)
		}
	}
	for _, tc := range []struct {
		request string
		status  string
	}{
		{"POST / HTTP/1.1\r\nHost: faucet\r\nContent-Length: nope\r\n\r\n", "400"},
		{"POST / HTTP/1.1\r\nHost: faucet\r\nContent-Length: -1\r\n\r\n", "400"},
		{"POST / HTTP/1.1\r\nHost: faucet\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n", "411"},
		{"POST / HTTP/1.1\r\nHost: faucet\r\nContent-Length: 45\r\n\r\nshort", "400"},
	} {
		got := repairRaw(t, url, tc.request, true)
		if !strings.HasPrefix(got, "HTTP/1.1 "+tc.status) {
			t.Fatalf("request framing: %q", got)
		}
	}
	start := time.Now()
	slow := repairRaw(t, url, "POST / HTTP/1.1\r\nHost: faucet\r\nContent-Length: 45\r\n\r\nc", false)
	if !strings.HasPrefix(slow, "HTTP/1.1 408") || time.Since(start) > 3*time.Second {
		t.Fatalf("body deadline not enforced: %q", slow)
	}
	start = time.Now()
	_ = repairRaw(t, url, "POST / HTTP/1.1\r\nHost:", false)
	if time.Since(start) > 3*time.Second {
		t.Fatal("header deadline not enforced")
	}
	status, body := repairPost(t, url, valid)
	if status != http.StatusOK || body != "funded\n" {
		t.Fatalf("server failed after malformed requests: %d %q", status, body)
	}
}

func TestRepairM_1356_FaucetTransferOutcome(t *testing.T) {
	for _, mode := range []string{"success", "exit", "rejected", "invalid-json", "uncommitted", "overflow", "trailing"} {
		t.Run(mode, func(t *testing.T) {
			url := repairServer(t, repairBank(t, mode, "-"))
			status, body := repairPost(t, url, repairFixture(t).Address)
			expected := http.StatusBadGateway
			if mode == "success" {
				expected = http.StatusOK
			}
			if status != expected {
				t.Fatalf("child outcome %s: %d %q", mode, status, body)
			}
			if strings.Contains(body, "insufficient") || strings.Contains(body, "private diagnostic") {
				t.Fatal("child diagnostics leaked over HTTP")
			}
			if status, _ := repairPost(t, url, "invalid"); status != 400 {
				t.Fatal("failed child terminated the server")
			}
		})
	}
	url := repairServer(t, BankProcess{Binary: filepath.Join(t.TempDir(), "absent-bank")})
	if status, _ := repairPost(t, url, repairFixture(t).Address); status != http.StatusBadGateway {
		t.Fatal("missing process returned success")
	}
}

func processRunning(pid int) bool {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	i := strings.LastIndex(string(b), ") ")
	return i < 0 || (b[i+2] != 'Z' && b[i+2] != 'X')
}

func TestRepairM_1356_FaucetProcessDeadlineAndCleanup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "processes")
	url := repairServer(t, repairBank(t, "timeout", marker))
	start := time.Now()
	status, body := repairPost(t, url, repairFixture(t).Address)
	if status != http.StatusGatewayTimeout || body != "funding timed out\n" || time.Since(start) > 7*time.Second {
		t.Fatalf("process deadline: %d %q after %v", status, body, time.Since(start))
	}
	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(b))
	if len(fields) != 2 {
		t.Fatalf("missing supervised child and descendant: %q", b)
	}
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(time.Second)
		for processRunning(pid) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if processRunning(pid) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("process %d survived funding deadline", pid)
		}
	}
	if status, _ := repairPost(t, url, "invalid"); status != 400 {
		t.Fatal("process timeout terminated server")
	}
}

func TestRepairM_1356_FaucetCurrency(t *testing.T) {
	f := repairFixture(t)
	dir := t.TempDir()
	actual, err := SnapshotMetadata(repairArchive(t), f)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMetadata(dir, actual); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	transfer, err := os.ReadFile(filepath.Join(dir, "transfer.ros"))
	if err != nil {
		t.Fatal(err)
	}
	currency, _ := json.Marshal(loaded.Currency)
	if strings.Count(string(transfer), "currency = "+string(currency)+";") != 2 || !strings.Contains(string(transfer), `"gas_price": "1`+loaded.Currency.Symbol+`"`) || strings.Contains(string(transfer), "uhpx") {
		t.Fatalf("construction does not consume shared currency: %s", transfer)
	}
	// The HTTP handler consumes metadata regenerated from the actual bank store;
	// the transcript peer asserts the exact bank send amount, denom and chain flags.
	faucet, err := NewFaucet(loaded, repairBank(t, "success", "-"))
	if err != nil {
		t.Fatal(err)
	}
	server := NewFaucetServer("127.0.0.1:0", faucet)
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	defer func() { _ = server.Close(); <-done }()
	status, body := repairPost(t, "http://"+ln.Addr().String(), loaded.Address)
	if status != 200 || body != "funded\n" {
		t.Fatalf("shared currency funding failed: %d %q", status, body)
	}
	for name, command := range map[string]string{"faucet.py": "\"serve\"", "send_funds.sh": "rosetta-fixture send --config", "data.sh": "rosetta-fixture restore --config"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), command) {
			t.Fatalf("%s bypasses shared production command", name)
		}
	}
}

// ChildPeer is a committed bank-CLI transcript peer, executed by the existing
// native test binary. No interpreter, build subprocess, node or RPC is needed.
func TestRepairM_1356_ChildPeer(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		b, err := os.ReadFile("testdata/bank-success.json")
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(b) {
			t.Fatal("invalid committed bank transcript")
		}
		return
	}
	args := os.Args[index+1:]
	if len(args) < 2 {
		os.Exit(90)
	}
	mode, marker := args[0], args[1]
	if mode == "descendant" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	f, err := LoadFixture(".")
	if err != nil {
		os.Exit(91)
	}
	want := []string{"tx", "bank", "send", f.Address, f.Address, f.FaucetAmount + f.Currency.Symbol, "--chain-id=" + f.ChainID, "--node=" + f.Node, "--yes", "--keyring-backend=test", "--broadcast-mode=block", "--output=json"}
	if !reflect.DeepEqual(args[2:], want) {
		fmt.Fprintf(os.Stderr, "incorrect bank command: %q", args[2:])
		os.Exit(92)
	}
	switch mode {
	case "timeout":
		executable, err := os.Executable()
		if err != nil {
			os.Exit(93)
		}
		child := exec.Command(executable, "-test.run=^TestRepairM_1356_ChildPeer$", "--", "descendant", "-")
		if err := child.Start(); err != nil {
			os.Exit(94)
		}
		if err := os.WriteFile(marker, []byte(fmt.Sprintf("%d %d", os.Getpid(), child.Process.Pid)), 0600); err != nil {
			os.Exit(95)
		}
		time.Sleep(30 * time.Second)
		_ = child.Wait()
	case "exit":
		fmt.Fprintln(os.Stderr, "private diagnostic")
		os.Exit(7)
	case "rejected":
		b, err := os.ReadFile("testdata/bank-rejected.json")
		if err != nil {
			os.Exit(96)
		}
		_, _ = os.Stdout.Write(b)
	case "invalid-json":
		fmt.Print("not JSON")
	case "uncommitted":
		fmt.Print(`{"code":0,"height":"0","txhash":"HASH"}`)
	case "overflow":
		fmt.Print(strings.Repeat("x", maxChildOutput+1))
	case "trailing":
		fmt.Print(`{"code":0,"height":"3","txhash":"HASH"} {}`)
	case "success":
		b, err := os.ReadFile("testdata/bank-success.json")
		if err != nil {
			os.Exit(97)
		}
		_, _ = os.Stdout.Write(b)
	default:
		os.Exit(98)
	}
	os.Exit(0)
}

func repairWireField(t *testing.T, b []byte, want protowire.Number) ([]byte, uint64, bool) {
	t.Helper()
	for len(b) > 0 {
		number, kind, n := protowire.ConsumeTag(b)
		if n < 0 {
			t.Fatal("invalid recorded protobuf tag")
		}
		b = b[n:]
		size := protowire.ConsumeFieldValue(number, kind, b)
		if size < 0 {
			t.Fatal("invalid recorded protobuf field")
		}
		if number == want {
			if kind == protowire.BytesType {
				value, n := protowire.ConsumeBytes(b)
				if n < 0 {
					t.Fatal("invalid bytes")
				}
				return value, 0, true
			}
			if kind == protowire.VarintType {
				value, n := protowire.ConsumeVarint(b)
				if n < 0 {
					t.Fatal("invalid varint")
				}
				return nil, value, true
			}
			t.Fatal("unexpected recorded field type")
		}
		b = b[size:]
	}
	return nil, 0, false
}

func TestRepairM_1356_RecordedBankTranscript(t *testing.T) {
	f := repairFixture(t)
	dir := t.TempDir()
	archive := repairArchive(t)
	if err := checkArchive(archive, f); err != nil {
		t.Fatal(err)
	}
	if err := unpackArchive(archive, dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/bank-tx-result.pb")
	if err != nil {
		t.Fatal(err)
	}
	tx, _, found := repairWireField(t, raw, 3)
	if !found {
		t.Fatal("recorded TxResult has no transaction")
	}
	hash := sha256.Sum256(tx)
	db, err := leveldb.OpenFile(filepath.Join(dir, "data/tx_index.db"), &opt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stored, err := db.Get(hash[:], nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, stored) {
		t.Fatal("bank transcript is not from actual committed snapshot")
	}
	_, height, found := repairWireField(t, raw, 1)
	if !found {
		t.Fatal("recorded TxResult has no height")
	}
	result, _, found := repairWireField(t, raw, 4)
	if !found {
		t.Fatal("recorded TxResult has no delivery result")
	}
	_, code, _ := repairWireField(t, result, 1)
	log, _, found := repairWireField(t, result, 3)
	if !found {
		t.Fatal("recorded TxResult has no log")
	}
	var response struct {
		Height string `json:"height"`
		TxHash string `json:"txhash"`
		Code   uint64 `json:"code"`
		RawLog string `json:"raw_log"`
	}
	b, err := os.ReadFile("testdata/bank-success.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &response); err != nil {
		t.Fatal(err)
	}
	if response.Height != strconv.FormatUint(height, 10) || response.TxHash != strings.ToUpper(hex.EncodeToString(hash[:])) || response.Code != code || response.RawLog != string(log) {
		t.Fatal("CLI response projection differs from real persisted producer output")
	}
}
