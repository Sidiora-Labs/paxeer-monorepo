#ifndef LAYERX_LX_STREAM_READ_H
#define LAYERX_LX_STREAM_READ_H

#include "layerx/lx_stream.h"

lxp_result lx_stream_accrued_read(lxp_module_ctx *ctx,
                                  const uint8_t stream_id[32],
                                  uint64_t timestamp_ms,
                                  lxp_u128 *accrued_total,
                                  lxp_u128 *unsettled);

#endif
