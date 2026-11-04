#include "signature-language-fixture.inc"

static int signature_bytes(uint8_t *out, size_t length, const char *hex)
{
    size_t index;
    LANG_CHECK(strlen(hex) == length * 2U);
    for (index = 0U; index < length; ++index) {
        unsigned int value;
        LANG_CHECK(sscanf(hex + index * 2U, "%2x", &value) == 1);
        out[index] = (uint8_t)value;
    }
    return 0;
}

static int signature_calldata(uint8_t input[192], uint8_t algorithm,
    unsigned int variant, size_t *length)
{
    static const char ed_key[] =
        "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a";
    static const char ed_signature[] =
        "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555f"
        "b8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b";
    static const char secp_message[] =
        "bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023";
    static const char secp_key[] =
        "04782c8ed17e3b2a783b5464f33b09652a71c678e05ec51e84e2bcfc663a3de963"
        "af9acb4280b8c7f7c42f4ef9aba6245ec1ec1712fd38a0fa96418d8cd6aa6152";
    static const char secp_signature[] =
        "d035ee1f17fdb0b2681b163e33c359932659990af77dca632012b30b27a057b31"
        "939d9f3b2858bc13e3474cb50e6a82be44faa71940f876c1cba4c3e989202b6";
    size_t key_length = algorithm == 1U ? 32U : 65U;
    size_t message_length = algorithm == 1U ? (variant == 1U ? 1U : 0U) : 32U;
    uint8_t *message = input + 3U;
    uint8_t *key = message + message_length;
    uint8_t *signature = key + key_length;
    input[0] = algorithm;
    input[1] = (uint8_t)key_length;
    input[2] = (uint8_t)message_length;
    if (algorithm == 1U) {
        if (message_length != 0U) message[0] = 1U;
        LANG_CHECK(signature_bytes(key, key_length, ed_key) == 0);
        LANG_CHECK(signature_bytes(signature, 64U, ed_signature) == 0);
    } else {
        LANG_CHECK(signature_bytes(message, message_length, secp_message) == 0);
        LANG_CHECK(signature_bytes(key, key_length, secp_key) == 0);
        LANG_CHECK(signature_bytes(signature, 64U, secp_signature) == 0);
        if (variant == 1U) message[0] ^= 1U;
    }
    if (variant == 2U) memset(key, algorithm == 1U ? 2 : 0, key_length);
    *length = 3U + message_length + key_length + 64U;
    return 0;
}

