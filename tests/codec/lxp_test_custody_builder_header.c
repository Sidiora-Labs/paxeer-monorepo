#include "layerx/lxp_genesis_builder.h"
#include "lxp_kernel_case.h"

#include <stdlib.h>

typedef lxp_result (*fresh_empty_builder)(
    const lxp_genesis_manifest *, const uint8_t *,
    const lx_programs_metering_schedule *,
    const lx_programs_fee_genesis_parameters *, const uint8_t *, lxp_arena *,
    lxp_genesis_manifest *, lxp_snapshot_manifest_record *, lxp_byte_span *,
    lxp_byte_span *);
typedef lxp_result (*registration_encoder)(const lxp_genesis_manifest *,
                                           uint8_t *);
typedef lxp_result (*handover_builder)(const lxp_genesis_manifest *, lxp_arena *,
                                      lxp_byte_span *);
typedef lxp_result (*deployment_encoder)(const lxp_genesis_manifest *,
                                         lxp_arena *, uint8_t *);
typedef lxp_result (*artifact_builder)(const char *, const char *, const char *);
typedef lxp_result (*migration_builder)(
    const lxp_genesis_manifest *, const lxp_snapshot_manifest_record *,
    const uint8_t *, size_t, const uint8_t *, lxp_arena *,
    lxp_snapshot_manifest_record *, lxp_byte_span *);
typedef int (*builder_cli)(int, char **);

static int builder_header_without_custody(void)
{
    FILE *header;
    long length;
    size_t bytes;
    char *contents;
    int read_ok;
    int close_ok;
    int without_custody;

    LXP_CASE_CHECK(_Generic(&lxp_genesis_build_fresh_empty,
                           fresh_empty_builder: 1, default: 0));
    LXP_CASE_CHECK(_Generic(&lxp_genesis_registration_request_encode,
                           registration_encoder: 1, default: 0));
    LXP_CASE_CHECK(_Generic(&lxp_genesis_handover_trust_build,
                           handover_builder: 1, default: 0));
    LXP_CASE_CHECK(_Generic(&lxp_genesis_deployment_descriptor_encode,
                           deployment_encoder: 1, default: 0));
    LXP_CASE_CHECK(_Generic(&lxp_genesis_build_artifacts,
                           artifact_builder: 1, default: 0));
    LXP_CASE_CHECK(_Generic(&lxp_genesis_build_snapshot_migration,
                           migration_builder: 1, default: 0));
    LXP_CASE_CHECK(_Generic(&lxp_genesis_builder_cli_main,
                           builder_cli: 1, default: 0));
    LXP_CASE_CHECK(LXP_GENESIS_REGISTRATION_REQUEST_BYTES == 73);
    LXP_CASE_CHECK(LXP_GENESIS_DEPLOYMENT_DESCRIPTOR_BYTES == 105);

    header = fopen("include/layerx/lxp_genesis_builder.h", "rb");
    LXP_CASE_CHECK(header != NULL);
    if (fseek(header, 0L, SEEK_END) != 0 || (length = ftell(header)) <= 0L ||
        fseek(header, 0L, SEEK_SET) != 0) {
        fclose(header);
        LXP_CASE_CHECK(0);
    }
    bytes = (size_t)length;
    LXP_CASE_CHECK(bytes < SIZE_MAX);
    contents = malloc(bytes + 1U);
    if (contents == NULL) {
        fclose(header);
        LXP_CASE_CHECK(0);
    }
    read_ok = fread(contents, 1U, bytes, header) == bytes && !ferror(header);
    close_ok = fclose(header) == 0;
    contents[bytes] = '\0';
    without_custody = read_ok &&
        strstr(contents, "lxp_genesis_build_fresh_custody") == NULL &&
        strstr(contents, "lxp_genesis_build_fresh_custody_registry") == NULL;
    free(contents);

    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_ok);
    LXP_CASE_CHECK(without_custody);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "builder_header_without_custody", builder_header_without_custody }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
