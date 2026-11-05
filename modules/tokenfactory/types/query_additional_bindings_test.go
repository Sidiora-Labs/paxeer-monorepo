package types

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

type additionalBindingsQueryClient struct {
	QueryClient
	authorityDenoms []string
	pausedDenoms    []string
}

func (c *additionalBindingsQueryClient) DenomAuthorityMetadata(_ context.Context, req *QueryDenomAuthorityMetadataRequest, _ ...grpc.CallOption) (*QueryDenomAuthorityMetadataResponse, error) {
	c.authorityDenoms = append(c.authorityDenoms, req.GetDenom())
	return &QueryDenomAuthorityMetadataResponse{
		AuthorityMetadata: DenomAuthorityMetadata{Admin: "issuer"},
	}, nil
}

func (c *additionalBindingsQueryClient) DenomPaused(_ context.Context, req *QueryDenomPausedRequest, _ ...grpc.CallOption) (*QueryDenomPausedResponse, error) {
	c.pausedDenoms = append(c.pausedDenoms, req.GetDenom())
	return &QueryDenomPausedResponse{Paused: true, IssuerDid: "did:layerx:issuer"}, nil
}

func TestDenomAuthorityMetadataQueryParamBinding(t *testing.T) {
	const denom = "factory/creator/asset"
	for _, tc := range []struct {
		name   string
		target string
		denom  string
	}{
		{"slashes", "/pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=" + denom, denom},
		{"escaped", "/pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=" + url.QueryEscape(denom), denom},
		{"path", "/pax-protocol/paxchain/tokenfactory/denoms/asset/authority_metadata", "asset"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &additionalBindingsQueryClient{}
			mux := runtime.NewServeMux()
			if err := RegisterQueryHandlerClient(context.Background(), mux, client); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if len(client.authorityDenoms) != 1 || client.authorityDenoms[0] != tc.denom {
				t.Fatalf("DenomAuthorityMetadata denoms = %q, want [%q]", client.authorityDenoms, tc.denom)
			}
			if len(client.pausedDenoms) != 0 {
				t.Fatalf("unexpected DenomPaused calls: %q", client.pausedDenoms)
			}
			var body struct {
				AuthorityMetadata struct {
					Admin string `json:"admin"`
				} `json:"authority_metadata"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.AuthorityMetadata.Admin != "issuer" {
				t.Fatalf("response admin = %q, want issuer", body.AuthorityMetadata.Admin)
			}
		})
	}
}

func TestDenomPausedQueryParamBinding(t *testing.T) {
	const denom = "factory/creator/asset"
	for _, tc := range []struct {
		name   string
		target string
		denom  string
	}{
		{"slashes", "/pax-protocol/paxchain/tokenfactory/denom_paused?denom=" + denom, denom},
		{"escaped", "/pax-protocol/paxchain/tokenfactory/denom_paused?denom=" + url.QueryEscape(denom), denom},
		{"path", "/pax-protocol/paxchain/tokenfactory/denoms/asset/paused", "asset"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &additionalBindingsQueryClient{}
			mux := runtime.NewServeMux()
			if err := RegisterQueryHandlerClient(context.Background(), mux, client); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if len(client.pausedDenoms) != 1 || client.pausedDenoms[0] != tc.denom {
				t.Fatalf("DenomPaused denoms = %q, want [%q]", client.pausedDenoms, tc.denom)
			}
			if len(client.authorityDenoms) != 0 {
				t.Fatalf("unexpected DenomAuthorityMetadata calls: %q", client.authorityDenoms)
			}
			var body struct {
				Paused    bool   `json:"paused"`
				IssuerDID string `json:"issuer_did"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !body.Paused || body.IssuerDID != "did:layerx:issuer" {
				t.Fatalf("response = %+v, want paused with issuer did:layerx:issuer", body)
			}
		})
	}
}
