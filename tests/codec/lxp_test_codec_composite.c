#include "layerx/lxp_codec.h"
#include "layerx/lxp_protocol.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define NFC_CHECK(condition) do { \
    if (!(condition)) { \
        fprintf(stderr, "%s:%d: check failed: %s\n", __FILE__, __LINE__, #condition); \
        return 1; \
    } \
} while (0)

static int codec_nfc_case(const char *text, size_t length, lxp_result expected)
{
    uint8_t storage[512];
    uint8_t before[sizeof(storage)];
    uint8_t transport[sizeof(storage)];
    lxp_arena arena;
    lxp_codec_writer writer;
    lxp_codec_reader reader;
    lxp_byte_span span;
    (void)memset(storage, 0xa5, sizeof(storage));
    NFC_CHECK(lxp_arena_init(&arena, storage, sizeof(storage)) == LXP_OK);
    NFC_CHECK(lxp_codec_writer_init(&writer, &arena, sizeof(storage)) == LXP_OK);
    NFC_CHECK(lxp_codec_write_u8(&writer, 0x7eU) == LXP_OK);
    (void)memcpy(before, storage, sizeof(storage));
    NFC_CHECK(lxp_codec_write_text(&writer, text, length, 256U) == expected);
    if (expected != LXP_OK) {
        NFC_CHECK(writer.length == 1U);
        NFC_CHECK(memcmp(storage, before, sizeof(storage)) == 0);
        return 0;
    }
    NFC_CHECK(writer.length == 5U + length);
    NFC_CHECK(writer.bytes[1] == 0U && writer.bytes[2] == 0U &&
        writer.bytes[3] == (uint8_t)(length >> 8U) &&
        writer.bytes[4] == (uint8_t)length);
    (void)memcpy(transport, writer.bytes, writer.length);
    NFC_CHECK(lxp_codec_reader_init(&reader, transport + 1U,
        writer.length - 1U) == LXP_OK);
    NFC_CHECK(lxp_codec_read_bytes(&reader, &span, 256U) == LXP_OK);
    NFC_CHECK(span.length == length &&
        (length == 0U || memcmp(span.bytes, text, length) == 0));
    NFC_CHECK(lxp_codec_finish(&reader) == LXP_OK);
    writer.length = 0U;
    NFC_CHECK(lxp_codec_write_text(&writer, (const char *)span.bytes,
        span.length, 256U) == LXP_OK);
    NFC_CHECK(writer.length == 4U + length &&
        memcmp(writer.bytes, transport + 1U, writer.length) == 0);
    return 0;
}

