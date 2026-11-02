package eip712_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/eip712"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/stretchr/testify/require"
)

func TestEip712MergeConstructorRegistered(t *testing.T) {
	for _, keepers := range []utils.Keepers{nil, &utils.EmptyKeepers{}} {
		p, err := eip712.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
		require.Nil(t, p)
		require.ErrorIs(t, err, agentcommon.ErrNoContext)
	}
	testApp := app.Setup(t, false, true, false)
	keepers := testApp.GetPrecompileKeepers()
	p, err := eip712.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	address := common.HexToAddress("0x0908")
	require.Equal(t, address, p.Address())
	require.Equal(t, "eip712", p.GetName())
	versions, found := precompiles.GetCustomPrecompiles(app.LatestUpgrade, keepers)[address]
	require.True(t, found)
	registered, ok := versions[precompiles.MergeUpgrade].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, address, registered.Address())
	require.Equal(t, "eip712", registered.GetName())
	require.Same(t, registered, versions[app.LatestUpgrade])
	p, err = eip712.NewMergePrecompile(keepers, "")
	require.Nil(t, p)
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
}

func TestEip712MergeNameCap(t *testing.T) {
	require.Equal(t, 256, eip712.MergeMaxNameBytes)
	require.Equal(t, 256, eip712.MergeMaxVersionBytes)
	testApp := app.Setup(t, false, true, false)
	address := common.HexToAddress("0x0908")
	versions := precompiles.GetCustomPrecompiles(app.LatestUpgrade, testApp.GetPrecompileKeepers())[address]
	p, ok := versions[precompiles.MergeUpgrade].(precompiles.IPrecompile)
	require.True(t, ok)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(100)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	testApp.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	a := p.GetABI()
	method := a.Methods[eip712.DomainSeparatorMethod]
	require.Equal(t, []byte{0xe6, 0xd3, 0x41, 0xcd}, method.ID)
	chainID := big.NewInt(125)
	typedChainID := ethmath.HexOrDecimal256(*chainID)
	typed := apitypes.TypedData{Types: apitypes.Types{
		"EIP712Domain": {
			{Name: "name", Type: "string"},
			{Name: "version", Type: "string"},
			{Name: "chainId", Type: "uint256"},
			{Name: "verifyingContract", Type: "address"},
		},
	}}
	for _, vector := range []struct {
		label   string
		name    string
		version string
		refused bool
	}{
		{"name at cap", strings.Repeat("n", 256), "1", false},
		{"both at cap", strings.Repeat("n", 256), strings.Repeat("v", 256), false},
		{"multibyte at cap", strings.Repeat("\u00e9", 128), strings.Repeat("\u03c0", 128), false},
		{"name over cap", strings.Repeat("n", 257), "1", true},
		{"version over cap", "Paxeer", strings.Repeat("v", 257), true},
		{"multibyte name over cap", strings.Repeat("\u00e9", 128) + "n", "1", true},
		{"multibyte version over cap", "Paxeer", strings.Repeat("\u03c0", 128) + "v", true},
	} {
		t.Run(vector.label, func(t *testing.T) {
			input, err := a.Pack(eip712.DomainSeparatorMethod, vector.name, vector.version, chainID, address)
			require.NoError(t, err)
			db := state.NewDBImpl(ctx, &testApp.EvmKeeper, false)
			data, err := p.Run(&vm.EVM{StateDB: db}, common.Address{}, address, input, nil, true, false, nil)
			if vector.refused {
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, decodeErr := abi.UnpackRevert(data)
				require.NoError(t, decodeErr)
				require.Equal(t, "LXP_AGENT_BATCH_TOO_LARGE: domain name or version exceeds its byte cap", reason)
				require.ErrorIs(t, db.GetPrecompileError(), agentcommon.ErrBatchTooLarge)
				return
			}
			require.NoError(t, err)
			typed.Domain = apitypes.TypedDataDomain{
				Name: vector.name, Version: vector.version,
				ChainId: &typedChainID, VerifyingContract: address.Hex(),
			}
			want, err := typed.HashStruct("EIP712Domain", apitypes.TypedDataMessage{
				"name": vector.name, "version": vector.version,
				"chainId": &typedChainID, "verifyingContract": address.Hex(),
			})
			require.NoError(t, err)
			require.Equal(t, []byte(want), data)
		})
	}
}
