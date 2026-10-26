package occ

import (
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestNewTestContextStoresFixturesUnderNodeCapabilities(t *testing.T) {
	tCtx := utils.NewTestContext(t, utils.NewTestAccounts(2), time.Unix(1700000000, 0), 1, false)
	testApp := tCtx.TestApp

	require.NotZero(t, tCtx.CodeID)
	require.Equal(t, tCtx.CW20CodeID, tCtx.CodeID)
	wasm, err := os.ReadFile("../../contracts/wasm/cw20_base.wasm")
	require.NoError(t, err)
	checksum := sha256.Sum256(wasm)

	for name, codeID := range map[string]uint64{"CodeID": tCtx.CodeID, "CW20CodeID": tCtx.CW20CodeID} {
		t.Run(name, func(t *testing.T) {
			codeInfo := testApp.WasmKeeper.GetCodeInfo(tCtx.Ctx, codeID)
			require.NotNil(t, codeInfo)
			require.Equal(t, checksum[:], codeInfo.CodeHash)
			require.Equal(t, tCtx.TestAccounts[0].AccountAddress.String(), codeInfo.Creator)
		})
	}
}
