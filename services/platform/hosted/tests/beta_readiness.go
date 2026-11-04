package hostedtests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
)

type NativeReadyState struct {
	ChainID     string `json:"chain_id"`
	Genesis     string `json:"genesis"`
	Treasury    string `json:"treasury"`
	Height      uint64 `json:"height"`
	BlockHash   string `json:"block_hash"`
	Transaction string `json:"transaction"`
}

type NativeReadiness struct {
	material *NativeMaterial
	filename string
	retained *NativeReadyState
	client   *http.Client
	closed   bool
}

func NewNativeReadiness(material *NativeMaterial, filename string) (*NativeReadiness, error) {
	if material == nil || material.closed {
		return nil, errors.New("native material is closed")
	}
	readiness := &NativeReadiness{material: material, filename: filename, client: &http.Client{Timeout: 5 * time.Second}}
	raw, err := readNativeFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return readiness, nil
	}
	if err != nil {
		return nil, err
	}
	var state NativeReadyState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.ChainID != material.ChainID || state.Genesis != material.Files["genesis.json"] || state.Treasury != material.Files["treasury-policy.json"] || state.Height == 0 || !nativeHash(state.BlockHash, false) || !nativeHash(state.Transaction, true) {
		return nil, errors.New("retained committed readiness identity changed")
	}
	readiness.retained = &state
	return readiness, nil
}

func nativeHash(value string, prefix bool) bool {
	if prefix {
		if !strings.HasPrefix(value, "0x") {
			return false
		}
		value = value[2:]
	}
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && !bytes.Equal(raw, make([]byte, 32))
}

func (readiness *NativeReadiness) call(ctx context.Context, origin, method string, params any, result any) error {
	endpoint, err := url.Parse(origin)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return errors.New("native RPC must use a root JSON-RPC origin")
	}
	endpoint.Path = "/"
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": method, "method": method, "params": params})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := readiness.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("native RPC returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	if err != nil {
		return err
	}
	if len(raw) > 4*1024*1024 {
		return errors.New("native RPC response exceeds bound")
	}
	var message struct {
		Version string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	if message.Version != "2.0" || message.ID != method || (len(message.Error) != 0 && string(message.Error) != "null") || len(message.Result) == 0 || string(message.Result) == "null" {
		return errors.New("native RPC refused or returned an unrelated response")
	}
	return json.Unmarshal(message.Result, result)
}

func parseNativeHeight(value string) (uint64, error) {
	if !strings.HasPrefix(value, "0x") {
		return 0, errors.New("native receipt height must be hexadecimal")
	}
	height, err := strconv.ParseUint(value[2:], 16, 64)
	if err != nil || height == 0 || value != fmt.Sprintf("0x%x", height) {
		return 0, errors.New("invalid native receipt height")
	}
	return height, nil
}

