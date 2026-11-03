package ante_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestChainTransactionsPayTheirOwnBaseFee(t *testing.T) {
	app := node.Setup(t, false, true, false)
	k := &app.EvmKeeper
	base, _ := app.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockTime(time.Now()).WithBlockHeight(100).WithIsCheckTx(false).CacheContext()
	rootFee := k.GetNextBaseFeePerGas(base)
	require.True(t, rootFee.GTE(k.GetMinimumFeePerGas(base)))
	domainFee := rootFee.MulInt64(2)
	params := app.LayerXChainsKeeper.GetParams(base)
	params.MinBond = sdk.NewInt(100)
	params.CreationFee = sdk.NewInt(10)
	app.LayerXChainsKeeper.SetParams(base, params)
	operator := common.HexToAddress("0x1234")
	bond := params.MinBond.Add(params.CreationFee)
	coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(base), bond))
	require.NoError(t, app.BankKeeper.MintCoins(base, evmtypes.ModuleName, coins))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(base, evmtypes.ModuleName, sdk.AccAddress(operator.Bytes()), coins))
	domain, err := app.LayerXChainsKeeper.Create(base, operator, "fee-chain", params.MinGasAllowance, params.MinByteAllowance, 0, bond, domainFee)
	require.NoError(t, err)
	require.Equal(t, uint32(1), domain.Id)
	lowCap := rootFee.TruncateInt().BigInt()
	require.True(t, sdk.NewDecFromBigInt(lowCap).LT(domainFee))
	for _, tc := range []struct {
		name    string
		chainID int64
		cap     *big.Int
		want    error
		domain  bool
	}{
		{name: "domain below its base fee", chainID: 125000001, cap: lowCap, want: sdkerrors.ErrInsufficientFee, domain: true},
		{name: "root accepts the same fee cap", chainID: 125, cap: lowCap},
		{name: "domain accepts its base fee", chainID: 125000001, cap: domainFee.Ceil().TruncateInt().BigInt(), domain: true},
		{name: "root below its base fee", chainID: 125, cap: new(big.Int).Sub(lowCap, big.NewInt(1)), want: sdkerrors.ErrInsufficientFee},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := base.CacheContext()
			key, err := crypto.GenerateKey()
			require.NoError(t, err)
			sender := crypto.PubkeyToAddress(key.PublicKey)
			funded := sdk.NewCoins(sdk.NewInt64Coin(k.GetBaseDenom(ctx), 1_000_000_000))
			require.NoError(t, app.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funded))
			require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, sdk.AccAddress(sender.Bytes()), funded))
			to := common.HexToAddress("0x5678")
			signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{ChainID: big.NewInt(tc.chainID), Gas: 21000, GasFeeCap: tc.cap, To: &to}), ethtypes.LatestSignerForChainID(big.NewInt(tc.chainID)), key)
			require.NoError(t, err)
			data, err := ethtx.NewTxDataFromTx(signed)
			require.NoError(t, err)
			msg, err := evmtypes.NewMsgEVMTransaction(data)
			require.NoError(t, err)
			builder := app.GetTxConfig().NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			out, err := app.AnteHandler(ctx, builder.GetTx(), false)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
				require.True(t, out.IsEVM())
				require.Equal(t, sender, out.EVMSenderAddress())
				expectedFee := rootFee.TruncateInt().BigInt()
				if tc.domain {
					expectedFee = domainFee.TruncateInt().BigInt()
				}
				expectedCost := new(big.Int).Mul(expectedFee, big.NewInt(21000))
				originalBalance := new(big.Int).Mul(funded.AmountOf(k.GetBaseDenom(ctx)).BigInt(), state.UhpxToSweiMultiplier)
				require.Equal(t, new(big.Int).Sub(originalBalance, expectedCost), k.GetBalance(ctx, msg.Derived.SenderPaxAddr))
			}
			id, ok := chainstypes.DomainFrom(out)
			require.Equal(t, tc.domain, ok)
			if tc.domain {
				require.Equal(t, domain.Id, id)
			}
			require.NotNil(t, msg.Derived)
			require.Equal(t, sender, msg.Derived.SenderEVMAddr)
		})
	}
}
