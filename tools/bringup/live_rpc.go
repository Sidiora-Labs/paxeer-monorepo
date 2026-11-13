package bringup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func protectedURL(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return errors.New("explicit HTTPS or loopback endpoint required")
	}
	return nil
}

func quantity(s string) (*big.Int, error) {
	if !strings.HasPrefix(s, "0x") || len(s) < 3 {
		return nil, errors.New("invalid RPC quantity")
	}
	n, ok := new(big.Int).SetString(s[2:], 16)
	if !ok || n.Sign() < 0 {
		return nil, errors.New("invalid RPC quantity")
	}
	return n, nil
}

func randomCorrelation() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func jsonRequest(ctx context.Context, client *http.Client, method, endpoint, token, correlation string, body any) (int, json.RawMessage, http.Header, error) {
	if err := protectedURL(endpoint); err != nil {
		return 0, nil, nil, err
	}
	var input io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
		input = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, input)
	if err != nil {
		return 0, nil, nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", token)
	}
	if correlation != "" {
		request.Header.Set("Idempotency-Key", correlation)
	}
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := bounded.Do(request)
	if err != nil {
		return 0, nil, nil, errors.New("HTTP transport failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return 0, nil, nil, errors.New("HTTP body unreadable")
	}
	return response.StatusCode, data, response.Header, nil
}

type KernelProcess struct {
	Executable string `json:"executable"`
	PID        int    `json:"pid"`
	UID        uint32 `json:"uid"`
	Owner      uint32 `json:"owner"`
	Home       string `json:"home"`
	Mounted    bool   `json:"mounted"`
	Genesis    bool   `json:"genesis"`
	State      bool   `json:"state"`
	Running    bool   `json:"running"`
	StartTicks uint64 `json:"start_ticks"`
	BootID     string `json:"boot_id"`
}

