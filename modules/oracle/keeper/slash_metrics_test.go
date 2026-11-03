package keeper_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
)

func TestSlashRecordsOtelCounterOnly(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	input := testutils.CreateTestInput(t)
	ctx := input.Ctx.WithBlockHeight(10)
	params := input.OracleKeeper.GetParams(ctx)
	params.MinValidPerWindow = sdk.NewDecWithPrec(5, 1)
	params.SlashFraction = sdk.NewDecWithPrec(1, 2)
	input.OracleKeeper.SetParams(ctx, params)

	operator := testutils.ValAddrs[0]
	amount := sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction)
	_, err := staking.NewHandler(input.StakingKeeper)(ctx,
		testutils.NewTestMsgCreateValidator(operator, testutils.ValPubKeys[0], amount))
	require.NoError(t, err)
	staking.EndBlocker(ctx, input.StakingKeeper)
	validator, found := input.StakingKeeper.GetValidator(ctx, operator)
	require.True(t, found)
	require.True(t, validator.IsBonded())
	require.False(t, validator.IsJailed())
	consAddr, err := validator.GetConsAddr()
	require.NoError(t, err)

	input.OracleKeeper.SetVotePenaltyCounter(ctx, operator, 1, 0, 0)
	input.OracleKeeper.SlashAndResetCounters(ctx)
	validator, found = input.StakingKeeper.GetValidator(ctx, operator)
	require.True(t, found)
	require.True(t, validator.IsJailed())
	require.Equal(t, amount.Sub(params.SlashFraction.MulInt(amount).TruncateInt()), validator.GetBondedTokens())

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	var slashMetrics int
	for _, scope := range data.ScopeMetrics {
		if scope.Scope.Name != "oracle_keeper" {
			continue
		}
		for _, metric := range scope.Metrics {
			if metric.Name != "oracle_validator_slashed" {
				continue
			}
			slashMetrics++
			counter, ok := metric.Data.(metricdata.Sum[int64])
			require.True(t, ok, "slash metric must be an integer counter")
			require.True(t, counter.IsMonotonic)
			require.Equal(t, metricdata.CumulativeTemporality, counter.Temporality)
			require.Len(t, counter.DataPoints, 1)
			point := counter.DataPoints[0]
			require.Equal(t, int64(1), point.Value)
			require.Equal(t, attribute.NewSet(
				attribute.String("type", "oracle"),
				attribute.String("validator", consAddr.String()),
			), point.Attributes)
		}
	}
	require.Equal(t, 1, slashMetrics, "oracle slash counter must be recorded exactly once")

	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)
	require.Contains(t, packages, "keeper")
	for name, file := range packages["keeper"].Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if identifier, ok := node.(*ast.Ident); ok {
				require.NotEqual(t, "IncrValidatorSlashedCounter", identifier.Name,
					"keeper must not reference the legacy slash counter in %s", name)
			}
			return true
		})
	}
}
