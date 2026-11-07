#include "layerx/lx_stream_read.h"

lxp_result lx_stream_accrued_read(lxp_module_ctx *ctx,
                                  const uint8_t stream_id[32],
                                  uint64_t timestamp_ms,
                                  lxp_u128 *accrued_total,
                                  lxp_u128 *unsettled)
{
    lx_stream_record record;
    lxp_u128 newly_accrued;
    lxp_u128 pending;
    lxp_result status;

    if (accrued_total == NULL || unsettled == NULL)
        return LXP_ERR_NON_CANONICAL;
    status = lx_stream_load(ctx, stream_id, &record);
    if (status != LXP_OK) return status;
    if (record.mode == LX_STREAM_MODE_TIME) {
        status = lx_stream_accrue(&record, timestamp_ms, &newly_accrued);
        if (status != LXP_OK) return status;
    }
    status = lxp_u128_sub(record.accrued_total, record.settled_total, &pending);
    if (status != LXP_OK) return LXP_FATAL_INVARIANT;
    *accrued_total = record.accrued_total;
    *unsettled = pending;
    return LXP_OK;
}
