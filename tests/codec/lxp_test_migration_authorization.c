#include "layerx/lxp_snapshot.h"
#include "lxp_kernel_case.h"

#include <stdint.h>
#include <string.h>

lxp_result lxp_snapshot_migration_authorization_encode(
    const lxp_snapshot_manifest_record *target, bool include_signature,
    uint8_t encoded[LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES],
    size_t *encoded_length);
lxp_result lxp_snapshot_migration_authorization_decode(
    const uint8_t *encoded, size_t encoded_length,
    lxp_snapshot_manifest_record *target);

static void fill_bytes(uint8_t *bytes, size_t length, uint8_t seed)
{
    size_t i;
    for (i = 0U; i < length; ++i) bytes[i] = (uint8_t)(seed + i);
}

static lxp_snapshot_manifest_record migration_manifest(void)
{
    lxp_snapshot_manifest_record target = { 0 };
    lxp_snapshot_migration_authorization *migration = &target.migration;

    target.global_sequence = UINT64_C(0x0102030405060708);
    fill_bytes(target.canonical_state_root, 32U, 0x10U);
    fill_bytes(target.receipt_state_root, 32U, 0x30U);
    fill_bytes(target.snapshot_digest, 32U, 0x50U);
    migration->present = true;
    migration->network_id = UINT32_C(0x10203040);
    migration->renamed_account_count = UINT16_C(0x1234);
    migration->source_global_sequence = target.global_sequence;
    fill_bytes(migration->source_canonical_state_root, 32U, 0x70U);
    fill_bytes(migration->source_receipt_state_root, 32U, 0x90U);
    fill_bytes(migration->source_snapshot_digest, 32U, 0xb0U);
    fill_bytes(migration->signer_public_key, 32U, 0xd0U);
    fill_bytes(migration->signature, 64U, 0x20U);
    return target;
}

static int migration_equal(
    const lxp_snapshot_migration_authorization *left,
    const lxp_snapshot_migration_authorization *right)
{
    return left->present == right->present &&
           left->network_id == right->network_id &&
           left->renamed_account_count == right->renamed_account_count &&
           left->source_global_sequence == right->source_global_sequence &&
           memcmp(left->source_canonical_state_root,
                  right->source_canonical_state_root, 32U) == 0 &&
           memcmp(left->source_receipt_state_root,
                  right->source_receipt_state_root, 32U) == 0 &&
           memcmp(left->source_snapshot_digest,
                  right->source_snapshot_digest, 32U) == 0 &&
           memcmp(left->signer_public_key, right->signer_public_key, 32U) == 0 &&
           memcmp(left->signature, right->signature, 64U) == 0;
}

static int manifest_equal(const lxp_snapshot_manifest_record *left,
                          const lxp_snapshot_manifest_record *right)
{
    return left->global_sequence == right->global_sequence &&
           memcmp(left->canonical_state_root,
                  right->canonical_state_root, 32U) == 0 &&
           memcmp(left->receipt_state_root,
                  right->receipt_state_root, 32U) == 0 &&
           memcmp(left->snapshot_digest, right->snapshot_digest, 32U) == 0 &&
           migration_equal(&left->migration, &right->migration);
}

static int migration_authorization_roundtrip(void)
{
    const lxp_snapshot_manifest_record target = migration_manifest();
    const lxp_snapshot_migration_authorization *migration = &target.migration;
    const uint8_t prefix[] = {
        'L', 'X', 'S', 'M', 1U, 0x10U, 0x20U, 0x30U, 0x40U,
        1U, 2U, 3U, 4U, 5U, 6U, 7U, 8U, 0x12U, 0x34U
    };
    const uint8_t sequence[] = { 1U, 2U, 3U, 4U, 5U, 6U, 7U, 8U };
    uint8_t signed_bytes[LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES + 1U];
    uint8_t unsigned_bytes[LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES];
    uint8_t reencoded[LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES];
    lxp_snapshot_manifest_record decoded = target;
    size_t signed_length = 0U;
    size_t unsigned_length = 0U;
    size_t reencoded_length = 0U;
    size_t i;

    (void)memset(signed_bytes, 0xa5, sizeof(signed_bytes));
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &target, true, signed_bytes, &signed_length) == LXP_OK);
    LXP_CASE_CHECK(signed_length == LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES);
    LXP_CASE_CHECK(signed_length == 315U);
    LXP_CASE_CHECK(signed_bytes[315] == 0xa5U);
    LXP_CASE_CHECK(memcmp(signed_bytes, prefix, sizeof(prefix)) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 19U,
                          migration->source_canonical_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 51U,
                          migration->source_receipt_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 83U,
                          migration->source_snapshot_digest, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 115U,
                          sequence, sizeof(sequence)) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 123U,
                          target.canonical_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 155U,
                          target.receipt_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 187U,
                          target.snapshot_digest, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 219U,
                          migration->signer_public_key, 32U) == 0);
    LXP_CASE_CHECK(memcmp(signed_bytes + 251U,
                          migration->signature, 64U) == 0);

    (void)memset(unsigned_bytes, 0xa5, sizeof(unsigned_bytes));
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &target, false, unsigned_bytes, &unsigned_length) == LXP_OK);
    LXP_CASE_CHECK(unsigned_length == 251U);
    LXP_CASE_CHECK(memcmp(unsigned_bytes, signed_bytes, unsigned_length) == 0);
    for (i = unsigned_length; i < sizeof(unsigned_bytes); ++i)
        LXP_CASE_CHECK(unsigned_bytes[i] == 0xa5U);

    (void)memset(&decoded.migration, 0, sizeof(decoded.migration));
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_decode(
        signed_bytes, signed_length, &decoded) == LXP_OK);
    LXP_CASE_CHECK(manifest_equal(&decoded, &target));
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &decoded, true, reencoded, &reencoded_length) == LXP_OK);
    LXP_CASE_CHECK(reencoded_length == signed_length);
    LXP_CASE_CHECK(memcmp(reencoded, signed_bytes, signed_length) == 0);
    return 0;
}

