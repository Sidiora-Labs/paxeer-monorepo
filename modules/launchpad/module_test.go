package launchpad_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/stretchr/testify/require"
)

func TestModuleSurfaceFrozen(t *testing.T) {
	t.Run("ConsensusVersion", func(t *testing.T) {
		require.Equal(t, uint64(1), launchpad.NewAppModule(nil).ConsensusVersion())
	})
	t.Run("GetTxCmd", func(t *testing.T) {
		require.Nil(t, launchpad.NewAppModuleBasic().GetTxCmd())
	})

	for _, tc := range []struct {
		file    string
		service string
		methods []string
	}{
		{
			file:    "launchpad/tx.proto",
			service: "paxprotocol.paxchain.launchpad.Msg",
			methods: []string{"UpdateParams", "TreasuryWithdraw"},
		},
		{
			file:    "launchpad/query.proto",
			service: "paxprotocol.paxchain.launchpad.Query",
			methods: []string{
				"Params",
				"Market",
				"MarketByAddress",
				"Markets",
				"MarketsByCreator",
				"PriceHistory",
				"Quote",
				"Treasury",
				"Surface",
				"Retired",
				"FeeRight",
				"AirdropEpoch",
				"AirdropClaimed",
				"HasRole",
				"QuoteLedger",
				"FeeAccounting",
				"PoolMetadata",
			},
		},
	} {
		t.Run(tc.service, func(t *testing.T) {
			compressed := gogoproto.FileDescriptor(tc.file)
			require.NotEmpty(t, compressed, "file must be registered: %s", tc.file)
			reader, err := gzip.NewReader(bytes.NewReader(compressed))
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			closeErr := reader.Close()
			require.NoError(t, err)
			require.NoError(t, closeErr)

			var file descriptor.FileDescriptorProto
			require.NoError(t, gogoproto.Unmarshal(data, &file))
			require.Len(t, file.GetService(), 1)
			service := file.GetService()[0]
			require.Equal(t, tc.service, file.GetPackage()+"."+service.GetName())

			methods := make([]string, 0, len(service.GetMethod()))
			for _, method := range service.GetMethod() {
				methods = append(methods, method.GetName())
			}
			require.ElementsMatch(t, tc.methods, methods, "RPC surface changed for %s", tc.service)
		})
	}
}
