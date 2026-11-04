package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// PerpsCollateralRPC reads canonical execution views without consulting latest
// or pending state. Its endpoint is supplied by the trusted chain adapter.
type PerpsCollateralRPC struct {
	endpoint string
	client   *http.Client
}

func NewPerpsCollateralRPC(endpoint string) (*PerpsCollateralRPC, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("perps collateral RPC requires an HTTP endpoint")
	}
	return &PerpsCollateralRPC{endpoint: endpoint, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *PerpsCollateralRPC) CallContractAtHash(ctx context.Context, call ethereum.CallMsg, block common.Hash) ([]byte, error) {
	if c == nil || c.client == nil || call.To == nil || block == (common.Hash{}) {
		return nil, errors.New("perps collateral RPC requires a destination and block hash")
	}
	request := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  []any  `json:"params"`
	}{JSONRPC: "2.0", ID: 1, Method: "eth_call", Params: []any{
		map[string]any{"from": call.From, "to": *call.To, "input": hexutil.Bytes(call.Data)},
		map[string]any{"blockHash": block, "requireCanonical": true},
	}}
	if call.Value != nil && call.Value.Sign() != 0 {
		return nil, errors.New("perps collateral execution view is nonpayable")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("perps collateral RPC HTTP status %d", response.StatusCode)
	}
	var result struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      int            `json:"id"`
		Result  *hexutil.Bytes `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil {
		return nil, err
	}
	if len(body) > 16384 {
		return nil, errors.New("perps collateral RPC response exceeds its size bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("perps collateral RPC response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("perps collateral RPC has trailing response data")
	}
	if result.JSONRPC != "2.0" || result.ID != 1 {
		return nil, errors.New("perps collateral RPC response does not match the request")
	}
	if result.Error != nil {
		return nil, fmt.Errorf("perps collateral RPC error %d: %s", result.Error.Code, result.Error.Message)
	}
	if result.Result == nil {
		return nil, errors.New("perps collateral RPC response is missing its result")
	}
	return append([]byte(nil), (*result.Result)...), nil
}
