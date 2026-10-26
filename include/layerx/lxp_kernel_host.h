#ifndef LAYERX_LXP_KERNEL_HOST_H
#define LAYERX_LXP_KERNEL_HOST_H

#include "layerx/lxp_kernel.h"

lxp_result lxp_kernel_module_kv_load(lxp_kernel *kernel,
                                     const lxp_module_kv_entry *rows,
                                     size_t count);

#endif
