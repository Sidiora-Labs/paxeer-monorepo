package ante

import (
	"math/big"

	chainskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/paxeer-network/paxlog"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	utilmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/utils/metrics"
)

var logger = paxlog.NewLogger("x", "evm", "ante")

type EVMSigVerifyDecorator struct {
	evmKeeper *evmkeeper.Keeper
}

func NewEVMSigVerifyDecorator(evmKeeper *evmkeeper.Keeper, _ func() sdk.Context) *EVMSigVerifyDecorator {
	return &EVMSigVerifyDecorator{
		evmKeeper: evmKeeper,
	}
}

func (svd *EVMSigVerifyDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	ethTx, _ := types.MustGetEVMTransactionMessage(tx).AsTransaction()

	evmAddr := types.MustGetEVMTransactionMessage(tx).Derived.SenderEVMAddr

	nextNonce := svd.evmKeeper.GetNonce(ctx, evmAddr)
	txNonce := ethTx.Nonce()

	// set EVM properties
	ctx = ctx.WithIsEVM(true)
	ctx = ctx.WithEVMNonce(txNonce)
	ctx = ctx.WithEVMSenderAddress(evmAddr)
	ctx = ctx.WithPaxSenderAddress(types.MustGetEVMTransactionMessage(tx).Derived.SenderPaxAddr)
	ctx = ctx.WithEVMTxHash(ethTx.Hash())

	readCtx := ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	chainID := svd.evmKeeper.ChainID(readCtx)
	txChainID := ethTx.ChainId()

	fee := new(big.Int).Mul(ethTx.GasPrice(), new(big.Int).SetUint64(ethTx.Gas()))
	if ethTx.Value() != nil {
		fee = new(big.Int).Add(fee, ethTx.Value())
	}

	// Resolve protected transactions against the registered chain before nonce checks.
	if txChainID.Cmp(chainID) != 0 && !(ethTx.Type() == ethtypes.LegacyTxType && txChainID.Sign() == 0) {
		id, mapped := chainstypes.DomainOfChainID(txChainID)
		if !mapped || id == 0 {
			return ctx, sdkerrors.ErrInvalidChainID
		}
		known := false
		for _, key := range readCtx.MultiStore().StoreKeys() {
			if key.Name() != chainstypes.StoreKey {
				continue
			}
			chains, err := chainskeeper.NewKeeper(nil, key, svd.evmKeeper.BankKeeper(), svd.evmKeeper.GetBaseDenom)
			if err != nil {
				return ctx, err
			}
			domain, ok := chains.ResolveChainID(readCtx, txChainID)
			if !ok {
				return ctx, sdkerrors.ErrInvalidChainID
			}
			if domain.Status != chainstypes.StatusActive {
				return ctx, chainstypes.ErrDomainNotActive
			}
			ctx = chainstypes.WithDomain(ctx, domain.Id)
			known = true
			break
		}
		if !known {
			return ctx, sdkerrors.ErrInvalidChainID
		}
	}

	if ctx.IsCheckTx() {
		if txNonce < nextNonce {
			evmAnteMetrics.pendingNonce.Add(ctx.Context(), 1, otelmetric.WithAttributes(attribute.String("type", "rejected")))
			return ctx, sdkerrors.ErrWrongSequence
		}
		if txNonce > nextNonce {
			evmAnteMetrics.pendingNonce.Add(ctx.Context(), 1, otelmetric.WithAttributes(attribute.String("type", "added")))
		}
		ctx = ctx.WithEVMRequiredBalance(fee)
	} else if txNonce != nextNonce {
		tooHigh := txNonce > nextNonce
		utilmetrics.IncrementNonceMismatch(tooHigh) // TODO(PLT-330): remove once evm_nonce_mismatch_total verified
		evmAnteMetrics.nonceMismatch.Add(ctx.Context(), 1, otelmetric.WithAttributes(attribute.Bool("too_high", tooHigh)))
		return ctx, sdkerrors.ErrWrongSequence
	}

	return next(ctx, tx, simulate)
}
