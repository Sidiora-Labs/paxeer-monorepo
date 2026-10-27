package types

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/genproto/googleapis/api/annotations"
)

type Query interface {
	Params(context.Context, *QueryParamsRequest) (*QueryParamsResponse, error)
	Market(context.Context, *QueryMarketRequest) (*QueryMarketResponse, error)
	MarketByAddress(context.Context, *QueryMarketByAddressRequest) (*QueryMarketByAddressResponse, error)
	Markets(context.Context, *QueryMarketsRequest) (*QueryMarketsResponse, error)
	MarketsByCreator(context.Context, *QueryMarketsByCreatorRequest) (*QueryMarketsByCreatorResponse, error)
	PriceHistory(context.Context, *QueryPriceHistoryRequest) (*QueryPriceHistoryResponse, error)
	Quote(context.Context, *QueryQuoteRequest) (*QueryQuoteResponse, error)
	Treasury(context.Context, *QueryTreasuryRequest) (*QueryTreasuryResponse, error)
	Surface(context.Context, *QuerySurfaceRequest) (*QuerySurfaceResponse, error)
	Retired(context.Context, *QueryRetiredRequest) (*QueryRetiredResponse, error)
	FeeRight(context.Context, *QueryFeeRightRequest) (*QueryFeeRightResponse, error)
	AirdropEpoch(context.Context, *QueryAirdropEpochRequest) (*QueryAirdropEpochResponse, error)
	AirdropClaimed(context.Context, *QueryAirdropClaimedRequest) (*QueryAirdropClaimedResponse, error)
	HasRole(context.Context, *QueryHasRoleRequest) (*QueryHasRoleResponse, error)
	QuoteLedger(context.Context, *QueryQuoteLedgerRequest) (*QueryQuoteLedgerResponse, error)
	FeeAccounting(context.Context, *QueryFeeAccountingRequest) (*QueryFeeAccountingResponse, error)
	PoolMetadata(context.Context, *QueryPoolMetadataRequest) (*QueryPoolMetadataResponse, error)
}

var _ Query = QueryServer(nil)
var _ QueryServer = Query(nil)

var launchpadQueryRoutes = []struct {
	method  string
	path    string
	pattern runtime.Pattern
}{
	{"Params", "params", pattern_Query_Params_0},
	{"Market", "market", pattern_Query_Market_0},
	{"MarketByAddress", "market_by_address/{address}", pattern_Query_MarketByAddress_0},
	{"Markets", "markets", pattern_Query_Markets_0},
	{"MarketsByCreator", "markets_by_creator/{creator}", pattern_Query_MarketsByCreator_0},
	{"PriceHistory", "price_history", pattern_Query_PriceHistory_0},
	{"Quote", "quote", pattern_Query_Quote_0},
	{"Treasury", "treasury/{token}", pattern_Query_Treasury_0},
	{"Surface", "surface/{address}", pattern_Query_Surface_0},
	{"Retired", "retired/{address}", pattern_Query_Retired_0},
	{"FeeRight", "fee_right/{collection}/{id}", pattern_Query_FeeRight_0},
	{"AirdropEpoch", "airdrop_epoch/{pool}/{epoch}", pattern_Query_AirdropEpoch_0},
	{"AirdropClaimed", "airdrop_claimed/{pool}/{holder}/{epoch}", pattern_Query_AirdropClaimed_0},
	{"HasRole", "has_role/{contract}/{role}/{account}", pattern_Query_HasRole_0},
	{"QuoteLedger", "quote_ledger/{kind}", pattern_Query_QuoteLedger_0},
	{"FeeAccounting", "fee_accounting", pattern_Query_FeeAccounting_0},
	{"PoolMetadata", "pool_metadata/{pool}", pattern_Query_PoolMetadata_0},
}

func launchpadFileDescriptor(t *testing.T, compressed []byte) *descriptor.FileDescriptorProto {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	var file descriptor.FileDescriptorProto
	if err := proto.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return &file
}

