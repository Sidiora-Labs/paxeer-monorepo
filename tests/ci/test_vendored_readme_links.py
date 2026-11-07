"""Keep links in the repaired vendored documentation usable locally or upstream."""

from collections import Counter
from pathlib import Path
import re
import tomllib
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]
WASM_TOOLS = ROOT / "tools/wasm-tools"
CRATES = (
    "wasm-compose",
    "wasm-encoder",
    "wasm-metadata",
    "wasm-mutate",
    "wasmprinter",
    "wast",
    "wat",
)
CRATE_READMES = tuple(WASM_TOOLS / "crates" / crate / "README.md" for crate in CRATES)
REFLECT = ROOT / "core/deps/cosmwasm/contracts/reflect/README.md"
EVM = ROOT / "core/deps/geth/cmd/evm/README.md"
RELEASE_PROCESS = (
    ROOT / "core/deps/geth/crypto/secp256k1/libsecp256k1/doc/release-process.md"
)
CHANGELOG = ROOT / "services/explorer/backend/CHANGELOG.md"
DOCUMENTS = (*CRATE_READMES, REFLECT, EVM, RELEASE_PROCESS, CHANGELOG)
INLINE_LINK = re.compile(r"\]\(\s*(?:<([^>\n]+)>|([^\s)]+))")
REFERENCE_LINK = re.compile(
    r"^[ \t]{0,3}\[[^\]\n]+\]:[ \t]*(?:<([^>\n]+)>|(\S+))", re.MULTILINE
)
HTML_LINK = re.compile(r"\b(?:href|src)\s*=\s*[\"']([^\"']+)[\"']", re.IGNORECASE)


def _link_targets(document):
    text = document.read_text(encoding="utf-8")
    return [
        next(value for value in match.groups() if value is not None)
        for pattern in (INLINE_LINK, REFERENCE_LINK, HTML_LINK)
        for match in pattern.finditer(text)
    ]


def _upstream_url(tree, path):
    vendor = tomllib.loads((ROOT / tree / "VENDOR.kvx").read_text(encoding="utf-8"))[
        "vendor"
    ]
    assert re.fullmatch(r"[0-9a-f]{40}", vendor["sha"]), vendor
    return f"{vendor['url']}/blob/{vendor['sha']}/{path}"


def test_wasm_tools_crate_license_links_resolve():
    license_file = WASM_TOOLS / "LICENSE-Apache-2.0_WITH_LLVM-exception"
    assert license_file.is_file()
    for document in CRATE_READMES:
        targets = re.findall(
            r"\[LICENSE\]\(([^)]+)\)", document.read_text(encoding="utf-8")
        )
        assert targets == ["../../LICENSE-Apache-2.0_WITH_LLVM-exception"], document
        destination = (document.parent / targets[0]).resolve()
        assert destination == license_file, document
        assert destination.is_file(), document


def test_vendored_readme_relative_links_resolve_or_are_absolute():
    broken = []
    for document in DOCUMENTS:
        targets = _link_targets(document)
        assert targets, f"No links checked in {document.relative_to(ROOT)}"
        for target in targets:
            url = urlsplit(target)
            if url.scheme or url.netloc:
                assert url.scheme in {"http", "https"} and url.netloc, (
                    document,
                    target,
                )
                continue
            path = unquote(url.path)
            if path.startswith("/"):
                destination = ROOT / path.lstrip("/")
            else:
                destination = document.parent / path if path else document
            if not destination.exists():
                broken.append(f"{document.relative_to(ROOT)}: {target}")
    assert not broken, "Unresolved relative links:\n" + "\n".join(broken)

    for name in ("Developing.md", "Publishing.md", "Importing.md"):
        expected = _upstream_url("core/deps/cosmwasm", f"contracts/reflect/{name}")
        assert _link_targets(REFLECT).count(expected) == 1
    assert _link_targets(EVM).count(
        _upstream_url("core/deps/geth", "cmd/evm/t8n_test.go")
    ) == 1
    assert _link_targets(RELEASE_PROCESS).count(
        _upstream_url(
            "core/deps/geth", "crypto/secp256k1/libsecp256k1/tools/check-abi.sh"
        )
    ) == 1

    time_format_links = [
        target for target in _link_targets(CHANGELOG) if "#time-format" in target
    ]
    docs = "https://docs.blockscout.com/setup/env-variables"
    assert Counter(time_format_links) == {
        f"{docs}/backend-env-variables#time-format": 23,
        f"{docs}#time-format": 6,
        f"{docs}/backend-envs-chain-specific#time-format": 1,
    }
