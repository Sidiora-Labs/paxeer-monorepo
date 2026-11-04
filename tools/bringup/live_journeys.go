package bringup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type PayExactFacts struct {
	Sender                     common.Address
	From, To, Asset, Reference string
	Amount                     string
	Expires                    uint64
}

func InspectPayExact(raw []byte, now time.Time) (*types.Transaction, PayExactFacts, error) {
	tx, err := DecodeSignedTransaction(raw)
	if err != nil {
		return nil, PayExactFacts{}, err
	}
	target := common.HexToAddress("0x000000000000000000000000000000000000101a")
	data := tx.Data()
	if tx.To() == nil || *tx.To() != target || tx.Value().Sign() != 0 || len(data) != 196 || hex.EncodeToString(data[:4]) != "78ea11db" || new(big.Int).SetBytes(data[100:116]).Sign() != 0 || new(big.Int).SetBytes(data[132:156]).Sign() != 0 {
		return nil, PayExactFacts{}, errors.New("interop requires canonical ledger payExact transaction")
	}
	sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, PayExactFacts{}, errors.New("payExact signature invalid")
	}
	facts := PayExactFacts{sender, hex.EncodeToString(data[4:36]), hex.EncodeToString(data[36:68]), hex.EncodeToString(data[68:100]), hex.EncodeToString(data[164:196]), new(big.Int).SetBytes(data[116:132]).String(), new(big.Int).SetBytes(data[156:164]).Uint64()}
	if facts.Amount == "0" || facts.Expires <= uint64(now.UnixMilli()) || facts.From == strings.Repeat("0", 64) || facts.To == strings.Repeat("0", 64) || facts.Reference == strings.Repeat("0", 64) {
		return nil, facts, errors.New("payExact amount, accounts, reference or validity invalid")
	}
	return tx, facts, nil
}