static int codec_nfc_boundary_vectors(void)
{
    static const struct {
        const char *text;
        size_t length;
        lxp_result expected;
    } vectors[] = {
#define NFC_VECTOR(text, result) {text, sizeof(text) - 1U, result}
        NFC_VECTOR("", LXP_OK),
        NFC_VECTOR("\0", LXP_OK),
        NFC_VECTOR("\xcc\x81", LXP_OK), /* Standalone U+0301. */
        NFC_VECTOR("q\xcc\x81", LXP_OK), /* No composition exists. */
        NFC_VECTOR("\xcc\x96\xcc\x81", LXP_OK), /* Ordered leading marks. */
        NFC_VECTOR("\xcc\x81\xcc\x96", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("e\xcc\x81", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xc3\xa9", LXP_OK),
        NFC_VECTOR("\xe1\xb8\x8a\xcc\xa3", LXP_ERR_NON_CANONICAL), /* Hidden dot above. */
        NFC_VECTOR("\xe1\xb8\x8c\xcc\x87", LXP_OK), /* Reordered, recomposed form. */
        NFC_VECTOR("\xc3\x85\xcc\x96", LXP_OK), /* Hidden ring need not change NFC. */
        NFC_VECTOR("A\xcc\x85\xcc\x81", LXP_OK), /* Equal class blocks. */
        NFC_VECTOR("A\xcc\x96\xcc\x81", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xcd\x80", LXP_ERR_NON_CANONICAL), /* Excluded U+0340. */
        NFC_VECTOR("\xe2\x84\xab", LXP_ERR_NON_CANONICAL), /* Angstrom singleton. */
        NFC_VECTOR("\xe0\xa5\x98", LXP_ERR_NON_CANONICAL), /* U+0958 exclusion. */
        NFC_VECTOR("\xe0\xa4\x95\xe0\xa4\xbc", LXP_OK), /* Excluded pair stays split. */
        NFC_VECTOR("\xd6\xb1\xd6\xb0", LXP_ERR_NON_CANONICAL), /* Order outside 0300. */
        NFC_VECTOR("\xe1\x84\x80\xe1\x85\xa1", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xe1\x84\x92\xe1\x85\xb5", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xea\xb0\x80\xe1\x86\xa8", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xed\x9e\x88\xe1\x87\x82", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xea\xb0\x80", LXP_OK), /* First Hangul LV. */
        NFC_VECTOR("\xed\x9e\xa3", LXP_OK), /* Last Hangul LVT. */
        NFC_VECTOR("\xe1\x83\xbf\xe1\x85\xa1", LXP_OK), /* Before L range. */
        NFC_VECTOR("\xe1\x84\x93\xe1\x85\xa1", LXP_OK), /* After L range. */
        NFC_VECTOR("\xe1\x84\x80\xe1\x85\xa0", LXP_OK), /* Before V range. */
        NFC_VECTOR("\xe1\x84\x80\xe1\x85\xb6", LXP_OK), /* After V range. */
        NFC_VECTOR("\xea\xb0\x80\xe1\x86\xa7", LXP_OK), /* Before T range. */
        NFC_VECTOR("\xea\xb0\x80\xe1\x87\x83", LXP_OK), /* After T range. */
        NFC_VECTOR("\xea\xb0\x81\xe1\x86\xa8", LXP_OK), /* LVT cannot compose. */
        NFC_VECTOR("\xe1\x84\x80\xcc\x81\xe1\x85\xa1", LXP_OK), /* Blocked Hangul. */
        NFC_VECTOR("\xf0\x9d\x85\x9e", LXP_ERR_NON_CANONICAL), /* U+1D15E exclusion. */
        NFC_VECTOR("\xf4\x8f\xbf\xbf", LXP_OK), /* Last scalar, unchanged by NFC. */
        NFC_VECTOR("\xc0\xaf", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xe0\x80\xaf", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xf0\x80\x80\xaf", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xed\xa0\x80", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xf4\x90\x80\x80", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xf5\x80\x80\x80", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\x80", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xe1\x84", LXP_ERR_NON_CANONICAL),
        NFC_VECTOR("\xe1\x41\x80", LXP_ERR_NON_CANONICAL),
#undef NFC_VECTOR
    };
    for (size_t i = 0U; i < sizeof(vectors) / sizeof(vectors[0]); ++i) {
        if (codec_nfc_case(vectors[i].text, vectors[i].length,
            vectors[i].expected) != 0) {
            fprintf(stderr, "NFC boundary vector %zu\n", i);
            return 1;
        }
    }
    NFC_CHECK(codec_nfc_case(NULL, 0U, LXP_OK) == 0);
    NFC_CHECK(codec_nfc_case(NULL, 1U, LXP_ERR_NON_CANONICAL) == 0);
    return 0;
}

static size_t nfc_encode_scalar(uint32_t cp, char *bytes)
{
    if (cp < 0x80U) {
        bytes[0] = (char)cp;
        return 1U;
    }
    if (cp < 0x800U) {
        bytes[0] = (char)(0xc0U | (cp >> 6U));
        bytes[1] = (char)(0x80U | (cp & 0x3fU));
        return 2U;
    }
    if (cp < 0x10000U) {
        bytes[0] = (char)(0xe0U | (cp >> 12U));
        bytes[1] = (char)(0x80U | ((cp >> 6U) & 0x3fU));
        bytes[2] = (char)(0x80U | (cp & 0x3fU));
        return 3U;
    }
    bytes[0] = (char)(0xf0U | (cp >> 18U));
    bytes[1] = (char)(0x80U | ((cp >> 12U) & 0x3fU));
    bytes[2] = (char)(0x80U | ((cp >> 6U) & 0x3fU));
    bytes[3] = (char)(0x80U | (cp & 0x3fU));
    return 4U;
}

static int codec_nfc_unicode_15_conformance(void)
{
    FILE *file = fopen("tests/vectors/codec/NormalizationTest-15.0.0.txt", "rb");
    char line[8192];
    size_t cases = 0U;
    NFC_CHECK(file != NULL);
    while (fgets(line, sizeof(line), file) != NULL) {
        char fields[5][256];
        size_t lengths[5] = {0};
        char *cursor = line;
        if (line[0] == '#' || line[0] == '@' || line[0] == '\n' || line[0] == '\r')
            continue;
        for (size_t field = 0U; field < 5U; ++field) {
            while (*cursor != ';') {
                char *end;
                unsigned long cp;
                while (*cursor == ' ' || *cursor == '\t') ++cursor;
                if (*cursor == ';') break;
                cp = strtoul(cursor, &end, 16);
                NFC_CHECK(end != cursor && cp <= 0x10ffffUL &&
                    !(cp >= 0xd800UL && cp <= 0xdfffUL));
                NFC_CHECK(lengths[field] + 4U <= sizeof(fields[field]));
                lengths[field] += nfc_encode_scalar((uint32_t)cp,
                    fields[field] + lengths[field]);
                cursor = end;
            }
            ++cursor;
        }
        for (size_t field = 0U; field < 5U; ++field) {
            size_t canonical = field < 3U ? 1U : 3U;
            int equal = lengths[field] == lengths[canonical] &&
                memcmp(fields[field], fields[canonical], lengths[field]) == 0;
            if (codec_nfc_case(fields[field], lengths[field],
                equal ? LXP_OK : LXP_ERR_NON_CANONICAL) != 0) {
                fprintf(stderr, "Unicode 15.0.0 case %zu column %zu\n", cases, field);
                (void)fclose(file);
                return 1;
            }
        }
        ++cases;
    }
    NFC_CHECK(ferror(file) == 0);
    NFC_CHECK(fclose(file) == 0);
    NFC_CHECK(cases == 19074U);
    return 0;
}

int main(void)
{
    uint8_t storage[256];
    lxp_arena arena;
    lxp_codec_writer writer;
    lxp_codec_reader reader;
    lxp_byte_span span;
    const uint8_t a[] = {0x01U};
    const uint8_t b[] = {0x01U, 0x00U};
    const uint8_t c[] = {0x02U};
    lxp_byte_span sorted[] = {{a, sizeof(a)}, {b, sizeof(b)}, {c, sizeof(c)}};
    lxp_byte_span duplicate[] = {{a, sizeof(a)}, {a, sizeof(a)}};
    const char canonical[] = "LayerX";
    const char decomposed[] = "e\xcc\x81";
    uint8_t tag = 0U;

    if (lxp_arena_init(&arena, storage, sizeof(storage)) != LXP_OK ||
        lxp_codec_writer_init(&writer, &arena, sizeof(storage)) != LXP_OK)
        return 1;
    if (lxp_codec_write_bytes(&writer, a, sizeof(a), LXP_MAX_PAYLOAD_BYTES) != LXP_OK ||
        lxp_codec_write_text(&writer, canonical, strlen(canonical),
                             LXP_MAX_DID_LENGTH) != LXP_OK ||
        lxp_codec_write_seq(&writer, 3U, LXP_MAX_EFFECTS) != LXP_OK ||
        lxp_codec_write_tag(&writer, 2U, 3U) != LXP_OK) return 1;
    if (lxp_codec_write_text(&writer, decomposed, sizeof(decomposed) - 1U,
                             LXP_MAX_DID_LENGTH) != LXP_ERR_NON_CANONICAL)
        return 1;
    if (lxp_codec_seq_check_sorted(sorted, 3U) != LXP_OK ||
        lxp_codec_seq_check_sorted(duplicate, 2U) != LXP_ERR_UNSORTED_SEQUENCE)
        return 1;
    if (lxp_codec_reader_init(&reader, writer.bytes, writer.length) != LXP_OK ||
        lxp_codec_read_bytes(&reader, &span, LXP_MAX_PAYLOAD_BYTES) != LXP_OK ||
        span.length != sizeof(a) || memcmp(span.bytes, a, sizeof(a)) != 0)
        return 1;
    if (lxp_codec_read_bytes(&reader, &span, LXP_MAX_DID_LENGTH) != LXP_OK ||
        span.length != strlen(canonical)) return 1;
    {
        uint32_t count = 0U;
        if (lxp_codec_read_u32(&reader, &count) != LXP_OK || count != 3U)
            return 1;
    }
    if (lxp_codec_read_tag(&reader, 3U, &tag) != LXP_OK || tag != 2U)
        return 1;
    if (lxp_codec_write_tag(&writer, 4U, 3U) != LXP_ERR_INVALID_TAG) return 1;
    if (codec_nfc_boundary_vectors() != 0) return 1;
    return codec_nfc_unicode_15_conformance();
}
