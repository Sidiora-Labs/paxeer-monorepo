#include "layerx/program.h"

LXP_PROGRAM_EXPORT(LXP_PROGRAM_CALL_RESERVE_EXPORT)
int32_t layerx_reserve(int32_t length)
{
    return lxp_program_reserve_call_input_versioned(length);
}

LXP_PROGRAM_EXPORT(LXP_PROGRAM_ENTRYPOINT)
int64_t layerx_main(int64_t selector)
{
    (void)selector;
    return 0;
}

LXP_PROGRAM_EXPORT(LXP_PROGRAM_CALL_ENTRY_EXPORT)
int32_t layerx_call(int32_t pointer, int32_t input_length)
{
    const uint8_t *input;
    size_t length;
    lxp_program_status status = lxp_program_call_input_versioned(
        pointer, input_length, &input, &length);
    if (status != LXP_PROGRAM_OK) return lxp_program_status_abi(status);
    if (length < 3U || length != 3U + input[1] + input[2] + 64U)
        return LXP_PROGRAM_ERR_INVALID;
    status = lxp_program_signature_verify((lxp_program_signature_algorithm)input[0],
        input + 3U, input[2], input + 3U + input[2], input[1],
        input + 3U + input[2] + input[1]);
    return lxp_program_status_abi(status);
}
