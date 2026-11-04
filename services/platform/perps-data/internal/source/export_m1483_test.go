package source

import "context"

// Test-only access to the real health monitor, with no alternate health logic.
func Repair1483HealthCheck(pool *RPC, ctx context.Context) { pool.runHealthCheck(ctx) }
func Repair1483RankedIndices(pool *RPC) []int              { return pool.rankedIndices() }
func Repair1483Failures(pool *RPC, index int) int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.slots[index].consecutiveFailures
}
