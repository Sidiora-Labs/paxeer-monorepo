package types_test

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/internal/service"
	crgtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/types"
)

func TestRepairM_1315_NetworkOptions(t *testing.T) {
	if crgtypes.SpecVersion != types.RosettaAPIVersion {
		t.Fatalf("supported specification %q differs from installed SDK specification %q", crgtypes.SpecVersion, types.RosettaAPIVersion)
	}

	cdc, registry := rosetta.MakeCodec()
	config := &rosetta.Config{
		Blockchain:        "paxeer",
		Network:           "repair-m1315",
		Codec:             cdc,
		InterfaceRegistry: registry,
	}
	nodeClient, err := rosetta.NewClient(config)
	if err != nil {
		t.Fatalf("create production client: %v", err)
	}
	network := config.NetworkIdentifier()
	adapter, err := service.NewOffline(network, nodeClient)
	if err != nil {
		t.Fatalf("create production network service: %v", err)
	}
	requestAsserter, err := asserter.NewServer(
		nodeClient.SupportedOperations(), true, []*types.NetworkIdentifier{network}, nil, false, "",
	)
	if err != nil {
		t.Fatalf("create server asserter: %v", err)
	}
	httpServer := httptest.NewServer(server.NewRouter(server.NewNetworkAPIController(adapter, requestAsserter)))
	t.Cleanup(httpServer.Close)
	api := client.NewAPIClient(client.NewConfiguration(httpServer.URL, "repair-m1315", httpServer.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	networks, apiErr, err := api.NetworkAPI.NetworkList(ctx, &types.MetadataRequest{})
	if err != nil || apiErr != nil {
		t.Fatalf("HTTP network/list: transport=%v API=%+v", err, apiErr)
	}
	if err := asserter.NetworkListResponse(networks); err != nil {
		t.Fatalf("client rejects network/list: %v", err)
	}
	if len(networks.NetworkIdentifiers) != 1 || !reflect.DeepEqual(networks.NetworkIdentifiers[0], network) {
		t.Fatalf("network/list differs from configured network: %+v", networks)
	}
	options, apiErr, err := api.NetworkAPI.NetworkOptions(ctx, &types.NetworkRequest{
		NetworkIdentifier: networks.NetworkIdentifiers[0],
	})
	if err != nil || apiErr != nil {
		t.Fatalf("HTTP network/options: transport=%v API=%+v", err, apiErr)
	}
	if err := asserter.NetworkOptionsResponse(options); err != nil {
		t.Fatalf("client rejects production network/options: %v", err)
	}
	// The SDK asserter checks NodeVersion but does not check RosettaVersion.
	if options.Version.RosettaVersion != types.RosettaAPIVersion {
		t.Fatalf("network/options advertises Rosetta specification %q, want %q", options.Version.RosettaVersion, types.RosettaAPIVersion)
	}
	if options.Version.NodeVersion != nodeClient.Version() {
		t.Fatalf("network/options node version %q, want %q", options.Version.NodeVersion, nodeClient.Version())
	}
	if !reflect.DeepEqual(options.Allow.OperationTypes, nodeClient.SupportedOperations()) {
		t.Fatalf("network/options operation types differ from production client: %v", options.Allow.OperationTypes)
	}
	if !reflect.DeepEqual(options.Allow.OperationStatuses, nodeClient.OperationStatuses()) {
		t.Fatalf("network/options operation statuses differ from production client: %+v", options.Allow.OperationStatuses)
	}
}