static int signature_terminal_failure(const lxp_receipt *receipt,
    const uint8_t program[32], int32_t expected, lxp_byte_span *output,
    lxp_byte_span *refusal)
{
    static const char *const wrappers[] = {
        "LXP/programs/terminal-applied-legs/v1",
        "LXP/program-execution-with-transfer-authority/v2",
        "LXP/program-execution-with-occupancy/v1",
        "LXP/programs/execution-authority/v1",
        "LXP/programs/execution-occupancy/v1",
        "LXP/programs/sandbox-settlement/v1"
    };
    lxp_byte_span detail = receipt->program_outcome.terminal_payload;
    language_reader reader;
    lxp_byte_span graph, trace;
    const uint8_t *bytes;
    size_t depth, index;
    LANG_CHECK(receipt->result_code == LXP_ERR_PROGRAM_REFUSED);
    LANG_CHECK(receipt->program_outcome.terminal_kind == LXP_PROGRAM_TERMINAL_FAILURE);
    LANG_CHECK(receipt->effects.count == 0U);
    LANG_CHECK(lxp_ct_is_zero(receipt->program_outcome.transfer_root, 32U));
    for (depth = 0U; depth < 6U; ++depth) {
        bool found = false;
        for (index = 0U; index < sizeof(wrappers) / sizeof(wrappers[0]); ++index) {
            if (language_domain(detail, wrappers[index])) {
                lxp_byte_span inner, attachment;
                reader = (language_reader){detail.bytes, detail.length,
                    strlen(wrappers[index]) + 1U};
                LANG_CHECK(language_blob(&reader, 4U, &inner) == 0);
                LANG_CHECK(language_blob(&reader, 4U, &attachment) == 0);
                if (index == 0U) LANG_CHECK(attachment.length == 0U);
                if (index == 1U || index == 3U || index == 5U)
                    LANG_CHECK(language_take(&reader, 32U, &bytes) == 0);
                LANG_CHECK(reader.offset == reader.length);
                detail = inner;
                found = true;
                break;
            }
        }
        if (!found) break;
    }
    LANG_CHECK(language_domain(detail, "LXP/program-execution/v5"));
    reader = (language_reader){detail.bytes, detail.length,
        sizeof("LXP/program-execution/v5")};
    LANG_CHECK(language_take(&reader, 10U, &bytes) == 0);
    LANG_CHECK(language_take(&reader, 8U, &bytes) == 0 && language_u64(bytes) == 1U);
    LANG_CHECK(language_take(&reader, 5U, &bytes) == 0 && bytes[0] == 1U);
    LANG_CHECK(language_u32(bytes + 1U) == (uint32_t)expected);
    *output = (lxp_byte_span){bytes, 5U};
    LANG_CHECK(language_take(&reader, 60U, &bytes) == 0);
    LANG_CHECK(language_take(&reader, 1U, &bytes) == 0 && bytes[0] <= 1U);
    if (bytes[0] == 1U) LANG_CHECK(language_blob(&reader, 8U, &trace) == 0);
    LANG_CHECK(language_take(&reader, 34U, &bytes) == 0);
    LANG_CHECK(memcmp(bytes, program, 32U) == 0 && bytes[32U] == 0U && bytes[33U] == 4U);
    LANG_CHECK(language_take(&reader, 1U, &bytes) == 0 && bytes[0] == 1U);
    LANG_CHECK(language_blob(&reader, 8U, refusal) == 0);
    LANG_CHECK(refusal->length == 40U && memcmp(refusal->bytes, program, 32U) == 0);
    LANG_CHECK(language_u32(refusal->bytes + 32U) == 255U);
    LANG_CHECK(language_u32(refusal->bytes + 36U) == 0U);
    LANG_CHECK(language_blob(&reader, 8U, &graph) == 0 && reader.offset == reader.length);
    LANG_CHECK(graph.length == receipt->program_outcome.call_graph_payload.length);
    LANG_CHECK(memcmp(graph.bytes, receipt->program_outcome.call_graph_payload.bytes,
        graph.length) == 0);
    return 0;
}

static int test_repair_374_signature_terminal(const char *path, size_t language,
    uint8_t baseline[6][64], size_t baseline_length[6])
{
    static const uint8_t access[] = "LayerX/programs/access-declaration/v1\0";
    static const uint8_t caps[] = {0U, 0U};
    static const char *const names[] = {"c", "rust", "assemblyscript"};
    language_fixture *f = calloc(1U, sizeof(*f));
    size_t index = 0U;
    int result = 1;
    if (f == NULL || language_setup(f) != 0 || language_deploy(f, path) != 0)
        goto finished;
    for (index = 0U; index < 6U; ++index) {
        uint8_t calldata[192];
        size_t length, payload_length;
        uint8_t algorithm = (uint8_t)(index / 3U + 1U);
        unsigned int variant = (unsigned int)(index % 3U);
        int32_t expected = variant == 0U ? 0 : variant == 1U ? -6 : -2;
        lxp_receipt receipt;
        lxp_byte_span output, refusal;
        if (signature_calldata(calldata, algorithm, variant, &length) != 0)
            goto finished;
        payload_length = call_payload_with_data(f->payload, f->program_id,
            caps, sizeof(caps), access, sizeof(access), calldata, length);
        write_u16(f->payload + 32U, 4U);
        write_u32(f->payload + 46U, 4096U);
        if (language_execute(f, LX_PROGRAMS_CALL, payload_length,
                (uint8_t)(0x50U + index), &receipt) != 0)
            goto finished;
        if (expected == 0) {
            if (receipt.result_code != LXP_OK) goto finished;
        } else {
            if (signature_terminal_failure(&receipt, f->program_id, expected,
                    &output, &refusal) != 0 ||
                receipt.program_outcome.cpu_fuel == 0U || lxp_u128_is_zero(receipt.fee_charged))
                goto finished;
            if (language == 0U) {
                memcpy(baseline[index], output.bytes, output.length);
                memcpy(baseline[index] + output.length, refusal.bytes, refusal.length);
                baseline_length[index] = output.length + refusal.length;
            } else if (baseline_length[index] != output.length + refusal.length ||
                memcmp(baseline[index], output.bytes, output.length) != 0 ||
                memcmp(baseline[index] + output.length, refusal.bytes, refusal.length) != 0)
                goto finished;
        }
        printf("PASS test_repair_374_signature_terminal language=%s algorithm=%u variant=%u code=%d\n",
            names[language], (unsigned int)algorithm, variant, (int)expected);
    }
    result = 0;
finished:
    if (result != 0) fprintf(stderr, "signature kernel regression failed: %s case=%zu\n", path, index);
    if (f != NULL && f->state_ready) (void)lxp_state_store_destroy(&f->state);
    free(f);
    return result;
}

