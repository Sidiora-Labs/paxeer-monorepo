package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/stretchr/testify/require"
)

type repair87Value struct {
	Value string `json:"value"`
}

func repair87HTTPClient(t *testing.T, delivery string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"echo": rpcserver.NewRPCFunc(func(_ context.Context, arg *repair87Value) (*repair87Value, error) {
			return &repair87Value{Value: arg.Value}, nil
		}),
		"refuse": rpcserver.NewRPCFunc(func(_ context.Context) (*repair87Value, error) {
			return nil, &rpctypes.RPCError{
				Code: int(rpctypes.CodeInvalidRequest), Message: "Refused", Data: "request denied",
			}
		}),
	})
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded := httptest.NewRecorder()
		mux.ServeHTTP(recorded, r)
		var replies []rpctypes.RPCResponse
		if err := json.Unmarshal(recorded.Body.Bytes(), &replies); err != nil {
			t.Errorf("decoding real handler responses: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch delivery {
		case "reverse":
			slices.Reverse(replies)
		case "duplicate":
			replies[1] = replies[0]
		case "missing":
			replies = replies[:len(replies)-1]
		case "extra":
			replies = append(replies, replies[0])
		case "unexpected":
			// Retain a genuine response, but give it an unrequested ID.
			var envelope map[string]json.RawMessage
			body, err := json.Marshal(replies[1])
			if err == nil {
				err = json.Unmarshal(body, &envelope)
			}
			if err == nil {
				envelope["id"] = json.RawMessage(`9999`)
				body, err = json.Marshal(envelope)
			}
			if err == nil {
				err = json.Unmarshal(body, &replies[1])
			}
			if err != nil {
				t.Errorf("changing response ID: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			t.Errorf("unknown delivery mode %q", delivery)
			http.Error(w, "unknown delivery mode", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorded.Code)
		if err := json.NewEncoder(w).Encode(replies); err != nil {
			t.Errorf("encoding handler responses: %v", err)
		}
	}))
	t.Cleanup(endpoint.Close)
	c, err := NewWithHTTPClient(endpoint.URL, endpoint.Client())
	require.NoError(t, err)
	return c
}

func TestRepair_87_ReorderedReplies(t *testing.T) {
	c := repair87HTTPClient(t, "reverse")
	// Consume an ID first so correlation cannot rely on zero-based batch indices.
	batch := c.NewRequestBatch()
	require.NoError(t, batch.Call(t.Context(), "echo", repair87Value{Value: "discarded"}, &repair87Value{}))
	require.Equal(t, 1, batch.Clear())
	first, second, third := &repair87Value{}, &repair87Value{}, &repair87Value{}
	for i, result := range []*repair87Value{first, second, third} {
		require.NoError(t, batch.Call(t.Context(), "echo", repair87Value{Value: []string{"first", "second", "third"}[i]}, result))
	}
	results, err := batch.Send(t.Context())
	require.NoError(t, err)
	require.Equal(t, "first", first.Value)
	require.Equal(t, "second", second.Value)
	require.Equal(t, "third", third.Value)
	require.Len(t, results, 3)
	require.Same(t, first, results[0])
	require.Same(t, second, results[1])
	require.Same(t, third, results[2])
	require.Zero(t, batch.Count())
}

func TestRepair_87_DuplicateReplies(t *testing.T) {
	c := repair87HTTPClient(t, "duplicate")
	batch := c.NewRequestBatch()
	first, second := &repair87Value{Value: "untouched first"}, &repair87Value{Value: "untouched second"}
	require.NoError(t, batch.Call(t.Context(), "echo", repair87Value{Value: "first"}, first))
	require.NoError(t, batch.Call(t.Context(), "echo", repair87Value{Value: "second"}, second))
	results, err := batch.Send(t.Context())
	require.Error(t, err)
	require.Nil(t, results)
	require.Equal(t, "untouched first", first.Value)
	require.Equal(t, "untouched second", second.Value)
	require.Zero(t, batch.Count())
}

func TestRepair_87_ExactIDSet(t *testing.T) {
	for _, delivery := range []string{"missing", "extra", "unexpected"} {
		t.Run(delivery, func(t *testing.T) {
			c := repair87HTTPClient(t, delivery)
			batch := c.NewRequestBatch()
			first, second := &repair87Value{Value: "unchanged"}, &repair87Value{Value: "unchanged"}
			require.NoError(t, batch.Call(t.Context(), "echo", repair87Value{Value: "first"}, first))
			require.NoError(t, batch.Call(t.Context(), "echo", repair87Value{Value: "second"}, second))
			results, err := batch.Send(t.Context())
			require.Error(t, err)
			require.Nil(t, results)
			require.Equal(t, "unchanged", first.Value)
			require.Equal(t, "unchanged", second.Value)
		})
	}
}

func TestRepair_87_MixedSuccessesAndRefusals(t *testing.T) {
	for _, methods := range [][]string{
		{"refuse", "echo", "unknown", "echo"},
		{"echo", "refuse", "echo", "unknown"},
	} {
		t.Run(methods[0], func(t *testing.T) {
			c := repair87HTTPClient(t, "reverse")
			batch := c.NewRequestBatch()
			values := make([]*repair87Value, len(methods))
			for i, method := range methods {
				values[i] = &repair87Value{Value: "unchanged"}
				if method == "echo" {
					require.NoError(t, batch.Call(t.Context(), method, repair87Value{Value: "success"}, values[i]))
				} else {
					require.NoError(t, batch.Call(t.Context(), method, nil, values[i]))
				}
			}
			results, err := batch.Send(t.Context())
			require.Error(t, err)
			require.Nil(t, results)
			var rpcErr *rpctypes.RPCError
			require.True(t, errors.As(err, &rpcErr))
			joined, ok := err.(interface{ Unwrap() []error })
			require.True(t, ok)
			failures := joined.Unwrap()
			require.Len(t, failures, 2)
			nextFailure := 0
			for i, method := range methods {
				if method == "echo" {
					require.Equal(t, "success", values[i].Value)
					continue
				}
				require.Equal(t, "unchanged", values[i].Value)
				require.True(t, errors.As(failures[nextFailure], &rpcErr))
				if method == "refuse" {
					require.Equal(t, int(rpctypes.CodeInvalidRequest), rpcErr.Code)
					require.Equal(t, "request denied", rpcErr.Data)
				} else {
					require.Equal(t, int(rpctypes.CodeMethodNotFound), rpcErr.Code)
					require.Equal(t, "method unknown not found", rpcErr.Data)
				}
				nextFailure++
			}
			require.Zero(t, batch.Count())
		})
	}
}
