//go:build cgo

package lxphost

/*
#cgo CFLAGS: -I${SRCDIR}/../../include -I${SRCDIR}/../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../build -L${SRCDIR}/../../programs/target/release -L${SRCDIR}/../../programs/target/debug -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include <stdlib.h>
#include <string.h>
#include "layerx/lx_asset.h"
#include "layerx/lx_budget.h"
#include "layerx/lx_escrow.h"
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
    int store_ready;
    uint8_t arena_bytes[LXPHOST_ARENA_BYTES];
} lxphost_node;

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

static lxphost_node *lxphost_node_new(uint64_t epoch, int32_t *status)
{
    lxphost_node *node = (lxphost_node *)calloc(1U, sizeof(*node));
    if (node == NULL) {
        *status = LXP_ERR_ARENA_EXHAUSTED;
        return NULL;
    }
    node->epoch = epoch;
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
    node->parameters = 1U;
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
    if (status == LXP_OK) status = lxp_idempotency_reserve(&node->store);
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
                                            programs_module_registration());
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
        status = lxp_kernel_set_capabilities(&node->kernel, NULL,
                                             lxp_kernel_canonical_ledger_apply);
    return status;
}

static void *lxphost_accounts(lxphost_node *node)
{
    return &node->accounts;
}

static int32_t lxphost_bind_assets(lxphost_node *node, uint32_t network_id,
                                   bool binding_authority)
{
    size_t count = 0U;
    size_t i;
    int32_t status = lx_asset_committed_records(
        &node->kernel, node->assets.assets, (size_t)LX_ASSET_REGISTRY_CAPACITY,
        &count);
    if (status != LXP_OK) return status;
    if (count > (size_t)LX_ASSET_REGISTRY_CAPACITY) return LXP_FATAL_INVARIANT;
    node->assets.count = count;
    for (i = 0U; i < count; ++i) {
        status = lx_asset_transfer_state(&node->assets.assets[i],
                                         &node->transfer_assets[i]);
        if (status != LXP_OK) return status;
    }
    (void)memset(&node->asset_runtime, 0, sizeof(node->asset_runtime));
    node->asset_runtime.accounts = &node->accounts;
    node->asset_runtime.assets = node->assets.assets;
    node->asset_runtime.asset_count = count;
    node->asset_runtime.transfer_assets = node->transfer_assets;
    node->asset_runtime.transfer_asset_count = count;
    node->asset_runtime.network_id = network_id;
    node->asset_runtime.protocol_version =
        (uint16_t)LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    node->asset_runtime.binding_authority = binding_authority;
    node->kernel.module_runtime[LXP_MODULE_ASSET] = &node->asset_runtime;
    node->escrow_runtime.accounts = &node->accounts;
    node->escrow_runtime.assets = &node->assets;
    node->kernel.module_runtime[LXP_MODULE_ESCROW] = &node->escrow_runtime;
    (void)memset(&node->programs_runtime, 0, sizeof(node->programs_runtime));
    node->programs_runtime.accounts = &node->accounts;
    node->programs_runtime.assets = node->transfer_assets;
    node->programs_runtime.asset_count = count;
    node->programs_runtime.resolve_metering_schedule =
        lxp_programs_metering_resolve_runtime;
    node->programs_runtime.metering_schedule_context = &node->kernel;
    node->programs_runtime.resolve_occupancy_parameters =
        lxp_programs_fee_governance_resolve_runtime;
    node->programs_runtime.occupancy_parameter_context = &node->kernel;
    node->kernel.module_runtime[LXP_MODULE_PROGRAMS] = &node->programs_runtime;
    return LXP_OK;
}

static int32_t lxphost_envelope_write(
    lxphost_envelope *envelope, uint16_t protocol_version, uint32_t network_id,
    uint32_t activity_type, const uint8_t *did, size_t did_length,
    const uint8_t authority[32], uint64_t account_sequence,
    uint64_t not_before, uint64_t not_after,
    const uint8_t idempotency_key[32], const uint8_t *payload,
    size_t payload_length, const uint8_t *signature, size_t signature_length)
{
    lxp_activity *activity = &envelope->activity;
    if (did == NULL || did_length == 0U || did_length > sizeof(envelope->did) ||
        (payload == NULL && payload_length != 0U) ||
        payload_length > sizeof(envelope->payload) ||
        (signature == NULL && signature_length != 0U) ||
        signature_length > sizeof(envelope->signature))
        return LXP_ERR_MALFORMED_ENVELOPE;
    (void)memset(envelope, 0, sizeof(*envelope));
    (void)memcpy(envelope->did, did, did_length);
    (void)memcpy(envelope->authority, authority, 32U);
    if (payload_length != 0U)
        (void)memcpy(envelope->payload, payload, payload_length);
    if (signature_length != 0U)
        (void)memcpy(envelope->signature, signature, signature_length);
    activity->protocol_version = protocol_version;
    activity->network_id = network_id;
    activity->activity_type = activity_type;
    activity->actor_did = (lxp_byte_span){envelope->did, did_length};
    activity->authority = (lxp_byte_span){envelope->authority, 32U};
    activity->account_sequence = account_sequence;
    activity->timestamp_bound = (lxp_timestamp_bound){not_before, not_after};
    (void)memcpy(activity->idempotency_key, idempotency_key, 32U);
    activity->payload = (lxp_byte_span){envelope->payload, payload_length};
    activity->signature = (lxp_byte_span){envelope->signature, signature_length};
    return lxp_hash_payload(envelope->payload, payload_length,
                            activity->payload_hash);
}

static int32_t lxphost_envelope_preimage(const lxphost_envelope *envelope,
                                         uint8_t preimage[32])
{
    return lxp_activity_signing_preimage(&envelope->activity, preimage);
}

static int32_t lxphost_authority_write(lxphost_node *node, const uint8_t actor[32],
                                    const uint8_t principal[32],
                                    const uint8_t verified_key[32], bool bound,
                                    uint8_t kind, const uint8_t grant_id[32],
                                    const uint8_t authority_hash[32],
                                    const lxp_authority_scope *scope)
{
    (void)memset(&node->authority, 0, sizeof(node->authority));
    (void)memset(&node->authority_scope, 0, sizeof(node->authority_scope));
    node->authority_bound = false;
    if (kind == 0U) kind = LXP_AUTHORITY_OWNER;
    if (kind < LXP_AUTHORITY_OWNER || kind > LXP_AUTHORITY_KIND_TIMELOCK)
        return LXP_ERR_NON_CANONICAL;
    (void)memcpy(node->authority.actor, actor, 32U);
    (void)memcpy(node->authority.principal, principal, 32U);
    (void)memcpy(node->authority.verified_key, verified_key, 32U);
    (void)memcpy(node->authority.grant_id, grant_id, 32U);
    (void)memcpy(node->authority.authority_hash, authority_hash, 32U);
    node->authority.kind = (lxp_authority_kind)kind;
    if (scope != NULL) {
        node->authority_scope = *scope;
        node->authority.scope = &node->authority_scope;
    }
    node->authority_bound = bound;
    return LXP_OK;
}

static int32_t lxphost_activity_module(const lxphost_node *node,
                                       uint16_t *module_id)
{
    const lxp_module_registration *registration;
    int32_t status = lxp_kernel_module_for_activity(
        &node->kernel, node->envelope.activity.activity_type,
        node->kernel.epoch, &registration);
    if (status == LXP_OK) *module_id = registration->module_id;
    return status;
}

static int32_t lxphost_load_kv(lxphost_node *node, uint16_t module_id,
                               const uint8_t *key, size_t key_length,
                               const uint8_t *value, size_t value_length)
{
    lxp_module_kv_entry *entry;
    if (module_id == 0U || key == NULL || key_length == 0U ||
        key_length > (size_t)LXP_MODULE_MAX_KEY_BYTES ||
        (value == NULL && value_length != 0U) ||
        value_length > (size_t)LXP_MODULE_MAX_VALUE_BYTES)
        return LXP_FATAL_INVARIANT;
    if (node->kernel.module_kv_count >= (size_t)LXP_KERNEL_MAX_MODULE_KV)
        return LXP_ERR_ARENA_EXHAUSTED;
    entry = &node->kernel.module_kv[node->kernel.module_kv_count++];
    entry->module_id = module_id;
    entry->key_length = (uint16_t)key_length;
    entry->value_length = (uint32_t)value_length;
    (void)memcpy(entry->key, key, key_length);
    if (value_length != 0U) (void)memcpy(entry->value, value, value_length);
    return LXP_OK;
}

typedef struct lxphost_blob_view {
    uint16_t module_id;
    const uint8_t *key;
    const uint8_t *bytes;
    size_t length;
} lxphost_blob_view;

static int32_t lxphost_load_blob(lxphost_node *node, uint16_t module_id,
                                 const uint8_t key[32], const uint8_t *bytes,
                                 size_t length)
{
    lxp_module_blob *blob;
    uint8_t digest[32];
    uint8_t *copy;
    int32_t status;
    if (module_id == 0U || key == NULL || bytes == NULL || length == 0U ||
        length > (size_t)LXP_KERNEL_MAX_BLOB_BYTES ||
        node->kernel.blob_total_bytes > (size_t)LXP_KERNEL_MAX_BLOB_TOTAL_BYTES ||
        length > (size_t)LXP_KERNEL_MAX_BLOB_TOTAL_BYTES -
                     node->kernel.blob_total_bytes)
        return LXP_FATAL_INVARIANT;
    if (node->kernel.blob_count >= (size_t)LXP_KERNEL_MAX_BLOBS)
        return LXP_ERR_ARENA_EXHAUSTED;
    status = lxp_hash_sha256(bytes, length, digest);
    if (status != LXP_OK) return status;
    if (memcmp(digest, key, 32U) != 0) return LXP_FATAL_INVARIANT;
    copy = (uint8_t *)malloc(length);
    if (copy == NULL) return LXP_ERR_ARENA_EXHAUSTED;
    (void)memcpy(copy, bytes, length);
    blob = &node->kernel.blobs[node->kernel.blob_count++];
    (void)memset(blob, 0, sizeof(*blob));
    blob->module_id = module_id;
    (void)memcpy(blob->key, key, 32U);
    blob->length = length;
    blob->bytes = copy;
    node->kernel.blob_total_bytes += length;
    return LXP_OK;
}

static size_t lxphost_blob_count(const lxphost_node *node)
{
    return node->kernel.blob_count;
}

static int32_t lxphost_blob_read(const lxphost_node *node, size_t index,
                                 lxphost_blob_view *view)
{
    const lxp_module_blob *blob;
    if (node->kernel.blob_count > (size_t)LXP_KERNEL_MAX_BLOBS ||
        index >= node->kernel.blob_count)
        return LXP_FATAL_INVARIANT;
    blob = &node->kernel.blobs[index];
    if (blob->module_id == 0U || blob->deleted || blob->bytes == NULL ||
        blob->length == 0U || blob->length > (size_t)LXP_KERNEL_MAX_BLOB_BYTES)
        return LXP_FATAL_INVARIANT;
    view->module_id = blob->module_id;
    view->key = blob->key;
    view->bytes = blob->bytes;
    view->length = blob->length;
    return LXP_OK;
}

static int32_t lxphost_account_kind(const uint8_t *name, size_t name_length,
                                    uint16_t *kind)
{
    lx_account_kind parsed;
    int32_t status;
    if (name == NULL || name_length == 0U ||
        name_length > (size_t)LX_ACCOUNT_NAME_MAX)
        return LXP_ERR_NON_CANONICAL;
    status = lx_account_kind_of(name, name_length, &parsed);
    if (status != LXP_OK) return status;
    *kind = (uint16_t)parsed;
    return LXP_OK;
}

static size_t lxphost_kv_count(const lxphost_node *node)
{
    return node->kernel.module_kv_count;
}

static int32_t lxphost_kv_read(const lxphost_node *node, size_t index,
                               lxphost_kv_view *view)
{
    const lxp_module_kv_entry *entry;
    if (node->kernel.module_kv_count > (size_t)LXP_KERNEL_MAX_MODULE_KV ||
        index >= node->kernel.module_kv_count)
        return LXP_FATAL_INVARIANT;
    entry = &node->kernel.module_kv[index];
    if (entry->module_id == 0U || entry->key_length == 0U ||
        entry->key_length > LXP_MODULE_MAX_KEY_BYTES ||
        entry->value_length > LXP_MODULE_MAX_VALUE_BYTES)
        return LXP_FATAL_INVARIANT;
    view->module_id = entry->module_id;
    view->key = entry->key;
    view->key_length = entry->key_length;
    view->value = entry->value;
    view->value_length = entry->value_length;
    return LXP_OK;
}

static int32_t lxphost_load_idempotency(lxphost_node *node,
                                        const uint8_t key_hash[32],
                                        const uint8_t *bytes, size_t length)
{
    lxp_idempotency_key_state *entry;
    int32_t status;
    if (node->store.idempotency == NULL)
        return LXP_FATAL_INVARIANT;
    if (node->store.idempotency_count >= (size_t)LXP_STATE_MAX_IDEMPOTENCY)
        return LXP_ERR_ARENA_EXHAUSTED;
    entry = &node->store.idempotency[node->store.idempotency_count];
    status = lxp_idempotency_snapshot_decode(entry, bytes, length);
    if (status != LXP_OK) return status;
    (void)memcpy(entry->key_hash, key_hash, 32U);
    ++node->store.idempotency_count;
    return LXP_OK;
}

static size_t lxphost_idempotency_count(const lxphost_node *node)
{
    return node->store.idempotency_count;
}

static int32_t lxphost_idempotency_read(const lxphost_node *node, size_t index,
                                        uint8_t key_hash[32], uint8_t *bytes,
                                        size_t capacity, size_t *length)
{
    const lxp_idempotency_key_state *entry;
    if (node->store.idempotency == NULL ||
        node->store.idempotency_count > (size_t)LXP_STATE_MAX_IDEMPOTENCY ||
        index >= node->store.idempotency_count)
        return LXP_FATAL_INVARIANT;
    entry = &node->store.idempotency[index];
    (void)memcpy(key_hash, entry->key_hash, 32U);
    return lxp_idempotency_snapshot_encode(entry, bytes, capacity, length);
}

static lxp_identity *lxphost_identity_find(lxphost_node *node,
                                           const uint8_t did_id[32])
{
    size_t i;
    for (i = 0U; i < node->identities.count &&
                 i < (size_t)LXP_IDENTITY_STORE_CAPACITY; ++i)
        if (memcmp(node->identities.identities[i].did_id, did_id, 32U) == 0)
            return &node->identities.identities[i];
    return NULL;
}

static int32_t lxphost_bind_actor(lxphost_node *node, uint8_t did_id[32])
{
    const lxp_activity *activity = &node->envelope.activity;
    lxp_identity *identity;
    int32_t status = lxp_governance_identities_restore(&node->kernel,
                                                       &node->identities);
    if (status == LXP_OK)
        status = lxp_did_id_derive(activity->actor_did.bytes,
                                   activity->actor_did.length, did_id);
    if (status != LXP_OK) return status;
    if (lxphost_identity_find(node, did_id) != NULL) return LXP_OK;
    return lxp_identity_register(&node->identities, activity->actor_did.bytes,
                                 activity->actor_did.length,
                                 node->authority.verified_key, &identity);
}

static int32_t lxphost_actor_sequence_set(lxphost_node *node,
                                          const uint8_t did_id[32],
                                          uint64_t sequence)
{
    lxp_identity *identity = lxphost_identity_find(node, did_id);
    if (identity == NULL) return LXP_ERR_UNKNOWN_DID;
    identity->next_sequence = sequence;
    return LXP_OK;
}

static int32_t lxphost_actor_sequence_get(lxphost_node *node,
                                          const uint8_t did_id[32],
                                          uint64_t *sequence)
{
    lxp_identity *identity = lxphost_identity_find(node, did_id);
    if (identity == NULL) return LXP_FATAL_INVARIANT;
    *sequence = identity->next_sequence;
    return LXP_OK;
}

static int32_t lxphost_execute(lxphost_node *node, uint32_t network_id,
                               uint64_t batch_number, uint64_t timestamp_ms,
                               uint64_t timestamp_window, uint64_t sequence,
                               uint32_t parameter_version, uint64_t gas_limit,
                               bool binding_authority)
{
    const lxp_module_registration *registration;
    lxp_kernel_execution *execution = &node->execution;
    int32_t status = lxp_kernel_module_for_activity(
        &node->kernel, node->envelope.activity.activity_type,
        node->kernel.epoch, &registration);
    if (status == LXP_OK)
        status = lxp_state_root(&node->kernel, node->kernel.current_state_root);
    if (status != LXP_OK) return status;
    (void)memset(execution, 0, sizeof(*execution));
    execution->network_id = network_id;
    execution->batch_number = batch_number;
    execution->batch_timestamp_ms = timestamp_ms;
    execution->maximum_timestamp_window = timestamp_window;
    execution->epoch = node->kernel.epoch;
    execution->global_sequence = sequence;
    execution->recorded_module_version = registration->abi_version;
    execution->parameter_version = parameter_version;
    execution->signature_valid = node->envelope.activity.signature.length != 0U ?
        lxp_activity_verify_signature(&node->envelope.activity) == LXP_OK :
        binding_authority;
    execution->identities = &node->identities;
    execution->authority = &node->authority;
    execution->fee_parameters = &node->fees;
    execution->gas_limit = gas_limit;
    execution->arena = &node->arena;
    return lxp_kernel_execute_activity(&node->kernel, &node->envelope.activity,
                                       execution, &node->receipt);
}

static int32_t lxphost_span_copy(lxp_byte_span source, size_t limit,
                                 lxp_byte_span *destination)
{
    uint8_t *copy;
    if (source.length > limit ||
        (source.bytes == NULL && source.length != 0U))
        return LXP_ERR_LENGTH_LIMIT;
    if (source.length == 0U) return LXP_OK;
    copy = malloc(source.length);
    if (copy == NULL) return LXP_ERR_ARENA_EXHAUSTED;
    (void)memcpy(copy, source.bytes, source.length);
    *destination = (lxp_byte_span){copy, source.length};
    return LXP_OK;
}

static void lxphost_outcome_release(lxphost_outcome *outcome)
{
    free((void *)outcome->program_outcome.terminal_payload.bytes);
    free((void *)outcome->program_outcome.call_graph_payload.bytes);
    free((void *)outcome->program_outcome.event_envelope_payload.bytes);
}

static int32_t lxphost_outcome_read(lxphost_node *node, lxphost_outcome *outcome)
{
    lxp_byte_span encoded;
    lxp_receipt receipt;
    int32_t status;
    (void)memset(outcome, 0, sizeof(*outcome));
    if (node->kernel.publication_poisoned || node->journal.open ||
        node->store.count != 0U || node->store.accounts != &node->accounts)
        return LXP_FATAL_INVARIANT;
    receipt = node->receipt;
    if (receipt.program_outcome.present) {
        const lxp_program_outcome *source = &node->receipt.program_outcome;
        lxp_program_outcome *program = &outcome->program_outcome;
        *program = *source;
        program->terminal_payload = (lxp_byte_span){NULL, 0U};
        program->call_graph_payload = (lxp_byte_span){NULL, 0U};
        program->event_envelope_payload = (lxp_byte_span){NULL, 0U};
        status = lxphost_span_copy(source->terminal_payload,
                                   LXP_MAX_ACTIVITY_BYTES,
                                   &program->terminal_payload);
        if (status == LXP_OK)
            status = lxphost_span_copy(source->call_graph_payload,
                                       LXP_MAX_ACTIVITY_BYTES,
                                       &program->call_graph_payload);
        if (status == LXP_OK)
            status = lxphost_span_copy(source->event_envelope_payload,
                                       LXP_PROGRAM_EVENT_LIST_MAX_BYTES,
                                       &program->event_envelope_payload);
        if (status != LXP_OK) return status;
        receipt.program_outcome = *program;
    }
    status = lxp_arena_reset(&node->arena, 0U);
    if (status == LXP_OK)
        status = lxp_receipt_encode(&receipt, false, &node->arena, &encoded);
    if (status != LXP_OK) return status;
    outcome->result_code = receipt.result_code;
    outcome->global_sequence = receipt.global_sequence;
    outcome->timestamp = receipt.timestamp;
    outcome->fee_hi = receipt.fee_charged.hi;
    outcome->fee_lo = receipt.fee_charged.lo;
    outcome->effects = node->receipt.effects.effects;
    outcome->effect_count = receipt.effects.count;
    (void)memcpy(outcome->asset, receipt.asset, 32U);
    outcome->amount = receipt.amount;
    (void)memcpy(outcome->from, receipt.from, 32U);
    (void)memcpy(outcome->to, receipt.to, 32U);
    (void)memcpy(outcome->previous_state_root,
                 receipt.previous_state_root, 32U);
    (void)memcpy(outcome->resulting_state_root,
                 receipt.resulting_state_root, 32U);
    outcome->encoded = encoded.bytes;
    outcome->encoded_length = encoded.length;
    return LXP_OK;
}
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"runtime"
	"sort"
	"sync"
	"unsafe"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func kernelResultName(code int32) string {
	return C.GoString(C.lxp_result_name(C.int32_t(code)))
}

type lxphost_outcome = C.lxphost_outcome

func lxphost_outcome_read(node *C.lxphost_node, outcome *lxphost_outcome) error {
	return result(C.lxphost_outcome_read(node, outcome))
}

func lxphost_activity_module(node *C.lxphost_node) (uint16, error) {
	var module C.uint16_t
	err := result(C.lxphost_activity_module(node, &module))
	return uint16(module), err
}

func lxphost_reset(node *C.lxphost_node, firstSequence uint64) error {
	return result(C.lxphost_reset(node, C.uint64_t(firstSequence)))
}

func writeAuthorityAmount(amount sdk.Int) (C.struct_lxp_u128, error) {
	if amount.IsNil() {
		return C.struct_lxp_u128{}, nil
	}
	if amount.IsNegative() || amount.BigInt().BitLen() > 128 {
		return C.struct_lxp_u128{}, codeNonCanonical
	}
	var encoded [16]byte
	amount.BigInt().FillBytes(encoded[:])
	return C.struct_lxp_u128{hi: C.uint64_t(binary.BigEndian.Uint64(encoded[:8])), lo: C.uint64_t(binary.BigEndian.Uint64(encoded[8:]))}, nil
}

func lxphost_authority_write(node *C.lxphost_node, authority KernelAuthority) error {
	var scope C.struct_lxp_authority_scope
	var scopePtr *C.struct_lxp_authority_scope
	if source := authority.Scope; source != nil {
		amounts := []sdk.Int{source.MaximumPerActivity, source.MaximumTotal, source.SpentTotal, source.MaximumPerPeriod, source.SpentThisPeriod}
		targets := []*C.struct_lxp_u128{&scope.maximum_per_activity, &scope.maximum_total, &scope.spent_total, &scope.maximum_per_period, &scope.spent_this_period}
		for i, amount := range amounts {
			value, err := writeAuthorityAmount(amount)
			if err != nil {
				return err
			}
			*targets[i] = value
		}
		scope.module_mask = C.uint64_t(source.ModuleMask)
		scope.activity_ordinal_min = C.uint16_t(source.ActivityOrdinalMin)
		scope.activity_ordinal_max = C.uint16_t(source.ActivityOrdinalMax)
		for i := range source.AssetID {
			scope.asset_id[i] = C.uint8_t(source.AssetID[i])
		}
		scope.period_length = C.uint64_t(source.PeriodLength)
		scope.period_start = C.uint64_t(source.PeriodStart)
		for i := range source.PurposeHash {
			scope.purpose_hash[i] = C.uint8_t(source.PurposeHash[i])
		}
		scope.earliest_sequence = C.uint64_t(source.EarliestSequence)
		scope.earliest_timestamp = C.uint64_t(source.EarliestTimestamp)
		scope.signer_threshold = C.uint8_t(source.SignerThreshold)
		scope.signer_count = C.uint8_t(source.SignerCount)
		scope.approval_count = C.uint8_t(source.ApprovalCount)
		for i := range source.Signers {
			for j := range source.Signers[i] {
				scope.signers[i][j] = C.uint8_t(source.Signers[i][j])
			}
		}
		for i := range source.Approvals {
			for j := range source.Approvals[i] {
				scope.approvals[i][j] = C.uint8_t(source.Approvals[i][j])
			}
		}
		scopePtr = &scope
	}
	return result(C.lxphost_authority_write(node, bytes32(&authority.Actor), bytes32(&authority.Principal), bytes32(&authority.VerifiedKey), C.bool(authority.Bound), C.uint8_t(authority.Kind), bytes32(&authority.GrantID), bytes32(&authority.AuthorityHash), scopePtr))
}

func authority_bound(node *C.lxphost_node) bool {
	return node != nil && bool(node.authority_bound)
}

func binding_authority(activity Activity, authority KernelAuthority) bool {
	return authority.Bound && len(activity.Signature) == 0 && authority.VerifiedKey == activity.AuthorityKey
}

func signature_valid(node *C.lxphost_node) bool {
	return node != nil && bool(node.execution.signature_valid)
}

func lxphost_bind_assets(node *C.lxphost_node, networkID uint32, bindingAuthority bool) error {
	return result(C.lxphost_bind_assets(node, C.uint32_t(networkID), C.bool(bindingAuthority)))
}

func lxphost_execute(node *C.lxphost_node, cfg Config, branch *branchCtx, bindingAuthority bool) error {
	return result(C.lxphost_execute(node, C.uint32_t(cfg.NetworkID), C.uint64_t(branch.batchNumber),
		C.uint64_t(branch.timestampMs), C.uint64_t(cfg.TimestampWindowMs), C.uint64_t(branch.sequence),
		C.uint32_t(cfg.ParameterVersion), C.uint64_t(cfg.GasLimit), C.bool(bindingAuthority)))
}

const (
	codeNonCanonical   = Code(C.LXP_ERR_NON_CANONICAL)
	codeSequenceReused = Code(C.LXP_ERR_SEQUENCE_REUSED)
	codeFatalInvariant = Code(C.LXP_FATAL_INVARIANT)
)

const blobPrefix byte = 0x05

const (
	accountKindMain   = uint16(C.LX_ACCOUNT_AGENT_MAIN)
	accountKindMargin = uint16(C.LX_ACCOUNT_AGENT_MARGIN)
	accountKindAsset  = uint16(C.LX_ACCOUNT_AGENT_ASSET)
)

// KernelAccount is one kernel account of a DID: its canonical name, kernel
// id, sub-account kind, parent DID kernel id and the bank address holding it.
type KernelAccount struct {
	Name    string
	ID      [32]byte
	Kind    uint16
	Parent  [32]byte
	Address sdk.AccAddress
}

type Config struct {
	StoreKey          sdk.StoreKey
	NetworkID         uint32
	Epoch             uint64
	ParameterVersion  uint32
	TimestampWindowMs uint64
	GasLimit          uint64
}

type Activity struct {
	ProtocolVersion uint16
	Type            uint32
	ActorDID        []byte
	AuthorityKey    [32]byte
	AccountSequence uint64
	NotBefore       uint64
	NotAfter        uint64
	IdempotencyKey  [32]byte
	FeeLimit        [16]byte
	Payload         []byte
	Signature       []byte
}

type KernelAuthority struct {
	Kind          uint8
	GrantID       [32]byte
	AuthorityHash [32]byte
	Scope         *AuthorityScope
	Bound         bool
	Actor         [32]byte
	Principal     [32]byte
	VerifiedKey   [32]byte
}

type ReceiptEvent struct {
	Code uint16
	Body []byte
}

type TransferLeg struct {
	Asset  [32]byte
	From   [32]byte
	To     [32]byte
	Amount sdk.Int
}

type ProgramOutcome struct {
	Status                  int32
	Kind                    uint8
	Response                []byte
	FeeScheduleVersion      uint64
	MeteringScheduleVersion uint64
	Events                  []byte
}

type Receipt struct {
	ResultCode         Code
	GlobalSequence     uint64
	Timestamp          uint64
	PreviousStateRoot  [32]byte
	ResultingStateRoot [32]byte
	Encoded            []byte
	FeeCharged         sdk.Int
	Events             []ReceiptEvent
	Transfers          []TransferLeg
	ProgramOutcome     *ProgramOutcome
}

type Host struct {
	mu   sync.Mutex
	cfg  Config
	node *C.lxphost_node
}

type kvEntry struct {
	module uint16
	key    []byte
	value  []byte
}

type idempotencyEntry struct {
	hash  [32]byte
	entry []byte
}

func NewHost(cfg Config) (*Host, error) {
	if cfg.StoreKey == nil || cfg.NetworkID == 0 || cfg.ParameterVersion == 0 || cfg.GasLimit == 0 {
		return nil, codeNonCanonical
	}
	var status C.int32_t
	node := C.lxphost_node_new(C.uint64_t(cfg.Epoch), &status)
	if node == nil {
		return nil, Code(status)
	}
	return &Host{cfg: cfg, node: node}, nil
}

func (h *Host) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	C.lxphost_node_free(h.node)
	h.node = nil
}

// DeriveKernelAccount derives the kernel account of the DID key for one
// sub-account kind: the main account takes a zero qualifier, a margin account
// the perps market id and an asset sub-account the asset id. The kernel
// classifies the name and derives its parent DID id; the bank address is
// address.Module("layerx", kernel account id).
func DeriveKernelAccount(didKey [32]byte, kind uint16, qualifier [32]byte) (KernelAccount, error) {
	var account KernelAccount
	var err error
	unqualified := qualifier == [32]byte{}
	switch {
	case kind == accountKindMain && unqualified:
		account.Name, account.ID, err = MainAccountID(didKey)
	case kind == accountKindMargin && !unqualified:
		account.Name = "agent:" + evmtypes.LayerXDid(didKey) + ":margin:" + hex.EncodeToString(qualifier[:])
		account.ID, err = kernelAccountID([]byte(account.Name))
	case kind == accountKindAsset && !unqualified:
		account.Name, account.ID, err = AssetAccountID(didKey, qualifier)
	default:
		return KernelAccount{}, codeNonCanonical
	}
	if err != nil {
		return KernelAccount{}, err
	}
	name := []byte(account.Name)
	var classified C.uint16_t
	if err := result(C.lxphost_account_kind(bytesPtr(name), C.size_t(len(name)), &classified)); err != nil {
		return KernelAccount{}, err
	}
	kernelID, err := kernelAccountID(name)
	if err != nil {
		return KernelAccount{}, err
	}
	parent, err := HolderDid(name)
	if err != nil {
		return KernelAccount{}, err
	}
	did, err := DIDID([]byte(evmtypes.LayerXDid(didKey)))
	if err != nil {
		return KernelAccount{}, err
	}
	if uint16(classified) != kind || kernelID != account.ID || parent != did {
		return KernelAccount{}, codeFatalInvariant
	}
	account.Kind = kind
	account.Parent = parent
	account.Address = BankAddress(account.ID)
	return account, nil
}

// Execute runs one activity through HostExecute.
func (h *Host) Execute(ctx sdk.Context, activity Activity, authority KernelAuthority) (Receipt, error) {
	return h.HostExecute(ctx, activity, authority)
}

// HostExecute enters the validate and execute functions of the kernel module
// registered for the activity type through cgo. Kernel time is the block-open
// timestamp of ctx and the batch number its height; module values, module
// blobs, idempotency records and identity sequences are loaded from and
// committed to the host store.
func (h *Host) HostExecute(ctx sdk.Context, activity Activity, authority KernelAuthority) (Receipt, error) {
	bindingAuthority := binding_authority(activity, authority)
	if len(activity.Signature) == 0 && !bindingAuthority {
		return Receipt{}, Code(C.LXP_ERR_BAD_SIGNATURE)
	}
	branch, err := newBranchCtx(ctx, h.cfg.StoreKey)
	if err != nil {
		return Receipt{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.node == nil {
		return Receipt{}, codeNonCanonical
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := lxphost_reset(h.node, branch.sequence); err != nil {
		return Receipt{}, err
	}
	if err := writeEnvelope(C.lxphost_node_envelope(h.node), h.cfg.NetworkID, activity); err != nil {
		return Receipt{}, err
	}
	module, err := lxphost_activity_module(h.node)
	if err != nil {
		return Receipt{}, err
	}
	modules := activityModules(module)
	if err := lxphost_authority_write(h.node, authority); err != nil {
		return Receipt{}, err
	}
	if err := branch.load(h, modules); err != nil {
		return Receipt{}, err
	}
	loadedBlobs, err := h.loadBlobs(branch.store, modules)
	if err != nil {
		return Receipt{}, err
	}
	if err := lxphost_bind_assets(h.node, h.cfg.NetworkID, bindingAuthority); err != nil {
		return Receipt{}, err
	}
	did, err := h.bindActor()
	if err != nil {
		return Receipt{}, err
	}
	next, err := branch.identitySequence(did)
	if err != nil {
		return Receipt{}, err
	}
	if err := result(C.lxphost_actor_sequence_set(h.node, bytes32(&did), C.uint64_t(next))); err != nil {
		return Receipt{}, err
	}
	if err := lxphost_execute(h.node, h.cfg, branch, bindingAuthority); err != nil {
		return Receipt{}, err
	}
	var outcome lxphost_outcome
	defer C.lxphost_outcome_release(&outcome)
	if err := lxphost_outcome_read(h.node, &outcome); err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{
		FeeCharged:         readAmount(C.struct_lxp_u128{hi: outcome.fee_hi, lo: outcome.fee_lo}),
		ResultCode:         Code(outcome.result_code),
		GlobalSequence:     uint64(outcome.global_sequence),
		Timestamp:          uint64(outcome.timestamp),
		PreviousStateRoot:  *(*[32]byte)(unsafe.Pointer(&outcome.previous_state_root[0])),
		ResultingStateRoot: *(*[32]byte)(unsafe.Pointer(&outcome.resulting_state_root[0])),
		Encoded:            C.GoBytes(unsafe.Pointer(outcome.encoded), C.int(outcome.encoded_length)),
	}
	for _, effect := range unsafe.Slice(outcome.effects, int(outcome.effect_count)) {
		if effect.kind == C.LXP_EFFECT_EVENT {
			receipt.Events = append(receipt.Events, ReceiptEvent{
				Code: uint16(effect.event_type),
				Body: C.GoBytes(unsafe.Pointer(&effect.body[0]), C.int(effect.body_length)),
			})
		}
	}
	if outcome.amount.hi != 0 || outcome.amount.lo != 0 {
		receipt.Transfers = []TransferLeg{{
			Asset:  *(*[32]byte)(unsafe.Pointer(&outcome.asset[0])),
			From:   *(*[32]byte)(unsafe.Pointer(&outcome.from[0])),
			To:     *(*[32]byte)(unsafe.Pointer(&outcome.to[0])),
			Amount: readAmount(outcome.amount),
		}}
	}
	if program := &outcome.program_outcome; program.present != C.bool(false) {
		receipt.ProgramOutcome = &ProgramOutcome{
			Status:                  int32(program.result_code),
			Kind:                    uint8(program.terminal_kind),
			Response:                C.GoBytes(unsafe.Pointer(program.terminal_payload.bytes), C.int(program.terminal_payload.length)),
			FeeScheduleVersion:      uint64(program.fee_schedule_version),
			MeteringScheduleVersion: uint64(program.metering_schedule_version),
			Events:                  C.GoBytes(unsafe.Pointer(program.event_envelope_payload.bytes), C.int(program.event_envelope_payload.length)),
		}
	}
	stagedBlobs, err := h.moduleBlobs()
	if err != nil {
		return Receipt{}, err
	}
	if err := branch.commit(h, did); err != nil {
		return Receipt{}, err
	}
	commitBlobs(branch.store, loadedBlobs, stagedBlobs)
	return receipt, nil
}

func (h *Host) SigningHash(activity Activity) ([32]byte, error) {
	var hash [32]byte
	envelope := (*C.lxphost_envelope)(C.calloc(1, C.sizeof_lxphost_envelope))
	if envelope == nil {
		return hash, Code(C.LXP_ERR_ARENA_EXHAUSTED)
	}
	defer C.free(unsafe.Pointer(envelope))
	if err := writeEnvelope(envelope, h.cfg.NetworkID, activity); err != nil {
		return hash, err
	}
	err := result(C.lxphost_envelope_preimage(envelope, bytes32(&hash)))
	return hash, err
}

func DIDID(did []byte) ([32]byte, error) {
	var id [32]byte
	err := result(C.lxp_did_id_derive(bytesPtr(did), C.size_t(len(did)), bytes32(&id)))
	return id, err
}

func writeEnvelope(envelope *C.lxphost_envelope, networkID uint32, a Activity) error {
	if err := result(C.lxphost_envelope_write(envelope, C.uint16_t(a.ProtocolVersion), C.uint32_t(networkID),
		C.uint32_t(a.Type), bytesPtr(a.ActorDID), C.size_t(len(a.ActorDID)), bytes32(&a.AuthorityKey),
		C.uint64_t(a.AccountSequence), C.uint64_t(a.NotBefore), C.uint64_t(a.NotAfter),
		bytes32(&a.IdempotencyKey), bytesPtr(a.Payload), C.size_t(len(a.Payload)),
		bytesPtr(a.Signature), C.size_t(len(a.Signature)))); err != nil {
		return err
	}
	envelope.activity.fee_limit.hi = C.uint64_t(binary.BigEndian.Uint64(a.FeeLimit[:8]))
	envelope.activity.fee_limit.lo = C.uint64_t(binary.BigEndian.Uint64(a.FeeLimit[8:]))
	return nil
}

func activityModules(module uint16) []uint16 {
	modules := []uint16{C.LXP_MODULE_ASSET, C.LXP_MODULE_GOVERNANCE}
	if module != C.LXP_MODULE_ASSET && module != C.LXP_MODULE_GOVERNANCE {
		modules = append(modules, module)
		sort.Slice(modules, func(i, j int) bool { return modules[i] < modules[j] })
	}
	return modules
}

func (h *Host) authorityBound() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return authority_bound(h.node)
}

func (h *Host) bindActor() ([32]byte, error) {
	var did [32]byte
	err := result(C.lxphost_bind_actor(h.node, bytes32(&did)))
	return did, err
}

func (h *Host) actorSequence(did [32]byte) (uint64, error) {
	var sequence C.uint64_t
	err := result(C.lxphost_actor_sequence_get(h.node, bytes32(&did), &sequence))
	return uint64(sequence), err
}

func (h *Host) accountRegistry() unsafe.Pointer {
	return C.lxphost_accounts(h.node)
}

func (h *Host) loadModuleKV(module uint16, key, value []byte) error {
	return result(C.lxphost_load_kv(h.node, C.uint16_t(module), bytesPtr(key), C.size_t(len(key)),
		bytesPtr(value), C.size_t(len(value))))
}

func (h *Host) loadIdempotency(hash [32]byte, entry []byte) error {
	return result(C.lxphost_load_idempotency(h.node, bytes32(&hash), bytesPtr(entry), C.size_t(len(entry))))
}

func (h *Host) moduleKV() ([]kvEntry, error) {
	count := int(C.lxphost_kv_count(h.node))
	entries := make([]kvEntry, 0, count)
	for i := 0; i < count; i++ {
		var view C.lxphost_kv_view
		if err := result(C.lxphost_kv_read(h.node, C.size_t(i), &view)); err != nil {
			return nil, err
		}
		entries = append(entries, kvEntry{
			module: uint16(view.module_id),
			key:    C.GoBytes(unsafe.Pointer(view.key), C.int(view.key_length)),
			value:  C.GoBytes(unsafe.Pointer(view.value), C.int(view.value_length)),
		})
	}
	return entries, nil
}

func (h *Host) loadBlobs(store sdk.KVStore, modules []uint16) (map[string][]byte, error) {
	loaded := map[string][]byte{}
	for _, module := range modules {
		iterator := sdk.KVStorePrefixIterator(store, blobKey(module, nil))
		for ; iterator.Valid(); iterator.Next() {
			key := append([]byte(nil), iterator.Key()...)
			if len(key) != 35 {
				iterator.Close()
				return nil, codeFatalInvariant
			}
			value := append([]byte(nil), iterator.Value()...)
			if err := result(C.lxphost_load_blob(h.node, C.uint16_t(module),
				bytesPtr(key[3:]), bytesPtr(value), C.size_t(len(value)))); err != nil {
				iterator.Close()
				return nil, err
			}
			loaded[string(key)] = value
		}
		if err := iterator.Close(); err != nil {
			return nil, codeFatalInvariant
		}
	}
	return loaded, nil
}

func (h *Host) moduleBlobs() (map[string][]byte, error) {
	count := int(C.lxphost_blob_count(h.node))
	blobs := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		var view C.lxphost_blob_view
		if err := result(C.lxphost_blob_read(h.node, C.size_t(i), &view)); err != nil {
			return nil, err
		}
		key := blobKey(uint16(view.module_id), C.GoBytes(unsafe.Pointer(view.key), 32))
		if _, duplicate := blobs[string(key)]; duplicate {
			return nil, codeFatalInvariant
		}
		blobs[string(key)] = C.GoBytes(unsafe.Pointer(view.bytes), C.int(view.length))
	}
	return blobs, nil
}

func commitBlobs(store sdk.KVStore, loaded, staged map[string][]byte) {
	keys := make([]string, 0, len(staged)+len(loaded))
	for key := range staged {
		keys = append(keys, key)
	}
	for key := range loaded {
		if _, kept := staged[key]; !kept {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, kept := staged[key]
		previous, existed := loaded[key]
		switch {
		case !kept:
			store.Delete([]byte(key))
		case !existed || !bytes.Equal(previous, value):
			store.Set([]byte(key), value)
		}
	}
}

func blobKey(module uint16, key []byte) []byte {
	out := make([]byte, 3, 3+len(key))
	out[0] = blobPrefix
	binary.BigEndian.PutUint16(out[1:3], module)
	return append(out, key...)
}

func (h *Host) idempotencyFrom(start int) ([]idempotencyEntry, error) {
	count := int(C.lxphost_idempotency_count(h.node))
	if count < start {
		return nil, codeFatalInvariant
	}
	buffer := make([]byte, C.LXP_STATE_MAX_REPLAY_ENTRY_BYTES)
	entries := make([]idempotencyEntry, 0, count-start)
	for i := start; i < count; i++ {
		var entry idempotencyEntry
		var length C.size_t
		if err := result(C.lxphost_idempotency_read(h.node, C.size_t(i), bytes32(&entry.hash),
			bytesPtr(buffer), C.size_t(len(buffer)), &length)); err != nil {
			return nil, err
		}
		entry.entry = append([]byte(nil), buffer[:length]...)
		entries = append(entries, entry)
	}
	return entries, nil
}

func result(status C.int32_t) error {
	if status == C.LXP_OK {
		return nil
	}
	return Code(status)
}

func bytesPtr(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}

func bytes32(b *[32]byte) *C.uint8_t {
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}
