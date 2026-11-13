package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPoolOptionAndRawAmountBoundaries(t *testing.T) {
	for _, option := range []PoolOptions{{Transport: "unknown"}, {Transport: TransportTransactionPooler, QueryClass: Migration}, {Service: strings.Repeat("a", 65), Instance: "one"}, {LockTimeout: time.Nanosecond}} {
		if err := option.Validate(); err == nil {
			t.Fatal("unbounded pool option admitted")
		}
	}
	if pool, err := NewPool(context.Background(), "", PoolOptions{MaxConns: -1}); err == nil || pool != nil {
		t.Fatal("invalid pool options created pool")
	}
	statement, lock := (PoolOptions{QueryClass: Batch}).Deadlines()
	if statement != 30*time.Second || lock != time.Second {
		t.Fatalf("batch deadlines: %s %s", statement, lock)
	}
	if minimum, err := BigintMin("3", "5"); err != nil || minimum != "3" {
		t.Fatalf("minimum amount: %s %v", minimum, err)
	}
	if err := ValidateRawUint256(strings.Repeat("9", 78)); err == nil {
		t.Fatal("uint256 overflow admitted")
	}
}
