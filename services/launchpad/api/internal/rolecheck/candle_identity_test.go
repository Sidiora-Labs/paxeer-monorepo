package rolecheck

import (
	"strings"
	"testing"
)

func TestCandleIdentityReadAuthority(t *testing.T) {
	statements, err := Statements()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, statement := range statements {
		if statement.Function != "AccountingReadModel.Candles" {
			continue
		}
		count++
		if len(statement.Kinds) != 1 || statement.Kinds[0] != "SELECT" && statement.Kinds[0] != "SET" {
			t.Fatalf("candle history statement: %+v", statement)
		}
		if strings.Contains(statement.SQL, "chart_exact") && (!strings.Contains(statement.SQL, "chart_exact_v1") || !strings.Contains(statement.SQL, "ORDER BY start ASC LIMIT $5")) {
			t.Fatalf("candle history source: %s", statement.SQL)
		}
	}
	if count != 2 {
		t.Fatalf("candle history has %d registered statements", count)
	}
}