func CheckInteropPayment(ctx context.Context, client *http.Client, origin, gateway, node, token string, raw []byte, now time.Time, output io.Writer) error {
	for _, endpoint := range []string{origin, gateway, node} {
		if err := protectedURL(endpoint); err != nil {
			return err
		}
	}
	tx, facts, err := InspectPayExact(raw, now)
	if err != nil {
		return err
	}
	account, err := callRPC(ctx, client, gateway, "px_getAccount", []string{facts.Sender.Hex()}, "", true)
	if err != nil {
		return err
	}
	if err := validateRouteResult("px_getAccount", account, facts.Sender.Hex()); err != nil {
		return err
	}
	var state struct {
		Paxeer struct {
			Nonce string `json:"nonce"`
		} `json:"paxeer"`
	}
	if json.Unmarshal(account, &state) != nil {
		return errors.New("account state unreadable")
	}
	nonce, err := quantity(state.Paxeer.Nonce)
	if err != nil || !nonce.IsUint64() || nonce.Uint64() != tx.Nonce() {
		return errors.New("signed transaction does not use current chain nonce")
	}
	balances, err := callRPC(ctx, client, gateway, "px_getBalances", []string{facts.Sender.Hex()}, "", true)
	if err != nil {
		return err
	}
	var balance struct {
		Height  *uint64 `json:"height"`
		Account struct {
			EVM string `json:"evm_address"`
		} `json:"account"`
		Balances *[]struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"balances"`
	}
	if json.Unmarshal(balances, &balance) != nil || balance.Height == nil || !strings.EqualFold(balance.Account.EVM, facts.Sender.Hex()) || balance.Balances == nil {
		return errors.New("hosted balance evidence unavailable")
	}
	for _, coin := range *balance.Balances {
		amount, ok := new(big.Int).SetString(coin.Amount, 10)
		if !ok || amount.Sign() < 0 || coin.Denom == "" {
			return errors.New("hosted balance encoding invalid")
		}
	}
	authorization := ""
	if token != "" {
		authorization = "LayerX-Key " + token
	}
	code, doc, _, err := jsonRequest(ctx, client, "GET", strings.TrimRight(origin, "/")+"/v1/http/x402/supported", authorization, "", nil)
	if err != nil {
		return err
	}
	var supported struct {
		Result struct {
			Supported struct {
				Kinds []struct {
					Scheme  string `json:"scheme"`
					Network string `json:"network"`
				} `json:"kinds"`
			} `json:"supported"`
		} `json:"result"`
	}
	if code != 200 || json.Unmarshal(doc, &supported) != nil {
		return errors.New("interop supported response invalid")
	}
	exact := false
	for _, kind := range supported.Result.Supported.Kinds {
		if kind.Scheme == "exact" && kind.Network == "eip155:125" {
			exact = true
		}
	}
	if !exact {
		return errors.New("chain 125 exact scheme unavailable")
	}
	correlation, err := randomCorrelation()
	if err != nil {
		return err
	}
	requirements := map[string]any{"scheme": "exact", "network": "eip155:125", "amount": facts.Amount, "asset": facts.Asset, "payTo": facts.To, "maxTimeoutSeconds": 120}
	offer := map[string]any{"x402Version": 2, "resource": map[string]string{"url": strings.TrimRight(origin, "/") + "/readyz"}, "accepts": []any{requirements}}
	code, doc, _, err = jsonRequest(ctx, client, "POST", strings.TrimRight(origin, "/")+"/v1/http/x402/seller/offer", authorization, correlation+"-offer", offer)
	if err != nil {
		return err
	}
	var offered struct {
		Result struct {
			Status  int             `json:"status"`
			Header  string          `json:"payment_required_header"`
			Payment json.RawMessage `json:"payment_required"`
		} `json:"result"`
	}
	if code != 200 || json.Unmarshal(doc, &offered) != nil || offered.Result.Status != 402 || offered.Result.Header == "" || len(offered.Result.Payment) == 0 {
		return errors.New("seller did not emit current payment requirements")
	}
	var offerCheck struct {
		Version int               `json:"x402Version"`
		Accepts []json.RawMessage `json:"accepts"`
	}
	header, decodeErr := base64.StdEncoding.DecodeString(offered.Result.Header)
	if decodeErr != nil || !equalJSON(header, offered.Result.Payment) || json.Unmarshal(offered.Result.Payment, &offerCheck) != nil || offerCheck.Version != 2 || len(offerCheck.Accepts) != 1 {
		return errors.New("seller header and offer disagree")
	}
	wantRequirements, _ := json.Marshal(requirements)
	if !equalJSON(offerCheck.Accepts[0], wantRequirements) {
		return errors.New("seller changed signed transfer terms")
	}
	build := map[string]any{"payment_required": offered.Result.Payment, "scheme_payload": map[string]string{"layerxTransaction": "0x" + hex.EncodeToString(raw), "layerxIdempotencyKey": facts.Reference}}
	code, doc, _, err = jsonRequest(ctx, client, "POST", strings.TrimRight(origin, "/")+"/v1/http/x402/buyer/build", authorization, correlation+"-build", build)
	if err != nil {
		return err
	}
	var built struct {
		Result struct {
			Header  string          `json:"payment_header"`
			Payload json.RawMessage `json:"payment_payload"`
		} `json:"result"`
	}
	if code != 200 || json.Unmarshal(doc, &built) != nil || built.Result.Header == "" || len(built.Result.Payload) == 0 {
		return errors.New("buyer did not emit payment signature")
	}
	var paymentCheck struct {
		Version  int             `json:"x402Version"`
		Accepted json.RawMessage `json:"accepted"`
		Payload  struct {
			Transaction string `json:"layerxTransaction"`
			Reference   string `json:"layerxIdempotencyKey"`
		} `json:"payload"`
	}
	header, decodeErr = base64.StdEncoding.DecodeString(built.Result.Header)
	if decodeErr != nil || !equalJSON(header, built.Result.Payload) || json.Unmarshal(built.Result.Payload, &paymentCheck) != nil || paymentCheck.Version != 2 || !equalJSON(paymentCheck.Accepted, wantRequirements) || paymentCheck.Payload.Transaction != "0x"+hex.EncodeToString(raw) || paymentCheck.Payload.Reference != facts.Reference {
		return errors.New("buyer changed signed transfer payload")
	}
	settle := map[string]any{"x402Version": 2, "paymentPayload": built.Result.Payload, "paymentRequirements": requirements}
	code, doc, _, err = jsonRequest(ctx, client, "POST", strings.TrimRight(origin, "/")+"/v1/http/x402/settle", authorization, correlation+"-settle", settle)
	if err != nil {
		return err
	}
	for code == 202 {
		var pending struct {
			Operation string `json:"operation"`
		}
		if json.Unmarshal(doc, &pending) != nil || !safeIdentity(pending.Operation) {
			return errors.New("settlement pending operation missing")
		}
		select {
		case <-ctx.Done():
			return errors.New("settlement deadline exceeded")
		case <-time.After(time.Second):
		}
		code, doc, _, err = jsonRequest(ctx, client, "GET", strings.TrimRight(origin, "/")+"/v1/operations/"+pending.Operation, authorization, "", nil)
		if err != nil {
			return err
		}
	}
	var settled struct {
		Result struct {
			Success     bool   `json:"success"`
			Transaction string `json:"transaction"`
			Extensions  struct {
				LayerX struct {
					Receipt string `json:"signedReceipt"`
					Digest  string `json:"receiptDigest"`
				} `json:"layerx"`
			} `json:"extensions"`
		} `json:"result"`
	}
	if code != 200 || json.Unmarshal(doc, &settled) != nil || !settled.Result.Success || settled.Result.Transaction != tx.Hash().Hex() || settled.Result.Extensions.LayerX.Receipt == "" || !isHexWord(settled.Result.Extensions.LayerX.Digest) {
		return errors.New("interop settlement lacks matching signed chain receipt")
	}
	evidence, err := waitCommittedTransaction(ctx, client, node, tx.Hash().Hex())
	if err != nil {
		return err
	}
	var native struct {
		Receipt string `json:"receipt"`
	}
	if json.Unmarshal(evidence.Receipt, &native) != nil || native.Receipt != settled.Result.Extensions.LayerX.Receipt {
		return errors.New("settlement receipt differs from committed chain receipt")
	}
	receiptBytes, err := hex.DecodeString(strings.TrimPrefix(native.Receipt, "0x"))
	if err != nil {
		return errors.New("settlement receipt encoding invalid")
	}
	receipt, err := codec.DecodeReceipt(receiptBytes)
	if err != nil {
		return err
	}
	digest := receipt.Digest()
	if !strings.EqualFold(settled.Result.Extensions.LayerX.Digest, "0x"+hex.EncodeToString(digest[:])) {
		return errors.New("settlement receipt digest mismatch")
	}
	_, err = fmt.Fprintf(output, "pass interop-transfer transaction=%s from=%s to=%s asset=%s amount=%s committed-height=%s\n", evidence.Hash, facts.From, facts.To, facts.Asset, facts.Amount, evidence.Height)
	return err
}

func CheckDevelopersDelivery(ctx context.Context, client *http.Client, hooks, receiver, source, token, sourceToken string, request json.RawMessage, attempts int, now func() time.Time, output io.Writer) error {
	if attempts < 1 || attempts > 999 || !json.Valid(request) {
		return errors.New("source request or delivery attempt bound invalid")
	}
	for _, endpoint := range []string{hooks, receiver, source} {
		if err := protectedURL(endpoint); err != nil {
			return err
		}
	}
	correlation, err := randomCorrelation()
	if err != nil {
		return err
	}
	started := uint64(now().Unix())
	code, body, _, err := jsonRequest(ctx, client, "POST", strings.TrimRight(hooks, "/")+"/v1/webhooks/endpoints", "Bearer "+token, correlation+"-subscription", map[string]any{"url": receiver, "kinds": []string{"journey", "payment", "approval", "program"}, "minimum_verification": "unverified"})
	if err != nil {
		return err
	}
	var registered struct {
		Endpoint string `json:"endpoint"`
	}
	if code != 201 || json.Unmarshal(body, &registered) != nil || !safeIdentity(registered.Endpoint) {
		return errors.New("webhook subscription registration refused")
	}
	code, body, _, err = jsonRequest(ctx, client, "GET", strings.TrimRight(hooks, "/")+"/v1/webhooks/endpoints", "Bearer "+token, "", nil)
	if err != nil {
		return err
	}
	var listed []struct {
		Endpoint string `json:"endpoint"`
		URL      string `json:"url"`
	}
	if code != 200 || json.Unmarshal(body, &listed) != nil {
		return errors.New("webhook subscription list unavailable")
	}
	found := false
	for _, e := range listed {
		if e.Endpoint == registered.Endpoint && e.URL == receiver {
			found = true
		}
	}
	if !found {
		return errors.New("registered receiver did not list back")
	}
	// The configured request is an authorized real Human intent submission.
	// Its new journey identity binds the source event to this operation key.
	code, body, _, err = jsonRequest(ctx, client, "POST", source, "Bearer "+sourceToken, correlation+"-source", request)
	if err != nil {
		return err
	}
	var submitted struct {
		OK     bool `json:"ok"`
		Result struct {
			Journey string `json:"journey_id"`
		} `json:"result"`
	}
	if (code != 200 && code != 202) || json.Unmarshal(body, &submitted) != nil || !submitted.OK || !safeIdentity(submitted.Result.Journey) {
		return errors.New("authorized source did not create a current journey")
	}
	for n := 0; n < attempts; n++ {
		if n > 0 {
			select {
			case <-ctx.Done():
				return errors.New("fresh delivery deadline exceeded")
			case <-time.After(5 * time.Second):
			}
		}
		code, events, _, err := jsonRequest(ctx, client, "GET", strings.TrimRight(hooks, "/")+"/v1/webhooks/events", "Bearer "+token, "", nil)
		if err != nil || code != 200 {
			continue
		}
		code, deliveries, _, err := jsonRequest(ctx, client, "GET", strings.TrimRight(hooks, "/")+"/v1/webhooks/deliveries", "Bearer "+token, "", nil)
		if err != nil || code != 200 {
			continue
		}
		if ValidateFreshDelivery(strings.NewReader(string(events)), strings.NewReader(string(deliveries)), registered.Endpoint, submitted.Result.Journey, started, uint64(now().Unix()), output) == nil {
			return nil
		}
	}
	return errors.New("no fresh delivery for this source operation")
}

func equalJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	p, _ := json.Marshal(x)
	q, _ := json.Marshal(y)
	return bytes.Equal(p, q)
}
