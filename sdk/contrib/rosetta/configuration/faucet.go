package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	MaxAddressBody = 128
	RequestTimeout = 2 * time.Second
	FundingTimeout = 5 * time.Second
	maxChildOutput = 16 << 10
)

// BankProcess executes the real node CLI; Prefix permits an installed wrapper
// (and the committed transcript peer in the native test executable).
type BankProcess struct {
	Binary string
	Prefix []string
}

type boundedOutput struct {
	mu       sync.Mutex
	b        bytes.Buffer
	overflow bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.b.Len()+len(p) > maxChildOutput {
		b.overflow = true
		return 0, errors.New("bank output exceeds limit")
	}
	return b.b.Write(p)
}

func (p BankProcess) Fund(ctx context.Context, f Fixture, address string) error {
	if err := ValidateAddress(address); err != nil {
		return err
	}
	if err := f.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, FundingTimeout)
	defer cancel()
	args := append([]string(nil), p.Prefix...)
	args = append(args, "tx", "bank", "send", f.Address, address, f.FaucetAmount+f.Currency.Symbol,
		"--chain-id="+f.ChainID, "--node="+f.Node, "--yes", "--keyring-backend=test", "--broadcast-mode=block", "--output=json")
	cmd := exec.CommandContext(ctx, p.Binary, args...)
	// The entire process group is killed on timeout, including descendants of the
	// bank command. WaitDelay also bounds inherited stdout/stderr pipe lifetimes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 100 * time.Millisecond
	var stdout, stderr boundedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || stdout.overflow || stderr.overflow {
		return errors.New("bank process failed")
	}
	var response struct {
		Code   *uint32 `json:"code"`
		Height string  `json:"height"`
		TxHash string  `json:"txhash"`
	}
	dec := json.NewDecoder(bytes.NewReader(stdout.b.Bytes()))
	if err := dec.Decode(&response); err != nil {
		return errors.New("invalid bank response")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errors.New("trailing bank response")
	}
	height, err := strconv.ParseInt(response.Height, 10, 64)
	if response.Code == nil || *response.Code != 0 || err != nil || height < 1 || response.TxHash == "" {
		return errors.New("bank transfer was not committed successfully")
	}
	return nil
}

type Faucet struct {
	fixture Fixture
	bank    BankProcess
	active  chan struct{}
}

func NewFaucet(f Fixture, bank BankProcess) (*Faucet, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	if bank.Binary == "" {
		return nil, errors.New("bank executable required")
	}
	return &Faucet{fixture: f, bank: bank, active: make(chan struct{}, 1)}, nil
}

func reply(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(message)))
	w.Header().Set("Connection", "close")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, message)
}

func (f *Faucet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		reply(w, http.StatusNotFound, "not found\n")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		reply(w, http.StatusMethodNotAllowed, "POST required\n")
		return
	}
	if r.ContentLength < 0 {
		reply(w, http.StatusLengthRequired, "Content-Length required\n")
		return
	}
	if r.ContentLength == 0 {
		reply(w, http.StatusBadRequest, "address required\n")
		return
	}
	if r.ContentLength > MaxAddressBody {
		reply(w, http.StatusRequestEntityTooLarge, "address too large\n")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxAddressBody)
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			reply(w, http.StatusRequestTimeout, "request timed out\n")
		} else {
			reply(w, http.StatusBadRequest, "invalid request body\n")
		}
		return
	}
	if len(b) != int(r.ContentLength) || ValidateAddress(string(b)) != nil {
		reply(w, http.StatusBadRequest, "invalid address\n")
		return
	}
	select {
	case f.active <- struct{}{}:
		defer func() { <-f.active }()
	default:
		reply(w, http.StatusServiceUnavailable, "faucet busy\n")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), FundingTimeout)
	defer cancel()
	if err := f.bank.Fund(ctx, f.fixture, string(b)); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			reply(w, http.StatusGatewayTimeout, "funding timed out\n")
		} else {
			reply(w, http.StatusBadGateway, "funding failed\n")
		}
		return
	}
	reply(w, http.StatusOK, "funded\n")
}

func NewFaucetServer(address string, faucet *Faucet) *http.Server {
	return &http.Server{Addr: address, Handler: faucet, ReadHeaderTimeout: RequestTimeout,
		ReadTimeout: RequestTimeout, WriteTimeout: RequestTimeout + FundingTimeout + time.Second,
		IdleTimeout: RequestTimeout, MaxHeaderBytes: 4096}
}

func Serve(ctx context.Context, address string, f Fixture) error {
	faucet, err := NewFaucet(f, BankProcess{Binary: "simd"})
	if err != nil {
		return err
	}
	server := NewFaucetServer(address, faucet)
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			_ = server.Close()
		case <-stopped:
		}
	}()
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("faucet: %w", err)
}
