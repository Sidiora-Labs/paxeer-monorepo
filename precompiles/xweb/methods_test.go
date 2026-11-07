package xweb_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestOriginTwoMethodConstantsResolveInTheABI(t *testing.T) {
	precompile := xweb.NewPrecompileWithKeeper(nil, nil, nil)
	for _, tc := range []struct {
		method   string
		name     string
		selector string
	}{
		{xweb.FulfilProgramMethod, "fulfilProgram", "0x2d4bf7ad"},
		{xweb.GetProgramResultMethod, "getProgramResult", "0x2b2e13fc"},
		{xweb.GetKernelNetworkIdMethod, "getKernelNetworkId", "0xfc0d26f4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.name, tc.method)
			method, ok := precompile.Methods[tc.method]
			require.True(t, ok, "method %q missing from embedded ABI", tc.method)
			require.Equal(t, common.FromHex(tc.selector), method.ID)
		})
	}
}
