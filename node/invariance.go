package app

import (
	"context"
	"fmt"
	"time"

	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/commitment"
	armonmetrics "github.com/armon/go-metrics"
	"github.com/spf13/cast"
	"go.opentelemetry.io/otel/attribute"
	otelmetrics "go.opentelemetry.io/otel/metric"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"

	paxmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/utils/metrics"
)

type LightInvarianceConfig struct {
	SupplyEnabled bool `mapstructure:"supply_enabled"`
}

var DefaultLightInvarianceConfig = LightInvarianceConfig{
	SupplyEnabled: true,
}

const (
	flagSupplyEnabled = "light_invariance.supply_enabled"
)

func ReadLightInvarianceConfig(opts servertypes.AppOptions) (LightInvarianceConfig, error) {
	cfg := DefaultLightInvarianceConfig // copy
	var err error
	if v := opts.Get(flagSupplyEnabled); v != nil {
		if cfg.SupplyEnabled, err = cast.ToBoolE(v); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func (app *App) LightInvarianceChecks(ctx context.Context, cms sdk.CommitMultiStore, config LightInvarianceConfig) (err error) {
	defer func() {
		if err != nil {
			app.lightInvarianceErr = err
		}
	}()
	// Skip invariance checks when mock_balances is enabled since we fake balances
	// without updating the actual store, which would fail the supply check.
	if MockBalancesEnabled {
		return nil
	}
	if config.SupplyEnabled {
		return app.LightInvarianceTotalSupply(ctx, cms)
	}
	return nil
}

// LightInvarianceTotalSupply checks the pending changes in a MemIAVL bank store
// against its committed state for the configured base denomination and wei only.
// It does not run keeper.TotalSupply or validate other denominations; see
// sdk/x/bank/spec/README.md for registration, configuration and error coverage.
func (app *App) LightInvarianceTotalSupply(ctx context.Context, cms sdk.CommitMultiStore) error {
	invarianceStart := time.Now()
	defer func() {
		armonmetrics.MeasureSince([]string{"pax", "lightinvariance_supply", "milliseconds"}, invarianceStart.UTC()) // TODO(PLT-327): remove once app_lightinvariance_supply_duration_seconds verified
		appMetrics.invarianceDuration.Record(ctx, time.Since(invarianceStart).Seconds())
	}()
	ckv, ok := cms.GetStore(app.BankKeeper.GetStoreKey()).(*commitment.Store)
	if !ok {
		return fmt.Errorf("light invariance: unsupported bank store %T", cms.GetStore(app.BankKeeper.GetStoreKey()))
	}
	balanceChangePairs := ckv.GetChangedPairs(banktypes.BalancesPrefix)
	uhpxPostTotal := sdk.ZeroInt()
	uhpxChangedAddr := []sdk.AccAddress{}
	for _, p := range balanceChangePairs {
		if len(p.Key) < 2 {
			// Reject malformed keys: conservation cannot be checked.
			paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "invalid_changed_key"}, 1, []armonmetrics.Label{{Name: "type", Value: "pax"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_invalid_key_total verified
			appMetrics.invarianceInvalidKey.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "pax")))
			return fmt.Errorf("light invariance: invalid changed pair key for uhpx %X", p.Key)
		}
		addrLen := int(p.Key[1])
		if len(p.Key) < addrLen+2 {
			// Reject malformed key lengths: conservation cannot be checked.
			paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "invalid_changed_key"}, 1, []armonmetrics.Label{{Name: "type", Value: "pax"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_invalid_key_total verified
			appMetrics.invarianceInvalidKey.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "pax")))
			return fmt.Errorf("light invariance: invalid changed pair key for uhpx %X", p.Key)
		}
		addr := p.Key[2 : addrLen+2]
		denom := p.Key[addrLen+2:]
		if string(denom) != sdk.MustGetBaseDenom() {
			continue
		}
		if !p.Delete {
			var balance sdk.Coin
			if err := balance.Unmarshal(p.Value); err != nil {
				paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "unmarshal_failure"}, 1, []armonmetrics.Label{{Name: "type", Value: "uhpx"}, {Name: "step", Value: "post_block"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_unmarshal_failure_total verified
				appMetrics.invarianceUnmarshalFail.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "uhpx"), attribute.String("step", "post_block")))
				return fmt.Errorf("light invariance: failed to unmarshal balance: %w", err)
			}
			if balance.Amount.IsNil() {
				return fmt.Errorf("light invariance: nil balance amount for %X", addr)
			}
			if balance.Amount.IsNegative() {
				panic(fmt.Sprintf("negative balance found for addr %s: %s", sdk.AccAddress(addr).String(), balance.String()))
			}
			uhpxPostTotal = uhpxPostTotal.Add(balance.Amount)
		}
		uhpxChangedAddr = append(uhpxChangedAddr, addr)
	}
	uhpxPreTotal := sdk.ZeroInt()
	for _, a := range uhpxChangedAddr {
		key := append(banktypes.CreateAccountBalancesPrefix(a), []byte(sdk.MustGetBaseDenom())...)
		val := ckv.Get(key)
		if val == nil {
			continue
		}
		var balance sdk.Coin
		if err := balance.Unmarshal(val); err != nil {
			paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "unmarshal_failure"}, 1, []armonmetrics.Label{{Name: "type", Value: "uhpx"}, {Name: "step", Value: "pre_block"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_unmarshal_failure_total verified
			appMetrics.invarianceUnmarshalFail.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "uhpx"), attribute.String("step", "pre_block")))
			return fmt.Errorf("light invariance: failed to unmarshal preblock balance: %w", err)
		}
		if balance.Amount.IsNil() {
			return fmt.Errorf("light invariance: nil preblock balance amount for %X", a)
		}
		uhpxPreTotal = uhpxPreTotal.Add(balance.Amount)
	}
	weiChangePairs := ckv.GetChangedPairs(banktypes.WeiBalancesPrefix)
	weiPostTotal := sdk.ZeroInt()
	weiChangedAddrs := []sdk.AccAddress{}
	for _, p := range weiChangePairs {
		var amt sdk.Int
		if len(p.Key) < 1 {
			paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "invalid_changed_key"}, 1, []armonmetrics.Label{{Name: "type", Value: "wei"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_invalid_key_total verified
			appMetrics.invarianceInvalidKey.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "wei")))
			return fmt.Errorf("light invariance: invalid changed pair key %X", p.Key)
		}
		if !p.Delete {
			if err := amt.Unmarshal(p.Value); err != nil {
				paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "unmarshal_failure"}, 1, []armonmetrics.Label{{Name: "type", Value: "wei"}, {Name: "step", Value: "post_block"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_unmarshal_failure_total verified
				appMetrics.invarianceUnmarshalFail.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "wei"), attribute.String("step", "post_block")))
				return fmt.Errorf("light invariance: failed to unmarshal wei balance: %w", err)
			}
			weiPostTotal = weiPostTotal.Add(amt)
			if amt.IsNegative() {
				panic(fmt.Sprintf("negative wei balance found for addr %s: %s", sdk.AccAddress(p.Key[1:]).String(), amt.String()))
			}
		}
		weiChangedAddrs = append(weiChangedAddrs, p.Key[1:])
	}
	weiPreTotal := sdk.ZeroInt()
	for _, a := range weiChangedAddrs {
		key := append(banktypes.WeiBalancesPrefix, a...)
		val := ckv.Get(key)
		if val == nil {
			continue
		}
		var amt sdk.Int
		if err := amt.Unmarshal(val); err != nil {
			paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "unmarshal_failure"}, 1, []armonmetrics.Label{{Name: "type", Value: "wei"}, {Name: "step", Value: "pre_block"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_unmarshal_failure_total verified
			appMetrics.invarianceUnmarshalFail.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "wei"), attribute.String("step", "pre_block")))
			return fmt.Errorf("light invariance: failed to unmarshal preblock wei balance: %w", err)
		}
		weiPreTotal = weiPreTotal.Add(amt)
	}
	totalSupplyChangePairs := ckv.GetChangedPairs(banktypes.SupplyKey)
	supplyChanged := sdk.ZeroInt()
	preTotalSupply := sdk.ZeroInt()
	if bz := ckv.Get(append(banktypes.SupplyKey, []byte(sdk.MustGetBaseDenom())...)); bz != nil {
		var amt sdk.Int
		if err := amt.Unmarshal(bz); err != nil {
			paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "unmarshal_failure"}, 1, []armonmetrics.Label{{Name: "type", Value: "total_supply"}, {Name: "step", Value: "pre_block"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_unmarshal_failure_total verified
			appMetrics.invarianceUnmarshalFail.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "total_supply"), attribute.String("step", "pre_block")))
			return fmt.Errorf("light invariance: failed to unmarshal pre total supply: %w", err)
		}
		preTotalSupply = amt
	}
	for _, p := range totalSupplyChangePairs {
		if string(p.Key[1:]) == sdk.MustGetBaseDenom() {
			if p.Delete {
				supplyChanged = preTotalSupply.Neg()
			} else {
				var amt sdk.Int
				if err := amt.Unmarshal(p.Value); err != nil {
					paxmetrics.SafeMetricsIncrCounterWithLabels([]string{"pax", "lightinvariance_supply", "unmarshal_failure"}, 1, []armonmetrics.Label{{Name: "type", Value: "total_supply"}, {Name: "step", Value: "post_block"}}) // TODO(PLT-327): remove once app_lightinvariance_supply_unmarshal_failure_total verified
					appMetrics.invarianceUnmarshalFail.Add(ctx, 1, otelmetrics.WithAttributes(attribute.String("type", "total_supply"), attribute.String("step", "post_block")))
					return fmt.Errorf("light invariance: failed to unmarshal total supply: %w", err)
				} else {
					supplyChanged = amt.Sub(preTotalSupply)
				}
			}
			break
		}
	}
	weiDiff := weiPostTotal.Sub(weiPreTotal)
	weiDiffInUhpx, weiDiffRemainder := bankkeeper.SplitUhpxWeiAmount(weiDiff)
	if !weiDiffRemainder.IsZero() {
		panic(fmt.Sprintf("non-zero wei diff found! Pre-block wei total %s, post-block wei total %s", weiPreTotal, weiPostTotal))
	}
	// Formula: uhpxDiff = uhpxPreTotal - uhpxPostTotal - weiDiffInUhpx + supplyChanged
	// If money is conserved, this should be zero
	// uhpxPreTotal - uhpxPostTotal = how much uhpx left balances (negative means uhpx entered balances)
	// weiDiffInUhpx = how much uhpx was moved to wei balances
	// supplyChanged = how much new uhpx was minted
	uhpxDiff := uhpxPreTotal.Sub(uhpxPostTotal).Sub(weiDiffInUhpx).Add(supplyChanged)
	if !uhpxDiff.IsZero() {
		panic(fmt.Sprintf("unexpected uhpx balance total found! Pre-block uhpx total %s wei total %s total supply %s, post-block uhpx total %s wei total %s total supply %s", uhpxPreTotal, weiPreTotal, preTotalSupply, uhpxPostTotal, weiPostTotal, preTotalSupply.Add(supplyChanged)))
	}
	return nil
}
