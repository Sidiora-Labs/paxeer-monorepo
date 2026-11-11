package evmrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

// geth deliberately accepts omitted/null params for zero-argument callbacks.
// Receipt-key discovery checks governed state before requiring an explicit [].
// px_listAssets allows cursor omission but rejects explicit null before pointer decoding.
// Route refusals to private, unregistered names and translate their errors after dispatch.
// The dispatcher still owns batch bounds, IDs, notifications and deny lists.
type pxParamsGuard struct {
	invalidMethod     string
	unavailableMethod string
	receipts          *SequencerReceiptAPI
	assets            bool
	connectionType    ConnectionType
}

func newPxParamsGuard(apis []rpc.API, denyList []string, connectionType ConnectionType) *pxParamsGuard {
	guard := &pxParamsGuard{connectionType: connectionType}
	for _, api := range apis {
		if api.Namespace != "px" {
			continue
		}
		switch service := api.Service.(type) {
		case *receiptKeysAPI:
			if !slices.Contains(denyList, ReceiptKeysMethod) {
				guard.receipts = service.receipts
			}
		case *pxAccountAPI:
			guard.assets = !slices.Contains(denyList, "px_listAssets")
		}
	}
	if guard.receipts == nil && !guard.assets {
		return nil
	}
	guard.invalidMethod = "invalid-params:" + string(rpc.NewID())
	guard.unavailableMethod = "unavailable-keys:" + string(rpc.NewID())
	return guard
}

func (g *pxParamsGuard) request(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	if g == nil {
		return raw, nil
	}
	return rewriteWSMessages(raw, func(message json.RawMessage) (json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(message, &fields) != nil {
			return message, nil
		}
		var method, version string
		if json.Unmarshal(fields["method"], &method) != nil || json.Unmarshal(fields["jsonrpc"], &version) != nil || version != "2.0" {
			return message, nil
		}
		start := time.Now()
		rejectedMethod := g.unavailableMethod
		refusal := pxChainUnavailable("receipt_keys_unavailable")
		switch {
		case method == "px_listAssets" && g.assets:
			var params []json.RawMessage
			if json.Unmarshal(fields["params"], &params) != nil || len(params) == 0 || !bytes.Equal(bytes.TrimSpace(params[0]), []byte("null")) {
				return message, nil
			}
			rejectedMethod, refusal = g.invalidMethod, pxInvalidParams()
		case method == ReceiptKeysMethod && g.receipts != nil:
			if _, err := g.receipts.receiptKeys(); err != nil {
				break
			}
			var params []json.RawMessage
			if json.Unmarshal(fields["params"], &params) == nil && params != nil && len(params) == 0 {
				return message, nil
			}
			rejectedMethod, refusal = g.invalidMethod, pxInvalidParams()
		default:
			return message, nil
		}
		fields["method"], _ = json.Marshal(rejectedMethod)
		recordMetricsWithError(ctx, method, g.connectionType, start, refusal, nil)
		return json.Marshal(fields)
	})
}

func (g *pxParamsGuard) response(raw json.RawMessage) (json.RawMessage, error) {
	if g == nil {
		return raw, nil
	}
	return rewriteWSMessages(raw, func(message json.RawMessage) (json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(message, &fields) != nil {
			return message, nil
		}
		var responseError struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(fields["error"], &responseError) != nil || responseError.Code != -32601 {
			return message, nil
		}
		switch responseError.Message {
		case "the method " + g.invalidMethod + " does not exist/is not available":
			fields["error"] = json.RawMessage(`{"code":-32602,"message":"Invalid params"}`)
		case "the method " + g.unavailableMethod + " does not exist/is not available":
			fields["error"] = json.RawMessage(`{"code":-32003,"message":"Chain read unavailable","data":{"code":"receipt_keys_unavailable"}}`)
		default:
			return message, nil
		}
		return json.Marshal(fields)
	})
}

func wrapPxParamsHTTP(inner http.Handler, guard *pxParamsGuard) http.Handler {
	if guard == nil {
		return inner
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.ContentLength > paxLegacyHTTPMaxBody {
			inner.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, paxLegacyHTTPMaxBody+1))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		originalBody := r.Body
		defer originalBody.Close()
		r = r.Clone(r.Context())
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), originalBody))
		if len(body) > paxLegacyHTTPMaxBody {
			inner.ServeHTTP(w, r)
			return
		}
		rewritten, err := guard.request(r.Context(), body)
		if err != nil || bytes.Equal(body, rewritten) {
			inner.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(rewritten))
		r.ContentLength = int64(len(rewritten))
		response := httptest.NewRecorder()
		inner.ServeHTTP(response, r)
		answer := response.Body.Bytes()
		if len(answer) > 0 {
			var err error
			answer, err = guard.response(answer)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		copyHTTPHeader(w.Header(), response.Header())
		w.Header().Del("Content-Length")
		w.WriteHeader(response.Code)
		_, _ = w.Write(answer)
	})
}
