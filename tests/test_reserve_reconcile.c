#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_reserve.h"
#include "../src/modules/asset/lx_asset_history.h"
#include "../src/modules/asset/lx_asset_native.h"

#include <string.h>
#include <unistd.h>

/* Reuse the signed production-kernel fixture, not a reserve/backing double. */
#define main reserve_commitment_reference_main
#include "test_state_commitment_transition.c"
#undef main

#include "lxp_test_harness.c"

/* Explicitly historical import/report compatibility. No external attestation
 * or outstanding claim in this case establishes native bank conservation. */
static int historical_reserve_import_contract(void)
{
    static const char *names[] = {
        "system:paxeer-reserve",
        "agent:did:key:a:main",
        "agent:did:key:a:escrow:order-1",
        "agent:did:key:a:budget:daily",
        "agent:did:key:a:stream:meter-1",
        "agent:did:key:a:margin:btc-usd",
        "system:liquidity:btc-usd",
        "system:insurance",
        "system:fees",
        "system:paxeer-withdrawals"
    };
    static const uint64_t balances[] = {70U, 10U, 3U, 2U, 1U,
                                        2U, 3U, 2U, 2U, 5U};
    lx_asset_registry assets;
    lx_asset_record asset;
    lx_account_registry accounts;
    lx_account *opened[sizeof(names) / sizeof(names[0])];
    lx_asset_custody_attestation attestation;
    lxp_outstanding_claims claims;
    lx_asset_reserve_report_record report;
    lxp_reserve_report_view refused;
    lxp_u128 total;
    uint8_t encoded[322];
    size_t encoded_length = 0U;
    size_t i;

    (void)memset(&asset, 0, sizeof(asset));
    asset.asset_id[0] = 1U;
    (void)memcpy(asset.symbol, "A", 2U);
    asset.symbol_length = 1U;
    asset.name[0] = (uint8_t)'A';
    asset.name_length = 1U;
    asset.issuer_kind = 2U;
    asset.issuer_did32[0] = 1U;
    asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    asset.custody_reference[0] = 1U;
    asset.custody_reference_length = 1U;
    if (lx_asset_registry_init(&assets, 0U) != LXP_OK ||
        lx_asset_register(&assets, &asset, 0U, (lxp_u128){0U, 0U}) != LXP_OK ||
        lx_account_registry_init(&accounts) != LXP_OK) return 1;
    for (i = 0U; i < sizeof(names) / sizeof(names[0]); ++i) {
        if (lx_asset_account_open(
                &assets, &accounts, asset.asset_id,
                (const uint8_t *)names[i], strlen(names[i]), i + 1U,
                LX_ACCOUNT_OPEN_GENESIS, NULL, &opened[i]) != LXP_OK ||
            lxp_ledger_bootstrap_balance(
                opened[i], asset.asset_id,
                (lxp_u128){0U, balances[i]}, 0U) != LXP_OK)
            return 1;
    }
    (void)memset(&attestation, 0, sizeof(attestation));
    (void)memcpy(attestation.asset_id, asset.asset_id, 32U);
    attestation.custody_amount = (lxp_u128){0U, 100U};
    attestation.checkpoint_id[0] = 2U;
    attestation.state_root[0] = 3U;
    attestation.finalized = true;
    (void)memset(&claims, 0, sizeof(claims));
    (void)memcpy(claims.asset_id, asset.asset_id, 32U);
    claims.amount = (lxp_u128){0U, 5U};
    if (lx_asset_total_units(
            &assets, &accounts, asset.asset_id, &total) != LXP_OK ||
        total.lo != 100U ||
        lx_asset_historical_reserve_reconcile_v1(
            &accounts, &attestation, &report) != LXP_OK ||
        report.agent_main.lo != 10U ||
        report.escrow.lo != 3U ||
        report.budget.lo != 2U ||
        report.stream.lo != 1U ||
        report.margin.lo != 2U ||
        report.liquidity.lo != 3U ||
        report.insurance.lo != 2U ||
        report.fees.lo != 2U ||
        report.withdrawals.lo != 5U ||
        report.reserve.lo != 70U ||
        report.circulating.lo != 30U ||
        report.expected_backing.lo != 100U ||
        report.raw_total.lo != 100U ||
        report.effective_total.lo != 100U ||
        lx_asset_historical_reserve_encode_v1(
            &report, encoded, sizeof(encoded),
            &encoded_length) != LXP_OK || encoded_length != 322U ||
        memcmp(encoded, asset.asset_id, 32U) != 0 ||
        encoded[273] != 1U ||
        memcmp(encoded + 274U, opened[2]->id, 32U) != 0 ||
        encoded[321] != 3U)
        return 1;

    /* Public custody entry points remain refused even for matching bytes. */
    if (lx_asset_reserve_report(&accounts, &attestation, &report) !=
            LXP_ERR_VERSION_UNSUPPORTED ||
        lx_asset_reserve_report_encode(&report, encoded, sizeof(encoded),
            &encoded_length) != LXP_ERR_VERSION_UNSUPPORTED ||
        lxp_reserve_reconcile(&accounts, &attestation, &claims, &refused) !=
            LXP_FATAL_SUPPLY_MISMATCH ||
        lxp_supply_invariant_check(
            &assets, &accounts, &attestation, 1U, &claims, 1U) !=
            LXP_FATAL_SUPPLY_MISMATCH)
        return 1;

    claims.amount.lo = 4U;
    if (lxp_reserve_reconcile(
            &accounts, &attestation, &claims, &refused) !=
            LXP_FATAL_SUPPLY_MISMATCH)
        return 1;
    claims.amount.lo = 5U;
    if (lxp_ledger_bootstrap_balance(
            opened[1], asset.asset_id, (lxp_u128){0U, 11U},
            opened[1]->next_sequence) != LXP_OK ||
        lxp_supply_invariant_check(
            &assets, &accounts, &attestation, 1U, &claims, 1U) !=
            LXP_FATAL_SUPPLY_MISMATCH ||
        lx_asset_historical_reserve_reconcile_v1(
            &accounts, &attestation, &report) != LXP_FATAL_SUPPLY_MISMATCH)
        return 1;
    lx_account_registry_release(&accounts);
    return 0;
}