static int decode_refuses(const uint8_t *encoded, size_t length,
                          const lxp_snapshot_manifest_record *target,
                          lxp_result expected)
{
    lxp_snapshot_manifest_record decoded = *target;
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_decode(
        encoded, length, &decoded) == expected);
    LXP_CASE_CHECK(manifest_equal(&decoded, target));
    return 0;
}

static int migration_authorization_refusals(void)
{
    const lxp_snapshot_manifest_record target = migration_manifest();
    lxp_snapshot_manifest_record invalid = target;
    uint8_t *required_fields[] = {
        invalid.migration.source_canonical_state_root,
        invalid.migration.source_receipt_state_root,
        invalid.migration.source_snapshot_digest,
        invalid.canonical_state_root,
        invalid.receipt_state_root,
        invalid.snapshot_digest,
        invalid.migration.signer_public_key
    };
    uint8_t *target_fields[] = {
        invalid.canonical_state_root,
        invalid.receipt_state_root,
        invalid.snapshot_digest
    };
    const size_t binding_offsets[] = { 115U, 123U, 155U, 187U };
    uint8_t encoded[LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES + 1U] = { 0U };
    uint8_t changed[LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES];
    size_t length = 0U;
    size_t i;

    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &target, true, encoded, &length) == LXP_OK);
    LXP_CASE_CHECK(length == LXP_SNAPSHOT_MIGRATION_AUTHORIZATION_BYTES);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        NULL, true, changed, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &target, true, NULL, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &target, true, changed, NULL) == LXP_ERR_NON_CANONICAL);

    for (i = 0U; i < sizeof(required_fields) / sizeof(required_fields[0]);
         ++i) {
        invalid = target;
        (void)memset(required_fields[i], 0, 32U);
        LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
            &invalid, true, changed, &length) == LXP_ERR_NON_CANONICAL);
        LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
            &invalid, false, changed, &length) == LXP_ERR_NON_CANONICAL);
    }
    invalid = target;
    invalid.migration.present = false;
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &invalid, true, changed, &length) == LXP_ERR_NON_CANONICAL);
    invalid = target;
    invalid.migration.network_id = 0U;
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &invalid, true, changed, &length) == LXP_ERR_NON_CANONICAL);
    invalid = target;
    invalid.migration.renamed_account_count = 0U;
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &invalid, true, changed, &length) == LXP_ERR_NON_CANONICAL);
    invalid = target;
    invalid.migration.source_global_sequence ^= UINT64_C(1);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &invalid, true, changed, &length) == LXP_ERR_NON_CANONICAL);
    invalid = target;
    (void)memset(invalid.migration.signature, 0, 64U);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &invalid, true, changed, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_encode(
        &invalid, false, changed, &length) == LXP_OK);
    LXP_CASE_CHECK(length == 251U);
    LXP_CASE_CHECK(memcmp(changed, encoded, length) == 0);

    LXP_CASE_CHECK(decode_refuses(NULL, sizeof(changed), &target,
                                LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(lxp_snapshot_migration_authorization_decode(
        encoded, sizeof(changed), NULL) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(decode_refuses(encoded, 251U, &target,
                                LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(decode_refuses(encoded, sizeof(changed) - 1U, &target,
                                LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(decode_refuses(encoded, sizeof(encoded), &target,
                                LXP_ERR_NON_CANONICAL) == 0);
    for (i = 0U; i < 5U; ++i) {
        (void)memcpy(changed, encoded, sizeof(changed));
        changed[i] ^= 1U;
        LXP_CASE_CHECK(decode_refuses(changed, sizeof(changed), &target,
                                    LXP_ERR_VERSION_UNSUPPORTED) == 0);
    }
    for (i = 0U; i < sizeof(binding_offsets) / sizeof(binding_offsets[0]);
         ++i) {
        (void)memcpy(changed, encoded, sizeof(changed));
        changed[binding_offsets[i]] ^= 1U;
        LXP_CASE_CHECK(decode_refuses(changed, sizeof(changed), &target,
                                    LXP_ERR_SNAPSHOT_MISMATCH) == 0);
    }
    invalid = target;
    invalid.global_sequence ^= UINT64_C(1);
    LXP_CASE_CHECK(decode_refuses(encoded, sizeof(changed), &invalid,
                                LXP_ERR_SNAPSHOT_MISMATCH) == 0);
    for (i = 0U; i < sizeof(target_fields) / sizeof(target_fields[0]); ++i) {
        invalid = target;
        target_fields[i][31] ^= 1U;
        LXP_CASE_CHECK(decode_refuses(encoded, sizeof(changed), &invalid,
                                    LXP_ERR_SNAPSHOT_MISMATCH) == 0);
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "migration_authorization_roundtrip",
          migration_authorization_roundtrip },
        { "migration_authorization_refusals", migration_authorization_refusals }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
