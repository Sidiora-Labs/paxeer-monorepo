package oracle

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	layerxexchangekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	OracleAddress           = "0x0000000000000000000000000000000000000903"
	PrecompileName          = "oracle aggregator"
	AggregateMethod         = "aggregate"
	SubmitPriceMethod       = "submitPrice"
	GetValidatorPriceMethod = "getValidatorPrice"
	abiJSON                 = `[
 {"type":"function","name":"aggregate","stateMutability":"pure","inputs":[{"name":"feeds","type":"tuple[]","components":[{"name":"price","type":"int256"},{"name":"confidence","type":"uint256"},{"name":"timestamp","type":"uint256"}]}],"outputs":[{"name":"price","type":"int256"},{"name":"confidence","type":"uint256"}]},
 {"type":"function","name":"submitPrice","stateMutability":"nonpayable","inputs":[{"name":"feedID","type":"bytes32"},{"name":"price","type":"int256"},{"name":"confidence","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"getValidatorPrice","stateMutability":"view","inputs":[{"name":"feedID","type":"bytes32"}],"outputs":[{"name":"price","type":"int256"},{"name":"quorum","type":"uint256"},{"name":"timestamp","type":"uint256"}]}
 ]`
)

type Feed struct{ Price, Confidence, Timestamp *big.Int }

func Aggregate(feeds []Feed, maxFeeds int) (*big.Int, *big.Int, error) {
	if len(feeds) == 0 {
		return nil, nil, fmt.Errorf("%w: no feeds provided", agentcommon.ErrOracleNoFeeds)
	}
	if maxFeeds < 1 || len(feeds) > maxFeeds {
		return nil, nil, fmt.Errorf("%w: batch over the cap (cap set at activation rehearsal)", agentcommon.ErrBatchTooLarge)
	}
	kept := make([]Feed, 0, len(feeds))
	total := new(big.Int)
	var minimum *big.Int
	for _, feed := range feeds {
		if feed.Price == nil || feed.Price.Sign() <= 0 {
			return nil, nil, fmt.Errorf("%w: invalid price", agentcommon.ErrOraclePrice)
		}
		if feed.Price.BitLen() > 255 {
			return nil, nil, fmt.Errorf("%w: argument decode failed", agentcommon.ErrABIDecode)
		}
		if feed.Confidence == nil || feed.Confidence.Sign() < 0 || feed.Confidence.BitLen() > 256 {
			return nil, nil, fmt.Errorf("%w: invalid confidence", agentcommon.ErrOracleConfidence)
		}
		if feed.Confidence.Sign() == 0 {
			continue
		}
		kept = append(kept, feed)
		total.Add(total, feed.Confidence)
		if minimum == nil || feed.Confidence.Cmp(minimum) < 0 {
			minimum = feed.Confidence
		}
	}
	if len(kept) == 0 {
		return nil, nil, fmt.Errorf("%w: no feeds with non-zero confidence", agentcommon.ErrOracleNoConfidentFeeds)
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Price.Cmp(kept[j].Price) < 0 })
	threshold := new(big.Int).Quo(total, big.NewInt(2))
	cumulative := new(big.Int)
	for _, feed := range kept {
		cumulative.Add(cumulative, feed.Confidence)
		if cumulative.Cmp(threshold) >= 0 {
			return new(big.Int).Set(feed.Price), new(big.Int).Set(minimum), nil
		}
	}
	return nil, nil, fmt.Errorf("%w: no feeds with non-zero confidence", agentcommon.ErrOracleNoConfidentFeeds)
}

type replayKey struct{}
type replayBinding struct {
	validator sdk.ValAddress
	publicKey [32]byte
}

func WithOracleReplay(ctx sdk.Context, host *lxphost.Host, storeKey sdk.StoreKey, validator sdk.ValAddress, activity lxphost.Activity, authority lxphost.KernelAuthority) sdk.Context {
	ctx = layerxexchangekeeper.WithOracleReplay(ctx, host, storeKey, validator, activity, authority)
	return ctx.WithValue(replayKey{}, replayBinding{bytes.Clone(validator), authority.VerifiedKey})
}

type OraclePrecompile struct {
	abi           abi.ABI
	exchange      *layerxexchangekeeper.Keeper
	evm           *evmkeeper.Keeper
	maxFeeds      int
	maxConfidence *big.Int
}

var _ pcommon.PrecompileExecutor = (*OraclePrecompile)(nil)

func NewPrecompile(exchange *layerxexchangekeeper.Keeper, evm *evmkeeper.Keeper, upgrade *upgradekeeper.Keeper, planName string, maxFeeds int, maxConfidence *big.Int) (*agentcommon.Precompile, error) {
	if exchange == nil || evm == nil {
		return nil, fmt.Errorf("%w: oracle keepers are unavailable", agentcommon.ErrNoContext)
	}
	if upgrade == nil || planName == "" {
		return nil, fmt.Errorf("%w: no merge upgrade activation reader bound", agentcommon.ErrNotActive)
	}
	if maxFeeds < 1 {
		return nil, fmt.Errorf("%w: batch over the cap (cap set at activation rehearsal)", agentcommon.ErrBatchTooLarge)
	}
	if maxConfidence == nil || maxConfidence.Sign() <= 0 || maxConfidence.BitLen() > 256 {
		return nil, fmt.Errorf("%w: invalid confidence", agentcommon.ErrOracleConfidence)
	}
	a, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	p := &OraclePrecompile{a, exchange, evm, maxFeeds, new(big.Int).Set(maxConfidence)}
	return agentcommon.NewPrecompile(a, p, common.HexToAddress(OracleAddress), PrecompileName, upgrade, planName), nil
}

