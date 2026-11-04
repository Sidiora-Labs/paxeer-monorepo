package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/snikch/goodman/transaction"

	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	jsonclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Fixtures are application-specific public inputs, not signing credentials.
// Transactions must already be signed by accounts funded on the target chain,
// in nonce order: Seed first, then one distinct transaction per Tx case.
type Fixtures struct {
	Seed         types.Tx   `json:"seed"`
	Transactions []types.Tx `json:"transactions"`
	Query        struct {
		Path  string `json:"path"`
		Data  []byte `json:"data"`
		Value []byte `json:"value"`
	} `json:"query"`
	Evidence    coretypes.RequestBroadcastEvidence `json:"evidence"`
	Unsupported map[string]rpctypes.RPCError       `json:"unsupported"`
}

type preparedCase struct {
	method string
	tx     types.Tx
	post   bool
	prove  bool
}

type HooksRunner struct {
	mu       sync.Mutex
	client   *jsonclient.Client
	fixtures Fixtures
	seedHash []byte
	cases    map[string]preparedCase
	ready    bool
}

func newHooksRunner(client *jsonclient.Client, fixtures Fixtures) (*HooksRunner, error) {
	if client == nil || len(fixtures.Seed) == 0 {
		return nil, errors.New("RPC client and funded seed transaction are required")
	}
	seen := map[string]bool{string(fixtures.Seed): true}
	for _, tx := range fixtures.Transactions {
		if len(tx) == 0 || seen[string(tx)] {
			return nil, errors.New("transaction fixtures must be nonempty and distinct")
		}
		seen[string(tx)] = true
	}
	for method, want := range fixtures.Unsupported {
		if !repairedMethod(method) {
			return nil, fmt.Errorf("unsupported declaration for unhandled method %q", method)
		}
		if want.Code != int(rpctypes.CodeMethodNotFound) &&
			!(method == "broadcast_evidence" && want.Code == int(rpctypes.CodeInternalError) && want.Data == coretypes.ErrEvidenceNotAccepted.Error()) {
			return nil, fmt.Errorf("%s: only method-unavailable or Autobahn evidence refusal may be declared unsupported", method)
		}
		if want.Message != rpctypes.ErrorCode(want.Code).String() {
			return nil, fmt.Errorf("%s: invalid RPC error message", method)
		}
	}
	return &HooksRunner{client: client, fixtures: fixtures, cases: make(map[string]preparedCase)}, nil
}

func repairedMethod(method string) bool {
	switch method {
	case "broadcast_tx", "broadcast_tx_sync", "broadcast_tx_async", "broadcast_tx_commit", "check_tx", "tx", "tx_search", "abci_query", "broadcast_evidence":
		return true
	}
	return false
}

func transactionMethod(t *transaction.Transaction) (string, error) {
	if t.Request == nil {
		return "", errors.New("contract transaction has no request")
	}
	u, err := url.Parse(t.Request.URI)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(u.Path, "/"), nil
}

func transactionKey(t *transaction.Transaction) string {
	if t.Id != "" {
		return t.Id
	}
	return t.Name
}

func (h *HooksRunner) RunBeforeAll(args []*transaction.Transaction, reply *[]*transaction.Transaction) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*reply = args
	if h.ready {
		return errors.New("contract setup already ran")
	}
	next := 0
	for _, t := range args {
		if t == nil {
			return errors.New("nil contract transaction")
		}
		method, err := transactionMethod(t)
		if err != nil {
			return err
		}
		if !repairedMethod(method) {
			continue
		}
		if t.Skip {
			return fmt.Errorf("%s: repaired contract case is marked skipped", method)
		}
		key := transactionKey(t)
		if key == "" {
			return errors.New("contract case requires an id or name")
		}
		if _, exists := h.cases[key]; exists {
			return fmt.Errorf("duplicate contract case %q", key)
		}
		u, err := url.Parse(t.Request.URI)
		if err != nil {
			return err
		}
		c := preparedCase{method: method, prove: u.Query().Get("prove") == "true"}
		switch method {
		case "broadcast_tx", "broadcast_tx_sync", "broadcast_tx_async", "broadcast_tx_commit", "check_tx":
			if next == len(h.fixtures.Transactions) {
				return errors.New("not enough funded transaction fixtures for Tx cases")
			}
			c.tx = h.fixtures.Transactions[next]
			next++
		case "tx":
			c.post = true
		case "abci_query":
			c.post = true
			if h.fixtures.Query.Path == "" || len(h.fixtures.Query.Data) == 0 {
				return errors.New("query path and data are required")
			}
		case "broadcast_evidence":
			if h.fixtures.Evidence.Evidence == nil {
				return errors.New("signed evidence fixture is required")
			}
			if err := h.fixtures.Evidence.Evidence.ValidateBasic(); err != nil {
				return fmt.Errorf("evidence fixture: %w", err)
			}
			c.post = true
		}
		if _, unsupported := h.fixtures.Unsupported[method]; unsupported {
			c.post = true
		}
		h.cases[key] = c
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var seed coretypes.ResultBroadcastTxCommit
	if err := h.client.Call(ctx, "broadcast_tx_commit", coretypes.RequestBroadcastTx{Tx: h.fixtures.Seed}, &seed); err != nil {
		return fmt.Errorf("submit funded seed: %w", err)
	}
	if seed.CheckTx.Code != 0 || seed.TxResult.Code != 0 || seed.Height <= 0 || !bytes.Equal(seed.Hash, h.fixtures.Seed.Hash().Bytes()) {
		return errors.New("seed must execute successfully, commit and return its transaction hash")
	}
	h.seedHash = bytes.Clone(seed.Hash)
	h.ready = true
	return nil
}