func CheckKernelNode(ctx context.Context, client *http.Client, node, comet string, process KernelProcess, output io.Writer) error {
	if process.Executable != "paxd" || process.PID <= 0 || !process.Running || !process.Mounted || !process.Genesis || !process.State || process.Home == "" || process.UID != 4020 || process.Owner != process.UID || process.StartTicks == 0 || process.BootID == "" {
		return errors.New("paxd process, mounted state or ownership evidence is invalid")
	}
	for _, endpoint := range []string{node, comet} {
		if err := protectedURL(endpoint); err != nil {
			return err
		}
	}
	chain, err := callRPC(ctx, client, node, "eth_chainId", []any{}, "", false)
	if err != nil || string(chain) != `"0x7d"` {
		return errors.New("paxd chain id mismatch")
	}
	headRaw, err := callRPC(ctx, client, node, "eth_blockNumber", []any{}, "", false)
	if err != nil {
		return err
	}
	var headText string
	if json.Unmarshal(headRaw, &headText) != nil {
		return errors.New("paxd head unreadable")
	}
	head, err := quantity(headText)
	if err != nil || head.Sign() == 0 || !head.IsUint64() {
		return errors.New("paxd has no committed height")
	}
	status, err := callRPC(ctx, client, comet, "status", map[string]any{}, "", false)
	if err != nil {
		return err
	}
	var sync struct {
		Node struct {
			Network string `json:"network"`
		} `json:"node_info"`
		Sync struct {
			Height   string `json:"latest_block_height"`
			Catching *bool  `json:"catching_up"`
		} `json:"sync_info"`
	}
	if json.Unmarshal(status, &sync) != nil || sync.Sync.Catching == nil || *sync.Sync.Catching || sync.Node.Network == "" {
		return errors.New("consensus is unavailable or catching up")
	}
	latest, ok := new(big.Int).SetString(sync.Sync.Height, 10)
	if !ok || latest.Cmp(head) < 0 {
		return errors.New("EVM head exceeds committed consensus")
	}
	commitRaw, err := callRPC(ctx, client, comet, "commit", map[string]string{"height": head.String()}, "", false)
	if err != nil {
		return err
	}
	var commit struct {
		Canonical bool `json:"canonical"`
		Header    struct {
			Header struct {
				Height  string `json:"height"`
				Chain   string `json:"chain_id"`
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
	if json.Unmarshal(commitRaw, &commit) != nil || !commit.Canonical || commit.Header.Header.Height != head.String() || commit.Header.Commit.Height != head.String() || commit.Header.Header.Chain != sync.Node.Network || commit.Header.Header.AppHash == "" || commit.Header.Commit.Block.Hash == "" || len(commit.Header.Commit.Signatures) == 0 {
		return errors.New("committed height lacks matching canonical consensus evidence")
	}
	_, err = fmt.Fprintf(output, "pass kernel-node process=paxd pid=%d uid=%d home=%s state=mounted committed-height=%s start-ticks=%d boot-id=%s\n", process.PID, process.UID, process.Home, head.String(), process.StartTicks, process.BootID)
	return err
}

type CommittedTransaction struct {
	Hash    string
	Height  string
	Block   string
	Receipt json.RawMessage
}

func ReadCommittedTransaction(ctx context.Context, client *http.Client, endpoint, hash string) (CommittedTransaction, error) {
	if err := protectedURL(endpoint); err != nil {
		return CommittedTransaction{}, err
	}
	raw, err := callRPC(ctx, client, endpoint, "eth_getTransactionReceipt", []string{hash}, "", false)
	if err != nil {
		return CommittedTransaction{}, err
	}
	var receipt struct {
		Hash   string `json:"transactionHash"`
		Block  string `json:"blockHash"`
		Height string `json:"blockNumber"`
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.Hash != hash || receipt.Status != "0x1" || !isHexWord(receipt.Block) {
		return CommittedTransaction{}, errors.New("transaction receipt is not a matching success")
	}
	height, err := quantity(receipt.Height)
	if err != nil || height.Sign() == 0 {
		return CommittedTransaction{}, errors.New("receipt has no inclusion height")
	}
	blockRaw, err := callRPC(ctx, client, endpoint, "eth_getBlockByNumber", []any{receipt.Height, false}, "", false)
	if err != nil {
		return CommittedTransaction{}, err
	}
	var block struct {
		Hash         string   `json:"hash"`
		Number       string   `json:"number"`
		Transactions []string `json:"transactions"`
	}
	if json.Unmarshal(blockRaw, &block) != nil || block.Hash != receipt.Block || block.Number != receipt.Height {
		return CommittedTransaction{}, errors.New("receipt does not belong to canonical committed block")
	}
	included := false
	for _, tx := range block.Transactions {
		if tx == hash {
			included = true
		}
	}
	if !included {
		return CommittedTransaction{}, errors.New("transaction is absent from canonical block")
	}
	nativeRaw, err := callRPC(ctx, client, endpoint, "pax_sequencerReceipt", []string{hash}, "", false)
	if err != nil {
		return CommittedTransaction{}, err
	}
	var native struct {
		Rung    string `json:"rung"`
		Hash    string `json:"txHash"`
		Height  string `json:"blockNumber"`
		Block   string `json:"blockHash"`
		Status  string `json:"status"`
		Receipt string `json:"receipt"`
	}
	if json.Unmarshal(nativeRaw, &native) != nil || native.Rung != "committed" || native.Hash != hash || native.Height != receipt.Height || native.Block != receipt.Block || native.Status != "0x1" || native.Receipt == "" {
		return CommittedTransaction{}, errors.New("native receipt is not committed to the same transaction and block")
	}
	keys, err := callRPC(ctx, client, endpoint, "px_getReceiptKeys", []any{}, "", false)
	if err != nil {
		return CommittedTransaction{}, err
	}
	if err := VerifyCommittedReceipt(nativeRaw, keys, hash); err != nil {
		return CommittedTransaction{}, err
	}
	return CommittedTransaction{hash, receipt.Height, receipt.Block, nativeRaw}, nil
}

func VerifyCommittedReceipt(nativeRaw, keysRaw json.RawMessage, hash string) error {
	var native struct {
		Receipt     string `json:"receipt"`
		KeyID       string `json:"keyId"`
		Height      string `json:"height"`
		BlockNumber string `json:"blockNumber"`
	}
	if json.Unmarshal(nativeRaw, &native) != nil {
		return errors.New("native receipt unreadable")
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(native.Receipt, "0x"))
	if err != nil {
		return errors.New("native receipt bytes malformed")
	}
	receipt, err := codec.DecodeReceipt(raw)
	if err != nil {
		return errors.New("native receipt codec refused")
	}
	height, err := quantity(native.Height)
	if err != nil || !height.IsUint64() {
		return errors.New("native receipt height malformed")
	}
	block, err := quantity(native.BlockNumber)
	if err != nil || block.Cmp(height) < 0 {
		return errors.New("native receipt inclusion precedes execution")
	}
	if receipt.ActivityID != [32]byte(common.HexToHash(hash)) || receipt.ResultCode != 0 || receipt.ContextHash != [32]byte(common.HexToHash(native.KeyID)) || !bytes.Equal(receipt.BatchID[:24], make([]byte, 24)) || binary.BigEndian.Uint64(receipt.BatchID[24:]) != height.Uint64() {
		return errors.New("native receipt bytes disagree with transaction or execution height")
	}
	var history struct {
		Keys []struct {
			ID     string `json:"keyId"`
			Public string `json:"publicKey"`
			First  uint64 `json:"firstHeight"`
			Last   uint64 `json:"lastHeight"`
		} `json:"keys"`
	}
	if json.Unmarshal(keysRaw, &history) != nil || len(history.Keys) == 0 {
		return errors.New("governed receipt key history unavailable")
	}
	seen := map[string]bool{}
	matched := false
	for _, key := range history.Keys {
		public, err := hex.DecodeString(strings.TrimPrefix(key.Public, "0x"))
		if !isHexWord(key.ID) || seen[key.ID] || err != nil || len(public) != 32 || key.First == 0 || key.First > key.Last || !verify.PublicKeyIsCanonical([32]byte(public)) {
			return errors.New("governed receipt key history malformed")
		}
		seen[key.ID] = true
		if key.ID == native.KeyID {
			if height.Uint64() < key.First || height.Uint64() > key.Last {
				return errors.New("receipt signing key is outside its governed range")
			}
			if _, err := verify.ReceiptSignature(raw, [32]byte(public)); err != nil {
				return errors.New("native receipt signature refused")
			}
			matched = true
		}
	}
	if !matched {
		return errors.New("native receipt key is not governed")
	}
	return nil
}

func isHexWord(s string) bool {
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	return strings.HasPrefix(s, "0x") && err == nil && len(b) == 32
}

func DecodeSignedTransaction(raw []byte) (*types.Transaction, error) {
	var tx types.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return nil, errors.New("signed chain transaction malformed")
	}
	if tx.ChainId().Cmp(big.NewInt(125)) != 0 {
		return nil, errors.New("signed transaction must target chain 125")
	}
	if _, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), &tx); err != nil {
		return nil, errors.New("transaction signature refused")
	}
	return &tx, nil
}

func CheckArchiveDelivery(ctx context.Context, client *http.Client, origin string, raw []byte, output io.Writer) error {
	if err := protectedURL(origin); err != nil {
		return err
	}
	tx, err := DecodeSignedTransaction(raw)
	if err != nil {
		return err
	}
	accepted, err := callRPC(ctx, client, origin, "eth_sendRawTransaction", []string{"0x" + hex.EncodeToString(raw)}, "", false)
	if err != nil {
		return err
	}
	var hash string
	if json.Unmarshal(accepted, &hash) != nil || hash != tx.Hash().Hex() {
		return errors.New("archive accepted a different transaction")
	}
	evidence, err := waitCommittedTransaction(ctx, client, origin, hash)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "pass archive-delivery transaction=%s committed-height=%s block=%s read-back=match\n", evidence.Hash, evidence.Height, evidence.Block)
	return err
}

