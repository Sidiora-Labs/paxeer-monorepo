package source

import "context"

// Test-only access to the real health monitor, with no alternate health logic.
func ProbeHealthCheck(pool *RPC, ctx context.Context) { pool.runHealthCheck(ctx) }
func ProbeRankedIndices(pool *RPC) []int              { return pool.rankedIndices() }
func ProbeFailures(pool *RPC, index int) int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.slots[index].consecutiveFailures
}
