package keeper_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestRepair_22_39_210(t *testing.T) {
	precompile := common.HexToAddress(bank.BankAddress)
	recipient := common.HexToAddress("0x123456")
	for _, tc := range []struct {
		name            string
		to              *common.Address
		data            []byte
		vmError         string
		precompileError string
	}{
		{
			name:            "precompile failure",
			to:              &precompile,
			data:            []byte{0x01},
			vmError:         vm.ErrExecutionReverted.Error(),
			precompileError: "input too short to extract method ID",
		},
		{
			name:    "EVM revert without precompile error",
			data:    common.FromHex("0x60006000fd"),
			vmError: vm.ErrExecutionReverted.Error(),
		},
		{
			name: "successful transfer",
			to:   &recipient,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := node.Setup(t, false, true, false)
			ctx := app.GetContextForDeliverTx(nil).WithBlockHeight(8)
			k := &app.EvmKeeper
			k.InitGenesis(ctx, *types.DefaultGenesis())
			key, err := crypto.GenerateKey()
			require.NoError(t, err)
			sender := crypto.PubkeyToAddress(key.PublicKey)
			funds := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(1000000)))
			require.NoError(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, funds))
			require.NoError(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdk.AccAddress(sender[:]), funds))

			signer := ethtypes.LatestSignerForChainID(k.ChainID(ctx))
			tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
				GasPrice: big.NewInt(1000000000000),
				Gas:      200000,
				To:       tc.to,
				Value:    big.NewInt(0),
				Data:     tc.data,
			}), signer, key)
			require.NoError(t, err)
			txData, err := ethtx.NewLegacyTx(tx)
			require.NoError(t, err)
			msg, err := types.NewMsgEVMTransaction(txData)
			require.NoError(t, err)
			require.NoError(t, ante.Preprocess(ctx, msg, k.ChainID(ctx), false))
			builder := app.GetTxConfig().NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			ctx, err = ante.NewEVMFeeCheckDecorator(k, &app.UpgradeKeeper).AnteHandle(ctx, builder.GetTx(), false, func(nextCtx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
				return nextCtx, nil
			})
			require.NoError(t, err)

			response, err := keeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(ctx), msg)
			require.NoError(t, err)
			receipt, err := k.GetTransientReceipt(ctx, tx.Hash(), uint64(ctx.TxIndex()))
			require.NoError(t, err)
			require.Equal(t, receipt.VmError, response.VmError)
			require.Equal(t, receipt.GasUsed, response.GasUsed)
			require.Equal(t, tx.Hash().Hex(), response.Hash)
			require.Positive(t, response.GasUsed)
			require.LessOrEqual(t, response.GasUsed, tx.Gas())
			if tc.precompileError != "" {
				vmError, precompileError, found := strings.Cut(response.VmError, "|")
				require.True(t, found)
				require.Equal(t, tc.vmError, vmError)
				require.Equal(t, tc.precompileError, precompileError)
			} else {
				require.Equal(t, tc.vmError, response.VmError)
				require.NotContains(t, response.VmError, "|")
			}
			if tc.vmError == "" {
				require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
			} else {
				require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
			}
		})
	}
}