static int paid_counter_event(const lxp_receipt *receipt, const language_fixture *f,
    const uint8_t note[8])
{
    static const char domain[] = "LayerX/programs/events/v1";
    static const char topic[] = "layerx.quickstart.noted";
    lxp_byte_span envelope = receipt->program_outcome.event_envelope_payload;
    language_reader reader = {envelope.bytes, envelope.length, sizeof(domain)};
    lxp_byte_span emitted_topic, data;
    const uint8_t *bytes;
    LANG_CHECK(language_domain(envelope, domain));
    LANG_CHECK(language_take(&reader, 4U, &bytes) == 0 && language_u32(bytes) == 1U);
    LANG_CHECK(language_take(&reader, 32U, &bytes) == 0 && memcmp(bytes, f->program_id, 32U) == 0);
    LANG_CHECK(language_take(&reader, 32U, &bytes) == 0 && memcmp(bytes, f->authority.principal, 32U) == 0);
    LANG_CHECK(language_take(&reader, 9U, &bytes) == 0);
    LANG_CHECK(language_blob(&reader, 4U, &emitted_topic) == 0);
    LANG_CHECK(emitted_topic.length == sizeof(topic) - 1U &&
        memcmp(emitted_topic.bytes, topic, sizeof(topic) - 1U) == 0);
    LANG_CHECK(language_blob(&reader, 4U, &data) == 0 && reader.offset == reader.length);
    LANG_CHECK(data.length == 16U && language_u64(data.bytes) == 0U &&
        memcmp(data.bytes + 8U, note, 8U) == 0);
    return 0;
}

static int test_repair_245_paid_counter_kernel(void)
{
    static const uint8_t access[] = "LayerX/programs/access-declaration/v1\0";
    static const uint8_t caps[] = {0U, 2U, 1U, 3U};
    static const uint8_t note[8] = {0U, 0U, 0U, 0U, 0U, 0U, 0U, 42U};
    language_fixture *f = calloc(1U, sizeof(*f));
    lxp_receipt receipt;
    size_t length;
    int result = 1;
    if (f == NULL || language_setup(f) != 0 ||
        language_deploy(f, "programs/sdk/c/examples/paid-counter/build/paid-counter.wasm") != 0)
        goto finished;
    length = call_payload_with_data(f->payload, f->program_id, caps, sizeof(caps),
        access, sizeof(access), note, sizeof(note));
    write_u16(f->payload + 32U, 4U);
    write_u32(f->payload + 46U, 4096U);
    if (language_execute(f, LX_PROGRAMS_CALL, length, 0x60U, &receipt) != 0 ||
        receipt.result_code != LXP_OK || receipt.program_outcome.cpu_fuel == 0U ||
        paid_counter_event(&receipt, f, note) != 0) goto finished;
    puts("PASS test_repair_245_paid_counter_kernel");
    result = 0;
finished:
    if (result != 0) fprintf(stderr, "paid-counter kernel regression failed\n");
    if (f != NULL && f->state_ready) (void)lxp_state_store_destroy(&f->state);
    free(f);
    return result;
}

int main(int argc, char **argv)
{
    uint8_t baseline[6][64] = {{0U}};
    size_t baseline_length[6] = {0U};
    size_t language;
    LANG_CHECK(argc == 4);
    LANG_CHECK(test_repair_245_paid_counter_kernel() == 0);
    for (language = 0U; language < 3U; ++language)
        LANG_CHECK(test_repair_374_signature_terminal(argv[language + 1U],
            language, baseline, baseline_length) == 0);
    return 0;
}
