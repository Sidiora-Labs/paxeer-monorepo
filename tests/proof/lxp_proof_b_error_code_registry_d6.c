/* Proves: include/layerx/lxp_result.h LXP_RESULT_CODE_LIST / lx_service_dispute_open_execute / lx_service_dispute_resolve_execute -> precompiles/common/lxp_errors.go LXPCodeName/LXPRevert and RevertReason Error(string) -> services/agent/sdk/python/layerx_sdk/revert.py decode_revert_reason/raise_for_receipt_status.
 *
 * Repairs revert.py's late size check: oversized receipt hex was allocated
 * as bytes, and oversized ABI data sliced, before the 64 KiB bound was checked.
 * The boundary still decodes; oversized real Go output is refused by both APIs.
 * The registry is stateless: every result is produced and consumed twice,
 * with fresh escrow reads and different delivery details, without a cache.
 *
 * Outside scope: precompiles/common/precompiles.go:LXPReason preserves an
 * LXP_-prefixed reason with no separator; current expected behaviour is for
 * the Python consumer to refuse this malformed producer output.
 * All producers run in this process; the Go adapter imports the real package.
 * No source extraction, replacement ABI codec, environment endpoint or skip.
 */
#define _POSIX_C_SOURCE 200809L
#include "../codec/lxp_kernel_case.h"
#include "../modules/test_service_helpers.h"
#include "layerx/lx_escrow.h"

#include <inttypes.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

typedef struct dispute_fixture {
    lx_service_agreement agreement;
    lx_escrow_record escrow;
    lxp_authority_resolved buyer;
    lxp_authority_resolved provider;
    lxp_authority_resolved arbiter;
    lxp_authority_resolved outsider;
} dispute_fixture;

static size_t escrow_reads;
static lxp_arena escrow_arena;
static uint8_t escrow_bytes[WORK_ARENA_BYTES];

static lxp_result escrow_ctx(lxp_module_ctx *ctx, lxp_kernel *kernel,
                              bool mutable)
{
    lxp_result status = lxp_arena_init(&escrow_arena, escrow_bytes,
                                       sizeof(escrow_bytes));
    if (status != LXP_OK) return status;
    return lxp_module_ctx_init(ctx, kernel, LXP_MODULE_ESCROW, 100U, 0U,
                               1U, (uint64_t)GAS_LIMIT, &escrow_arena,
                               mutable);
}

static lxp_result read_escrow_link(void *user, const uint8_t escrow_id[32],
                                   lx_service_escrow_link *link)
{
    lxp_module_ctx ctx;
    lx_escrow_record record;
    lxp_result status;
    ++escrow_reads;
    status = escrow_ctx(&ctx, user, false);
    if (status != LXP_OK) return status;
    status = lx_escrow_lookup(&ctx, escrow_id, &record);
    if (status != LXP_OK) return status;
    (void)memcpy(link->holder, record.owner, 32U);
    (void)memcpy(link->beneficiary, record.beneficiary, 32U);
    (void)memcpy(link->asset_id, record.asset_id, 32U);
    (void)memcpy(link->arbiter, record.arbiter, 32U);
    link->state = (uint8_t)record.state;
    return LXP_OK;
}

static lx_service_runtime runtime = {read_escrow_link, &service_kernel};

