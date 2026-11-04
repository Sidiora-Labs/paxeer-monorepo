package ante_test

import "testing"

func TestRepairM_1420_ProducerGrantRevokeRegrant(t *testing.T) {
	testAuthzExecProducerGrant(t, false)
}

func TestRepairM_1420_ProducerGrantScopeBudgets(t *testing.T) {
	testAuthzExecProducerGrant(t, true)
}
