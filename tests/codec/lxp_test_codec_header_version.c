#include "layerx/lxp_codec.h"
#include "layerx/lxp_protocol.h"
#include "lxp_kernel_case.h"

static int codec_header_version_roundtrip(void)
{
    const struct {
        uint16_t version;
        uint8_t wire[4];
    } cases[] = {
        { LXP_PROTOCOL_VERSION_LEGACY, { 0x00U, 0x01U, 0x12U, 0x34U } },
        { LXP_PROTOCOL_VERSION_OCCUPANCY, { 0x00U, 0x02U, 0x12U, 0x34U } },
        { LXP_PROTOCOL_VERSION_STATE_COMMITMENT,
          { 0x00U, 0x03U, 0x12U, 0x34U } }
    };
    size_t i;

    for (i = 0U; i < sizeof(cases) / sizeof(cases[0]); ++i) {
        uint8_t storage[4];
        lxp_arena arena;
        lxp_codec_writer writer;
        lxp_codec_reader reader;
        uint16_t version = 0U;

        LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(storage)) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_writer_init(&writer, &arena,
                                             sizeof(storage)) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_write_struct_header_version(
                           &writer, 0x1234U, cases[i].version) == LXP_OK);
        LXP_CASE_CHECK(writer.length == sizeof(cases[i].wire));
        LXP_CASE_CHECK(memcmp(writer.bytes, cases[i].wire,
                              sizeof(cases[i].wire)) == 0);
        LXP_CASE_CHECK(lxp_codec_reader_init(&reader, writer.bytes,
                                             writer.length) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_read_struct_header_version(
                           &reader, 0x1234U, &version) == LXP_OK);
        LXP_CASE_CHECK(version == cases[i].version);
        LXP_CASE_CHECK(reader.offset == sizeof(cases[i].wire));
        LXP_CASE_CHECK(lxp_codec_finish(&reader) == LXP_OK);
    }
    return 0;
}

static int codec_header_version_refusals(void)
{
    const uint16_t unsupported[] = { 0U, 4U, UINT16_MAX };
    const uint8_t valid[] = { 0x00U, 0x01U, 0x12U, 0x34U };
    const struct {
        uint8_t wire[4];
        uint16_t expected_tag;
    } refused[] = {
        { { 0x00U, 0x01U, 0x12U, 0x34U }, 0x1235U },
        { { 0x00U, 0x01U, 0x12U, 0x34U }, 0U },
        { { 0x00U, 0x01U, 0x00U, 0x00U }, 0U },
        { { 0x00U, 0x00U, 0x12U, 0x34U }, 0x1234U },
        { { 0x00U, 0x04U, 0x12U, 0x34U }, 0x1234U },
        { { 0xffU, 0xffU, 0x12U, 0x34U }, 0x1234U }
    };
    uint8_t storage[4];
    lxp_arena arena;
    lxp_codec_writer writer;
    lxp_codec_reader reader;
    size_t i;

    LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_writer_init(&writer, &arena,
                                         sizeof(storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_write_struct_header_version(
                       &writer, 0U, LXP_PROTOCOL_VERSION_LEGACY) ==
                   LXP_ERR_INVALID_TAG);
    LXP_CASE_CHECK(writer.length == 0U);
    for (i = 0U; i < sizeof(unsupported) / sizeof(unsupported[0]); ++i) {
        LXP_CASE_CHECK(lxp_codec_write_struct_header_version(
                           &writer, 0x1234U, unsupported[i]) ==
                       LXP_ERR_INVALID_TAG);
        LXP_CASE_CHECK(writer.length == 0U);
    }

    for (i = 0U; i < sizeof(refused) / sizeof(refused[0]); ++i) {
        uint16_t version = 0xbeefU;

        LXP_CASE_CHECK(lxp_codec_reader_init(&reader, refused[i].wire,
                                             sizeof(refused[i].wire)) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_read_struct_header_version(
                           &reader, refused[i].expected_tag, &version) ==
                       LXP_ERR_VERSION_UNSUPPORTED);
        LXP_CASE_CHECK(version == 0xbeefU);
        LXP_CASE_CHECK(reader.offset == sizeof(refused[i].wire));
    }
    LXP_CASE_CHECK(lxp_codec_reader_init(&reader, valid, sizeof(valid)) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_read_struct_header_version(
                       &reader, 0x1234U, NULL) == LXP_ERR_VERSION_UNSUPPORTED);
    LXP_CASE_CHECK(reader.offset == sizeof(valid));

    for (i = 0U; i < sizeof(valid); ++i) {
        uint16_t version = 0xbeefU;

        LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(storage)) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_writer_init(&writer, &arena, i) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_write_struct_header_version(
                           &writer, 0x1234U, LXP_PROTOCOL_VERSION_LEGACY) ==
                       LXP_ERR_LENGTH_LIMIT);
        LXP_CASE_CHECK(writer.length == (i < 2U ? 0U : 2U));
        LXP_CASE_CHECK(lxp_codec_reader_init(&reader, valid, i) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_read_struct_header_version(
                           &reader, 0x1234U, &version) == LXP_ERR_TRUNCATED);
        LXP_CASE_CHECK(version == 0xbeefU);
        LXP_CASE_CHECK(reader.offset == (i < 2U ? 0U : 2U));
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "codec_header_version_roundtrip", codec_header_version_roundtrip },
        { "codec_header_version_refusals", codec_header_version_refusals }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
