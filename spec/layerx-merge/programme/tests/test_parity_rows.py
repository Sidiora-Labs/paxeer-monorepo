import ast
import re
from hashlib import sha256
from pathlib import Path, PurePosixPath

from programme.kvx import load
from programme.parity import render_doc

ROOT = Path(__file__).resolve().parents[4]
FEATURE = ROOT / "spec/layerx-merge"
DATA = FEATURE / "programme/data/parity.kvx"
INVENTORY_REFERENCE = FEATURE / "programme/data/parity-inventory-original.md"


def _inventory_rows() -> dict[tuple[int, str], list[str]]:
    rows = {}
    section = 0
    for line in INVENTORY_REFERENCE.read_text().splitlines():
        heading = re.match(r"## (\d+) ", line)
        if heading:
            section = int(heading[1])
        if section not in (2, 3, 5) or not line.startswith("| "):
            continue
        cells = re.split(r"(?<!\\)\|", line)
        paths = re.findall(r"`([^`]+)`", cells[2])
        if paths:
            title = re.sub(r" \(\d+\)$", "", cells[1].strip())
            key = (section, title)
            assert key not in rows, key
            rows[key] = paths
    return rows


def _declares_test(selector: str) -> bool:
    path, name = selector.split("::")
    source = ROOT / path
    if not source.is_file():
        return False
    text = source.read_text(encoding="utf-8")
    escaped = re.escape(name)
    if source.suffix == ".go":
        return bool(
            name.startswith("Test")
            and re.search(rf"(?m)^func\s+{escaped}\s*\([^)]*\*testing\.T\b", text)
        )
    if source.suffix == ".rs":
        return bool(
            re.search(
                rf"#\[(?:tokio::)?test(?:\([^\]]*\))?\]\s*"
                rf"(?:async\s+)?fn\s+{escaped}\s*\(",
                text,
            )
        )
    if source.suffix == ".c":
        return name == "main" and bool(re.search(r"(?m)^int\s+main\s*\(", text))
    if source.suffix == ".py":
        return any(
            isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))
            and node.name == name
            and name.startswith("test_")
            for node in ast.parse(text).body
        )
    return False


def test_inventory_reference_is_the_frozen_original() -> None:
    original = INVENTORY_REFERENCE.read_bytes()
    assert sha256(original).hexdigest() == (
        "671ca477247cd9d2d9fd7a3e8a2d33d4d107739527db22f926cb06c81c8d4dc8"
    )
    assert original != (FEATURE / "docs/parity-inventory.md").read_bytes()
    assert b"Generated from `programme/data/parity.kvx`" not in original
    assert {section for section, _ in _inventory_rows()} == {2, 3, 5}


def test_kernel_replay_corpus_row_selects_the_qualification_replay() -> None:
    sections = load(DATA)
    replay = sections["construct.existing-kernel-replay-corpus"]
    assert replay["selectors"] == ["tests/test_qual_replay.c::main"]
    assert re.search(
        r"(?m)^int\s+main\s*\(",
        (ROOT / "tests/test_qual_replay.c").read_text(encoding="utf-8"),
    )
    assert "tests/test_replay_crossarch.c::main" in replay["dropped"]

    web = sections["construct.existing-web-search-sidecar-and-402lxp-payment"]
    web_tests = "services/interop/crates/x-websearch/tests"
    assert web["selectors"] == [
        f"{web_tests}/submit.rs::the_kernel_network_id_is_read_from_the_xweb_precompile",
        f"{web_tests}/submit.rs::fulfil_calldata_and_the_signed_transaction_match_an_independent_encoder",
        f"{web_tests}/payment.rs::a_pending_draw_recovers_from_the_sequencer_receipt_and_releases_once",
        f"{web_tests}/attest.rs::the_watcher_follows_requested_logs_to_the_confirmation_depth",
        f"{web_tests}/attest.rs::scanned_requests_stay_journalled_across_restart_until_retired",
        f"{web_tests}/api.rs::the_api_payload_codec_matches_every_pinned_vector_and_refusal",
    ]
    kernel_path = "services/interop/crates/x-websearch/tests/kernel.rs"
    retired_web_tests = [
        "a_request_record_decodes_only_in_its_canonical_form",
        "a_program_attestation_matches_the_shared_origin_two_vector",
        "the_observation_activity_equals_the_kernel_adapter_fixture",
        "the_watcher_follows_request_records_through_the_recorded_gateway",
        "the_watcher_reads_only_request_topics",
        "the_program_exchange_route_serves_records_by_program_and_request",
        "two_programs_with_the_same_request_id_are_both_answered_through_the_exchange",
        "the_relay_answers_a_program_request_and_posts_its_observation",
    ]
    assert {f"{kernel_path}::{name}" for name in retired_web_tests}.issubset(
        web["dropped"]
    )

    mirror = sections["construct.existing-mirror"]
    mirror_tests = "services/interop/crates/layerx-mirror/tests"
    assert mirror["selectors"] == [
        f"{mirror_tests}/publish.rs::confirms_and_retrieves_the_same_archive_on_both_test_networks",
        f"{mirror_tests}/publish.rs::publishes_pure_archives_without_custody_semantics",
        f"{mirror_tests}/sequencer_vectors.rs::every_shared_receipted_case_verifies_in_the_mirror",
        f"{mirror_tests}/sequencer_vectors.rs::every_shared_refusal_case_is_refused_by_the_mirror",
        f"{mirror_tests}/committed_freshness.rs::freshness_reports_committed_lag_in_blocks",
    ]
    assert (
        "services/interop/crates/layerx-mirror/tests/checkpoint_vectors.rs"
        "::mirror_checkpoint_identity_and_freshness_follow_the_shared_vectors"
    ) in mirror["dropped"]
    for row in (replay, web, mirror):
        for selector in row["selectors"]:
            assert _declares_test(selector), selector

    assert (FEATURE / "docs/parity-inventory.md").read_bytes() == render_doc(
        sections
    ).encode("utf-8")

    for name, row in sections.items():
        if not name.startswith("construct."):
            continue
        for selector in row["selectors"]:
            path = selector.split("::")[0]
            assert path != "tests/test_replay_crossarch.c", (name, selector)
            assert (ROOT / path).is_file(), (name, selector)


