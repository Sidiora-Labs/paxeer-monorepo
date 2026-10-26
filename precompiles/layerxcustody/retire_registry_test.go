package layerxcustody_test

import (
	"math/big"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxcustody"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestCustodyAddressStaysInThePrecompileSetAfterTheMergePlan(t *testing.T) {
	app := testkeeper.EVMTestApp
	ctx, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(20).WithBlockTime(genesisTime).CacheContext()
	k := &app.EvmKeeper
	app.UpgradeKeeper.SetDone(ctx, "v7.0")
	require.Equal(t, ctx.BlockHeight(), app.UpgradeKeeper.GetDoneHeight(ctx, "v7.0"))

	address := common.HexToAddress(types.CustodyAddress)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001013"), address)
	active := k.CustomPrecompiles(ctx)
	registered, present := active[address]
	require.True(t, present, "the merge plan must keep custody registered")
	precompile, ok := registered.(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, address, precompile.Address())
	require.Equal(t, "layerxCustody", precompile.GetName())
	callABI := precompile.GetABI()
	data, err := callABI.Pack(layerxcustody.DepositMethod, beneficiary)
	require.NoError(t, err)

	privateKey := testkeeper.MockPrivateKey()
	key, err := crypto.ToECDSA(privateKey.Bytes())
	require.NoError(t, err)
	paxAddress, sender := testkeeper.PrivateKeyToAddresses(privateKey)
	k.SetAddressMapping(ctx, paxAddress, sender)
	coins := sdk.NewCoins(sdk.NewInt64Coin(k.GetBaseDenom(ctx), 50_000_000))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, paxAddress, coins))

	chainConfig := evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(ctx))
	signer := ethtypes.MakeSigner(chainConfig, big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix()))
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    k.GetNonce(ctx, sender),
		GasPrice: big.NewInt(1_000_000_000_000),
		Gas:      500_000,
		To:       &address,
		Value:    new(big.Int).Set(weiPerBase),
		Data:     data,
	}), signer, key)
	require.NoError(t, err)
	txData, err := ethtx.NewLegacyTx(tx)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	require.NoError(t, ante.Preprocess(ctx, msg, k.ChainID(ctx), false))
	response, err := evmkeeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	require.Equal(t, vm.ErrExecutionReverted.Error(), response.VmError)
	require.Equal(t, pcommon.RetiredRevert("layerxCustody"), response.ReturnData)

	receipt, err := k.GetTransientReceipt(ctx, tx.Hash(), uint64(ctx.TxIndex()))
	require.NoError(t, err)
	require.Equal(t, tx.Hash().Hex(), receipt.TxHashHex)
	require.Equal(t, address.Hex(), receipt.To)
	require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
	require.Contains(t, receipt.VmError, vm.ErrExecutionReverted.Error())
}
