package ante_test

import "testing"

func TestProducerGrantRevokeRegrant(t *testing.T) {
	testAuthzExecProducerGrant(t, false)
}

func TestProducerGrantScopeBudgets(t *testing.T) {
	testAuthzExecProducerGrant(t, true)
}