func (h *HooksRunner) RunBeforeEach(args transaction.Transaction, reply *transaction.Transaction) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*reply = args
	method, err := transactionMethod(reply)
	if err != nil {
		return err
	}
	if !repairedMethod(method) {
		return nil
	}
	if !h.ready {
		return errors.New("contract setup has not committed the seed")
	}
	c, ok := h.cases[transactionKey(reply)]
	if !ok || c.method != method {
		return errors.New("contract case was not prepared")
	}
	if reply.Skip {
		return errors.New("prepared contract case may not be skipped")
	}
	u, err := url.Parse(reply.Request.URI)
	if err != nil {
		return err
	}
	q := u.Query()
	var params any
	switch method {
	case "broadcast_tx", "broadcast_tx_sync", "broadcast_tx_async", "broadcast_tx_commit", "check_tx":
		q.Set("tx", "0x"+hex.EncodeToString(c.tx))
		params = coretypes.RequestBroadcastTx{Tx: c.tx}
	case "tx":
		q.Set("hash", "0x"+hex.EncodeToString(h.seedHash))
		params = coretypes.RequestTx{Hash: h.seedHash, Prove: q.Get("prove") == "true"}
	case "tx_search":
		query := fmt.Sprintf("tx.hash='%X'", h.seedHash)
		q.Set("query", query)
		params = coretypes.RequestTxSearch{Query: query}
	case "abci_query":
		q.Set("path", string(mustJSON(h.fixtures.Query.Path)))
		q.Set("data", "0x"+hex.EncodeToString(h.fixtures.Query.Data))
		q.Set("height", "0")
		params = coretypes.RequestABCIQuery{Path: h.fixtures.Query.Path, Data: h.fixtures.Query.Data, Prove: q.Get("prove") == "true"}
	case "broadcast_evidence":
		params = h.fixtures.Evidence
	}
	if c.post {
		req := rpctypes.NewRequest(1151)
		if err := req.SetMethodAndParams(method, params); err != nil {
			return err
		}
		body, err := json.Marshal(req)
		if err != nil {
			return err
		}
		u.Path, u.RawQuery = "/", ""
		reply.Request.Method = "POST"
		reply.Request.Body = string(body)
		if reply.Request.Headers == nil {
			reply.Request.Headers = make(map[string]interface{})
		}
		reply.Request.Headers["Content-Type"] = "application/json"
	} else {
		u.RawQuery = q.Encode()
	}
	reply.Request.URI = u.String()
	reply.FullPath = u.RequestURI()
	return nil
}

func mustJSON(s string) []byte {
	b, _ := json.Marshal(s)
	return b
}

