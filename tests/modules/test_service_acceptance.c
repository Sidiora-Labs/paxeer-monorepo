#include "test_service_helpers.h"

#include "layerx/lxp_snapshot.h"

#include <stdio.h>
#include <string.h>

static size_t offer_payload(uint8_t *out, uint8_t offer_marker,
                            uint8_t default_outcome)
{
    size_t offset = 0U;
    (void)memset(out, 0, (size_t)LX_SERVICE_OFFER_PUBLISH_PAYLOAD_BYTES);
    out[offset++] = 0U;
    out[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(out + offset, offer_marker); offset += 32U;
    id32(out + offset, 11U); offset += 32U;
    be64(out + offset + 8U, 25U); offset += 16U;
    id32(out + offset, 12U); offset += 32U;
    id32(out + offset, 13U); offset += 32U;
    be64(out + offset, 1000U); offset += 8U;
    be64(out + offset, 200U); offset += 8U;
    be64(out + offset, 300U); offset += 8U;
    out[offset++] = default_outcome;
    be64(out + offset, 900U); offset += 8U;
    return offset;
}

static size_t propose_payload(uint8_t *out, const uint8_t agreement_id[32],
                              const uint8_t offer_id[32])
{
    size_t offset = 0U;
    out[offset++] = 0U;
    out[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    (void)memcpy(out + offset, agreement_id, 32U); offset += 32U;
    (void)memcpy(out + offset, offer_id, 32U); offset += 32U;
    id32(out + offset, 12U); offset += 32U;
    id32(out + offset, 41U); offset += 32U;
    return offset;
}

static size_t commit_payload(uint8_t *out, const uint8_t commitment_id[32],
                             const uint8_t agreement_id[32])
{
    size_t offset = 0U;
    out[offset++] = 0U;
    out[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    (void)memcpy(out + offset, commitment_id, 32U); offset += 32U;
    (void)memcpy(out + offset, agreement_id, 32U); offset += 32U;
    id32(out + offset, 51U); offset += 32U;
    id32(out + offset, 41U); offset += 32U;
    be64(out + offset, 900U); offset += 8U;
    be64(out + offset, 100U); offset += 8U;
    return offset;
}

static size_t deliver_payload(uint8_t *out, const uint8_t delivery_id[32],
                              const uint8_t agreement_id[32],
                              const uint8_t *availability_markers,
                              size_t count)
{
    size_t offset = 0U;
    size_t i;
    out[offset++] = 0U;
    out[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    (void)memcpy(out + offset, delivery_id, 32U); offset += 32U;
    (void)memcpy(out + offset, agreement_id, 32U); offset += 32U;
    out[offset++] = (uint8_t)count;
    for (i = 0U; i < count; ++i) {
        id32(out + offset, 13U); offset += 32U;
        be64(out + offset, 1000U + i); offset += 8U;
        id32(out + offset, availability_markers[i]); offset += 32U;
    }
    return offset;
}

static size_t reject_payload(uint8_t *out, const uint8_t agreement_id[32],
                             uint16_t reason, const uint8_t *hash_markers,
                             size_t count)
{
    size_t offset = 0U;
    size_t i;
    out[offset++] = 0U;
    out[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    (void)memcpy(out + offset, agreement_id, 32U); offset += 32U;
    be16(out + offset, reason); offset += 2U;
    out[offset++] = (uint8_t)count;
    for (i = 0U; i < count; ++i) {
        id32(out + offset, hash_markers[i]);
        offset += 32U;
    }
    return offset;
}

static int lifecycle(uint8_t marker, uint8_t *agreement_id,
                     const uint8_t *availability_markers, size_t count,
                     const lxp_authority_resolved *provider,
                     const lxp_authority_resolved *buyer, uint8_t outcome,
                     uint64_t *sequence)
{
    uint8_t payload[LX_SERVICE_DELIVER_PAYLOAD_MAX_BYTES];
    uint8_t offer_id[32];
    uint8_t commitment_id[32];
    uint8_t delivery_id[32];
    lxp_result status = LXP_OK;
    size_t length;

    id32(offer_id, marker);
    id32(agreement_id, (uint8_t)(marker + 1U));
    id32(commitment_id, (uint8_t)(marker + 2U));
    id32(delivery_id, (uint8_t)(marker + 3U));
    length = offer_payload(payload, marker, outcome);
    if (dispatch(LX_SERVICE_OFFER_PUBLISH, payload, length, provider, 100U,
                 ++(*sequence), marker, &status) != LXP_OK || status != LXP_OK)
        return 1;
    length = propose_payload(payload, agreement_id, offer_id);
    if (dispatch(LX_SERVICE_AGREEMENT_PROPOSE, payload, length, buyer, 100U,
                 ++(*sequence), marker, &status) != LXP_OK || status != LXP_OK)
        return 1;
    length = identifier_payload(payload, agreement_id);
    if (dispatch(LX_SERVICE_AGREEMENT_ACCEPT, payload, length, provider, 100U,
                 ++(*sequence), marker, &status) != LXP_OK || status != LXP_OK)
        return 1;
    length = commit_payload(payload, commitment_id, agreement_id);
    if (dispatch(LX_SERVICE_COMMIT_TASK, payload, length, provider, 100U,
                 ++(*sequence), marker, &status) != LXP_OK || status != LXP_OK)
        return 1;
    length = deliver_payload(payload, delivery_id, agreement_id,
                             availability_markers, count);
    if (dispatch(LX_SERVICE_DELIVER, payload, length, provider, 100U,
                 ++(*sequence), marker, &status) != LXP_OK || status != LXP_OK)
        return 1;
    return 0;
}

static int correction_failed(int line)
{
    (void)fprintf(stderr, "corrected delivery check failed at line %d\n", line);
    return 1;
}

static int correction_reopen(void)
{
    static uint8_t snapshot_bytes[65536];
    lxp_arena arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest;
    uint8_t before[32];
    uint8_t after[32];
    FILE *file;
    size_t length;
    int failed = 1;

    if (lxp_state_root(&service_kernel, before) != LXP_OK ||
        lxp_arena_init(&arena, snapshot_bytes, sizeof(snapshot_bytes)) != LXP_OK ||
        lxp_snapshot_write(&service_kernel, store_state.next_sequence - 1U,
                           &arena, &snapshot) != LXP_OK ||
        lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
            store_state.next_sequence - 1U, before,
            service_kernel.current_state_root, &manifest) != LXP_OK)
        return correction_failed(__LINE__);
    file = tmpfile();
    if (file == NULL) return correction_failed(__LINE__);
    length = snapshot.length;
    if (fwrite(snapshot.bytes, 1U, length, file) != length ||
        fflush(file) != 0 || fseek(file, 0L, SEEK_SET) != 0)
        goto finish;
    if (lxp_state_store_destroy(&store_state) != LXP_OK ||
        lxp_state_store_init(&store_state, 0U) != LXP_OK ||
        lxp_kernel_create(&service_kernel, &store_state, &state_journal,
                          &parameter_set, 0U) != LXP_OK ||
        lxp_kernel_register_module(&service_kernel,
                                   lx_service_module_iface()) != LXP_OK)
        goto finish;
    (void)memset(snapshot_bytes, 0, sizeof(snapshot_bytes));
    if (fread(snapshot_bytes, 1U, length, file) != length ||
        lxp_snapshot_load(snapshot_bytes, length, &manifest,
                          &service_kernel) != LXP_OK ||
        lxp_snapshot_verify_root(&service_kernel, &manifest) != LXP_OK ||
        lxp_state_root(&service_kernel, after) != LXP_OK ||
        memcmp(before, after, sizeof(before)) != 0)
        goto finish;
    failed = 0;
finish:
    if (fclose(file) != 0) failed = 1;
    return failed ? correction_failed(__LINE__) : 0;
}

static int corrected_delivery_outcome(uint8_t default_outcome, unsigned route)
{
    uint8_t payload[LX_SERVICE_DELIVER_PAYLOAD_MAX_BYTES];
    uint8_t agreement_id[32];
    uint8_t original_id[32];
    uint8_t corrected_id[32];
    uint8_t commitment_id[32];
    uint8_t original_activity[32];
    uint8_t corrected_activity[32];
    uint8_t markers[1] = {22U};
    uint8_t contested[1] = {13U};
    uint8_t zero_hashes[LX_SERVICE_MAX_DELIVERABLES][32] = {{0U}};
    uint8_t before[32];
    uint8_t after[32];
    lxp_authority_resolved provider = {0};
    lxp_authority_resolved buyer = {0};
    lx_service_agreement agreement;
    lx_service_delivery delivery;
    lx_service_commitment commitment;
    lxp_module_ctx ctx;
    lxp_result outcome = LXP_OK;
    uint64_t sequence = 0U;
    uint64_t outcome_time = route == 0U ? 1100U : 1200U;
    lx_service_agreement_state expected_state = route == 0U ||
        default_outcome == LX_SERVICE_DEFAULT_ACCEPT ?
        LX_SERVICE_AGREEMENT_ACCEPTED : LX_SERVICE_AGREEMENT_REJECTED;
    size_t length;
    bool complete = false;

    provider.principal[0] = 1U;
    buyer.principal[0] = 2U;
    id32(original_id, 103U);
    id32(corrected_id, 104U);
    id32(commitment_id, 102U);
    if (lxp_state_store_init(&store_state, 1U) != LXP_OK ||
        lxp_kernel_create(&service_kernel, &store_state, &state_journal,
                          &parameter_set, 0U) != LXP_OK ||
        lxp_kernel_register_module(&service_kernel,
                                   lx_service_module_iface()) != LXP_OK ||
        lifecycle(100U, agreement_id, markers, 1U, &provider, &buyer,
                  default_outcome, &sequence) != 0 ||
        open_ctx(&ctx, 100U, sequence) != 0 ||
        lx_service_delivery_lookup(&ctx, original_id, &delivery) != LXP_OK)
        return correction_failed(__LINE__);
    (void)memcpy(original_activity, delivery.activity_id, 32U);

    length = reject_payload(payload, agreement_id, 9U, contested, 1U);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 200U, 6U,
                 6U, &outcome) != LXP_OK || outcome != LXP_OK ||
        event_is((uint16_t)LX_SERVICE_EVENT_OUTCOME_REJECTED, agreement_id,
                 provider.principal, 1U, 6U) != 0 ||
        open_ctx(&ctx, 200U, 6U) != 0 ||
        lx_service_agreement_lookup(&ctx, agreement_id, &agreement) != LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_REJECTED ||
        agreement.rejection_reason != 9U ||
        agreement.contested_hash_count != 1U ||
        agreement.contested_hashes[0][0] != 13U ||
        agreement.outcome_sequence != 6U || agreement.outcome_timestamp != 200U ||
        lxp_state_root(&service_kernel, before) != LXP_OK)
        return correction_failed(__LINE__);

    markers[0] = 44U;
    length = deliver_payload(payload, corrected_id, agreement_id, markers, 1U);
    if (dispatch(LX_SERVICE_DELIVER, payload, length, &buyer, 300U, 7U,
                 7U, &outcome) != LXP_OK ||
        outcome != LXP_ERR_AGREEMENT_STATE ||
        dispatch(LX_SERVICE_DELIVER, payload, length, &provider, 1001U, 7U,
                 7U, &outcome) != LXP_OK ||
        outcome != LXP_ERR_DELIVERY_DEADLINE_PASSED ||
        lxp_state_root(&service_kernel, after) != LXP_OK ||
        memcmp(before, after, sizeof(before)) != 0)
        return correction_failed(__LINE__);
    if (expected_activity_id(LX_SERVICE_DELIVER, payload, length, 8U,
                              corrected_activity) != LXP_OK ||
        dispatch(LX_SERVICE_DELIVER, payload, length, &provider, 1000U, 8U,
                 8U, &outcome) != LXP_OK || outcome != LXP_OK ||
        event_is((uint16_t)LX_SERVICE_EVENT_DELIVERED, corrected_id,
                 agreement_id, 1U, 8U) != 0 || correction_reopen() != 0)
        return correction_failed(__LINE__);
    if (open_ctx(&ctx, 1000U, 9U) != 0 ||
        lx_service_agreement_lookup(&ctx, agreement_id, &agreement) != LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_DELIVERED ||
        agreement.rejection_reason != 0U || agreement.contested_hash_count != 0U ||
        memcmp(agreement.contested_hashes, zero_hashes, sizeof(zero_hashes)) != 0 ||
        agreement.default_applied || agreement.outcome_sequence != 0U ||
        agreement.outcome_timestamp != 0U || agreement.accepted_sequence != 3U ||
        agreement.delivery_deadline != 1000U ||
        agreement.acceptance_window_end != 1200U ||
        agreement.dispute_window_end != 1500U ||
        agreement.default_outcome != (lx_service_default_outcome)default_outcome ||
        lx_service_commitment_lookup(&ctx, commitment_id, &commitment) != LXP_OK ||
        memcmp(commitment.agreement_id, agreement_id, 32U) != 0 ||
        commitment.global_sequence != 4U ||
        lx_service_delivery_lookup(&ctx, original_id, &delivery) != LXP_OK ||
        memcmp(delivery.activity_id, original_activity, 32U) != 0 ||
        delivery.global_sequence != 5U || delivery.deliverable_count != 1U ||
        delivery.deliverables[0].artifact_size != 1000U ||
        delivery.deliverables[0].availability_reference[0] != 22U ||
        lx_service_delivery_latest(&ctx, agreement_id, &delivery) != LXP_OK ||
        memcmp(delivery.delivery_id, corrected_id, 32U) != 0 ||
        memcmp(delivery.activity_id, corrected_activity, 32U) != 0 ||
        delivery.global_sequence != 8U || delivery.deliverable_count != 1U ||
        delivery.deliverables[0].artifact_size != 1000U ||
        delivery.deliverables[0].availability_reference[0] != 44U)
        return correction_failed(__LINE__);

    if (route == 0U) {
        length = identifier_payload(payload, agreement_id);
        if (dispatch(LX_SERVICE_ACCEPT, payload, length, &buyer, outcome_time,
                     9U, 9U, &outcome) != LXP_OK || outcome != LXP_OK ||
            event_is((uint16_t)LX_SERVICE_EVENT_OUTCOME_ACCEPTED, agreement_id,
                     provider.principal,
                     (uint8_t)LX_SERVICE_AGREEMENT_ACCEPTED, 9U) != 0)
            return correction_failed(__LINE__);
    } else {
        if (open_ctx(&ctx, 1199U, 9U) != 0 ||
            lx_service_module_iface()->epoch_begin(&ctx, 0U, 1199U) != LXP_OK ||
            event_buffer.count != 0U ||
            lx_service_agreement_lookup(&ctx, agreement_id, &agreement) != LXP_OK ||
            agreement.state != LX_SERVICE_AGREEMENT_DELIVERED ||
            lxp_module_ctx_commit(&ctx) != LXP_OK ||
            open_ctx(&ctx, outcome_time, 9U) != 0)
            return correction_failed(__LINE__);
        if (route == 1U) {
            if (lx_service_module_iface()->epoch_begin(&ctx, 0U,
                                                       outcome_time) != LXP_OK)
                return correction_failed(__LINE__);
        } else {
            if (lx_service_batch_maintenance(&ctx, &complete) != LXP_OK || complete ||
                lx_service_batch_maintenance(&ctx, &complete) != LXP_OK || !complete)
                return correction_failed(__LINE__);
        }
        {
            uint8_t event[50];
            const lxp_effect *effect = &event_buffer.effects[0];
            (void)memcpy(event, agreement_id, 32U);
            event[32U] = (uint8_t)expected_state;
            event[33U] = 1U;
            be64(event + 34U, 9U);
            be64(event + 42U, outcome_time);
            if (event_buffer.count != 1U || effect->kind != LXP_EFFECT_EVENT ||
                effect->module_id != LXP_MODULE_SERVICE || effect->monetary ||
                effect->event_type != LX_SERVICE_EVENT_DEFAULT_APPLIED ||
                effect->body_length != sizeof(event) ||
                memcmp(effect->body, event, sizeof(event)) != 0 ||
                lxp_module_ctx_commit(&ctx) != LXP_OK)
                return correction_failed(__LINE__);
        }
    }
    if (correction_reopen() != 0 || open_ctx(&ctx, outcome_time, 10U) != 0 ||
        lx_service_agreement_lookup(&ctx, agreement_id, &agreement) != LXP_OK ||
        agreement.state != expected_state ||
        agreement.default_applied != (route != 0U) ||
        agreement.outcome_sequence != 9U ||
        agreement.outcome_timestamp != outcome_time ||
        agreement.rejection_reason != 0U || agreement.contested_hash_count != 0U ||
        lx_service_module_iface()->epoch_begin(&ctx, 0U, outcome_time) != LXP_OK ||
        event_buffer.count != 0U || lxp_module_ctx_commit(&ctx) != LXP_OK ||
        lxp_state_store_destroy(&store_state) != LXP_OK)
        return correction_failed(__LINE__);
    return 0;
}

static int test_corrected_delivery_reopen_accept(void)
{
    return corrected_delivery_outcome((uint8_t)LX_SERVICE_DEFAULT_REJECT, 0U);
}

static int test_corrected_delivery_reopen_default_accept(void)
{
    return corrected_delivery_outcome((uint8_t)LX_SERVICE_DEFAULT_ACCEPT, 1U);
}

static int test_corrected_delivery_reopen_default_reject(void)
{
    return corrected_delivery_outcome((uint8_t)LX_SERVICE_DEFAULT_REJECT, 2U);
}

int main(void)
{
    uint8_t payload[LX_SERVICE_DELIVER_PAYLOAD_MAX_BYTES];
    uint8_t accepted_id[32];
    uint8_t rejected_id[32];
    uint8_t swept_id[32];
    uint8_t markers[2];
    uint8_t contested[2];
    uint8_t record[LX_SERVICE_AGREEMENT_RECORD_BYTES];
    size_t record_length = 0U;
    lx_service_agreement agreement;
    lx_service_outcome_request request;
    lxp_authority_resolved provider;
    lxp_authority_resolved buyer;
    lxp_authority_resolved outsider;
    lxp_module_ctx ctx;
    lxp_result outcome = LXP_OK;
    uint64_t sequence = 0U;
    size_t length;

    (void)memset(&provider, 0, sizeof(provider));
    (void)memset(&buyer, 0, sizeof(buyer));
    (void)memset(&outsider, 0, sizeof(outsider));
    provider.principal[0] = 1U;
    buyer.principal[0] = 2U;
    outsider.principal[0] = 3U;
    markers[0] = 44U;
    markers[1] = 22U;
    contested[0] = 13U;
    contested[1] = 14U;

    if (lxp_state_store_init(&store_state, 0U) != LXP_OK ||
        lxp_kernel_create(&service_kernel, &store_state, &state_journal,
                          &parameter_set, 0U) != LXP_OK ||
        lxp_kernel_register_module(&service_kernel,
                                   lx_service_module_iface()) != LXP_OK)
        return 1;

    if (lifecycle(60U, accepted_id, markers, 2U, &provider, &buyer,
                  (uint8_t)LX_SERVICE_DEFAULT_ACCEPT, &sequence) != 0 ||
        lifecycle(70U, rejected_id, markers, 2U, &provider, &buyer,
                  (uint8_t)LX_SERVICE_DEFAULT_REJECT, &sequence) != 0 ||
        lifecycle(80U, swept_id, markers, 1U, &provider, &buyer,
                  (uint8_t)LX_SERVICE_DEFAULT_ACCEPT, &sequence) != 0)
        return 1;

    length = identifier_payload(payload, accepted_id);
    if (dispatch(LX_SERVICE_ACCEPT, payload, length, &provider, 100U, 40U,
                 40U, &outcome) != LXP_OK ||
        outcome != LXP_ERR_AGREEMENT_STATE ||
        dispatch(LX_SERVICE_ACCEPT, payload, length, &outsider, 100U, 40U,
                 40U, &outcome) != LXP_OK ||
        outcome != LXP_ERR_AGREEMENT_STATE ||
        dispatch(LX_SERVICE_ACCEPT, payload, length, &buyer, 1201U, 40U, 40U,
                 &outcome) != LXP_OK || outcome != LXP_ERR_AGREEMENT_STATE)
        return 1;
    if (dispatch(LX_SERVICE_ACCEPT, payload, length, &buyer, 1200U, 41U, 41U,
                 &outcome) != LXP_OK || outcome != LXP_OK ||
        event_is((uint16_t)LX_SERVICE_EVENT_OUTCOME_ACCEPTED, accepted_id,
                 provider.principal,
                 (uint8_t)LX_SERVICE_AGREEMENT_ACCEPTED, 41U) != 0 ||
        dispatch(LX_SERVICE_ACCEPT, payload, length, &buyer, 1200U, 42U, 42U,
                 &outcome) != LXP_OK || outcome != LXP_ERR_AGREEMENT_STATE)
        return 1;
    if (open_ctx(&ctx, 1200U, 43U) != 0 ||
        lx_service_agreement_lookup(&ctx, accepted_id, &agreement) !=
            LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_ACCEPTED ||
        agreement.default_applied || agreement.outcome_sequence != 41U ||
        agreement.outcome_timestamp != 1200U ||
        agreement.acceptance_window_end != 1200U ||
        agreement.dispute_window_end != 1500U)
        return 1;

    length = reject_payload(payload, rejected_id, 0U, contested, 1U);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 44U, 44U,
                 &outcome) != LXP_OK || outcome != LXP_ERR_NON_CANONICAL)
        return 1;
    length = reject_payload(payload, rejected_id, 9U, contested, 1U);
    payload[LX_SERVICE_REJECT_PAYLOAD_FIXED_BYTES - 1U] = 0U;
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 44U, 44U,
                 &outcome) != LXP_OK || outcome != LXP_ERR_LENGTH_LIMIT)
        return 1;
    payload[LX_SERVICE_REJECT_PAYLOAD_FIXED_BYTES - 1U] =
        (uint8_t)(LX_SERVICE_MAX_DELIVERABLES + 1);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 44U, 44U,
                 &outcome) != LXP_OK || outcome != LXP_ERR_LENGTH_LIMIT)
        return 1;
    length = reject_payload(payload, rejected_id, 9U, contested, 2U);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 44U, 44U,
                 &outcome) != LXP_OK ||
        outcome != LXP_ERR_DELIVERABLE_MISMATCH)
        return 1;
    contested[1] = 13U;
    length = reject_payload(payload, rejected_id, 9U, contested, 2U);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 44U, 44U,
                 &outcome) != LXP_OK ||
        outcome != LXP_ERR_DUPLICATE_ENTRY)
        return 1;
    contested[1] = 0U;
    length = reject_payload(payload, rejected_id, 9U, contested, 2U);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 44U, 44U,
                 &outcome) != LXP_OK ||
        outcome != LXP_ERR_NON_CANONICAL)
        return 1;
    contested[1] = 13U;
    length = reject_payload(payload, rejected_id, 9U, contested, 1U);
    if (dispatch(LX_SERVICE_REJECT, payload, length, &outsider, 100U, 44U,
                 44U, &outcome) != LXP_OK ||
        outcome != LXP_ERR_AGREEMENT_STATE ||
        dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 1201U, 44U, 44U,
                 &outcome) != LXP_OK || outcome != LXP_ERR_AGREEMENT_STATE)
        return 1;
    if (dispatch(LX_SERVICE_REJECT, payload, length, &buyer, 100U, 45U, 45U,
                 &outcome) != LXP_OK || outcome != LXP_OK ||
        event_is((uint16_t)LX_SERVICE_EVENT_OUTCOME_REJECTED, rejected_id,
                 provider.principal, 1U, 45U) != 0)
        return 1;
    if (open_ctx(&ctx, 100U, 46U) != 0 ||
        lx_service_agreement_lookup(&ctx, rejected_id, &agreement) !=
            LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_REJECTED ||
        agreement.rejection_reason != 9U ||
        agreement.contested_hash_count != 1U ||
        agreement.contested_hashes[0][0] != 13U ||
        agreement.default_applied || agreement.outcome_sequence != 45U)
        return 1;

    (void)memset(&request, 0, sizeof(request));
    request.authority = &buyer;
    (void)memcpy(request.agreement_id, swept_id, 32U);
    request.rejection_reason = 0U;
    request.contested_hash_count = 1U;
    id32(request.contested_hashes[0], 13U);
    if (open_ctx(&ctx, 100U, 47U) != 0 ||
        lx_service_reject_execute(&ctx, &request, &agreement) !=
            LXP_ERR_NON_CANONICAL)
        return 1;
    request.rejection_reason = 4U;
    request.contested_hash_count = 0U;
    if (lx_service_reject_execute(&ctx, &request, &agreement) !=
        LXP_ERR_NON_CANONICAL)
        return 1;
    request.contested_hash_count = (size_t)LX_SERVICE_MAX_DELIVERABLES + 1U;
    if (lx_service_reject_execute(&ctx, &request, &agreement) !=
        LXP_ERR_NON_CANONICAL)
        return 1;
    request.contested_hash_count = 2U;
    id32(request.contested_hashes[1], 13U);
    if (lx_service_reject_execute(&ctx, &request, &agreement) !=
            LXP_ERR_DUPLICATE_ENTRY ||
        lx_service_agreement_lookup(&ctx, swept_id, &agreement) != LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_DELIVERED ||
        agreement.contested_hash_count != 0U)
        return 1;
    (void)memset(request.contested_hashes[1], 0, 32U);
    request.contested_hash_count = 1U;
    request.attempts_balance_mutation = true;
    if (lx_service_reject_execute(&ctx, &request, &agreement) !=
            LXP_ERR_MODULE_MAY_NOT_WRITE_BALANCE ||
        lx_service_accept_execute(&ctx, &request, &agreement) !=
            LXP_ERR_MODULE_MAY_NOT_WRITE_BALANCE)
        return 1;
    request.attempts_balance_mutation = false;

    if (lx_service_agreement_lookup(&ctx, rejected_id, &agreement) != LXP_OK ||
        agreement.contested_hash_count != 1U)
        return 1;
    agreement.contested_hash_count = 2U;
    id32(agreement.contested_hashes[0], 13U);
    id32(agreement.contested_hashes[1], 13U);
    if (lx_service_agreement_encode(&agreement, record, &record_length) !=
        LXP_ERR_DUPLICATE_ENTRY)
        return 1;
    id32(agreement.contested_hashes[1], 14U);
    if (lx_service_agreement_encode(&agreement, record, &record_length) !=
            LXP_OK ||
        record_length < 64U ||
        lx_service_agreement_decode(record, record_length, &agreement) !=
            LXP_OK ||
        agreement.contested_hash_count != 2U)
        return 1;
    (void)memcpy(record + record_length - 32U, record + record_length - 64U,
                 32U);
    if (lx_service_agreement_decode(record, record_length, &agreement) !=
        LXP_ERR_DUPLICATE_ENTRY)
        return 1;
    (void)memset(record + record_length - 32U, 0, 32U);
    if (lx_service_agreement_decode(record, record_length, &agreement) !=
        LXP_ERR_NON_CANONICAL)
        return 1;

    if (lx_service_epoch_begin(&ctx, 1U, 100U) !=
            LXP_ERR_TIMESTAMP_REGRESSION ||
        lx_service_epoch_begin(&ctx, 0U, 101U) !=
            LXP_ERR_TIMESTAMP_REGRESSION ||
        lx_service_epoch_begin(NULL, 0U, 100U) !=
            LXP_ERR_TIMESTAMP_REGRESSION ||
        lx_service_epoch_begin(&ctx, 0U, 100U) != LXP_OK)
        return 1;
    if (lx_service_agreement_lookup(&ctx, swept_id, &agreement) != LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_DELIVERED ||
        agreement.default_applied)
        return 1;

    if (open_ctx(&ctx, 1300U, 48U) != 0 ||
        lx_service_epoch_begin(&ctx, 0U, 1300U) != LXP_OK ||
        lx_service_agreement_lookup(&ctx, swept_id, &agreement) != LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_ACCEPTED ||
        !agreement.default_applied || agreement.outcome_sequence != 48U ||
        agreement.outcome_timestamp != 1300U ||
        lx_service_agreement_lookup(&ctx, accepted_id, &agreement) !=
            LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_ACCEPTED ||
        agreement.default_applied ||
        lx_service_agreement_lookup(&ctx, rejected_id, &agreement) !=
            LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_REJECTED ||
        agreement.default_applied ||
        lx_service_epoch_begin(&ctx, 0U, 1300U) != LXP_OK ||
        lxp_module_ctx_commit(&ctx) != LXP_OK)
        return 1;

    {
        uint8_t event[50];
        const lxp_effect *effect = &event_buffer.effects[0];
        (void)memcpy(event, swept_id, 32U);
        event[32U] = LX_SERVICE_AGREEMENT_ACCEPTED;
        event[33U] = 1U;
        be64(event + 34U, 48U);
        be64(event + 42U, 1300U);
        if (event_buffer.count != 1U || effect->kind != LXP_EFFECT_EVENT ||
            effect->module_id != LXP_MODULE_SERVICE || effect->monetary ||
            effect->event_type != LX_SERVICE_EVENT_DEFAULT_APPLIED ||
            effect->body_length != sizeof(event) || memcmp(effect->body, event, sizeof(event)) != 0)
            return 1;
    }

    if (open_ctx(&ctx, 1400U, 49U) != 0 ||
        lx_service_agreement_lookup(&ctx, swept_id, &agreement) != LXP_OK ||
        agreement.state != LX_SERVICE_AGREEMENT_ACCEPTED ||
        !agreement.default_applied || agreement.outcome_sequence != 48U)
        return 1;

    if (lxp_state_store_destroy(&store_state) != LXP_OK ||
        test_corrected_delivery_reopen_accept() != 0 ||
        test_corrected_delivery_reopen_default_accept() != 0 ||
        test_corrected_delivery_reopen_default_reject() != 0)
        return 1;
    (void)puts("corrected delivery: reopen/accept/default accept/default reject passed");
    return 0;
}
