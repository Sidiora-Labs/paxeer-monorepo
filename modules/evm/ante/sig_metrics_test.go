package ante

import (
	"context"
	"math/big"
	"testing"
	"time"

	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPendingNonceMetricRecorded(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousProvider := otel.GetMeterProvider()
	previousPendingNonce := evmAnteMetrics.pendingNonce
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		evmAnteMetrics.pendingNonce = previousPendingNonce
		otel.SetMeterProvider(previousProvider)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	var err error
	evmAnteMetrics.pendingNonce, err = otel.Meter("evm_ante").Int64Counter("evm_pending_nonce",
		otelmetric.WithDescription("EVM pending nonce events by type (added, expired, rejected, accepted)"),
		otelmetric.WithUnit("{count}"))
	require.NoError(t, err)

	collect := func(t *testing.T) map[string]int64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &data))
		counts := make(map[string]int64)
		for _, scope := range data.ScopeMetrics {
			for _, instrument := range scope.Metrics {
				if instrument.Name != "evm_pending_nonce" {
					continue
				}
				require.Equal(t, "evm_ante", scope.Scope.Name)
				require.Equal(t, "EVM pending nonce events by type (added, expired, rejected, accepted)", instrument.Description)
				require.Equal(t, "{count}", instrument.Unit)
				sum, ok := instrument.Data.(metricdata.Sum[int64])
				require.True(t, ok, "pending nonce metric must be an Int64 counter")
				require.True(t, sum.IsMonotonic)
				require.Equal(t, metricdata.CumulativeTemporality, sum.Temporality)
				for _, point := range sum.DataPoints {
					require.Equal(t, 1, point.Attributes.Len())
					kind, ok := point.Attributes.Value(attribute.Key("type"))
					require.True(t, ok)
					require.Equal(t, attribute.STRING, kind.Type())
					_, duplicate := counts[kind.AsString()]
					require.False(t, duplicate)
					counts[kind.AsString()] = point.Value
				}
			}
		}
		return counts
	}

	keyStore := sdk.NewKVStoreKey(types.StoreKey)
	transientStore := sdk.NewTransientStoreKey(types.TransientStoreKey)
	ctx := testutil.DefaultContext(keyStore, transientStore).WithBlockTime(time.Now())
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keyStore, transientStore, types.ModuleName)
	k := evmkeeper.NewKeeper(keyStore, transientStore, params, nil, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	sender := crypto.PubkeyToAddress(key.PublicKey)
	k.SetNonce(ctx, sender, 5)
	chainID := k.ChainID(ctx)
	signer := ethtypes.LatestSignerForChainID(chainID)
	to := common.HexToAddress("0x1234")
	handler := NewEVMSigVerifyDecorator(k, func() sdk.Context { return ctx })
	expected := make(map[string]int64)
	require.Equal(t, expected, collect(t))

	for _, tc := range []struct {
		name    string
		nonce   uint64
		checkTx bool
		kind    string
		reject  bool
	}{
		{name: "future CheckTx", nonce: 6, checkTx: true, kind: "added"},
		{name: "past CheckTx", nonce: 4, checkTx: true, kind: "rejected", reject: true},
		{name: "equal CheckTx", nonce: 5, checkTx: true},
		{name: "another future CheckTx", nonce: 7, checkTx: true, kind: "added"},
		{name: "another past CheckTx", nonce: 3, checkTx: true, kind: "rejected", reject: true},
		{name: "future DeliverTx", nonce: 6, reject: true},
		{name: "past DeliverTx", nonce: 4, reject: true},
		{name: "equal DeliverTx", nonce: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
				Nonce: tc.nonce, GasPrice: big.NewInt(10), Gas: 21000, To: &to, Value: big.NewInt(1000),
			}), signer, key)
			require.NoError(t, err)
			txData, err := ethtx.NewLegacyTx(signed)
			require.NoError(t, err)
			msg, err := types.NewMsgEVMTransaction(txData)
			require.NoError(t, err)
			caseCtx := ctx.WithIsCheckTx(tc.checkTx)
			require.NoError(t, Preprocess(caseCtx, msg, chainID, false))
			require.Equal(t, sender, msg.Derived.SenderEVMAddr)
			builder := txConfig.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			calledNext := false
			result, err := handler.AnteHandle(caseCtx, builder.GetTx(), false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
				calledNext = true
				return ctx, nil
			})
			if tc.reject {
				require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
			} else {
				require.NoError(t, err)
				if tc.checkTx {
					require.Equal(t, big.NewInt(211000), result.EVMRequiredBalance())
				}
			}
			require.Equal(t, !tc.reject, calledNext)
			require.Equal(t, uint64(5), k.GetNonce(ctx, sender))
			if tc.kind != "" {
				expected[tc.kind]++
			}
			require.Equal(t, expected, collect(t))
		})
	}
}