func TestQueryServiceDescriptor(t *testing.T) {
	const serviceName = "paxprotocol.paxchain.launchpad.Query"
	if _Query_serviceDesc.ServiceName != serviceName {
		t.Fatalf("service name = %q, want %q", _Query_serviceDesc.ServiceName, serviceName)
	}
	if _Query_serviceDesc.Metadata != "launchpad/query.proto" {
		t.Fatalf("metadata = %v", _Query_serviceDesc.Metadata)
	}
	if len(_Query_serviceDesc.Streams) != 0 {
		t.Fatal("Query must contain only unary RPCs")
	}
	if len(_Query_serviceDesc.Methods) != 17 {
		t.Fatalf("method count = %d, want 17", len(_Query_serviceDesc.Methods))
	}
	serverType := reflect.TypeOf((*QueryServer)(nil)).Elem()
	if serverType.NumMethod() != 17 {
		t.Fatalf("QueryServer method count = %d", serverType.NumMethod())
	}
	compressed, _ := (&QueryParamsRequest{}).Descriptor()
	file := launchpadFileDescriptor(t, compressed)
	if file.GetPackage() != "paxprotocol.paxchain.launchpad" {
		t.Fatalf("package = %q", file.GetPackage())
	}
	if file.GetOptions().GetGoPackage() != "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types" {
		t.Fatalf("go_package = %q", file.GetOptions().GetGoPackage())
	}
	if len(file.Service) != 1 || file.Service[0].GetName() != "Query" {
		t.Fatalf("services = %v", file.Service)
	}
	service := file.Service[0]
	if len(service.Method) != 17 {
		t.Fatalf("proto method count = %d", len(service.Method))
	}
	for i, route := range launchpadQueryRoutes {
		t.Run(route.method, func(t *testing.T) {
			registered := _Query_serviceDesc.Methods[i]
			if registered.MethodName != route.method || registered.Handler == nil {
				t.Fatalf("registered method = %+v, want %s with a handler", registered, route.method)
			}
			method := service.Method[i]
			if method.GetName() != route.method || method.GetClientStreaming() || method.GetServerStreaming() {
				t.Fatalf("invalid unary method descriptor: %v", method)
			}
			requestName, responseName := "Query"+route.method+"Request", "Query"+route.method+"Response"
			if method.GetInputType() != ".paxprotocol.paxchain.launchpad."+requestName || method.GetOutputType() != ".paxprotocol.paxchain.launchpad."+responseName {
				t.Fatalf("request/response pair = %s -> %s", method.GetInputType(), method.GetOutputType())
			}
			goMethod, ok := serverType.MethodByName(route.method)
			if !ok {
				t.Fatalf("QueryServer missing %s", route.method)
			}
			if goMethod.Type.In(1).Elem().Name() != requestName || goMethod.Type.Out(0).Elem().Name() != responseName {
				t.Fatalf("Go request/response pair = %v", goMethod.Type)
			}
			extension, err := proto.GetExtension(method.GetOptions(), annotations.E_Http)
			if err != nil {
				t.Fatal(err)
			}
			rule, ok := extension.(*annotations.HttpRule)
			if !ok {
				t.Fatalf("HTTP option type = %T", extension)
			}
			if rule.GetGet() != "/pax-protocol/paxchain/launchpad/"+route.path || rule.GetBody() != "" || len(rule.GetAdditionalBindings()) != 0 {
				t.Fatalf("expected exactly one GET binding: %v", rule)
			}
			if strings.Contains(rule.GetGet(), "{denom") {
				t.Fatal("denom must be a query parameter")
			}
		})
	}
	compressed, _ = (&Params{}).Descriptor()
	txFile := launchpadFileDescriptor(t, compressed)
	if len(txFile.Service) != 1 || txFile.Service[0].GetName() != "Msg" {
		t.Fatalf("tx.proto services = %v", txFile.Service)
	}
}

func TestQueryGatewayPatterns(t *testing.T) {
	const prefix = "/pax-protocol/paxchain/launchpad/"
	for _, route := range launchpadQueryRoutes {
		t.Run(route.method, func(t *testing.T) {
			components := strings.Split(strings.TrimPrefix(prefix+route.path, "/"), "/")
			wantBindings := map[string]string{}
			normalized := append([]string(nil), components...)
			for i, component := range components {
				if strings.HasPrefix(component, "{") {
					name := strings.TrimSuffix(strings.TrimPrefix(component, "{"), "}")
					value := "0x1111111111111111111111111111111111111111"
					switch name {
					case "id", "epoch":
						value = "7"
					case "kind":
						value = "held"
					case "creator":
						value = "pax1creator"
					case "role":
						value = "0x" + strings.Repeat("11", 32)
					}
					components[i] = value
					normalized[i] = "{" + name + "=*}"
					wantBindings[name] = value
				}
			}
			if route.pattern.String() != "/"+strings.Join(normalized, "/") {
				t.Fatalf("pattern = %s", route.pattern.String())
			}
			bindings, err := route.pattern.Match(components, "")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(bindings, wantBindings) {
				t.Fatalf("bindings = %v, want %v", bindings, wantBindings)
			}
			if _, captured := bindings["denom"]; captured {
				t.Fatal("gateway captured denom in path")
			}
			if _, err := route.pattern.Match(append(append([]string(nil), components...), "unexpected"), ""); err == nil {
				t.Fatal("pattern accepted an extra path component")
			}
			components[0] = "other-protocol"
			if _, err := route.pattern.Match(components, ""); err == nil {
				t.Fatal("pattern accepted another namespace")
			}
		})
	}
	for _, encoded := range []string{"factory/pax1creator/token", "factory%2Fpax1creator%2Ftoken"} {
		values, err := url.ParseQuery("denom=" + encoded)
		if err != nil {
			t.Fatal(err)
		}
		market := &QueryMarketRequest{}
		if err := runtime.PopulateQueryParameters(market, values, filter_Query_Market_0); err != nil {
			t.Fatal(err)
		}
		history := &QueryPriceHistoryRequest{}
		if err := runtime.PopulateQueryParameters(history, values, filter_Query_PriceHistory_0); err != nil {
			t.Fatal(err)
		}
		quote := &QueryQuoteRequest{}
		if err := runtime.PopulateQueryParameters(quote, values, filter_Query_Quote_0); err != nil {
			t.Fatal(err)
		}
		if market.Denom != "factory/pax1creator/token" || history.Denom != market.Denom || quote.Denom != market.Denom {
			t.Fatalf("slash-containing denom changed: market=%q history=%q quote=%q", market.Denom, history.Denom, quote.Denom)
		}
	}
}