func (h *HooksRunner) RunBeforeEachValidation(args transaction.Transaction, reply *transaction.Transaction) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*reply = args
	c, ok := h.cases[transactionKey(reply)]
	if !ok {
		return nil
	}
	if reply.Skip || reply.Real == nil || reply.Real.StatusCode != 200 {
		return errors.New("prepared contract case must execute and return HTTP 200")
	}
	body := []byte(reply.Real.Body)
	var rpcErr *rpctypes.RPCError
	if c.post {
		var envelope struct {
			Version string             `json:"jsonrpc"`
			ID      int                `json:"id"`
			Result  json.RawMessage    `json:"result"`
			Error   *rpctypes.RPCError `json:"error"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return err
		}
		if envelope.Version != "2.0" || envelope.ID != 1151 {
			return errors.New("invalid prepared JSON-RPC envelope")
		}
		rpcErr = envelope.Error
		if rpcErr != nil {
			if len(envelope.Result) != 0 && string(envelope.Result) != "null" {
				return errors.New("RPC response contains both result and error")
			}
			body, _ = json.Marshal(rpcErr)
		} else {
			body = envelope.Result
		}
	} else {
		var e rpctypes.RPCError
		if err := json.Unmarshal(body, &e); err != nil {
			return err
		}
		if e.Code != 0 {
			rpcErr = &e
		}
	}
	if want, unsupported := h.fixtures.Unsupported[c.method]; unsupported {
		if rpcErr == nil || *rpcErr != want {
			return fmt.Errorf("%s must return its explicitly declared unsupported RPC error", c.method)
		}
	} else {
		if rpcErr != nil {
			return fmt.Errorf("%s unexpectedly failed: %w", c.method, rpcErr)
		}
		if err := h.assertResult(c, body); err != nil {
			return err
		}
	}
	if c.post {
		// The OpenAPI URI schemas describe bare results/errors. Validate the
		// POST envelope above before passing its payload to the same assertions.
		reply.Real.Body = string(body)
	}
	return nil
}

func (h *HooksRunner) assertResult(c preparedCase, body []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	if len(fields) == 0 {
		return errors.New("RPC success must contain a nonempty result object")
	}
	switch c.method {
	case "broadcast_tx", "broadcast_tx_sync", "broadcast_tx_async":
		var r coretypes.ResultBroadcastTx
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if r.Code != 0 || !bytes.Equal(r.Hash, c.tx.Hash().Bytes()) {
			return errors.New("broadcast must accept the prepared transaction and return its hash")
		}
	case "broadcast_tx_commit":
		var r coretypes.ResultBroadcastTxCommit
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if r.CheckTx.Code != 0 || r.TxResult.Code != 0 || r.Height <= 0 || !bytes.Equal(r.Hash, c.tx.Hash().Bytes()) {
			return errors.New("broadcast must successfully commit the prepared transaction")
		}
	case "check_tx":
		var r coretypes.ResultCheckTx
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if _, present := fields["code"]; !present {
			return errors.New("CheckTx response is missing its code")
		}
		if r.ResponseCheckTx.Code != 0 {
			return errors.New("CheckTx must accept the funded transaction")
		}
	case "tx":
		var r coretypes.ResultTx
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if r.Height <= 0 || r.TxResult.Code != 0 || !bytes.Equal(r.Hash, h.seedHash) || !bytes.Equal(r.Tx, h.fixtures.Seed) {
			return errors.New("lookup must return the committed seed transaction")
		}
		if c.prove {
			if !bytes.Equal(r.Proof.Data, h.fixtures.Seed) {
				return errors.New("lookup proof must contain the committed seed transaction")
			}
			if err := r.Proof.Validate(r.Proof.RootHash); err != nil {
				return err
			}
		}
	case "tx_search":
		var r coretypes.ResultTxSearch
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if r.TotalCount != 1 || len(r.Txs) != 1 || r.Txs[0] == nil {
			return errors.New("search must find exactly the committed seed")
		}
		encoded, err := tmjson.Marshal(r.Txs[0])
		if err != nil {
			return err
		}
		return h.assertResult(preparedCase{method: "tx", prove: c.prove}, encoded)
	case "abci_query":
		var r coretypes.ResultABCIQuery
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if r.Response.Code != 0 || !bytes.Equal(r.Response.Key, h.fixtures.Query.Data) || !bytes.Equal(r.Response.Value, h.fixtures.Query.Value) {
			return errors.New("ABCI query must return the prepared key and value successfully")
		}
	case "broadcast_evidence":
		var r coretypes.ResultBroadcastEvidence
		if err := tmjson.Unmarshal(body, &r); err != nil {
			return err
		}
		if !bytes.Equal(r.Hash, h.fixtures.Evidence.Evidence.Hash()) {
			return errors.New("evidence broadcast must return the signed evidence hash")
		}
	}
	return nil
}

func (h *HooksRunner) RunBefore(args transaction.Transaction, reply *transaction.Transaction) error {
	*reply = args
	return nil
}
func (h *HooksRunner) RunBeforeValidation(args transaction.Transaction, reply *transaction.Transaction) error {
	*reply = args
	return nil
}
func (h *HooksRunner) RunAfter(args transaction.Transaction, reply *transaction.Transaction) error {
	*reply = args
	return nil
}
func (h *HooksRunner) RunAfterEach(args transaction.Transaction, reply *transaction.Transaction) error {
	*reply = args
	return nil
}
func (h *HooksRunner) RunAfterAll(args []*transaction.Transaction, reply *[]*transaction.Transaction) error {
	*reply = args
	return nil
}
