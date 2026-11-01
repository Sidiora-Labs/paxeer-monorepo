package app

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	authprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	ledgerprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNodeImportsRegisterMergeConstructors(t *testing.T) {
	a := Setup(t, true, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	custom, err := precompiles.GetCustomPrecompilesE(LatestUpgrade, a.GetPrecompileKeepers())
	require.NoError(t, err)
	for _, entry := range []struct {
		address string
		name    string
	}{
		{"0x101A", "layerxledger"},
		{"0x101B", "layerxpay"},
		{"0x101C", "layerxauth"},
		{"0x0901", "orob"},
		{"0x0902", "clearing"},
		{"0x0904", "pofq"},
		{"0x0907", "tee attestor"},
		{"0x0908", "eip712"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			address := common.HexToAddress(entry.address)
			require.Contains(t, custom, address)
			require.Contains(t, a.customPrecompiles, address)
			versions := custom[address]
			for _, version := range []string{LatestUpgrade, precompiles.MergeUpgrade} {
				require.Contains(t, versions, version)
				require.Contains(t, a.customPrecompiles[address], version)
				contract, ok := versions[version].(precompiles.IPrecompile)
				require.True(t, ok)
				require.Equal(t, address, contract.Address())
				require.Equal(t, entry.name, contract.GetName())
				require.NotEmpty(t, contract.GetABI().Methods)
				require.IsType(t, contract, a.customPrecompiles[address][version])
			}
		})
	}
}

func TestPrecompileKeepersServeLedgerAndAuthKeepers(t *testing.T) {
	a := Setup(t, true, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	keepers := a.GetPrecompileKeepers()
	ledger, ok := keepers.(ledgerprecompile.LedgerKeepers)
	require.True(t, ok)
	require.NotNil(t, ledger.LayerXLedgerK())
	require.Same(t, a.LayerXLedgerKeeper, ledger.LayerXLedgerK())

	auth, ok := keepers.(authprecompile.AuthKeepers)
	require.True(t, ok)
	require.NotNil(t, auth.LayerXAuthK())
	require.Same(t, a.LayerXAuthKeeper, auth.LayerXAuthK())
}
