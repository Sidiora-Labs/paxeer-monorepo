package ante_test

import (
	"math/big"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/ante"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func domainAnteApp(t *testing.T) (*node.App, sdk.Context, chainstypes.Domain) {
	t.Helper()
	app := node.Setup(t, false, true, false)
	ctx, _ := app.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockTime(time.Now()).WithBlockHeight(100).WithIsCheckTx(true).CacheContext()
	params := app.LayerXChainsKeeper.GetParams(ctx)
	params.MinBond = sdk.NewInt(100)
	params.CreationFee = sdk.NewInt(10)
	app.LayerXChainsKeeper.SetParams(ctx, params)
	operator := common.HexToAddress("0x1234")
	coins := sdk.NewCoins(sdk.NewCoin(app.EvmKeeper.GetBaseDenom(ctx), params.MinBond.Add(params.CreationFee)))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, sdk.AccAddress(operator.Bytes()), coins))
	domain, err := app.LayerXChainsKeeper.Create(ctx, operator, "ante-chain", params.MinGasAllowance, params.MinByteAllowance, 0, params.MinBond.Add(params.CreationFee), app.EvmKeeper.GetNextBaseFeePerGas(ctx))
	require.NoError(t, err)
	require.Equal(t, uint32(1), domain.Id)
	return app, ctx, domain
}

func signedDomainTx(t *testing.T, app *node.App, id int64, legacy bool) sdk.Tx {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	to := common.HexToAddress("0x5678")
	var data ethtypes.TxData = &ethtypes.DynamicFeeTx{ChainID: big.NewInt(id), Gas: 21000, GasFeeCap: big.NewInt(1_000_000_000_000), To: &to}
	var signer ethtypes.Signer
	if legacy {
		data = &ethtypes.LegacyTx{Gas: 21000, GasPrice: big.NewInt(1_000_000_000_000), To: &to}
		signer = ethtypes.FrontierSigner{}
	} else {
		signerID := id
		if signerID == 0 {
			signerID = 125
		}
		signer = ethtypes.LatestSignerForChainID(big.NewInt(signerID))
	}
	tx, err := ethtypes.SignTx(ethtypes.NewTx(data), signer, key)
	require.NoError(t, err)
	txData, err := ethtx.NewTxDataFromTx(tx)
	require.NoError(t, err)
	if !legacy && id == 0 {
		zero := sdk.ZeroInt()
		txData.(*ethtx.DynamicFeeTx).ChainID = &zero
	}
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	builder := app.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	return builder.GetTx()
}

func TestChainIdOfAnActiveChainIsAccepted(t *testing.T) {
	app, ctx, _ := domainAnteApp(t)
	tx := signedDomainTx(t, app, 125000001, false)
	require.NoError(t, ante.EvmStatelessChecks(ctx, tx, big.NewInt(125), app.LayerXChainsKeeper))
	data, err := evmtypes.UnpackTxData(tx.GetMsgs()[0].(*evmtypes.MsgEVMTransaction).Data)
	require.NoError(t, err)
	_, _, _, _, err = ante.CheckAndDecodeSignature(ctx, data.(ethtx.EthereumTxData), big.NewInt(125), false)
	require.NoError(t, err)
	handler := sdk.ChainAnteDecorators(ante.EVMChainIDDecorator{EVMKeeper: &app.EvmKeeper, ChainsKeeper: app.LayerXChainsKeeper})
	out, err := handler(ctx, tx, false)
	require.NoError(t, err)
	id, ok := chainstypes.DomainFrom(out)
	require.True(t, ok)
	require.Equal(t, uint32(1), id)
	_, ok = chainstypes.DomainFrom(ctx)
	require.False(t, ok)
}

func TestChainIdOfAPausedOrRetiredChainIsRefused(t *testing.T) {
	app, base, domain := domainAnteApp(t)
	for _, status := range []uint8{chainstypes.StatusPaused, chainstypes.StatusGovernancePaused, chainstypes.StatusRetired} {
		t.Run(big.NewInt(int64(status)).String(), func(t *testing.T) {
			ctx, _ := base.CacheContext()
			switch status {
			case chainstypes.StatusPaused:
				require.NoError(t, app.LayerXChainsKeeper.Pause(ctx, domain.Operator, domain.Id))
			case chainstypes.StatusGovernancePaused:
				require.NoError(t, app.LayerXChainsKeeper.GovernancePause(ctx, app.LayerXChainsKeeper.GetParams(ctx).Authority, domain.Id, true))
			case chainstypes.StatusRetired:
				require.NoError(t, app.LayerXChainsKeeper.Retire(ctx, domain.Operator, domain.Id))
			}
			tx := signedDomainTx(t, app, 125000001, false)
			require.ErrorIs(t, ante.EvmStatelessChecks(ctx, tx, big.NewInt(125), app.LayerXChainsKeeper), chainstypes.ErrDomainNotActive)
			handler := sdk.ChainAnteDecorators(ante.EVMChainIDDecorator{EVMKeeper: &app.EvmKeeper, ChainsKeeper: app.LayerXChainsKeeper})
			out, err := handler(ctx, tx, false)
			require.ErrorIs(t, err, chainstypes.ErrDomainNotActive)
			_, ok := chainstypes.DomainFrom(out)
			require.False(t, ok)
		})
	}
}

func TestUnknownAndRootChainIdsKeepTodaysBehaviour(t *testing.T) {
	app, ctx, _ := domainAnteApp(t)
	for _, tc := range []struct {
		name   string
		id     int64
		legacy bool
		want   error
	}{
		{name: "root", id: 125},
		{name: "legacy zero", id: 0, legacy: true},
		{name: "unknown", id: 125000999, want: sdkerrors.ErrInvalidChainID},
		{name: "typed zero", id: 0, want: sdkerrors.ErrInvalidChainID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := signedDomainTx(t, app, tc.id, tc.legacy)
			handler := sdk.ChainAnteDecorators(ante.EVMChainIDDecorator{EVMKeeper: &app.EvmKeeper, ChainsKeeper: app.LayerXChainsKeeper})
			out, err := handler(chainstypes.WithDomain(ctx, 1), tx, false)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
				_, ok := chainstypes.DomainFrom(out)
				require.False(t, ok)
			}
		})
	}
}
