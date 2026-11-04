"""Offline regression proof for coverage-rust-classification, using the real producer."""

import ast
import importlib.util
from pathlib import Path
import re
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
EXPLORER = ROOT / "scripts/explorer"
FIXTURES = EXPLORER / "fixtures/test-ratio"
SHELL = (EXPLORER / "test-ratio.sh").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_772.kvx")
SPEC = importlib.util.spec_from_file_location(
    "explorer_rust_test_ranges", EXPLORER / "rust-test-ranges.py"
)
RUST = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUST)


def fixture(name):
    return (FIXTURES / name).read_text(encoding="utf-8")


def shell_function(name):
    match = re.search(r"^" + re.escape(name) + r"\(\) \{\n(.*?)^\}", SHELL, re.M | re.S)
    assert match is not None, name
    return match.group(1)


def test_production_change_cannot_reuse_unchanged_inline_tests():
    before = fixture("base.rs")
    after = fixture("production_only.rs")
    kind, production, tests = RUST.classify_changes(before, after)
    assert kind == "source"
    assert "amount + amount" in production
    assert "fn quote_amount" in production
    assert "assert_eq!" not in production
    assert [token.text for token in RUST.rust_tokens(tests)] == [
        token.text for token in RUST.rust_tokens(RUST.split_rust(before)[1])
    ]
    assert tests[tests.index("#[cfg(test)]"):] == before[before.index("#[cfg(test)]"):]
    assert "quote_amount(4)" in tests


def test_mixed_changes_keep_production_and_test_items_separate():
    kind, production, tests = RUST.classify_changes(fixture("base.rs"), fixture("mixed.rs"))
    assert kind == "mixed"
    assert "fn quote_amount" in production and "amount * 3" in production
    assert "quotes_triple_amount" not in production
    assert "assert_eq!(quote_amount(4), 12)" in tests
    assert "fn quote_amount" not in tests and "amount * 3" not in tests
    assert production.count("\n") == tests.count("\n") == fixture("mixed.rs").count("\n")


def test_inline_test_changes_do_not_add_a_production_file():
    before = fixture("base.rs")
    kind, production, tests = RUST.classify_changes(before, fixture("tests_only.rs"))
    assert kind == "test"
    assert production == RUST.split_rust(before)[0]
    assert "assert_eq!(quote_amount(5), 10)" in tests
    assert RUST.classify_changes(before, before)[0] == "neither"
    # Moving unchanged inline items by adding whitespace does not invent a test change.
    assert RUST.classify_changes(before, before.replace("#[cfg(test)]", "\n#[cfg(test)]"))[0] == "neither"


def test_new_mixed_and_test_only_modules_have_correct_roles():
    assert RUST.classify_changes("", fixture("mixed.rs"))[0] == "mixed"
    kind, production, tests = RUST.classify_changes("", fixture("test_only.rs"))
    assert kind == "test"
    assert not production.strip()
    assert "fn arithmetic" in tests
    assert RUST.classify_changes("", "pub fn production() {}\n")[0] == "source"
    assert RUST.classify_changes(fixture("base.rs"), "fn quote_amount(amount: u64) -> u64 { amount * 3 }\n")[0] == "source"
    assert RUST.classify_changes("", "// test-only fixture\n" + fixture("test_only.rs"))[0] == "test"


def test_unrelated_inline_tests_cannot_claim_own_file_by_its_stem():
    kind, production, tests = RUST.classify_changes(fixture("base.rs"), fixture("unrelated.rs"))
    assert kind == "mixed"
    # Read the identifier producer's actual pattern, including private Rust items.
    tokens = shell_function("build_tokens").split("*.rs)", 1)[1]
    command = re.search(r"grep -oE ('[^']+') \"\$source_body\"", tokens)
    assert command is not None
    pattern = shlex.split(command.group(1))[0].replace("[[:space:]]", r"\s")
    names = {match.group().split()[-1] for match in re.finditer(pattern, production, re.M)}
    assert names == {"quote_amount"}
    assert all(re.search(r"\b" + re.escape(name) + r"\b", tests) is None for name in names)
    assert "fn helper" not in production
    references = shell_function("references")
    guard = references.index('[ "$reference_test" != "$reference_source" ] &&')
    same_stem = references.index('[ "$reference_test_stem" = "$reference_source_stem" ]')
    assert guard < same_stem
    assert 'grep -qE -- "$reference_pattern" "$reference_body"' in references
    assert 'grep -qF -- "$reference_value" "$reference_body"' in references


def test_cfg_items_keep_production_scopes_and_extract_actual_tests():
    kind, production, tests = RUST.classify_changes("", fixture("cfg_items.rs"))
    assert kind == "mixed"
    for name in ("available_in_production", "production_only", "nested_production"):
        assert "fn " + name in production
        assert "fn " + name not in tests
    for name in ("nested_check", "standalone_test", "inner_attribute_test"):
        assert "fn " + name not in production
        assert "fn " + name in tests
    assert "mod nested_tests" in tests and "mod nested_tests" not in production
    assert "mod only_tests" in tests and "mod only_tests" not in production
    _, production, tests = RUST.classify_changes("", fixture("cfg_fields.rs"))
    assert "pub value: u64" in production and "fn read_counter" in production
    assert "test_seed:" not in production
    assert "test_seed: u64" in tests and "pub value: u64" not in tests
    assert "fn read_counter" not in tests


