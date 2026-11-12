package l16

import (
	"strings"
	"testing"
)

func TestPortfolioBatchChoosesLatestSourceAcrossTokens(t *testing.T) {
	fill, p := inventoryWireFixture(t, 1, sideBuy)
	first := portfolioRow{LogicalPartition: p.Positions[0].PartitionKey, Revision: 1, Inventory: Empty(), Position: p.Positions[0], Fill: fill}
	second := first
	second.Fill.Token.Address = "0x" + strings.Repeat("7", 40)
	second.LogicalPartition = strings.Join([]string{"125", strings.Split(first.LogicalPartition, ":")[1], second.Fill.Token.Address}, ":")
	second.Position.PartitionKey = second.LogicalPartition
	second.Fill.Position.Height = "101"
	second.Position.SourceOrder = second.Fill.Position
	second.Revision = 2
	owner := strings.Split(first.LogicalPartition, ":")[1]
	for _, rows := range [][]portfolioRow{{first, second}, {second, first}} {
		got, watermark, _, err := portfolioBatch("g1", owner, p.Positions[0].EffectIdentity, rows)
		if err != nil || watermark.Height != "101" || got.SummaryRevision != "3" || len(got.Changes) != 2 || got.Summary.PositionCount != 2 || got.Summary.UnpricedAssetCount != 0 {
			t.Fatal(got, watermark, err)
		}
	}
}