func waitCommittedTransaction(ctx context.Context, client *http.Client, endpoint, hash string) (CommittedTransaction, error) {
	for {
		evidence, err := ReadCommittedTransaction(ctx, client, endpoint, hash)
		if err == nil {
			return evidence, nil
		}
		select {
		case <-ctx.Done():
			return CommittedTransaction{}, errors.New("committed receipt deadline exceeded")
		case <-time.After(time.Second):
		}
	}
}

type SearchOffer struct {
	Scheme  string `json:"scheme"`
	Network string `json:"network"`
	Amount  string `json:"amount"`
	Asset   string `json:"asset"`
	PayTo   string `json:"payTo"`
	Extra   struct {
		LayerX struct {
			Currency string `json:"currency"`
		} `json:"layerx"`
	} `json:"extra"`
}

type committedAssetInventory struct {
	Height *uint64 `json:"height"`
	Assets []struct {
		ID           string `json:"asset_id"`
		Symbol       string `json:"symbol"`
		Decimals     *int   `json:"decimals"`
		Paused       *bool  `json:"paused"`
		Verification string `json:"verification"`
	} `json:"assets"`
}

func readCommittedAssetInventory(raw json.RawMessage) (committedAssetInventory, error) {
	var inventory committedAssetInventory
	if uniqueJSON(raw) != nil || json.Unmarshal(raw, &inventory) != nil || inventory.Height == nil || *inventory.Height == 0 {
		return inventory, errors.New("hosted asset inventory lacks an unambiguous committed height")
	}
	return inventory, nil
}

