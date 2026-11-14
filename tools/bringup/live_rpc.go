package bringup

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
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
	"os"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
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
	Scheme            string `json:"scheme"`
	Network           string `json:"network"`
	Amount            string `json:"amount"`
	Asset             string `json:"asset"`
	PayTo             string `json:"payTo"`
	MaxTimeoutSeconds uint64 `json:"maxTimeoutSeconds"`
	Extra             struct {
		LayerX struct {
			Currency string `json:"currency"`
			Payer    string `json:"payer"`
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
	if offer.Scheme != "exact" || offer.Network != "eip155:125" || offer.Extra.LayerX.Currency != "PAX" {
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

const searchLedgerPrecompile = "0x000000000000000000000000000000000000101a"

func readSearchPayerKey(path string) (*ecdsa.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("search payer key file required")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("search payer key file refused")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("search payer key file unreadable")
	}
	text := strings.TrimSpace(string(raw))
	if len(text) != 64 {
		return nil, errors.New("search payer key encoding invalid")
	}
	key, err := gethcrypto.HexToECDSA(text)
	if err != nil {
		return nil, errors.New("search payer key encoding invalid")
	}
	return key, nil
}

func searchPayerAccount(address common.Address) string {
	var owner [32]byte
	copy(owner[12:], address.Bytes())
	name := "agent:did:layerx:" + hex.EncodeToString(owner[:]) + ":main"
	hash := sha256.New()
	hash.Write([]byte("LX:ACCOUNT:v1"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(name)))
	hash.Write(length[:])
	hash.Write([]byte(name))
	return hex.EncodeToString(hash.Sum(nil))
}

func searchWord(value, name string) ([]byte, error) {
	text := strings.TrimPrefix(value, "0x")
	decoded, err := hex.DecodeString(text)
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("%s encoding invalid", name)
	}
	return decoded, nil
}

func searchAmount(value string) (*big.Int, error) {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok || amount.Sign() < 0 || amount.BitLen() > 128 {
		return nil, errors.New("search offer amount invalid")
	}
	return amount, nil
}

func searchPayExactData(offer SearchOffer, payer common.Address, reference []byte, expires uint64) ([]byte, error) {
	asset, err := searchWord(offer.Asset, "search offer asset")
	if err != nil {
		return nil, err
	}
	payTo, err := searchWord(offer.PayTo, "search offer payee")
	if err != nil {
		return nil, err
	}
	amount, err := searchAmount(offer.Amount)
	if err != nil {
		return nil, err
	}
	selector := gethcrypto.Keccak256([]byte("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"))[:4]
	data := make([]byte, 4+32*6)
	copy(data, selector)
	payerAccount, err := searchWord("0x"+searchPayerAccount(payer), "search payer account")
	if err != nil {
		return nil, err
	}
	copy(data[4:36], payerAccount)
	copy(data[36:68], payTo)
	copy(data[68:100], asset)
	amount.FillBytes(data[116:132])
	binary.BigEndian.PutUint64(data[156:164], expires)
	copy(data[164:196], reference)
	return data, nil
}

type searchReceipt struct {
	Rung    string `json:"rung"`
	TxHash  string `json:"txHash"`
	Status  string `json:"status"`
	Receipt string `json:"receipt"`
}

var errSearchReceiptPending = errors.New("search payment receipt pending")

func readSearchReceipt(raw json.RawMessage, hash string) ([]byte, error) {
	var receipt searchReceipt
	if json.Unmarshal(raw, &receipt) != nil || !strings.EqualFold(receipt.TxHash, hash) {
		return nil, errors.New("search payment receipt unavailable")
	}
	if receipt.Rung == "submitted" {
		return nil, errSearchReceiptPending
	}
	if (receipt.Rung != "receipted" && receipt.Rung != "committed") || receipt.Status != "0x1" {
		return nil, errors.New("search payment receipt unavailable")
	}
	encoded := strings.TrimPrefix(receipt.Receipt, "0x")
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) == 0 {
		return nil, errors.New("search payment receipt malformed")
	}
	return decoded, nil
}

