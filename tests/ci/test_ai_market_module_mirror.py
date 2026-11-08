import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
CRATE = ROOT / "programs/crates/layerx-programs-ai-market"
GATEWAY = ROOT / "services/platform/hosted/gateway/tests/repair_22_33_1439.rs"
RUST_TOKEN = re.compile(
    r'//[^\n]*|/\*[\s\S]*?\*/|r(?P<hashes>\#*)"[\s\S]*?"(?P=hashes)'
    r'|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])\'|[A-Za-z_]\w*|[^\s]'
)


def file_modules(source, directory):
    tokens = [
        match.group()
        for match in RUST_TOKEN.finditer(source)
        if not match.group().startswith(("//", "/*"))
    ]
    files = set()

    def group_end(start, opening, closing):
        depth = 1
        end = start + 1
        while depth:
            assert end < len(tokens), f"Unclosed {opening} in {directory}"
            if tokens[end] == opening:
                depth += 1
            elif tokens[end] == closing:
                depth -= 1
            end += 1
        return end

    def modules(start, end, base):
        path = None
        i = start
        while i < end:
            if tokens[i : i + 2] == ["#", "["]:
                attribute_end = group_end(i + 1, "[", "]")
                attribute = tokens[i + 2 : attribute_end - 1]
                if attribute[:2] == ["path", "="]:
                    assert len(attribute) == 3, f"Invalid path attribute: {attribute}"
                    path = json.loads(attribute[2])
                i = attribute_end
            elif tokens[i] == "mod":
                name, delimiter = tokens[i + 1 : i + 3]
                if delimiter == ";":
                    files.add((base / (path or f"{name}.rs")).resolve())
                    i += 3
                else:
                    assert delimiter == "{", f"Invalid module declaration: {name}"
                    module_end = group_end(i + 2, "{", "}")
                    modules(i + 3, module_end - 1, base / (path or name))
                    i = module_end
                path = None
            elif tokens[i] in ("{", "(", "["):
                opening = tokens[i]
                i = group_end(i, opening, {"{": "}", "(": ")", "[": "]"}[opening])
                path = None
            else:
                if tokens[i] == ";":
                    path = None
                i += 1

    modules(0, len(tokens), directory)
    return files


def test_gateway_mirror_matches_crate_modules():
    library = CRATE / "src/lib.rs"
    declared = file_modules(library.read_text(encoding="utf-8"), library.parent)
    included = file_modules(GATEWAY.read_text(encoding="utf-8"), GATEWAY.parent)
    mirrored = {path for path in included if path.is_relative_to(CRATE)}
    assert declared, f"No file modules parsed from {library}"

    missing_from_gateway = sorted(
        str(path.relative_to(CRATE)) for path in declared - mirrored
    )
    missing_from_crate = sorted(
        str(path.relative_to(CRATE)) for path in mirrored - declared
    )
    assert declared == mirrored, (
        f"Modules missing from gateway mirror: {missing_from_gateway}; "
        f"modules missing from ai-market crate declarations: {missing_from_crate}"
    )