def test_literals_nested_comments_and_attribute_groups_do_not_hide_production():
    source = fixture("lexing.rs")
    kind, production, tests = RUST.classify_changes("", source)
    assert kind == "mixed"
    for name in ("marker_text", "after_comments", "after_tests"):
        assert "fn " + name in production
        assert "fn " + name not in tests
    assert 'r###"\n#[cfg(test)]\nmod imaginary' in production
    assert "nested comment with }" in production
    assert "mod actual_tests" in tests and "mod actual_tests" not in production
    assert "#[allow(dead_code)]" in tests and "#[allow(dead_code)]" not in production
    assert "fn marker_is_a_literal" in tests
    changed = source.replace("    8\n", "    9\n")
    assert RUST.classify_changes(source, changed)[0] == "source"


@pytest.mark.parametrize(
    "attribute,is_test",
    [
        ("#[cfg(test)]", True),
        ("# [ cfg ( test ) ]", True),
        ("#[cfg(all(test, feature = \"extra\"))]", True),
        ("#[cfg(not(not(test)))]", True),
        ("#[cfg(any(test, feature = \"extra\"))]", False),
        ("#[cfg(not(test))]", False),
        ("#[cfg_attr(test, allow(dead_code))]", False),
        ("#[cfg(all())]", False),
        ("#[tokio::test]", True),
    ],
)
def test_only_predicates_that_require_test_exclude_an_item(attribute, is_test):
    source = attribute + "\nfn candidate() {}\npub fn following() {}\n"
    production, tests = RUST.split_rust(source)
    assert ("fn candidate" in tests) is is_test
    assert ("fn candidate" in production) is not is_test
    assert "fn following" in production and "fn following" not in tests


@pytest.mark.parametrize(
    "name,kind",
    [
        ("production_only.rs", "source"),
        ("mixed.rs", "mixed"),
        ("tests_only.rs", "test"),
        ("base.rs", "neither"),
    ],
)
def test_real_cli_reads_blobs_and_transports_separate_projection_files(tmp_path, capsys, name, kind):
    output = tmp_path / "rust" / "services/explorer/module with spaces.rs"
    result = RUST.main([str(FIXTURES / "base.rs"), str(FIXTURES / name), str(output)])
    assert result == 0
    assert capsys.readouterr().out == kind + "\n"
    assert {path.name for path in output.iterdir()} == {"source", "test"}
    expected_source, expected_test = RUST.split_rust(fixture(name))
    assert (output / "source").read_text(encoding="utf-8") == expected_source
    assert (output / "test").read_text(encoding="utf-8") == expected_test
    assert "fn quote_amount" in expected_source
    assert "fn quote_amount" not in expected_test


@pytest.mark.parametrize("source", ["/* unclosed", 'r###"unclosed', '"unclosed', "#[cfg(test)] mod tests {", "#[cfg(test)]"])
def test_malformed_items_fail_closed_without_classification_or_artifacts(tmp_path, capsys, source):
    head = tmp_path / "head.rs"
    head.write_text(source, encoding="utf-8")
    output = tmp_path / "out"
    with pytest.raises(SystemExit) as error:
        RUST.main([str(FIXTURES / "base.rs"), str(head), str(output)])
    assert error.value.code == 2
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "rust-test-ranges:" in captured.err
    assert not output.exists()


def test_shell_wires_blob_producer_lists_and_guarded_projection_consumers():
    classify = shell_function("classify")
    rust_branch = classify.split("*.rs)", 1)[1]
    assert 'tests/* | */tests/*)' in rust_branch
    assert rust_branch.index("printf 'test\\n'") < rust_branch.index("python3")
    assert 'git show "$base_rev:$classify_path"' in rust_branch
    assert 'file_at_head "$classify_path" >"$work_dir/rust-head"' in rust_branch
    command = re.search(r'python3 .*?\|\| return \$\?', rust_branch, re.S)
    assert command is not None
    argv = shlex.split(command.group().replace("\\\n", " "))
    assert argv == [
        "python3", "$rust_classifier", "$work_dir/rust-base", "$work_dir/rust-head",
        "$work_dir/rust/$classify_path", "||", "return", "$?",
    ]
    assert 'rust_classifier=$script_dir/rust-test-ranges.py' in SHELL
    assert 'changed_kind=$(classify "$changed_path") || die "cannot classify $changed_path"' in SHELL
    assert re.search(
        r'mixed\)\s*printf .*?"\$source_list"\s*printf .*?"\$test_list"\s*;;', SHELL, re.S
    )
    assert 'if [ -f "$work_dir/rust/$build_path/source" ]; then\n        cat "$work_dir/rust/$build_path/source" >"$source_body"\n    fi' in shell_function("build_tokens")
    assert 'if [ -f "$work_dir/rust/$test_path/test" ]; then\n        cat "$work_dir/rust/$test_path/test" >"$test_body"\n    fi' in SHELL
    assert 'if references "$test_path" "$test_body" "$source_path" "$work_dir/tokens.$source_index"; then' in SHELL
    assert 'if ! grep -qxF -- "$source_path" "$referenced_list"; then' in SHELL
    assert 'if [ "$counted_tests" -lt "$source_count" ]; then' in SHELL
    assert 'if [ "$failed" -ne 0 ]; then' in SHELL
    assert 'grep -q \'#\\[cfg(test)\\]\'' not in classify
    assert "python3" not in shell_function("references")


def test_repair_record_maps_finding_and_states_gate_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["coverage-rust-classification"]
    original = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.174.kvx").read_text(encoding="utf-8"))["finding"]["coverage-rust-classification"]
    for key in ("file", "line", "class", "severity", "observed", "expected", "proposal"):
        assert finding[key] == original[key], key
    assert finding["audit"] == "22.30.174"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    repair = record["repair"]
    assert repair["task"] == "22.33.772"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["build_cmd"] == "bash -n scripts/explorer/test-ratio.sh"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_772.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["wiring_requests"] == []