func (p *OraclePrecompile) RequiredGas(input []byte, method *abi.Method) uint64 {
	if p == nil || method == nil {
		return pcommon.UnknownMethodCallGas
	}
	if _, err := p.abi.MethodById(method.ID); err != nil {
		return pcommon.UnknownMethodCallGas
	}
	if len(input) > math.MaxInt-4 {
		return math.MaxUint64
	}
	return agentcommon.MethodGas(method.ID, len(input)+4)
}

func (p *OraclePrecompile) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	if err := agentcommon.HeightGate(ctx); err != nil {
		return nil, err
	}
	if p == nil || method == nil {
		return nil, fmt.Errorf("%w: unknown oracle method", agentcommon.ErrUnknownMethod)
	}
	resolved, err := p.abi.MethodById(method.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: unknown oracle method", agentcommon.ErrUnknownMethod)
	}
	if resolved.Name == SubmitPriceMethod {
		if err := agentcommon.GuardWrite(ctx, readOnly, false); err != nil {
			return nil, err
		}
	}
	if err := agentcommon.ValidateNonPayable(value); err != nil {
		return nil, err
	}
	count := 1
	if resolved.Name == SubmitPriceMethod {
		count = 3
	}
	if err := agentcommon.ValidateArgsLength(args, count); err != nil {
		return nil, err
	}
	switch resolved.Name {
	case AggregateMethod:
		decoded, ok := args[0].([]struct {
			Price      *big.Int `json:"price"`
			Confidence *big.Int `json:"confidence"`
			Timestamp  *big.Int `json:"timestamp"`
		})
		if !ok {
			return nil, fmt.Errorf("%w: argument decode failed", agentcommon.ErrABIDecode)
		}
		if len(decoded) > p.maxFeeds {
			return nil, fmt.Errorf("%w: batch over the cap (cap set at activation rehearsal)", agentcommon.ErrBatchTooLarge)
		}
		feeds := make([]Feed, len(decoded))
		for i, f := range decoded {
			if f.Confidence == nil || f.Confidence.Sign() < 0 || f.Confidence.Cmp(p.maxConfidence) > 0 {
				return nil, fmt.Errorf("%w: invalid confidence", agentcommon.ErrOracleConfidence)
			}
			feeds[i] = Feed{f.Price, f.Confidence, f.Timestamp}
		}
		price, confidence, err := Aggregate(feeds, p.maxFeeds)
		if err != nil {
			return nil, err
		}
		return resolved.Outputs.Pack(price, confidence)
	case SubmitPriceMethod, GetValidatorPriceMethod:
		feed, ok := args[0].([32]byte)
		if !ok {
			return nil, fmt.Errorf("%w: argument decode failed", agentcommon.ErrABIDecode)
		}
		var callerKey [32]byte
		if resolved.Name == SubmitPriceMethod {
			key, bound := p.evm.GetLayerXDid(ctx, caller)
			if !bound {
				return nil, fmt.Errorf("%w: caller is not a validator oracle signer", agentcommon.ErrOracleNotSigner)
			}
			callerKey = key
		}
		replay, ok := ctx.Value(replayKey{}).(replayBinding)
		if !ok || len(replay.validator) == 0 {
			return nil, fmt.Errorf("%w: oracle replay context is unavailable", agentcommon.ErrNoContext)
		}
		validator := replay.validator
		if resolved.Name == SubmitPriceMethod {
			if callerKey != replay.publicKey {
				return nil, fmt.Errorf("%w: caller is not a validator oracle signer", agentcommon.ErrOracleNotSigner)
			}
			digest := sha256.Sum256(callerKey[:])
			if !bytes.Equal(validator, digest[:20]) {
				return nil, fmt.Errorf("%w: caller is not a validator oracle signer", agentcommon.ErrOracleNotSigner)
			}
			price, ok := args[1].(*big.Int)
			if !ok || price == nil {
				return nil, fmt.Errorf("%w: argument decode failed", agentcommon.ErrABIDecode)
			}
			if price.Sign() <= 0 || price.BitLen() > 128 {
				return nil, fmt.Errorf("%w: invalid price", agentcommon.ErrOraclePrice)
			}
			confidence, ok := args[2].(*big.Int)
			if !ok || confidence == nil {
				return nil, fmt.Errorf("%w: argument decode failed", agentcommon.ErrABIDecode)
			}
			if confidence.Sign() < 0 || confidence.Cmp(p.maxConfidence) > 0 || !confidence.IsUint64() {
				return nil, fmt.Errorf("%w: invalid confidence", agentcommon.ErrOracleConfidence)
			}
			if err := p.exchange.SubmitOracleObservation(ctx, validator, feed, sdk.NewIntFromBigInt(price), confidence.Uint64()); err != nil {
				if errors.Is(err, lxphost.Code(-729)) {
					return nil, fmt.Errorf("%w: caller is not a validator oracle signer", agentcommon.ErrOracleNotSigner)
				}
				return nil, err
			}
			return resolved.Outputs.Pack()
		}
		committed, found, err := p.exchange.CommittedOraclePrice(ctx, validator, feed)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("%w: no feeds provided", agentcommon.ErrOracleNoFeeds)
		}
		return resolved.Outputs.Pack(committed.Price.BigInt(), big.NewInt(1), new(big.Int).SetUint64(committed.ObservedAt/1000))
	default:
		return nil, fmt.Errorf("%w: unknown oracle method", agentcommon.ErrUnknownMethod)
	}
}
