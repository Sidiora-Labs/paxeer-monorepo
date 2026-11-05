import re
from pathlib import Path


def test_search_location_clears_the_payer_did():
    config = (Path(__file__).resolve().parents[1] / "nginx.conf").read_text()
    config = re.sub(r"#[^\n]*", "", config)
    location = re.search(
        r"location\s+~\s+\^/\(search\|fetch\|content/\)\s*\{", config
    )
    assert location is not None, "Search location must exist"

    depth = 1
    end = location.end()
    while end < len(config) and depth:
        depth += (config[end] == "{") - (config[end] == "}")
        end += 1
    assert depth == 0, "Search location must have a closing brace"

    search_block = config[location.end() : end - 1]
    payer_headers = re.findall(
        r"\bproxy_set_header\s+LAYERX-PAYER-DID\s+([^;]+);",
        search_block,
        re.IGNORECASE,
    )
    assert [value.strip() for value in payer_headers] == ['""']