func (readiness *NativeReadiness) Probe(ctx context.Context, rpc, consensus, transaction string) (NativeReadyState, error) {
	var empty NativeReadyState
	if readiness.closed || readiness.material.closed {
		return empty, errors.New("native readiness is closed")
	}
	if !nativeHash(transaction, true) {
		return empty, errors.New("native readiness requires the transaction hash of a committed receipt")
	}
	var status struct {
		Node struct {
			Network string `json:"network"`
		} `json:"node_info"`
		Sync struct {
			Height     string `json:"latest_block_height"`
			Committed  string `json:"last_committed_block_height"`
			Hash       string `json:"latest_block_hash"`
			CatchingUp bool   `json:"catching_up"`
		} `json:"sync_info"`
	}
	if err := readiness.call(ctx, consensus, "status", map[string]any{}, &status); err != nil {
		return empty, err
	}
	height, err := strconv.ParseUint(status.Sync.Height, 10, 64)
	if err != nil || height == 0 || status.Sync.Height != strconv.FormatUint(height, 10) || status.Node.Network != NativeChainID || status.Sync.CatchingUp || !nativeHash(status.Sync.Hash, false) {
		return empty, errors.New("native status is not a committed caught-up chain")
	}
	committed, err := strconv.ParseUint(status.Sync.Committed, 10, 64)
	if err != nil || committed < height {
		return empty, errors.New("native head is not committed")
	}
	var commit struct {
		Canonical bool `json:"canonical"`
		Signed    struct {
			Header struct {
				Chain   string `json:"chain_id"`
				Height  string `json:"height"`
				AppHash string `json:"app_hash"`
			} `json:"header"`
			Commit struct {
				Height string `json:"height"`
				Block  struct {
					Hash string `json:"hash"`
				} `json:"block_id"`
				Signatures []json.RawMessage `json:"signatures"`
			} `json:"commit"`
		} `json:"signed_header"`
	}
	if err := readiness.call(ctx, consensus, "commit", map[string]string{"height": status.Sync.Height}, &commit); err != nil {
		return empty, err
	}
	if !commit.Canonical || commit.Signed.Header.Chain != NativeChainID || commit.Signed.Header.Height != status.Sync.Height || commit.Signed.Commit.Height != status.Sync.Height || !strings.EqualFold(commit.Signed.Commit.Block.Hash, status.Sync.Hash) || !nativeHash(commit.Signed.Header.AppHash, false) || len(commit.Signed.Commit.Signatures) == 0 {
		return empty, errors.New("native canonical commit does not match status")
	}
	var chain string
	if err := readiness.call(ctx, rpc, "eth_chainId", []any{}, &chain); err != nil {
		return empty, err
	}
	if chain != "0x7d" {
		return empty, errors.New("native EVM chain is not 125")
	}
	var block struct {
		Number string `json:"number"`
		Hash   string `json:"hash"`
	}
	if err := readiness.call(ctx, rpc, "eth_getBlockByNumber", []any{fmt.Sprintf("0x%x", height), false}, &block); err != nil {
		return empty, err
	}
	if block.Number != fmt.Sprintf("0x%x", height) || !nativeHash(block.Hash, true) || !strings.EqualFold(block.Hash[2:], status.Sync.Hash) {
		return empty, errors.New("native root RPC does not expose the committed consensus block")
	}
	var receipt struct {
		Rung        string `json:"rung"`
		Hash        string `json:"txHash"`
		Height      string `json:"height"`
		Receipt     string `json:"receipt"`
		BlockNumber string `json:"blockNumber"`
		BlockHash   string `json:"blockHash"`
	}
	if err := readiness.call(ctx, rpc, "pax_sequencerReceipt", []string{transaction}, &receipt); err != nil {
		return empty, err
	}
	receiptHeight, err := parseNativeHeight(receipt.Height)
	if err != nil {
		return empty, err
	}
	coveredHeight, err := parseNativeHeight(receipt.BlockNumber)
	if err != nil || coveredHeight < receiptHeight || coveredHeight > height || receipt.Rung != "committed" || receipt.Hash != transaction || !nativeHash(receipt.BlockHash, true) || !strings.HasPrefix(receipt.Receipt, "0x") {
		return empty, errors.New("native receipt has not reached committed")
	}
	keysRaw, err := readNativeFile(filepath.Join(readiness.material.directory, "receipt-keys.json"))
	if err != nil {
		return empty, err
	}
	keys, err := LoadGatewayReceiptKeys(keysRaw)
	if err != nil {
		return empty, err
	}
	receiptRaw, err := hex.DecodeString(receipt.Receipt[2:])
	if err != nil {
		return empty, err
	}
	verified, err := keys.VerifyReceipt(receiptRaw)
	if err != nil {
		return empty, err
	}
	signedHeight, err := seqkeeper.ReceiptHeight(verified.Receipt)
	if err != nil || signedHeight != receiptHeight || "0x"+hex.EncodeToString(verified.Receipt.ActivityID[:]) != transaction {
		return empty, errors.New("native receipt view does not match its signed identity")
	}
	if err := readiness.call(ctx, rpc, "eth_getBlockByNumber", []any{receipt.BlockNumber, false}, &block); err != nil {
		return empty, err
	}
	if block.Number != receipt.BlockNumber || !strings.EqualFold(block.Hash, receipt.BlockHash) {
		return empty, errors.New("native receipt belongs to another committed block")
	}
	state := NativeReadyState{ChainID: NativeChainID, Genesis: readiness.material.Files["genesis.json"], Treasury: readiness.material.Files["treasury-policy.json"], Height: height, BlockHash: strings.ToLower(status.Sync.Hash), Transaction: transaction}
	if previous := readiness.retained; previous != nil && (state.Height < previous.Height || (state.Height == previous.Height && state.BlockHash != previous.BlockHash)) {
		return empty, errors.New("native committed state rolled back or diverged after restart")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return empty, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(readiness.filename), ".native-ready-")
	if err != nil {
		return empty, err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(raw); err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return empty, err
	}
	if closeErr != nil {
		return empty, closeErr
	}
	if err := os.Rename(temporary.Name(), readiness.filename); err != nil {
		return empty, err
	}
	directory, err := os.Open(filepath.Dir(readiness.filename))
	if err != nil {
		return empty, err
	}
	err = directory.Sync()
	directory.Close()
	if err != nil {
		return empty, err
	}
	readiness.retained = &state
	return state, nil
}

func (readiness *NativeReadiness) Close() {
	readiness.closed = true
	readiness.client.CloseIdleConnections()
}

func NativePolicyDigest(policy TreasuryPolicy) string {
	raw, _ := json.Marshal(policy)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