func ValidateSearchPrice(offer SearchOffer, assets json.RawMessage) error {
	listed, err := readCommittedAssetInventory(assets)
	if err != nil {
		return err
	}
	if offer.Scheme != "metered" || offer.Network != "eip155:125" || offer.Extra.LayerX.Currency != "PAX" {
		return errors.New("search offer or hosted asset response malformed")
	}
	found := 0
	for _, asset := range listed.Assets {
		if asset.ID != offer.Asset {
			continue
		}
		found++
		if asset.Symbol != "PAX" || asset.Paused == nil || *asset.Paused || asset.Verification != "authenticated_committed_snapshot" || asset.Decimals == nil || *asset.Decimals < 0 || *asset.Decimals > 38 {
			return errors.New("offer asset is not an available committed PAX asset")
		}
		units := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(*asset.Decimals)), nil)
		units.Add(units, big.NewInt(13439))
		units.Div(units, big.NewInt(13440))
		if offer.Amount != units.String() {
			return errors.New("search price differs from approved base-unit amount")
		}
	}
	if found != 1 {
		return errors.New("offer asset missing or duplicated in hosted inventory")
	}
	return nil
}

func CheckSearchPayment(ctx context.Context, client *http.Client, endpoint, gateway, payer, grant, token string, output io.Writer) error {
	if !common.IsHexAddress(payer) || !isHexWord("0x"+strings.TrimPrefix(grant, "0x")) {
		return errors.New("payer EVM address and standing grant id required")
	}
	searchURL, err := url.Parse(endpoint)
	if err != nil {
		return errors.New("search URL malformed")
	}
	query := searchURL.RawQuery
	searchURL.RawQuery = ""
	if query == "" || protectedURL(searchURL.String()) != nil {
		return errors.New("search endpoint must be explicit and protected")
	}
	if err := protectedURL(gateway); err != nil {
		return err
	}
	fetch := func(payment string) (*http.Response, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("LAYERX-PAYER-ADDRESS", payer)
		if payment != "" {
			request.Header.Set("PAYMENT-SIGNATURE", payment)
		}
		bounded := *client
		bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		return bounded.Do(request)
	}
	response, err := fetch("")
	if err != nil {
		return errors.New("search challenge transport failed")
	}
	io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	response.Body.Close()
	if response.StatusCode != 402 {
		return errors.New("search did not emit payment challenge")
	}
	decoded, err := base64.StdEncoding.DecodeString(response.Header.Get("PAYMENT-REQUIRED"))
	if err != nil {
		return errors.New("payment challenge encoding invalid")
	}
	var challenge struct {
		Version int               `json:"x402Version"`
		Accepts []json.RawMessage `json:"accepts"`
	}
	if json.Unmarshal(decoded, &challenge) != nil || challenge.Version != 2 {
		return errors.New("payment challenge version invalid")
	}
	var selected json.RawMessage
	var offer SearchOffer
	for _, raw := range challenge.Accepts {
		var candidate SearchOffer
		if json.Unmarshal(raw, &candidate) == nil && candidate.Scheme == "metered" && candidate.Extra.LayerX.Currency == "PAX" {
			if selected != nil {
				return errors.New("ambiguous metered PAX offers")
			}
			selected = raw
			offer = candidate
		}
	}
	if selected == nil {
		return errors.New("metered PAX offer absent")
	}
	assets, err := callRPC(ctx, client, gateway, "px_listAssets", []any{}, token, true)
	if err != nil {
		return err
	}
	if err := ValidateSearchPrice(offer, assets); err != nil {
		return err
	}
	correlation, err := randomCorrelation()
	if err != nil {
		return err
	}
	payment, err := json.Marshal(map[string]any{"x402Version": 2, "accepted": selected, "payload": map[string]string{"grantId": strings.TrimPrefix(grant, "0x"), "idempotencyKey": correlation}})
	if err != nil {
		return err
	}
	paid, err := fetch(base64.StdEncoding.EncodeToString(payment))
	if err != nil {
		return errors.New("paid search transport failed")
	}
	io.Copy(io.Discard, io.LimitReader(paid.Body, 1<<20))
	paid.Body.Close()
	if paid.StatusCode != 200 || (response.Header.Get("X-Search-Node") != "" && response.Header.Get("X-Search-Node") != paid.Header.Get("X-Search-Node")) {
		return errors.New("paid search status or selected backend mismatch")
	}
	settlement, err := base64.StdEncoding.DecodeString(paid.Header.Get("PAYMENT-RESPONSE"))
	if err != nil {
		return errors.New("payment response encoding invalid")
	}
	var result struct {
		Success     bool   `json:"success"`
		Transaction string `json:"transaction"`
	}
	if json.Unmarshal(settlement, &result) != nil || !result.Success || !isHexWord(result.Transaction) {
		return errors.New("paid search lacks successful transaction evidence")
	}
	_, err = fmt.Fprintf(output, "pass paid-search asset=%s amount=%s currency=PAX payment-response=success node=%s\n", offer.Asset, offer.Amount, paid.Header.Get("X-Search-Node"))
	return err
}