def test_every_row_has_exact_selectors() -> None:
    sections = load(DATA)
    spec = load(FEATURE / "spec.kvx")
    planned = {
        selector
        for section, task in spec.items()
        if section.startswith("task.")
        for selector in task["tests"]
    }
    constructs = {
        name: row for name, row in sections.items() if name.startswith("construct.")
    }
    dropped = {
        name: row for name, row in sections.items() if name.startswith("dropped.")
    }
    assert constructs
    assert set(sections) == {"meta"} | constructs.keys() | dropped.keys()

    inventory = _inventory_rows()
    represented = set()
    active_selectors = set()
    dropped_selectors = set()
    for name, row in (constructs | dropped).items():
        key = (row["source_section"], row["title"])
        assert key in inventory, (name, key)
        assert key not in represented, key
        represented.add(key)
        assert row["source_paths"] == inventory[key], name
        assert type(row["owner_domain"]) is int, name
        assert f"feature.{row['owner_domain']}" in spec, name
        assert type(row["wave"]) is int and 0 <= row["wave"] <= 7, name

        if name in constructs:
            selectors = row["selectors"]
            assert isinstance(selectors, list) and selectors, name
            assert len(selectors) == len(set(selectors)), name
            for selector in selectors:
                assert re.fullmatch(
                    r"(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+::[A-Za-z_][A-Za-z0-9_]*",
                    selector,
                ), (name, selector)
                path = PurePosixPath(selector.split("::")[0])
                assert not {".", ".."}.intersection(path.parts), (name, selector)
                assert path.suffix in {
                    ".go",
                    ".rs",
                    ".c",
                    ".py",
                    ".ts",
                    ".tsx",
                    ".sol",
                    ".exs",
                }, (name, selector)
                assert selector in planned or _declares_test(selector), (
                    name,
                    selector,
                    "test neither exists nor is planned by a phase-2 task",
                )
                active_selectors.add(selector)
            for task_id in row["planned_by"]:
                task = spec[f"task.{task_id}"]
                assert set(task["tests"]).issubset(
                    set(selectors) | set(row.get("dropped", []))
                ), (name, task_id)
                owner = spec[f"feature.{row['owner_domain']}"]["name"]
                assert task["owner_role"] == f"fm-{owner}", name
        else:
            assert "selectors" not in row, name
            assert isinstance(row["reason"], str) and row["reason"].strip(), name
            assert isinstance(row["dropped"], list) and row["dropped"], name

        for selector in row.get("dropped", []):
            assert (
                selector not in planned
                or not (ROOT / selector.split("::")[0]).is_file()
                or ("::" in selector and not _declares_test(selector))
            ), (name, selector)
            if "::" in selector:
                assert not _declares_test(selector), (name, selector)
            dropped_selectors.add(selector)

    assert represented == set(inventory)
    assert active_selectors.isdisjoint(dropped_selectors)


def test_cutover_goldens_are_listed() -> None:
    sections = load(DATA)
    expected = {title for section, title in _inventory_rows() if section == 5}
    assert expected == {"Balance conservation", "Bond return", "Receipt-order equality"}
    goldens = {
        row["title"]: row
        for name, row in sections.items()
        if name.startswith("construct.") and row["source_section"] == 5
    }
    assert set(goldens) == expected
    required = {
        "Balance conservation": (
            "modules/layerxcustody/keeper/release_test.go"
            "::TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority"
        ),
        "Bond return": (
            "modules/layerxanchor/keeper/retire_merge_test.go"
            "::TestRetireAtMergeReturnsBondsAndEmits"
        ),
        "Receipt-order equality": (
            "node/sequencer/receipt_equality_test.go::TestReceiptEqualsCommittedReceipt"
        ),
    }
    for title, selector in required.items():
        assert goldens[title]["wave"] == 6, title
        assert selector in goldens[title]["selectors"], title
