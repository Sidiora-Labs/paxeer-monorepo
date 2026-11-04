package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func TestRepairM_1314_RegistrySnapshots(t *testing.T) {
	TestRegisterError(t)
}

func TestRepairM_1314_NetworkOptionsVocabulary(t *testing.T) {
	isolateErrorRegistry(t)
	registered := []*Error{
		ErrUnknown, ErrOffline, ErrNetworkNotSupported, ErrCodec,
		ErrInvalidOperation, ErrInvalidTransaction, ErrInvalidAddress,
		ErrInvalidPubkey, ErrInterpreting, ErrInvalidMemo, ErrBadArgument,
		ErrNotFound, ErrInternal, ErrBadGateway, ErrNotImplemented, ErrUnsupportedCurve,
		RegisterError(69, "custom error", true, "registered during initialization"),
	}
	before := snapshotErrorRegistry(t)
	options := &types.NetworkOptionsResponse{Allow: &types.Allow{Errors: SealAndListErrors()}}
	if !registry.sealed {
		t.Fatal("network/options did not seal the registry")
	}
	RegisterError(69, "conflicting custom error", false, "replacement definition")
	RegisterError(1024, "late error", false, "registered after initialization")
	if after := snapshotErrorRegistry(t); after != before {
		t.Fatalf("sealed registry changed: before=%s after=%s", before, after)
	}

	emitted := make(map[string]*Error)
	for _, err := range registered {
		emitted[fmt.Sprint(ToRosetta(err).Code)] = err
	}
	emitted["wrapped"] = WrapError(ErrNotFound, "transaction was not found")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/network/options":
			if err := json.NewEncoder(w).Encode(options); err != nil {
				t.Errorf("encode network options: %v", err)
			}
		case "/network/status":
			var request types.NetworkRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode network request: %v", err)
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			key, _ := request.Metadata["error"].(string)
			err, exists := emitted[key]
			if !exists {
				t.Errorf("unrecognized error request %q", key)
				http.Error(w, "unknown error", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(ToRosetta(err)); err != nil {
				t.Errorf("encode registered error: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	api := client.NewAPIClient(client.NewConfiguration(server.URL, "repair-m1314", server.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	network := &types.NetworkIdentifier{Blockchain: "paxeer", Network: "registry-test"}
	advertised, apiErr, err := api.NetworkAPI.NetworkOptions(ctx, &types.NetworkRequest{NetworkIdentifier: network})
	if err != nil || apiErr != nil || advertised == nil || advertised.Allow == nil {
		t.Fatalf("consume network/options: response=%+v apiError=%+v error=%v", advertised, apiErr, err)
	}
	vocabulary := make(map[int32]*types.Error)
	for _, definition := range advertised.Allow.Errors {
		if _, exists := vocabulary[definition.Code]; exists {
			t.Fatalf("network/options contains duplicate code %d", definition.Code)
		}
		vocabulary[definition.Code] = definition
	}
	if len(vocabulary) != len(registered) {
		t.Fatalf("advertised %d errors, want all %d registered errors", len(vocabulary), len(registered))
	}
	for key, source := range emitted {
		t.Run(key, func(t *testing.T) {
			response, received, err := api.NetworkAPI.NetworkStatus(ctx, &types.NetworkRequest{
				NetworkIdentifier: network,
				Metadata:          map[string]interface{}{"error": key},
			})
			if err == nil || response != nil || received == nil {
				t.Fatalf("consume registered error: response=%+v apiError=%+v error=%v", response, received, err)
			}
			if !reflect.DeepEqual(received, ToRosetta(source)) {
				t.Fatalf("transport changed error: received=%+v source=%+v", received, ToRosetta(source))
			}
			definition, exists := vocabulary[received.Code]
			if !exists {
				t.Fatalf("emitted error %d was not advertised", received.Code)
			}
			base := *received
			base.Details = nil
			if !reflect.DeepEqual(&base, definition) {
				t.Fatalf("emitted definition differs from network/options: received=%+v advertised=%+v", &base, definition)
			}
		})
	}
	if after := snapshotErrorRegistry(t); after != before {
		t.Fatalf("registry changed while consuming errors: before=%s after=%s", before, after)
	}
}
