#include "lxp_gateway_internal.h"

#ifdef LXP_TESTING
static _Thread_local lxp_gateway_transaction_boundary test_failure_boundary;
void lxp_gateway_receive_test_fail_after(
    lxp_gateway_transaction_boundary boundary)
{
    test_failure_boundary = boundary;
}
#endif