static int update_escrow(const dispute_fixture *fixture)
{
    lxp_module_ctx ctx;
    LXP_CASE_CHECK(escrow_ctx(&ctx, &service_kernel, true) == LXP_OK);
    LXP_CASE_CHECK(lx_escrow_state_update(&ctx, &fixture->escrow) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static int setup(dispute_fixture *fixture)
{
    lxp_module_ctx ctx;
    lx_service_offer offer;
    escrow_reads = 0U;
    (void)memset(fixture, 0, sizeof(*fixture));
    LXP_CASE_CHECK(lxp_state_store_init(&store_state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&service_kernel, &store_state,
                                     &state_journal, &parameter_set,
                                     0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
                          lx_service_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
                          lx_escrow_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&service_kernel,
                          LXP_MODULE_SERVICE, &runtime) == LXP_OK);
    id32(fixture->provider.principal, 1U);
    id32(fixture->buyer.principal, 2U);
    id32(fixture->arbiter.principal, 3U);
    id32(fixture->outsider.principal, 4U);
    id32(fixture->agreement.agreement_id, 10U);
    id32(fixture->agreement.offer_id, 11U);
    (void)memcpy(fixture->agreement.provider,
                 fixture->provider.principal, 32U);
    (void)memcpy(fixture->agreement.buyer, fixture->buyer.principal, 32U);
    id32(fixture->agreement.terms_hash, 12U);
    id32(fixture->agreement.escrow_id, 41U);
    fixture->agreement.delivery_deadline = 1000U;
    fixture->agreement.acceptance_window_end = 1100U;
    fixture->agreement.dispute_window_end = 1200U;
    fixture->agreement.default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    fixture->agreement.state = LX_SERVICE_AGREEMENT_REJECTED;
    (void)memset(&offer, 0, sizeof(offer));
    (void)memcpy(offer.offer_id, fixture->agreement.offer_id, 32U);
    id32(offer.activity_id, 13U);
    (void)memcpy(offer.offering_agent, fixture->agreement.provider, 32U);
    id32(offer.asset_id, 43U);
    offer.price.lo = 100U;
    (void)memcpy(offer.terms_hash, fixture->agreement.terms_hash, 32U);
    id32(offer.deliverable_specification_hash, 14U);
    offer.delivery_deadline = 1000U;
    offer.acceptance_window = 100U;
    offer.dispute_window = 100U;
    offer.default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    offer.offer_expiry = 900U;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 1U) == 0);
    LXP_CASE_CHECK(lx_service_offer_put(&ctx, &offer) == LXP_OK);
    LXP_CASE_CHECK(lx_service_agreement_put(&ctx,
                                            &fixture->agreement) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    (void)memcpy(fixture->escrow.escrow_id,
                 fixture->agreement.escrow_id, 32U);
    (void)memcpy(fixture->escrow.owner, fixture->agreement.buyer, 32U);
    (void)memcpy(fixture->escrow.beneficiary,
                 fixture->agreement.provider, 32U);
    (void)memcpy(fixture->escrow.arbiter, fixture->arbiter.principal, 32U);
    (void)memcpy(fixture->escrow.asset_id, offer.asset_id, 32U);
    id32(fixture->escrow.escrow_account, 42U);
    fixture->escrow.state = LX_ESCROW_STATE_OPEN;
    fixture->escrow.locked_amount.lo = 100U;
    fixture->escrow.expiry = 2000U;
    fixture->escrow.dispute_window = 1000U;
    (void)memcpy(fixture->escrow.terms_hash,
                 fixture->agreement.terms_hash, 32U);
    (void)memcpy(fixture->escrow.agreement_reference,
                 fixture->agreement.agreement_id, 32U);
    LXP_CASE_CHECK(escrow_ctx(&ctx, &service_kernel, true) == LXP_OK);
    LXP_CASE_CHECK(lx_escrow_state_put(&ctx, &fixture->escrow) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static size_t open_payload(uint8_t *payload, const dispute_fixture *fixture)
{
    payload[0] = 0U;
    payload[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(payload + 2U, 50U);
    (void)memcpy(payload + 34U, fixture->agreement.agreement_id, 32U);
    payload[66] = 2U;
    id32(payload + 67U, 52U);
    id32(payload + 99U, 51U);
    return 131U;
}

static size_t resolve_payload(uint8_t *payload)
{
    payload[0] = 0U;
    payload[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(payload + 2U, 50U);
    be16(payload + 34U, 1U);
    be32(payload + 36U, 5000U);
    (void)memset(payload + 40U, 53, 32U);
    return (size_t)LX_SERVICE_DISPUTE_RESOLVE_PAYLOAD_BYTES;
}

typedef struct proof_delivery {
    lxp_result value;
    const char *code;
    const char *message;
    unsigned round;
} proof_delivery;

/* X is the registry visitor: send the actual dispatched int32 result to Go.
 * This JSON is only an interprocess carrier, never a replacement revert. */
static int X(FILE *stream, const proof_delivery *delivery)
{
    return fprintf(stream,
        "{\"Value\":%" PRId32 ",\"Code\":\"%s\",\"Message\":\"%s\","
        "\"Detail\":\"delivery %u\"}", delivery->value, delivery->code,
        delivery->message, delivery->round) < 0 ? 1 : 0;
}

static const char *const consumer_parts[] = {
    "import base64\n"
    "import json\n"
    "from pathlib import Path\n"
    "import subprocess\n"
    "import sys\n"
    "import tempfile\n"
    "import unittest\n"
    "\n"
    "root = Path.cwd()\n"
    "assert (root / \"precompiles/common/lxp_errors.go\").is_file(), \"missing precompiles/common/lxp_errors"
    ".go\"\n"
    "sys.path.insert(0, str(root / \"services/agent/sdk/python\"))\n"
    "from layerx_sdk.production import PlatformSdkError, SdkErrorCode\n"
    "from layerx_sdk.revert import RevertReasonError, decode_revert_reason, raise_for_receipt_status\n"
    "check = unittest.TestCase()\n"
    "request = json.load(sys.stdin)\n"
    "source = 'package main\\nimport (\\n \"encoding/json\"\\n \"errors\"\\n \"os\"\\n \"strings\"\\n common \"github.co"
    "m/Sidiora-Labs/Paxeer-X-Network/precompiles/common\"\\n)\\ntype Row struct { Value int32; Code, Message"
    ", Detail string }\\ntype Request struct { Rows []Row; Refusal bool }\\ntype Delivery struct { Code str"
    "ing; Data []byte }\\nfunc main() {\\n var input Request\\n if err := json.NewDecoder(os.Stdin).Decode(&"
    "input); err != nil { panic(err) }\\n output := struct { Rows []Delivery; Malformed, Oversized, Bounda"
    "ry []byte }{}\\n for _, row := range input.Rows {\\n  code := common.LXPCodeName(row.Value)\\n  output."
    "Rows = append(output.Rows, Delivery{code, common.LXPRevert(code, row.Detail)})\\n }\\n if input.Refusa"
    "l {\\n  row := input.Rows[0]\\n  code := common.LXPCodeName(row.Value)\\n  output.Malformed = common.LX"
    "PReason(errors.New(code))\\n  output.Oversized = common.LXPRevert(code, strings.Repeat(\"x\", 65536))\\n"
    "  n := 65536 - len(code + \": \" + row.Message + \"; \")\\n  output.Boundary = common.LXPRevert(code, str"
    "ings.Repeat(\"x\", n))\\n }\\n if err := json.NewEncoder(os.Stdout).Encode(output); err != nil { panic(e"
    "rr) }\\n}\\n'\n"
    "with tempfile.TemporaryDirectory(prefix=\"lxp-d6-\", dir=\"/tmp\") as directory:\n"
    "    main = Path(directory) / \"main.go\"\n"
    "    main.write_text(source)\n"
    "    produced = subprocess.ru",
    "n([\"go\", \"run\", str(main)], input=json.dumps(request), text=True,\n"
    "                              stdout=subprocess.PIPE, check=True, timeout=600)\n"
    "output = json.loads(produced.stdout)\n"
    "check.assertEqual(len(output[\"Rows\"]), len(request[\"Rows\"]))\n"
    "previous = None\n"
    "for row, delivery in zip(request[\"Rows\"], output[\"Rows\"], strict=True):\n"
    "    check.assertEqual(delivery[\"Code\"], row[\"Code\"])\n"
    "    data = base64.b64decode(delivery[\"Data\"], validate=True)\n"
    "    decoded = decode_revert_reason(data)\n"
    "    check.assertEqual((decoded.code, decoded.message, decoded.detail, decoded.retryable),\n"
    "                      (row[\"Code\"], row[\"Message\"], row[\"Detail\"], False))\n"
    "    check.assertIsNot(decoded, previous)\n"
    "    previous = decoded\n"
    "    with check.assertRaises(RevertReasonError) as caught:\n"
    "        raise_for_receipt_status({\"status\": \"0x0\", \"returnData\": \"0x\" + data.hex()})\n"
    "    check.assertEqual(str(caught.exception), str(decoded))\n"
    "    check.assertIsNot(caught.exception, decoded)\n"
    "if request[\"Refusal\"]:\n"
    "    row = request[\"Rows\"][0]\n"
    "    boundary = base64.b64decode(output[\"Boundary\"], validate=True)\n"
    "    check.assertEqual(len(boundary), 4 + 64 + 65536)\n"
    "    result = decode_revert_reason(boundary)\n"
    "    check.assertEqual((result.code, result.message, result.retryable), (row[\"Code\"], row[\"Message\"],"
    " False))\n"
    "    check.assertEqual(len(result.detail), 65536 - len(row[\"Code\"] + \": \" + row[\"Message\"] + \"; \"))\n"
    "    with check.assertRaises(RevertReasonError):\n"
    "        raise_for_receipt_status({\"status\": \"0x0\", \"returnData\": \"0x\" + boundary.hex()})\n"
    "    wire = base64.b64decode(output[\"Rows\"][0][\"Data\"], validate=True)\n"
    "    bad_offset = bytearray(wire)\n"
    "    bad_offset[35] = 64\n"
    "    bad_padding = bytearray(wire)\n"
    "    bad_padding[-1] = 1\n"
    "    for bad in (base64.b64decode(output[\"Malformed\"], validate=True),\n"
    "                ba",
    "se64.b64decode(output[\"Oversized\"], validate=True), wire[:-1],\n"
    "                wire + bytes(32), bytes(bad_offset), bytes(bad_padding)):\n"
    "        with check.assertRaises(PlatformSdkError) as caught:\n"
    "            decode_revert_reason(bad)\n"
    "        check.assertIs(caught.exception.code, SdkErrorCode.DECODE_FAILURE)\n"
    "        check.assertEqual(caught.exception.retry, \"never\")\n"
    "        with check.assertRaises(PlatformSdkError) as caught:\n"
    "            raise_for_receipt_status({\"status\": \"0x0\", \"returnData\": \"0x\" + bad.hex()})\n"
    "        check.assertIs(caught.exception.code, SdkErrorCode.DECODE_FAILURE)\n"
    "        check.assertEqual(caught.exception.retry, \"never\")\n"
    "check.assertIsNone(raise_for_receipt_status({\"status\": \"0x1\"}))\n"
    "print(\"d6 consumer observed\", len(request[\"Rows\"]), \"fresh kernel results; refusal =\", request[\"Refu"
    "sal\"])\n",
};

static int consume(const proof_delivery *deliveries, size_t count, bool refusal)
{
    char program[16000];
    size_t used = 0U;
    size_t i;
    int descriptors[2];
    int status;
    int write_failed = 0;
    pid_t child;
    FILE *stream;
    for (i = 0U; i < sizeof(consumer_parts) / sizeof(consumer_parts[0]); ++i) {
        size_t length = strlen(consumer_parts[i]);
        LXP_CASE_CHECK(used + length < sizeof(program));
        (void)memcpy(program + used, consumer_parts[i], length);
        used += length;
    }
    program[used] = '\0';
    LXP_CASE_CHECK(pipe(descriptors) == 0);
    child = fork();
    LXP_CASE_CHECK(child >= 0);
    if (child == 0) {
        (void)close(descriptors[1]);
        if (dup2(descriptors[0], STDIN_FILENO) < 0) _exit(126);
        (void)close(descriptors[0]);
        execlp("python3", "python3", "-B", "-c", program, (char *)NULL);
        perror("missing real Python SDK consumer services/agent/sdk/python/layerx_sdk/revert.py");
        _exit(127);
    }
    (void)close(descriptors[0]);
    stream = fdopen(descriptors[1], "w");
    LXP_CASE_CHECK(stream != NULL);
    if (fprintf(stream, "{\"Refusal\":%s,\"Rows\":[", refusal ? "true" : "false") < 0)
        write_failed = 1;
    for (i = 0U; i < count; ++i) {
        if (i != 0U && fputc(',', stream) == EOF) write_failed = 1;
        if (X(stream, &deliveries[i]) != 0) write_failed = 1;
    }
    if (fputs("]}", stream) == EOF) write_failed = 1;
    if (fclose(stream) != 0) write_failed = 1;
    LXP_CASE_CHECK(waitpid(child, &status, 0) == child);
    LXP_CASE_CHECK(write_failed == 0);
    LXP_CASE_CHECK(WIFEXITED(status) && WEXITSTATUS(status) == 0);
    return 0;
}

static int produce_twice(proof_delivery *deliveries, uint32_t activity_type,
                         const uint8_t *payload, size_t length,
                         const lxp_authority_resolved *authority,
                         lxp_result expected, const char *code,
                         const char *message)
{
    unsigned round;
    for (round = 0U; round < 2U; ++round) {
        lxp_result outcome;
        size_t reads_before = escrow_reads;
        uint8_t before[32];
        uint8_t after[32];
        LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel,
                            LXP_MODULE_SERVICE, before) == LXP_OK);
        LXP_CASE_CHECK(dispatch(activity_type, payload, length, authority,
                                100U, 3U, 60U, &outcome) == LXP_OK);
        LXP_CASE_CHECK(outcome == expected);
        LXP_CASE_CHECK(lxp_result_is_registered(outcome));
        LXP_CASE_CHECK(lxp_result_domain(outcome) == LXP_RESULT_DOMAIN_MODULE);
        LXP_CASE_CHECK(!lxp_result_is_fatal(outcome));
        LXP_CASE_CHECK(escrow_reads == reads_before + 1U);
        LXP_CASE_CHECK(event_buffer.count == 0U);
        LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel,
                            LXP_MODULE_SERVICE, after) == LXP_OK);
        LXP_CASE_CHECK(memcmp(before, after, sizeof(before)) == 0);
        deliveries[round] = (proof_delivery){outcome, code, message, round};
    }
    return 0;
}

static int proof_b_error_code_registry_d6_happy_path(void)
{
    dispute_fixture fixture;
    proof_delivery deliveries[6];
    uint8_t payload[160];
    size_t length;
    lxp_result outcome;
    lxp_module_ctx ctx;
    lx_service_dispute dispute;
    uint8_t dispute_id[32];

    LXP_CASE_CHECK(LXP_ERR_ARBITER_REQUIRED == -737);
    LXP_CASE_CHECK(LXP_ERR_UNAUTHORIZED_ARBITER == -738);
    LXP_CASE_CHECK(LXP_ERR_ESCROW_LINK_MISMATCH == -739);
    LXP_CASE_CHECK(setup(&fixture) == 0);
    length = open_payload(payload, &fixture);
    (void)memset(fixture.escrow.arbiter, 0, 32U);
    LXP_CASE_CHECK(update_escrow(&fixture) == 0);
    LXP_CASE_CHECK(produce_twice(deliveries, LX_SERVICE_DISPUTE_OPEN,
        payload, length, &fixture.buyer, LXP_ERR_ARBITER_REQUIRED,
        "LXP_ARBITER_REQUIRED", "arbiter required") == 0);

    (void)memcpy(fixture.escrow.arbiter, fixture.arbiter.principal, 32U);
    fixture.escrow.owner[31] ^= 1U;
    LXP_CASE_CHECK(update_escrow(&fixture) == 0);
    LXP_CASE_CHECK(produce_twice(deliveries + 2U, LX_SERVICE_DISPUTE_OPEN,
        payload, length, &fixture.buyer, LXP_ERR_ESCROW_LINK_MISMATCH,
        "LXP_ESCROW_LINK_MISMATCH", "escrow link mismatch") == 0);

    fixture.escrow.owner[31] ^= 1U;
    LXP_CASE_CHECK(update_escrow(&fixture) == 0);
    LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
        &fixture.buyer, 100U, 2U, 61U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    length = resolve_payload(payload);
    LXP_CASE_CHECK(produce_twice(deliveries + 4U, LX_SERVICE_DISPUTE_RESOLVE,
        payload, length, &fixture.outsider, LXP_ERR_UNAUTHORIZED_ARBITER,
        "LXP_UNAUTHORIZED_ARBITER", "unauthorized arbiter") == 0);
    LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload, length,
        &fixture.arbiter, 100U, 3U, 62U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    id32(dispute_id, 50U);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 4U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, dispute_id, &dispute) == LXP_OK);
    LXP_CASE_CHECK(dispute.resolved && dispute.provider_basis_points == 5000U);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    LXP_CASE_CHECK(consume(deliveries, 6U, false) == 0);
    return 0;
}

