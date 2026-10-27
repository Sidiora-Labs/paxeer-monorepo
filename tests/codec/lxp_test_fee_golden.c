#include "lxp_kernel_case.h"

#include "layerx/lxp_fee.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"

#include <inttypes.h>
#include <stdlib.h>

lxp_result lxp_fee_committed_schedule(const struct lxp_kernel *kernel,
    uint32_t parameter_version, lxp_fee_params *schedule);
lxp_result lxp_fee_compute(const lxp_fee_params *parameters,
    uint32_t activity_type, lxp_fee_meter meter, lxp_u128 *fee);

typedef struct fee_genesis_fixture {
    lxp_genesis_manifest manifest;
    lx_account_registry accounts;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_arena arena;
    uint8_t arena_bytes[8388608U];
    lxp_fee_params schedule;
} fee_genesis_fixture;

static int fee_genesis_open(fee_genesis_fixture *fixture)
{
    uint8_t encoded[65536U];
    lxp_genesis_module_plan plan;
    uint32_t parameter_version;
    FILE *file = fopen(
        "tests/fixtures/public-testnet-genesis/genesis.manifest", "rb");
    size_t length;
    int read_failed;

    LXP_CASE_CHECK(file != NULL);
    length = fread(encoded, 1U, sizeof(encoded), file);
    read_failed = ferror(file) || !feof(file);
    if (fclose(file) != 0) read_failed = 1;
    LXP_CASE_CHECK(!read_failed);
    LXP_CASE_CHECK(lxp_genesis_parse(encoded, length,
        LXP_GENESIS_INPUT_MANIFEST, &fixture->manifest) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&fixture->arena, fixture->arena_bytes,
        sizeof(fixture->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&fixture->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&fixture->state,
        &fixture->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
        &fixture->journal, &fixture->manifest, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(&fixture->manifest,
        &plan) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_register(&plan,
        &fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_materialize(&fixture->manifest,
        &fixture->arena, &fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_parameter_version(&fixture->manifest,
        &parameter_version) == LXP_OK);
    LXP_CASE_CHECK(lxp_fee_committed_schedule(&fixture->kernel,
        parameter_version, &fixture->schedule) == LXP_OK);
    return 0;
}

static int fee_genesis_close(fee_genesis_fixture *fixture)
{
    while (fixture->kernel.blob_count != 0U)
        free(fixture->kernel.blobs[--fixture->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&fixture->state) == LXP_OK);
    lx_account_registry_release(&fixture->accounts);
    free(fixture);
    return 0;
}

static int fee_schedule_golden_bytes(void)
{
    static const uint8_t golden_digest[32] = {
        0x37U, 0xfcU, 0x42U, 0x34U, 0xaaU, 0x53U, 0x87U, 0xceU,
        0xf1U, 0x41U, 0x99U, 0x7aU, 0xc9U, 0x2cU, 0x16U, 0xf0U,
        0xb7U, 0x6eU, 0x92U, 0xafU, 0xd3U, 0x23U, 0xc8U, 0xadU,
        0x5bU, 0x74U, 0xf1U, 0xaeU, 0xb9U, 0x05U, 0x70U, 0xe4U
    };
    fee_genesis_fixture *fixture = calloc(1U, sizeof(*fixture));
    uint8_t encoded[LXP_FEE_PARAMS_V4_BYTES];
    uint8_t digest[32];
    size_t length = 0U;

    LXP_CASE_CHECK(fixture != NULL);
    LXP_CASE_CHECK(fee_genesis_open(fixture) == 0);
    LXP_CASE_CHECK(fixture->schedule.version == 4U);
    LXP_CASE_CHECK(lxp_fee_params_encode(&fixture->schedule, encoded,
        sizeof(encoded), &length) == LXP_OK);
    LXP_CASE_CHECK(length == 368U);
    LXP_CASE_CHECK(lxp_hash_sha256(encoded, length, digest) == LXP_OK);
    LXP_CASE_CHECK(memcmp(digest, golden_digest, sizeof(digest)) == 0);
    LXP_CASE_CHECK(fee_genesis_close(fixture) == 0);
    return 0;
}

typedef struct fee_golden_entry {
    uint32_t activity_type;
    lxp_u128 fee;
} fee_golden_entry;

static int fee_per_activity_golden_table(void)
{
    static const fee_golden_entry golden[] = {
        {0x00010001U, {0U, 0U}},
        {0x00010002U, {0U, 0U}},
        {0x00010003U, {0U, 0U}},
        {0x00010004U, {0U, 0U}},
        {0x00010005U, {0U, 0U}},
        {0x00010006U, {0U, 0U}},
        {0x00010007U, {0U, 0U}},
        {0x00010008U, {0U, 0U}},
        {0x0001000aU, {0U, 0U}},
        {0x0001000bU, {0U, 0U}},
        {0x00010009U, {0U, 0U}},
        {0x00020001U, {0U, 4U}},
        {0x00020002U, {0U, 4U}},
        {0x00020003U, {0U, 4U}},
        {0x00020004U, {0U, 4U}},
        {0x00020005U, {0U, 4U}},
        {0x00020006U, {0U, 4U}},
        {0x00020007U, {0U, 4U}},
        {0x00030001U, {0U, 4U}},
        {0x00030002U, {0U, 4U}},
        {0x00030003U, {0U, 4U}},
        {0x00030004U, {0U, 4U}},
        {0x00030005U, {0U, 4U}},
        {0x00030006U, {0U, 4U}},
        {0x00030007U, {0U, 4U}},
        {0x00030008U, {0U, 4U}},
        {0x00030009U, {0U, 4U}},
        {0x0003000aU, {0U, 4U}},
        {0x0003000bU, {0U, 4U}},
        {0x00040001U, {0U, 4U}},
        {0x00040002U, {0U, 4U}},
        {0x00040003U, {0U, 4U}},
        {0x00040004U, {0U, 4U}},
        {0x00040005U, {0U, 4U}},
        {0x00040006U, {0U, 4U}},
        {0x00040007U, {0U, 4U}},
        {0x00040008U, {0U, 4U}},
        {0x00050001U, {0U, 4U}},
        {0x00050002U, {0U, 4U}},
        {0x00050003U, {0U, 4U}},
        {0x00050004U, {0U, 4U}},
        {0x00050005U, {0U, 4U}},
        {0x00050006U, {0U, 4U}},
        {0x00050007U, {0U, 4U}},
        {0x00050008U, {0U, 4U}},
        {0x00050009U, {0U, 4U}},
        {0x0005000aU, {0U, 4U}},
        {0x0005000bU, {0U, 4U}},
        {0x0005000cU, {0U, 4U}},
        {0x0005000dU, {0U, 4U}},
        {0x00060001U, {0U, 4U}},
        {0x00060002U, {0U, 4U}},
        {0x00060003U, {0U, 4U}},
        {0x00060004U, {0U, 4U}},
        {0x00060005U, {0U, 4U}},
        {0x00060006U, {0U, 4U}},
        {0x00060007U, {0U, 4U}},
        {0x00060008U, {0U, 4U}},
        {0x00060009U, {0U, 4U}},
        {0x0006000aU, {0U, 4U}},
        {0x0006000bU, {0U, 4U}},
        {0x00070001U, {0U, 4U}},
        {0x00070002U, {0U, 4U}},
        {0x00070003U, {0U, 4U}},
        {0x00070005U, {0U, 4U}},
        {0x00070006U, {0U, 4U}},
        {0x00070008U, {0U, 4U}},
        {0x0007000aU, {0U, 4U}},
        {0x0007000bU, {0U, 4U}},
        {0x00080001U, {0U, 0U}}
    };
    const size_t golden_count = sizeof(golden) / sizeof(golden[0]);
    fee_genesis_fixture *fixture = calloc(1U, sizeof(*fixture));
    const lxp_genesis_module_entry *modules;
    size_t module_count = 0U;
    size_t covered = 0U;
    bool seen[sizeof(golden) / sizeof(golden[0])] = {false};

    LXP_CASE_CHECK(fixture != NULL);
    LXP_CASE_CHECK(fee_genesis_open(fixture) == 0);
    LXP_CASE_CHECK(fixture->schedule.version == 4U);
    LXP_CASE_CHECK(fixture->schedule.asset_price_count == 11U);
    LXP_CASE_CHECK(fixture->schedule.module_price_count == 7U);
    for (size_t i = 0U; i < golden_count; ++i) {
        lxp_u128 actual = {UINT64_MAX, UINT64_MAX};
        LXP_CASE_CHECK(lxp_fee_compute(&fixture->schedule,
            golden[i].activity_type, (lxp_fee_meter){0}, &actual) == LXP_OK);
        if (lxp_u128_cmp(actual, golden[i].fee) != 0) {
            fprintf(stderr, "activity 0x%08" PRIx32
                ": expected={%" PRIu64 ",%" PRIu64
                "} actual={%" PRIu64 ",%" PRIu64 "}\n",
                golden[i].activity_type, golden[i].fee.hi, golden[i].fee.lo,
                actual.hi, actual.lo);
            return 1;
        }
    }
    modules = lxp_genesis_module_table(&module_count);
    LXP_CASE_CHECK(modules != NULL);
    for (size_t i = 0U; i < module_count; ++i) {
        const lxp_module_iface *iface;
        if (modules[i].module_id < LXP_MODULE_ASSET ||
            modules[i].module_id > LXP_MODULE_BRIDGE) continue;
        iface = modules[i].iface();
        LXP_CASE_CHECK(iface != NULL && iface->activity_types != NULL);
        LXP_CASE_CHECK(iface->activity_type_count != 0U);
        for (size_t j = 0U; j < iface->activity_type_count; ++j) {
            size_t k;
            for (k = 0U; k < golden_count; ++k)
                if (golden[k].activity_type == iface->activity_types[j]) break;
            LXP_CASE_CHECK(k < golden_count);
            LXP_CASE_CHECK(!seen[k]);
            seen[k] = true;
            ++covered;
        }
    }
    LXP_CASE_CHECK(covered == golden_count);
    LXP_CASE_CHECK(fee_genesis_close(fixture) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"fee_schedule_golden_bytes", fee_schedule_golden_bytes},
        {"fee_per_activity_golden_table", fee_per_activity_golden_table}
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
        argc, argv);
}
