package sequencer

import (
	"log/slog"
	"sync"
	"testing"
)

func AttachLeaseForTest(a *Accepter, holder *LeaseHolder) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lease = holder
}

func PauseLeaseRenewalsForTest(t *testing.T, holder *LeaseHolder) func() {
	t.Helper()
	holder.renewMu.Lock()
	var once sync.Once
	resume := func() { once.Do(holder.renewMu.Unlock) }
	t.Cleanup(resume)
	return resume
}

func SetLeaseNodeLoggerForTest(n *Node, logger *slog.Logger) {
	n.logger = logger
}
