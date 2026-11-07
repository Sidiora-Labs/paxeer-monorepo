package addr_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"math/big"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddrGasGolden(t *testing.T) {
	const planHeight = int64(100)
	const suppliedGas = uint64(200000)
	testApp := testkeeper.EVMTestApp
	k := &testApp.EvmKeeper
	ctx := testApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(planHeight - 1).WithChainID("addr-gas-golden")
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(1), "v6.4.0")
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	p, err := addr.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)

	private, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(private.PublicKey)
	pubKey := &secp256k1.PubKey{Key: crypto.CompressPubkey(&private.PublicKey)}
	paxAddress := sdk.AccAddress(pubKey.Address())
	const message = "addr gas golden"
	associationSignature, err := crypto.Sign(crypto.Keccak256([]byte(message)), private)
	require.NoError(t, err)

	seed := [ed25519.SeedSize]byte{1}
	didPrivate := ed25519.NewKeyFromSeed(seed[:])
	var didPublicKey [32]byte
	copy(didPublicKey[:], didPrivate.Public().(ed25519.PublicKey))
	bindMessage, err := types.LayerXBindMessage(k.ChainID(ctx), caller, 0)
	require.NoError(t, err)
	bindSignature := ed25519.Sign(didPrivate, bindMessage)
	var keeperSignature [64]byte
	copy(keeperSignature[:], bindSignature)
	mainAccountID, err := types.LayerXMainAccountID(didPublicKey)
	require.NoError(t, err)

	tests := []struct {
		method       string
		args         []interface{}
		requiredGas  uint64
		executionGas uint64
		associate    bool
		bind         bool
		want         []interface{}
	}{
		{
			method: "associate", requiredGas: 50000, executionGas: 32210,
			args: []interface{}{
				hex.EncodeToString(associationSignature[64:]),
				hex.EncodeToString(associationSignature[:32]),
				hex.EncodeToString(associationSignature[32:64]), message,
			},
			want: []interface{}{paxAddress.String(), caller},
		},
		{
			method: "associatePubKey", requiredGas: 50000, executionGas: 32210,
			args: []interface{}{hex.EncodeToString(pubKey.Bytes())},
			want: []interface{}{paxAddress.String(), caller},
		},
		{
			method: "getPaxAddr", requiredGas: 1096, executionGas: 1123, associate: true,
			args: []interface{}{caller}, want: []interface{}{paxAddress.String()},
		},
		{
			method: "getEvmAddr", requiredGas: 1384, executionGas: 1123, associate: true,
			args: []interface{}{paxAddress.String()}, want: []interface{}{caller},
		},
		{
			method: "bindLayerX", requiredGas: 6800, executionGas: 35300, associate: true,
			args: []interface{}{didPublicKey, bindSignature},
		},
		{
			method: "unbindLayerX", requiredGas: 2000, executionGas: 7116, associate: true, bind: true,
		},
		{
			method: "getLayerXDid", requiredGas: 1096, executionGas: 1159, associate: true, bind: true,
			args: []interface{}{caller}, want: []interface{}{didPublicKey, types.LayerXDid(didPublicKey)},
		},
		{
			method: "getEvmAddrByLayerX", requiredGas: 1096, executionGas: 1159, associate: true, bind: true,
			args: []interface{}{didPublicKey}, want: []interface{}{caller},
		},
		{
			method: "layerXBindNonce", requiredGas: 1096, executionGas: 1087, associate: true, bind: true,
			args: []interface{}{caller}, want: []interface{}{uint64(1)},
		},
		{
			method: "getUnifiedAccount", requiredGas: 1096, executionGas: 2282, associate: true, bind: true,
			args: []interface{}{caller}, want: []interface{}{caller, paxAddress.String(), didPublicKey, mainAccountID},
		},
	}
	require.Len(t, p.ABI.Methods, len(tests), "every ABI method needs a gas golden")
	covered := make(map[string]bool, len(tests))
	for _, tt := range tests {
		require.False(t, covered[tt.method], "duplicate gas golden for %s", tt.method)
		covered[tt.method] = true
		t.Run(tt.method, func(t *testing.T) {
			method, found := p.ABI.Methods[tt.method]
			require.True(t, found)
			input, err := p.ABI.Pack(tt.method, tt.args...)
			require.NoError(t, err)
			var execution [2][2]uint64
			for sourceIndex, source := range []string{"constructed", "application-served"} {
				t.Run(source, func(t *testing.T) {
					var required, used [2]uint64
					for i, height := range []int64{planHeight - 1, planHeight} {
						callCtx := ctx.WithBlockHeight(height).WithMultiStore(ctx.MultiStore().CacheMultiStore())
						require.Equal(t, i == 1, k.UpgradeKeeper().IsUpgradeActiveAtHeight(callCtx, pcommon.MergeUpgrade, height))
						contract := p
						if source == "application-served" {
							served, ok := k.CustomPrecompiles(callCtx)[p.Address()].(*pcommon.DynamicGasPrecompile)
							require.True(t, ok, "addr must be served at height %d", height)
							contract = served
						}
						require.Equal(t, p.Address(), contract.Address())
						require.Equal(t, p.GetABI(), contract.GetABI())
						executor, ok := contract.GetExecutor().(*addr.PrecompileExecutor)
						require.True(t, ok)
						if tt.associate {
							k.SetAddressMapping(callCtx, paxAddress, caller)
						}
						if tt.bind {
							_, err := k.BindLayerX(callCtx, caller, didPublicKey, keeperSignature)
							require.NoError(t, err)
						}
						required[i] = executor.RequiredGas(input[4:], &method)
						require.Equal(t, tt.requiredGas, required[i], "RequiredGas at height %d", height)
						stateDB := state.NewDBImpl(callCtx, k, false)
						ret, remaining, err := contract.RunAndCalculateGas(&vm.EVM{StateDB: stateDB}, caller, caller, input, suppliedGas, big.NewInt(0), nil, false, false)
						require.NoError(t, err, "height %d: %v", height, stateDB.GetPrecompileError())
						require.LessOrEqual(t, remaining, suppliedGas)
						used[i] = suppliedGas - remaining
						require.Positive(t, used[i])
						assert.Equal(t, tt.executionGas, used[i], "execution gas at height %d", height)
						execution[sourceIndex][i] = used[i]
						want, err := method.Outputs.Pack(tt.want...)
						require.NoError(t, err)
						require.Equal(t, want, ret, "output at height %d", height)

						resultCtx := stateDB.Ctx()
						mappedPax, found := k.GetPaxAddress(resultCtx, caller)
						require.True(t, found)
						require.Equal(t, paxAddress, mappedPax)
						mappedEVM, found := k.GetEVMAddress(resultCtx, paxAddress)
						require.True(t, found)
						require.Equal(t, caller, mappedEVM)
						if tt.method == addr.Associate || tt.method == addr.AssociatePubKey {
							account := testApp.AccountKeeper.GetAccount(resultCtx, paxAddress)
							require.NotNil(t, account)
							require.Equal(t, pubKey, account.GetPubKey())
						}
						boundKey, bound := k.GetLayerXDid(resultCtx, caller)
						reverseEVM, reverseBound := k.GetEVMAddressByLayerXDid(resultCtx, didPublicKey)
						wantBound := tt.method == addr.BindLayerXMethod || (tt.bind && tt.method != addr.UnbindLayerXMethod)
						require.Equal(t, wantBound, bound)
						require.Equal(t, wantBound, reverseBound)
						if wantBound {
							require.Equal(t, didPublicKey, boundKey)
							require.Equal(t, caller, reverseEVM)
						}
						var wantNonce uint64
						if tt.method == addr.UnbindLayerXMethod {
							wantNonce = 2
						} else if wantBound {
							wantNonce = 1
						}
						require.Equal(t, wantNonce, k.GetLayerXBindNonce(resultCtx, caller))
						if tt.method == addr.BindLayerXMethod || tt.method == addr.UnbindLayerXMethod {
							logs := stateDB.GetAllLogs()
							require.Len(t, logs, 1)
							eventName := addr.LayerXBoundEvent
							if tt.method == addr.UnbindLayerXMethod {
								eventName = addr.LayerXUnboundEvent
							}
							event := contract.ABI.Events[eventName]
							require.Equal(t, contract.Address(), logs[0].Address)
							require.Equal(t, []common.Hash{event.ID, common.BytesToHash(caller.Bytes()), common.Hash(didPublicKey)}, logs[0].Topics)
							data, err := event.Inputs.NonIndexed().Pack(wantNonce - 1)
							require.NoError(t, err)
							require.Equal(t, data, logs[0].Data)
						} else {
							require.Empty(t, stateDB.GetAllLogs())
						}
						t.Logf("height=%d RequiredGas=%d executionGas=%d", height, required[i], used[i])
					}
					require.Equal(t, required[0], required[1], "RequiredGas changed at the merge height")
					require.Equal(t, used[0], used[1], "execution gas changed at the merge height")
				})
			}
			require.Equal(t, execution[0], execution[1], "application-served execution gas differs from the constructor")
		})
	}
}
