"""Regenerate the codec's pinned Unicode 15.0.0 NFC properties (no build dependency)."""

from pathlib import Path
import unicodedata as ucd


def main():
    if ucd.unidata_version != "15.0.0":
        raise SystemExit("Regeneration requires Python's Unicode 15.0.0 database")
    ranges = []
    pairs = []
    decompositions = []
    for cp in range(0x110000):
        char = chr(cp)
        ccc = ucd.combining(char)
        excluded = int(ucd.normalize("NFC", char) != char)
        if ccc or excluded:
            if (
                ranges
                and ranges[-1][1] + 1 == cp
                and ranges[-1][2:] == (ccc, excluded)
            ):
                ranges[-1] = (ranges[-1][0], cp, ccc, excluded)
            else:
                ranges.append((cp, cp, ccc, excluded))
        decomposition = ucd.decomposition(char)
        if decomposition and not decomposition.startswith("<"):
            components = tuple(int(value, 16) for value in decomposition.split())
            if (
                len(components) == 2
                and ucd.normalize("NFC", "".join(map(chr, components))) == char
            ):
                pairs.append(components)
                decompositions.append(
                    (cp, components[0], ucd.combining(chr(components[1])))
                )
    lines = [
        "/* BEGIN PINNED NFC TABLES",
        " * Unicode 15.0.0: canonical combining classes, NFC exclusions and",
        " * canonical composition pairs from the Unicode Character Database.",
        " * Regenerate with tests/codec/generate_nfc_tables.py using UCD 15.0.0.",
        " * https://www.unicode.org/Public/15.0.0/ucd/ ; https://www.unicode.org/license.txt",
        " */",
        "static const nfc_property_range nfc_properties[] = {",
    ]
    lines.extend(
        f"    {{0x{lo:06x}U, 0x{hi:06x}U, {ccc}U, {excluded}U}},"
        for lo, hi, ccc, excluded in ranges
    )
    lines.extend(["};", "", "static const nfc_composition_pair nfc_compositions[] = {"])
    lines.extend(
        f"    {{0x{first:06x}U, 0x{second:06x}U}},"
        for first, second in sorted(pairs)
    )
    lines.extend(
        ["};", "", "static const nfc_starter_decomposition nfc_decompositions[] = {"]
    )
    lines.extend(
        f"    {{0x{cp:06x}U, 0x{first:06x}U, {ccc}U}},"
        for cp, first, ccc in decompositions
        if ccc
    )
    lines.extend(["};", "/* END PINNED NFC TABLES */"])
    source = Path(__file__).resolve().parents[2] / "src/codec/lxp_codec.c"
    text = source.read_text()
    start = text.index("/* BEGIN PINNED NFC TABLES")
    end_marker = "/* END PINNED NFC TABLES */"
    end = text.index(end_marker, start) + len(end_marker)
    source.write_text(text[:start] + "\n".join(lines) + text[end:])


if __name__ == "__main__":
    main()
