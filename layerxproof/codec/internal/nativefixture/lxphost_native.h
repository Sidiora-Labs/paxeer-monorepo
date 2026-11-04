
#include <stdlib.h>
#include <string.h>
#include "layerx/lx_asset.h"
#include "layerx/lx_budget.h"
#include "layerx/lx_escrow.h"
#include "layerx/lx_perps.h"
#include "layerx/lx_service.h"
#include "layerx/lx_spot.h"
#include "layerx/lx_stream.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/lxp_protocol.h"
#include "layerx/programs.h"

enum { LXPHOST_ARENA_BYTES = 4194304 };

typedef struct lxphost_envelope {
    lxp_activity activity;
    uint8_t did[LXP_MAX_DID_LENGTH];
    uint8_t authority[32];
    uint8_t signature[128];
    uint8_t payload[LXP_MAX_PAYLOAD_BYTES];
} lxphost_envelope;

typedef struct lxphost_node {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lxp_fee_params fees;
    lxp_authority_resolved authority;
    lxp_authority_scope authority_scope;
    bool authority_bound;
    lxp_kernel_execution execution;
    lxp_receipt receipt;
    lxp_arena arena;
    lxphost_envelope envelope;
    lx_account_registry accounts;
    lx_asset_registry assets;
    lxp_transfer_asset_state transfer_assets[LX_ASSET_REGISTRY_CAPACITY];
    lx_asset_runtime asset_runtime;
    lx_escrow_runtime escrow_runtime;
    lx_programs_transfer_runtime programs_runtime;
    uint64_t parameters;
    uint64_t epoch;
    uint16_t programs_guest_abi;
    int store_ready;
    uint8_t arena_bytes[LXPHOST_ARENA_BYTES];
    lxp_verified_receipt_index verified_receipts;
} lxphost_node;

typedef struct lxphost_service_node {
    lxphost_node node;
    lx_service_runtime service_runtime;
    lxp_arena escrow_arena;
    uint8_t escrow_arena_bytes[LXPHOST_ARENA_BYTES];
} lxphost_service_node;

typedef struct lxphost_kv_view {
    uint16_t module_id;
    const uint8_t *key;
    size_t key_length;
    const uint8_t *value;
    size_t value_length;
} lxphost_kv_view;

typedef struct lxphost_outcome {
    int32_t result_code;
    uint64_t global_sequence;
    uint64_t timestamp;
    uint64_t fee_hi;
    uint64_t fee_lo;
    const lxp_effect *effects;
    size_t effect_count;
    const lxp_applied_transfer_leg *applied_legs;
    size_t applied_leg_count;
    uint8_t asset[32];
    lxp_u128 amount;
    uint8_t from[32];
    uint8_t to[32];
    lxp_program_outcome program_outcome;
    uint8_t previous_state_root[32];
    uint8_t resulting_state_root[32];
    const uint8_t *encoded;
    size_t encoded_length;
} lxphost_outcome;

static void lxphost_release_blobs(lxp_kernel *kernel)
{
    size_t i;
    for (i = 0U; i < kernel->blob_count && i < (size_t)LXP_KERNEL_MAX_BLOBS; ++i) {
        free(kernel->blobs[i].bytes);
        kernel->blobs[i].bytes = NULL;
    }
    kernel->blob_count = 0U;
}

static lxphost_node *lxphost_node_new(uint64_t epoch,
                                       uint16_t programs_guest_abi,
                                       int32_t *status)
{
    lxphost_node *node = (lxphost_node *)calloc(1U, sizeof(lxphost_service_node));
    if (node == NULL) {
        *status = LXP_ERR_ARENA_EXHAUSTED;
        return NULL;
    }
    node->epoch = epoch;
    node->programs_guest_abi = programs_guest_abi;
    node->parameters = 1U;
    *status = lxp_kernel_create(&node->kernel, &node->store, &node->journal,
                                &node->parameters, epoch);
    if (*status != LXP_OK) {
        free(node);
        return NULL;
    }
    return node;
}

static void lxphost_node_free(lxphost_node *node)
{
    if (node == NULL) return;
    lxphost_release_blobs(&node->kernel);
    if (node->store_ready != 0) (void)lxp_state_store_destroy(&node->store);
    lx_account_registry_release(&node->accounts);
    free(node);
}

static lxphost_envelope *lxphost_node_envelope(lxphost_node *node)
{
    return &node->envelope;
}

static int32_t lxphost_reset(lxphost_node *node, uint64_t first_sequence)
{
    uint64_t epoch = node->epoch;
    uint16_t programs_guest_abi = node->programs_guest_abi;
    int32_t status;
    lxphost_release_blobs(&node->kernel);
    if (node->store_ready != 0) {
        node->store_ready = 0;
        status = lxp_state_store_destroy(&node->store);
        if (status != LXP_OK) return status;
    }
    lx_account_registry_release(&node->accounts);
    (void)memset(node, 0, sizeof(*node));
    node->epoch = epoch;
    node->programs_guest_abi = programs_guest_abi;
    node->parameters = 1U;
    status = lxp_verified_receipt_index_init(&node->verified_receipts);
    if (status != LXP_OK) return status;
    node->fees.version = 1U;
    node->fees.multiplier_basis_points = 10000U;
    status = lx_account_registry_init(&node->accounts);
    if (status == LXP_OK) status = lx_asset_registry_init(&node->assets, 0U);
    if (status != LXP_OK) return status;
    status = lxp_state_store_init(&node->store, first_sequence);
    if (status != LXP_OK) return status;
    node->store_ready = 1;
    status = lxp_state_store_bind_accounts(&node->store, &node->accounts);
    if (status == LXP_OK)
        status = lxp_state_store_require_account_root(&node->store);
    if (status == LXP_OK)
        status = lxp_arena_init(&node->arena, node->arena_bytes,
                                sizeof(node->arena_bytes));
    if (status == LXP_OK)
        status = lxp_kernel_create(&node->kernel, &node->store, &node->journal,
                                   &node->parameters, epoch);
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lxp_governance_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            programs_guest_abi == LX_PROGRAMS_GUEST_ABI_V5_VERSION ?
                                                programs_module_registration_v5() :
                                                programs_module_registration_v4_storage_retirement());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_asset_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_budget_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_escrow_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_perps_oracle_transport_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_spot_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_stream_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&node->kernel,
                                            lx_service_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_set_capabilities(&node->kernel, NULL,
                                             lxp_kernel_canonical_ledger_apply);
    return status;
}