func CheckSearchPayment(ctx context.Context, client *http.Client, endpoint, gateway, node, keyFile, payerAddress, token string, output io.Writer) error {
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
	if err := protectedURL(node); err != nil {
		return err
	}
	key, err := readSearchPayerKey(keyFile)
	if err != nil {
		return err
	}
	payer := gethcrypto.PubkeyToAddress(key.PublicKey)
	if payerAddress != "" && (!common.IsHexAddress(payerAddress) || !strings.EqualFold(common.HexToAddress(payerAddress).Hex(), payer.Hex())) {
		return errors.New("configured search payer address differs from key")
	}
	fetch := func(payment string) (*http.Response, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("LAYERX-PAYER-ADDRESS", payer.Hex())
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
		if json.Unmarshal(raw, &candidate) == nil && candidate.Scheme == "exact" && candidate.Network == "eip155:125" && candidate.Extra.LayerX.Currency == "PAX" {
			if selected != nil {
				return errors.New("ambiguous exact PAX offers")
			}
			selected = raw
			offer = candidate
		}
	}
	if selected == nil {
		return errors.New("exact PAX offer absent")
	}
	assets, err := callRPC(ctx, client, gateway, "px_listAssets", []any{}, token, true)
	if err != nil {
		return err
	}
	if err := ValidateSearchPrice(offer, assets); err != nil {
		return err
	}
	if offer.MaxTimeoutSeconds == 0 || offer.MaxTimeoutSeconds > 3600 {
		return errors.New("search offer timeout invalid")
	}
	if offer.Extra.LayerX.Payer != "" && !strings.EqualFold(strings.TrimPrefix(offer.Extra.LayerX.Payer, "0x"), searchPayerAccount(payer)) {
		return errors.New("search offer payer account mismatch")
	}
	correlation, err := randomCorrelation()
	if err != nil {
		return err
	}
	expires := uint64(time.Now().Add(time.Duration(offer.MaxTimeoutSeconds)*time.Second - time.Millisecond).UnixMilli())
	reference, err := hex.DecodeString(correlation)
	if err != nil || len(reference) != 32 {
		return errors.New("search payment reference generation failed")
	}
	data, err := searchPayExactData(offer, payer, reference, expires)
	if err != nil {
		return err
	}
	chain, err := callRPC(ctx, client, node, "eth_chainId", []any{}, "", false)
	if err != nil || string(chain) != `"0x7d"` {
		return errors.New("search node chain id mismatch")
	}
	nonceRaw, err := callRPC(ctx, client, node, "eth_getTransactionCount", []any{payer.Hex(), "pending"}, "", false)
	if err != nil {
		return err
	}
	var nonceText string
	if json.Unmarshal(nonceRaw, &nonceText) != nil {
		return errors.New("search payer nonce unreadable")
	}
	nonce, err := quantity(nonceText)
	if err != nil || !nonce.IsUint64() {
		return errors.New("search payer nonce invalid")
	}
	gasPriceRaw, err := callRPC(ctx, client, node, "eth_gasPrice", []any{}, "", false)
	if err != nil {
		return err
	}
	var gasPriceText string
	if json.Unmarshal(gasPriceRaw, &gasPriceText) != nil {
		return errors.New("search gas price unreadable")
	}
	gasPrice, err := quantity(gasPriceText)
	if err != nil {
		return err
	}
	to := common.HexToAddress(searchLedgerPrecompile)
	estimateRaw, err := callRPC(ctx, client, node, "eth_estimateGas", []any{map[string]string{"from": payer.Hex(), "to": to.Hex(), "data": "0x" + hex.EncodeToString(data), "value": "0x0"}}, "", false)
	if err != nil {
		return err
	}
	var estimateText string
	if json.Unmarshal(estimateRaw, &estimateText) != nil {
		return errors.New("search gas estimate unreadable")
	}
	estimate, err := quantity(estimateText)
	if err != nil || !estimate.IsUint64() || estimate.Sign() == 0 {
		return errors.New("search gas estimate invalid")
	}
	tx := types.NewTx(&types.LegacyTx{Nonce: nonce.Uint64(), GasPrice: gasPrice, Gas: estimate.Uint64(), To: &to, Value: new(big.Int), Data: data})
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(125)), key)
	if err != nil {
		return errors.New("search payment signing failed")
	}
	rawTransaction, err := signed.MarshalBinary()
	if err != nil {
		return errors.New("search payment serialization failed")
	}
	acceptedHash, err := callRPC(ctx, client, node, "eth_sendRawTransaction", []string{"0x" + hex.EncodeToString(rawTransaction)}, "", false)
	if err != nil {
		return err
	}
	var hash string
	if json.Unmarshal(acceptedHash, &hash) != nil || !strings.EqualFold(hash, signed.Hash().Hex()) {
		return errors.New("search node accepted a different transaction")
	}
	deadline := time.NewTimer(time.Duration(offer.MaxTimeoutSeconds) * time.Second)
	defer deadline.Stop()
	var receiptBytes []byte
	for receiptBytes == nil {
		raw, callErr := callRPC(ctx, client, gateway, "pax_sequencerReceipt", []string{signed.Hash().Hex()}, token, true)
		if callErr == nil {
			receiptBytes, err = readSearchReceipt(raw, signed.Hash().Hex())
			if err != nil && !errors.Is(err, errSearchReceiptPending) {
				return err
			}
		}
		if receiptBytes != nil {
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("search payment receipt deadline exceeded")
		case <-deadline.C:
			return errors.New("search payment receipt deadline exceeded")
		case <-time.After(100 * time.Millisecond):
		}
	}
	receipt, err := codec.DecodeReceipt(receiptBytes)
	if err != nil {
		return errors.New("search payment receipt refused")
	}
	digest := receipt.Digest()
	payment, err := json.Marshal(map[string]any{"x402Version": 2, "accepted": selected, "payload": map[string]string{"txHash": signed.Hash().Hex(), "receiptDigest": hex.EncodeToString(digest[:]), "verificationLevel": "receipted", "idempotencyKey": correlation}})
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
	if json.Unmarshal(settlement, &result) != nil || !result.Success || !strings.EqualFold(result.Transaction, signed.Hash().Hex()) {
		return errors.New("paid search lacks successful transaction evidence")
	}
	_, err = fmt.Fprintf(output, "pass paid-search asset=%s amount=%s currency=PAX payment-response=success node=%s transaction=%s\n", offer.Asset, offer.Amount, paid.Header.Get("X-Search-Node"), signed.Hash().Hex())
	return err
}
