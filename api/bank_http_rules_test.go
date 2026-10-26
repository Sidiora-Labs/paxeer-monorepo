package api_test

import (
	"testing"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestBankQueryAdditionalBindings(t *testing.T) {
	file := bufFile(t, "cosmos/bank/v1beta1/query.proto")
	var query *descriptorpb.ServiceDescriptorProto
	for _, service := range file.GetService() {
		if service.GetName() == "Query" {
			query = service
		}
	}
	if query == nil {
		t.Fatal("bank Query service missing from buf workspace image")
	}

	for _, tc := range []struct {
		method     string
		path       string
		additional string
	}{
		{
			method:     "SupplyOf",
			path:       "/cosmos/bank/v1beta1/supply/{denom}",
			additional: "/cosmos/bank/v1beta1/supply/by_denom",
		},
		{
			method:     "DenomMetadata",
			path:       "/cosmos/bank/v1beta1/denoms_metadata/{denom}",
			additional: "/cosmos/bank/v1beta1/denoms_metadata_by_query_string",
		},
	} {
		t.Run(tc.method, func(t *testing.T) {
			var method *descriptorpb.MethodDescriptorProto
			for _, candidate := range query.GetMethod() {
				if candidate.GetName() == tc.method {
					method = candidate
				}
			}
			if method == nil {
				t.Fatalf("bank Query.%s missing from buf workspace image", tc.method)
			}
			if !proto.HasExtension(method.GetOptions(), annotations.E_Http) {
				t.Fatal("google.api.http option missing")
			}
			rule, ok := proto.GetExtension(method.GetOptions(), annotations.E_Http).(*annotations.HttpRule)
			if !ok || rule == nil {
				t.Fatal("google.api.http option is not an HttpRule")
			}
			if got := rule.GetGet(); got != tc.path {
				t.Errorf("primary GET = %q, want %q", got, tc.path)
			}
			if got := rule.GetBody(); got != "" {
				t.Errorf("primary body = %q, want no body", got)
			}
			if got := len(rule.GetAdditionalBindings()); got != 1 {
				t.Fatalf("additional bindings = %d, want exactly 1", got)
			}
			binding := rule.GetAdditionalBindings()[0]
			if got := binding.GetGet(); got != tc.additional {
				t.Errorf("additional GET = %q, want %q", got, tc.additional)
			}
			if got := binding.GetBody(); got != "" {
				t.Errorf("additional body = %q, want no body", got)
			}
			if got := len(binding.GetAdditionalBindings()); got != 0 {
				t.Errorf("nested additional bindings = %d, want 0", got)
			}
		})
	}
}