static int reserve_submit(fixture *f, size_t from_slot, size_t to_slot,
                           uint64_t amount, uint8_t ordinal)
{
    lxp_send send;
    lxp_identity *identity;
    uint8_t material[144];
    uint8_t message[512];
    uint8_t digest[32];
    uint8_t before[32];
    uint8_t after[32];
    size_t length;
    lx_asset_native_supply_report supply;
    lxp_u128 from_before = f->accounts.accounts[from_slot].balance;
    lxp_u128 to_before = f->accounts.accounts[to_slot].balance;

    REQUIRE(lx_asset_native_reconcile(&f->asset, &f->accounts,
        &supply) == LXP_OK && supply.bank_units.hi == 0U &&
        supply.bank_units.lo == 10U && supply.registered_units.hi == 0U &&
        supply.registered_units.lo == 10U);
    REQUIRE(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    (void)memset(&send, 0, sizeof(send));
    (void)memcpy(send.from, f->accounts.accounts[from_slot].id, 32U);
    (void)memcpy(send.to, f->accounts.accounts[to_slot].id, 32U);
    (void)memcpy(send.asset, f->asset.asset_id, 32U);
    send.amount = (lxp_u128){0U, amount};
    send.sequence = f->accounts.accounts[from_slot].next_sequence;
    send.expires_at = 100U;
    send.idempotency_key[0] = ordinal;
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    (void)memcpy(send.authorization.controller, send.from, 32U);
    (void)memcpy(send.authorization.public_key, f->public_key, 32U);
    (void)memcpy(material, send.from, 32U);
    (void)memcpy(material + 32U, send.to, 32U);
    (void)memcpy(material + 64U, send.asset, 32U);
    REQUIRE(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    (void)memcpy(material + 112U, send.idempotency_key, 32U);
    REQUIRE(lxp_hash_context_value(material, sizeof(material),
                                    send.context_hash) == LXP_OK);
    (void)memcpy(send.authorization.signed_context_hash,
                 send.context_hash, 32U);
    REQUIRE(lxp_send_authorization_message(&send, message, sizeof(message),
                                            &length) == LXP_OK);
    REQUIRE(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, length,
                             digest) == LXP_OK);
    REQUIRE(sign_digest(digest, send.authorization.signature,
                         f->public_key) == 0);
    REQUIRE(lxp_send_encode(&send, f->payload, sizeof(f->payload),
                             &length) == LXP_OK);
    REQUIRE(lxp_identity_resolve(&f->identities, did, sizeof(did) - 1U,
                                  &identity) == LXP_OK);
    f->activity.account_sequence = identity->next_sequence;
    f->activity.payload = (lxp_byte_span){f->payload, length};
    (void)memcpy(f->activity.idempotency_key, send.idempotency_key, 32U);
    (void)memcpy(f->authority.principal, send.from, 32U);
    f->execution.global_sequence = f->state.next_sequence;
    f->execution.batch_number = ordinal;
    REQUIRE(lxp_hash_payload(f->payload, length,
                              f->activity.payload_hash) == LXP_OK);
    REQUIRE(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    REQUIRE(sign_digest(digest, f->signature, f->public_key) == 0);
    REQUIRE(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    REQUIRE(lxp_state_root(&f->kernel, before) == LXP_OK);
    (void)memcpy(f->kernel.current_state_root, before, 32U);
    REQUIRE(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_OK);
    REQUIRE(f->receipt.result_code == LXP_OK);
    REQUIRE(f->receipt.global_sequence == ordinal);
    REQUIRE(f->receipt.from_balance_before.hi == 0U &&
            f->receipt.from_balance_before.lo == from_before.lo);
    REQUIRE(f->receipt.from_balance_after.hi == 0U &&
            f->receipt.from_balance_after.lo == from_before.lo - amount);
    REQUIRE(f->receipt.to_balance_before.hi == 0U &&
            f->receipt.to_balance_before.lo == to_before.lo);
    REQUIRE(f->receipt.to_balance_after.hi == 0U &&
            f->receipt.to_balance_after.lo == to_before.lo + amount);
    REQUIRE(f->receipt.supply_binding_version == 1U &&
            f->receipt.total_units_before.hi == 0U &&
            f->receipt.total_units_before.lo == 10U &&
            f->receipt.total_units_after.hi == 0U &&
            f->receipt.total_units_after.lo == 10U);
    REQUIRE(lxp_receipt_verify(&f->receipt, f->public_key,
                                &f->arena) == LXP_OK);
    REQUIRE(lxp_state_root(&f->kernel, after) == LXP_OK);
    REQUIRE(memcmp(after, f->receipt.resulting_state_root, 32U) == 0 &&
            memcmp(before, f->receipt.previous_state_root, 32U) == 0 &&
            memcmp(before, after, 32U) != 0);
    REQUIRE(f->accounts.accounts[from_slot].balance.lo == from_before.lo - amount);
    REQUIRE(f->accounts.accounts[to_slot].balance.lo == to_before.lo + amount);
    REQUIRE(lx_asset_native_reconcile(&f->asset, &f->accounts,
        &supply) == LXP_OK && supply.bank_units.hi == 0U &&
        supply.bank_units.lo == 10U && supply.registered_units.hi == 0U &&
        supply.registered_units.lo == 10U);
    return 0;
}

/* C ledger conservation at recorded sequences, including a real disk snapshot
 * and destruction/reopening of the component. Native bank block commits are
 * a separate owner wiring request in test_lxgb_producers.py. */
static int kernel_ledger_conservation_recovery(void)
{
    static const uint8_t main_name[] = "agent:did:key:alice:main";
    static const uint8_t subaccount_name[] =
        "agent:did:key:alice:asset:0300000000000000000000000000000000000000000000000000000000000000";
    static const uint8_t merchant_name[] =
        "agent:did:key:bob:asset:0300000000000000000000000000000000000000000000000000000000000000";
    static uint8_t io_bytes[4U * 1024U * 1024U];
    char directory[] = "/tmp/lxp-ledger-conservation-XXXXXX";
    char path[128];
    fixture *f = calloc(1U, sizeof(*f));
    lxp_snapshot_manifest_record manifest;
    lxp_snapshot_manifest_record loaded_manifest;
    lxp_byte_span snapshot;
    lxp_byte_span loaded;
    lxp_arena io;
    lx_account *merchant;
    uint8_t merchant_id[32];
    uint8_t root[32];
    uint8_t restored_root[32];
    uint8_t ids[3][32];
    uint64_t sequences[3];
    static const uint64_t final_balances[3] = {8U, 0U, 2U};
    size_t index;

    REQUIRE(f != NULL && prepare_names(f,
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, true,
        main_name, subaccount_name) == 0);
    f->asset.total_units = (lxp_u128){0U, 10U};
    f->asset.custody_kind = LX_ASSET_CUSTODY_NATIVE;
    REQUIRE(lx_account_id_from_string(merchant_name,
        sizeof(merchant_name) - 1U, merchant_id) == LXP_OK);
    REQUIRE(lx_account_open(&f->accounts, merchant_name,
        sizeof(merchant_name) - 1U, merchant_id, 1U,
        LX_ACCOUNT_OPEN_GENESIS, NULL, &merchant) == LXP_OK);
    REQUIRE(lxp_ledger_bootstrap_balance(merchant, f->asset.asset_id,
        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    f->accounts.accounts[1].has_authority_key = true;
    (void)memcpy(f->accounts.accounts[1].authority_key, f->public_key, 32U);

    REQUIRE(reserve_submit(f, 0U, 1U, 6U, 1U) == 0); /* deposit */
    REQUIRE(f->accounts.accounts[0].balance.lo == 4U &&
            f->accounts.accounts[1].balance.lo == 6U);
    REQUIRE(reserve_submit(f, 1U, 2U, 2U, 2U) == 0); /* spend */
    REQUIRE(f->accounts.accounts[1].balance.lo == 4U &&
            f->accounts.accounts[2].balance.lo == 2U);
    REQUIRE(reserve_submit(f, 1U, 0U, 4U, 3U) == 0); /* withdraw */
    REQUIRE(f->state.next_sequence == 4U && f->accounts.count == 3U);
    for (index = 0U; index < 3U; ++index) {
        REQUIRE(f->accounts.accounts[index].balance.hi == 0U &&
                f->accounts.accounts[index].balance.lo == final_balances[index]);
        (void)memcpy(ids[index], f->accounts.accounts[index].id, 32U);
        sequences[index] = f->accounts.accounts[index].next_sequence;
    }
    REQUIRE(lxp_state_root(&f->kernel, root) == LXP_OK);
    REQUIRE(lxp_arena_init(&io, io_bytes, sizeof(io_bytes)) == LXP_OK);
    REQUIRE(lxp_snapshot_write(&f->kernel, 3U, &io, &snapshot) == LXP_OK);
    REQUIRE(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
        3U, root, f->kernel.current_state_root, &manifest) == LXP_OK);
    REQUIRE(mkdtemp(directory) != NULL);
    REQUIRE(lxp_snapshot_store_write(directory, &manifest,
        snapshot.bytes, snapshot.length) == LXP_OK);
    REQUIRE(snprintf(path, sizeof(path), "%s/%020u.lxs", directory, 3U) > 0);
    REQUIRE(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);

    f = calloc(1U, sizeof(*f));
    REQUIRE(f != NULL && prepare_names(f,
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, true,
        main_name, subaccount_name) == 0);
    f->asset.total_units = (lxp_u128){0U, 10U};
    f->asset.custody_kind = LX_ASSET_CUSTODY_NATIVE;
    REQUIRE(lxp_arena_reset(&io, 0U) == LXP_OK);
    REQUIRE(lxp_snapshot_store_read(path, &io, &loaded_manifest,
        &loaded) == LXP_OK);
    REQUIRE(lxp_snapshot_load(loaded.bytes, loaded.length,
        &loaded_manifest, &f->kernel) == LXP_OK);
    REQUIRE(lxp_snapshot_verify_root(&f->kernel, &loaded_manifest) == LXP_OK);
    REQUIRE(lxp_state_root(&f->kernel, restored_root) == LXP_OK);
    REQUIRE(memcmp(root, restored_root, 32U) == 0 &&
            f->state.next_sequence == 4U && f->accounts.count == 3U);
    REQUIRE(loaded_manifest.global_sequence == 3U);
    {
        lx_asset_native_supply_report supply;
        REQUIRE(lx_asset_native_reconcile(&f->asset, &f->accounts,
            &supply) == LXP_OK && supply.bank_units.hi == 0U &&
            supply.bank_units.lo == 10U && supply.registered_units.hi == 0U &&
            supply.registered_units.lo == 10U);
    }
    for (index = 0U; index < 3U; ++index) {
        size_t slot;
        REQUIRE(lx_account_registry_index_lookup(&f->accounts, ids[index],
            &slot) == LXP_OK);
        REQUIRE(f->accounts.accounts[slot].balance.hi == 0U &&
                f->accounts.accounts[slot].balance.lo == final_balances[index]);
        REQUIRE(f->accounts.accounts[slot].next_sequence == sequences[index]);
    }
    REQUIRE(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    REQUIRE(unlink(path) == 0 && rmdir(directory) == 0);
    return 0;
}

/* The account codec and derivation used by lxphost. Native bank InitGenesis
 * remains an owner wiring request recorded in test_lxgb_producers.py. */
static int genesis_subaccount_allocation_commitment(void)
{
    static const uint8_t names[2][128] = {
        "agent:did:key:alice:main",
        "agent:did:key:alice:asset:0300000000000000000000000000000000000000000000000000000000000000"
    };
    static const uint8_t expected_subaccount[32] = {
        0xcdU, 0x99U, 0xe0U, 0x0bU, 0x8cU, 0x68U, 0x97U, 0xb6U,
        0xbaU, 0x6eU, 0x25U, 0x04U, 0x92U, 0xdeU, 0x75U, 0x10U,
        0x3bU, 0x65U, 0xfcU, 0xb5U, 0xe7U, 0xc4U, 0x83U, 0x13U,
        0xbbU, 0xb9U, 0x18U, 0x32U, 0x37U, 0xa2U, 0xc3U, 0xe7U
    };
    static const uint64_t allocations[2] = {60U, 40U};
    lx_account_registry allocated;
    lx_account_registry imported;
    uint8_t asset[32] = {3U};
    uint8_t owner[32] = {0};
    uint8_t root[32];
    uint8_t restored_root[32];
    uint8_t encoded[2][1024];
    uint8_t malformed[1025];
    size_t lengths[2];
    size_t index;

    if (lx_account_registry_init(&allocated) != LXP_OK ||
        lx_account_registry_init(&imported) != LXP_OK)
        return 1;
    for (index = 0U; index < 2U; ++index) {
        uint8_t id[32];
        uint8_t derived_owner[32];
        lx_account *account;
        if (lx_account_id_from_string(names[index],
                strlen((const char *)names[index]), id) != LXP_OK ||
            lx_account_open(&allocated, names[index],
                strlen((const char *)names[index]), id, 1U,
                LX_ACCOUNT_OPEN_GENESIS, NULL, &account) != LXP_OK ||
            lxp_ledger_bootstrap_balance(account, asset,
                (lxp_u128){0U, allocations[index]}, 0U) != LXP_OK ||
            lx_account_owner_did(account, derived_owner) != LXP_OK)
            return 1;
        if (index == 0U) {
            (void)memcpy(owner, derived_owner, sizeof(owner));
        } else if (account->kind != LX_ACCOUNT_AGENT_ASSET ||
                   memcmp(id, expected_subaccount, 32U) != 0 ||
                   memcmp(owner, derived_owner, 32U) != 0) {
            return 1;
        }
        if (lx_account_record_encode(account, encoded[index],
                sizeof(encoded[index]), &lengths[index]) != LXP_OK)
            return 1;
    }
    if (lx_account_registry_root(&allocated, root) != LXP_OK)
        return 1;
    lx_account_registry_release(&allocated);
    for (index = 0U; index < 2U; ++index) {
        lx_account decoded;
        size_t prefix;
        for (prefix = 0U; prefix < lengths[index]; ++prefix)
            if (lx_account_record_decode(encoded[index], prefix,
                    &decoded) == LXP_OK)
                return 1;
        (void)memcpy(malformed, encoded[index], lengths[index]);
        malformed[lengths[index]] = 0U;
        if (lx_account_record_decode(malformed, lengths[index] + 1U,
                &decoded) == LXP_OK)
            return 1;
        if (lx_account_record_decode(encoded[index], lengths[index],
                &decoded) != LXP_OK ||
            decoded.balance.hi != 0U ||
            decoded.balance.lo != allocations[index] ||
            memcmp(decoded.asset_id, asset, 32U) != 0 ||
            lx_account_registry_slot_insert(&imported, &decoded, NULL) != LXP_OK)
            return 1;
    }
    if (lx_account_registry_root(&imported, restored_root) != LXP_OK ||
        memcmp(root, restored_root, 32U) != 0)
        return 1;
    {
        lx_asset_record native = {0};
        lx_asset_native_supply_report supply;
        (void)memcpy(native.asset_id, asset, 32U);
        native.custody_kind = LX_ASSET_CUSTODY_NATIVE;
        native.total_units = (lxp_u128){0U, 100U};
        if (lx_asset_native_reconcile(&native, &imported, &supply) != LXP_OK ||
            supply.bank_units.hi != 0U || supply.bank_units.lo != 100U ||
            supply.registered_units.hi != 0U ||
            supply.registered_units.lo != 100U)
            return 1;
        native.total_units.lo = 99U;
        if (lx_asset_native_reconcile(&native, &imported, &supply) !=
                LXP_FATAL_SUPPLY_MISMATCH)
            return 1;
    }
    lx_account_registry_release(&imported);
    return 0;
}

int main(int argc, char **argv)
{
    if (lxp_test_register("genesis_subaccount_allocation_commitment",
            genesis_subaccount_allocation_commitment) != 0 ||
        lxp_test_register("kernel_ledger_conservation_recovery",
            kernel_ledger_conservation_recovery) != 0 ||
        lxp_test_register("historical_reserve_import_contract",
            historical_reserve_import_contract) != 0)
        return 1;
    return lxp_test_main(argc, argv);
}