static int proof_b_error_code_registry_d6_refusal(void)
{
    dispute_fixture fixture;
    proof_delivery deliveries[2];
    uint8_t payload[160];
    size_t length;
    lxp_result outcome;
    LXP_CASE_CHECK(setup(&fixture) == 0);
    length = open_payload(payload, &fixture);
    LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
        &fixture.buyer, 100U, 2U, 61U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    length = resolve_payload(payload);
    LXP_CASE_CHECK(produce_twice(deliveries, LX_SERVICE_DISPUTE_RESOLVE,
        payload, length, &fixture.buyer, LXP_ERR_UNAUTHORIZED_ARBITER,
        "LXP_UNAUTHORIZED_ARBITER", "unauthorized arbiter") == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    LXP_CASE_CHECK(consume(deliveries, 2U, true) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"proof_b_error_code_registry_d6_happy_path",
         proof_b_error_code_registry_d6_happy_path},
        {"proof_b_error_code_registry_d6_refusal",
         proof_b_error_code_registry_d6_refusal}
    };
    char *all[] = {argv[0], (char *)cases[0].name, (char *)cases[1].name};
    if (argc == 1)
        return lxp_kernel_case_main(cases, 2U, 3, all);
    return lxp_kernel_case_main(cases, 2U, argc, argv);
}
