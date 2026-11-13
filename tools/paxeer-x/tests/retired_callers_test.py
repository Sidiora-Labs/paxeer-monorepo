#!/usr/bin/env python3
import argparse
import contextlib
from fnmatch import fnmatchcase
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
CATALOGUE = ROOT / "tools/paxeer-x/retired_callers.json"
CATALOGUE_PATH = "tools/paxeer-x/retired_callers.json"
CHECKER_PATH = "tools/paxeer-x/tests/retired_callers_test.py"
DOCUMENTATION_SUFFIXES = {".kvx", ".md"}
KEPT_CODES = {"LXP_MODULE_DISABLED", "LXP_ERR_MODULE_DISABLED"}
COUNT = 0


class Invalid(ValueError):
    pass


def require(value, message):
    if not value:
        raise Invalid(message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate catalogue key: " + key)
        result[key] = value
    return result


def relative_path(value):
    require(
        isinstance(value, str) and value and "\x00" not in value,
        "path must be a nonempty string",
    )
    path = PurePosixPath(value)
    require(
        not path.is_absolute()
        and ".." not in path.parts
        and ".git" not in path.parts
        and str(path) != ".",
        "path must name a repository-relative file or tree",
    )
    return str(path)


def string_list(value, name, empty=False):
    require(isinstance(value, list) and (empty or value), name + " must be a list")
    require(
        all(
            isinstance(item, str)
            and item.strip()
            and "\n" not in item
            and "\r" not in item
            and "\x00" not in item
            for item in value
        ),
        name + " contains an invalid string",
    )
    require(len(value) == len(set(value)), name + " contains duplicates")


def validate_catalogue(document):
    require(isinstance(document, dict), "catalogue must be an object")
    require(
        set(document)
        <= {"components", "allowed_references", "documentation_paths", "schema"},
        "unknown catalogue field",
    )
    components = document.get("components")
    require(isinstance(components, dict) and components, "components are missing")
    for name, entry in components.items():
        require(
            isinstance(name, str) and re.fullmatch(r"[a-z][a-z0-9-]*", name),
            "invalid component ID",
        )
        require(
            isinstance(entry, dict)
            and {"symbols", "deleted_trees", "replacement"} <= set(entry),
            "incomplete component: " + name,
        )
        require(
            set(entry) <= {"symbols", "deleted_trees", "retiring_trees", "replacement", "req", "title"},
            "unknown component field: " + name,
        )
        string_list(entry["symbols"], name + " symbols", empty=name == "router-joins")
        string_list(entry["deleted_trees"], name + " deleted trees", empty=True)
        for path in entry["deleted_trees"]:
            require(relative_path(path) == path, "noncanonical deleted tree: " + path)
        retiring = entry.get("retiring_trees", {})
        require(isinstance(retiring, dict), "retiring trees must map paths to tasks: " + name)
        for path, task in retiring.items():
            require(relative_path(path) == path, "noncanonical retiring tree: " + path)
            require(path not in entry["deleted_trees"], "tree is both deleted and retiring: " + path)
            require(isinstance(task, str) and re.fullmatch(r"[1-9][0-9]*(?:\.[1-9][0-9]*)+", task),
                    "invalid retiring task: " + path)
        require(
            isinstance(entry["replacement"], str) and entry["replacement"].strip(),
            "replacement is missing: " + name,
        )
    allowances = document.get("allowed_references", [])
    require(isinstance(allowances, list), "allowed references must be a list")
    for entry in allowances:
        require(
            isinstance(entry, dict)
            and "path" in entry
            and set(entry) <= {"path", "symbols", "lines", "citation"}
            and ("symbols" in entry or "lines" in entry),
            "invalid allowed reference",
        )
        require(
            "citation" not in entry
            or (
                isinstance(entry["citation"], str)
                and entry["citation"].strip()
                and "\n" not in entry["citation"]
            ),
            "allowed reference citation is invalid: " + str(entry["path"]),
        )
        require(
            relative_path(entry["path"]) == entry["path"], "noncanonical allowed path"
        )
        for key in ("symbols", "lines"):
            if key in entry:
                string_list(entry[key], "allowed " + key)
    documentation = document.get("documentation_paths", [])
    require(isinstance(documentation, list), "documentation paths must be a list")
    deleted_trees = [
        tree for entry in components.values() for tree in entry["deleted_trees"]
    ]
    seen = set()
    for entry in documentation:
        require(
            isinstance(entry, dict)
            and set(entry) in ({"path", "reason"}, {"pattern", "reason"}),
            "invalid documentation path",
        )
        key = "pattern" if "pattern" in entry else "path"
        path = entry[key]
        require(relative_path(path) == path, "noncanonical documentation path: " + path)
        if key == "pattern":
            require(
                any(char in path for char in "*?")
                and not any(char in path for char in "\n\r[]\\"),
                "invalid documentation pattern: " + path,
            )
        require((key, path) not in seen, "duplicate documentation path: " + path)
        seen.add((key, path))
        require(
            isinstance(entry["reason"], str)
            and entry["reason"].strip()
            and "\n" not in entry["reason"],
            "documentation reason is missing: " + path,
        )
        prefix = (
            path.split("*", 1)[0].split("?", 1)[0].rsplit("/", 1)[0]
            if key == "pattern" and "/" in path
            else path
        )
        require(
            not any(below(prefix, tree) or below(tree, prefix) for tree in deleted_trees),
            "documentation path overlaps deleted tree: " + path,
        )
        require(
            not any(
                documentation_matches(candidate, entry)
                or (key == "path" and below(candidate, path))
                for candidate in (CATALOGUE_PATH, CHECKER_PATH)
            ),
            "documentation path covers the checker: " + path,
        )
    return document


def load_catalogue(path):
    return validate_catalogue(
        json.loads(path.read_text(), object_pairs_hook=unique_object)
    )


def below(path, tree):
    return path == tree or path.startswith(tree + "/")


def documentation_matches(path, entry):
    if "pattern" in entry:
        pattern = entry["pattern"]
        return fnmatchcase(path if "/" in pattern else PurePosixPath(path).name, pattern)
    return path == entry["path"] or (
        PurePosixPath(path).suffix in DOCUMENTATION_SUFFIXES
        and below(path, entry["path"])
    )


def is_test_file(path):
    parts = PurePosixPath(path).parts
    if any(part in {"test", "tests", "testdata", "__tests__"} for part in parts[:-1]):
        return True
    name = parts[-1]
    return (
        name == "conftest.py"
        or PurePosixPath(name).stem in {"test", "tests"}
        or name.startswith("test_")
        or re.search(r"(?:_tests?|[.]test|[.]spec)[.][^.]+$", name) is not None
    )


def symbol_pattern(symbol):
    left = r"(?<![A-Za-z0-9])" if symbol[0].isalnum() or symbol[0] == "_" else ""
    right = r"s?(?![A-Za-z0-9])" if symbol[-1].isalnum() else ""
    return re.compile(left + re.escape(symbol) + right)


def allowed(path, text, symbol, allowances):
    if symbol in KEPT_CODES:
        return True
    for entry in allowances:
        if (
            below(path, entry["path"]) and symbol in entry.get("symbols", [])
        ) or (
            path == entry["path"]
            and " ".join(text.split())
            in {" ".join(line.split()) for line in entry.get("lines", [])}
        ):
            return True
    return False


def match_line(path, number, text, patterns, allowances):
    return [
        (path, number, component, symbol)
        for component, symbol, pattern in patterns
        if symbol in text
        and pattern.search(text)
        and not allowed(path, text, symbol, allowances)
    ]


def git(root, arguments, data=None):
    result = subprocess.run(
        ["git", "-C", str(root), *arguments],
        input=data,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
        env={**os.environ, "LC_ALL": "C"} if arguments[0] == "grep" else None,
    )
    require(
        (
            result.returncode in {0, 1}
            if arguments[0] == "grep"
            else result.returncode == 0
        ),
        "git " + arguments[0] + " failed with exit " + str(result.returncode),
    )
    return result


def tracked_paths(root):
    data = git(root, ["ls-files", "-z", "--cached"]).stdout
    return sorted(set(os.fsdecode(path) for path in data.split(b"\x00") if path))


def path_filters(root, paths, tracked):
    result = []
    for value in paths:
        path = Path(value)
        if path.is_absolute():
            try:
                value = str(path.relative_to(root))
            except ValueError as error:
                raise Invalid("path filter is outside the repository") from error
        if value in {".", "./"}:
            result.append("")
            continue
        value = relative_path(value)
        require(
            any(below(candidate, value) for candidate in tracked),
            "path filter has no tracked files: " + value,
        )
        result.append(value)
    return result


def scan(root, catalogue, component_ids, paths, deleted=False):
    components = catalogue["components"]
    selected = sorted(set(component_ids)) if component_ids else sorted(components)
    require(all(name in components for name in selected), "unknown component")
    tracked = tracked_paths(root)
    filters = path_filters(root, paths, tracked)
    documentation = catalogue.get("documentation_paths", [])
    for entry in documentation:
        tree = entry.get("path", entry.get("pattern"))
        require(
            any(
                documentation_matches(candidate, entry)
                if "pattern" in entry
                else below(candidate, tree)
                for candidate in tracked
            ),
            "documentation path has no tracked files: " + tree,
        )
    deleted_trees = {
        tree for entry in components.values() for tree in entry["deleted_trees"]
    }
    findings = []
    if deleted:
        for name in selected:
            for tree, task in components[name].get("retiring_trees", {}).items():
                if filters and not any(
                    not path or below(tree, path) or below(path, tree)
                    for path in filters
                ):
                    continue
                if os.path.lexists(root / tree):
                    findings.append((tree, 1, name, "tree retirement pending task " + task))
            for tree in components[name]["deleted_trees"]:
                if filters and not any(
                    not path or below(tree, path) or below(path, tree)
                    for path in filters
                ):
                    continue
                if os.path.lexists(root / tree):
                    findings.append((tree, 1, name, "deleted tree still exists"))
    files = {
        path
        for path in tracked
        if (not filters or any(not scope or below(path, scope) for scope in filters))
        and path != CATALOGUE_PATH
        and not is_test_file(path)
        and not any(documentation_matches(path, entry) for entry in documentation)
        and not any(below(path, tree) for tree in deleted_trees)
    }
    patterns = [
        (name, symbol, symbol_pattern(symbol))
        for name in selected
        for symbol in components[name]["symbols"]
    ]
    if not files:
        return sorted(set(findings))
    literals = sorted({symbol for _, symbol, _ in patterns})
    pathspecs = [":(top,literal)" + (path or ".") for path in filters]
    retiring_trees = {
        tree for entry in components.values() for tree in entry.get("retiring_trees", {})
    }
    pathspecs.extend(
        ":(top,exclude,literal)" + path
        for path in sorted({CATALOGUE_PATH, "core/deps"} | deleted_trees | retiring_trees)
    )
    pathspecs.extend(
        ":(top,exclude,glob)**/" + directory + "/**"
        for directory in ("test", "tests", "testdata", "__tests__")
    )
    pathspecs.extend(
        ":(top,exclude,glob)" + tree + "/**/*" + suffix
        for tree in ("spec/layerx-merge", "docs")
        for suffix in sorted(DOCUMENTATION_SUFFIXES)
    )
    for entry in documentation:
        if "pattern" in entry:
            pattern = entry["pattern"]
            pathspecs.append(
                ":(top,exclude,glob)"
                + (pattern if "/" in pattern else "**/" + pattern)
            )
            continue
        tree = entry["path"]
        if tree in tracked:
            pathspecs.append(":(top,exclude,literal)" + tree)
        escaped_tree = re.sub(r"([\\*?\[\]])", r"\\\1", tree)
        pathspecs.extend(
            ":(top,exclude,glob)" + escaped_tree + "/**/*" + suffix
            for suffix in sorted(DOCUMENTATION_SUFFIXES)
        )
    with tempfile.NamedTemporaryFile(prefix="retired-callers-patterns-") as patterns_file:
        patterns_file.write(("\n".join(literals) + "\n").encode())
        patterns_file.flush()
        result = git(
            root,
            [
                "grep",
                "--no-ext-grep",
                "--no-textconv",
                "--no-color",
                "--full-name",
                "-I",
                "-n",
                "-z",
                "-F",
                "-f",
                patterns_file.name,
                "--",
                *pathspecs,
            ],
        )
    output = result.stdout
    offset = 0
    while offset < len(output):
        path_end = output.find(b"\x00", offset)
        line_end = output.find(b"\x00", path_end + 1)
        text_end = output.find(b"\n", line_end + 1)
        require(
            path_end >= offset and line_end > path_end and text_end > line_end,
            "malformed git grep output",
        )
        path = os.fsdecode(output[offset:path_end])
        number = int(output[path_end + 1 : line_end])
        text = output[line_end + 1 : text_end].decode(
            "utf-8", errors="surrogateescape"
        )
        if path in files:
            findings.extend(
                match_line(
                    path,
                    number,
                    text,
                    patterns,
                    catalogue.get("allowed_references", []),
                )
            )
        offset = text_end + 1
    return sorted(set(findings))


def check_equal(actual, expected, name):
    global COUNT
    require(actual == expected, "self-test failed: " + name)
    COUNT += 1
    print("ok retired-callers-case " + str(COUNT) + " " + name, flush=True)


def write_tracked(root, files):
    for path, text in files.items():
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)
    git(root, ["add", "--", *files])


def documentation_files(catalogue, text):
    files = {}
    for entry in catalogue.get("documentation_paths", []):
        if "pattern" in entry:
            pattern = entry["pattern"]
            path = (
                pattern[:-3] + "/retirements.md"
                if pattern.endswith("/**")
                else pattern.replace("*", "record").replace("?", "x")
            )
        else:
            path = entry["path"]
            if not PurePosixPath(path).suffix:
                path += "/retirements.md"
        files[path] = text
    return files


def refusal(function, *arguments):
    try:
        function(*arguments)
    except Invalid as error:
        return str(error)
    raise Invalid("self-test failed: refusal expected")


def self_test(catalogue):
    settlement_document = json.loads((ROOT / "contracts/config/checkpoint-settlement.json").read_text())
    check_equal(settlement_document["schema"], "layerx/checkpoint-settlement/1", "retained settlement schema")
    check_equal(settlement_document["chain_id"], 125, "retained offline export chain")
    components = catalogue["components"]
    spec = (ROOT / "spec/layerx-merge/spec.kvx").read_text()
    required = set()
    for line in spec.splitlines():
        if "retired_callers_test.py" in line:
            required.update(
                re.findall(
                    r"--component ([a-z][a-z0-9-]*)(?= --| with --| and\b|[\"']|$)",
                    line,
                )
            )
    missing = sorted(required - set(components))
    check_equal(
        missing,
        [],
        "complete component catalogue"
        + (": missing " + ", ".join(missing) if missing else ""),
    )
    for retiring, message in (
        ([], "retiring trees must map paths to tasks: lni"),
        ({"../lni": "9.24.7"}, "path must name a repository-relative file or tree"),
        ({"src/./lni": "9.24.7"}, "noncanonical retiring tree: src/./lni"),
        ({"src/lni": "done"}, "invalid retiring task: src/lni"),
        ({"src/lni": True}, "invalid retiring task: src/lni"),
        ({"cmd/layerxd": "9.24.7"}, "tree is both deleted and retiring: cmd/layerxd"),
    ):
        document = {
            **catalogue,
            "components": {
                **components,
                "lni": {**components["lni"], "retiring_trees": retiring},
            },
        }
        check_equal(
            refusal(validate_catalogue, document), message,
            "retiring tree contract exact refusal " + message,
        )
    check_equal(
        {entry.get("req") for entry in components.values()},
        {str(11000 + number) for number in range(1, 50) if number not in {6, 7}},
        "every retirement requirement represented",
    )
    check_equal(
        [
            entry.get("path", entry.get("pattern"))
            for entry in catalogue["documentation_paths"]
        ],
        [
            "contracts/*.md",
            "spec/layerx-merge/spec.kvx",
            "spec/layerx-merge/qualification.kvx",
            "spec/layerx-merge/docs",
            "spec/layerx-merge/programme/**",
            "spec/release-v6-12-0/**",
            "spec/monorepo-layout/**",
            "repair_*.kvx",
            "services/platform/hosted/authority/src/repair_22_31_207/**",
            "services/platform/ramps/toolkit/src/repair_22_31_211/**",
            "tools/hyperfide/**",
            "core/deps/geth/signer/fourbyte/**",
            "programs/vendor/**",
            "apps/launchpad/web/public/charting_library/**",
            "services/platform/examples/marketplace/repair-22-31-349.kvx",
            "tools/bringup/repair-*.kvx",
            "api/README.md",
            "modules/README.md",
            "GOTCHA.kvx",
            "CHANGELOG.md",
            "CODEMAP.md",
        ],
        "reviewed documentation paths",
    )
    check_equal(
        components["router-joins"]["symbols"],
        [],
        "replacement keeps router method names",
    )
    check_equal(
        refusal(
            validate_catalogue,
            {
                **catalogue,
                "components": {
                    **components,
                    "verify": {**components["verify"], "symbols": []},
                },
            },
        ),
        "verify symbols must be a list",
        "retired verifier symbols remain required",
    )
    check_equal(
        "tests/daemon" in components["finality-authority"]["deleted_trees"],
        False,
        "recorded Python daemon cases remain live",
    )
    check_equal(
        "services/agent/crates/layerx-wire" in components["wire-codec"]["deleted_trees"],
        False,
        "generic wire crate remains live",
    )
    check_equal(
        "layerxverify" in components["verify"]["symbols"], True, "real verifier symbol"
    )
    check_equal(
        "layerx_paxeer_verifier" in components["paxeer-verifier"]["symbols"],
        True,
        "real checkpoint verifier symbol",
    )
    patterns = [
        ("verify", "layerxverify", symbol_pattern("layerxverify")),
        ("guarantor-runtime", "lxp_guarantor", symbol_pattern("lxp_guarantor")),
        ("guarantor-runtime", "LAYERX_GUARANTOR_", symbol_pattern("LAYERX_GUARANTOR_")),
        (
            "wire-codec",
            "layerx_wire::activity",
            symbol_pattern("layerx_wire::activity"),
        ),
    ]
    allowances = catalogue.get("allowed_references", [])
    check_equal(
        match_line("live.rs", 2, "use layerxverify;", patterns, allowances),
        [("live.rs", 2, "verify", "layerxverify")],
        "exact file and line",
    )
    check_equal(
        match_line(
            "live.rs",
            1,
            "mylayerxverify layerxverifyExtra layerxverifying layerxverify2",
            patterns,
            allowances,
        ),
        [],
        "unrelated longer words",
    )
    check_equal(
        match_line(
            "live_test.py", 7, "def test_layerxverify_case():", patterns, allowances
        ),
        [("live_test.py", 7, "verify", "layerxverify")],
        "symbol embedded in underscore-joined name",
    )
    check_equal(
        match_line("live.rs", 3, "retained_layerxverify", patterns, allowances),
        [("live.rs", 3, "verify", "layerxverify")],
        "underscore prefix is a separator",
    )
    settlement = [
        (
            "settlement-domain",
            "settlement_domain",
            symbol_pattern("settlement_domain"),
        )
    ]
    check_equal(
        match_line(
            "genesis.json", 5, '"settlement_domains": [],', settlement, allowances
        ),
        [("genesis.json", 5, "settlement-domain", "settlement_domain")],
        "plural key",
    )
    check_equal(
        match_line(
            "genesis.json",
            5,
            '"settlement_domainsX": [], "settlement_domainss": []',
            settlement,
            allowances,
        ),
        [],
        "plural is a single trailing s",
    )
    check_equal(
        match_line(
            "live.c", 4, "lxp_guarantor_init LAYERX_GUARANTOR_KEY", patterns, allowances
        ),
        [
            ("live.c", 4, "guarantor-runtime", "lxp_guarantor"),
            ("live.c", 4, "guarantor-runtime", "LAYERX_GUARANTOR_"),
        ],
        "retired namespace prefixes",
    )
    check_equal(
        match_line("live.rs", 1, "layerx_wire::activity::decode", patterns, allowances),
        [("live.rs", 1, "wire-codec", "layerx_wire::activity")],
        "qualified symbol",
    )
    check_equal(
        bool(
            symbol_pattern("checkpoint-settlement.json").search(
                "checkpoint-settlementXjson"
            )
        ),
        False,
        "literal punctuation",
    )
    check_equal(
        is_test_file("services/agent/crates/layerx-client/examples/go_verifier_vectors.rs"),
        False,
        "examples remain live",
    )
    for path in (
        "pkg/keeper_test.go",
        "pkg/test_keeper.c",
        "src/tests.rs",
        "web/page.test.ts",
        "web/page.spec.tsx",
        "tests/live.rs",
        "pkg/testdata/live.json",
        "tools/paxeer-x/tests/routes_test.py",
    ):
        check_equal(is_test_file(path), True, "test file " + path)
    check_equal(
        below("src/retired-next/live.c", "src/retired"), False, "deleted tree boundary"
    )
    all_patterns = [
        (name, symbol, symbol_pattern(symbol))
        for name, component in components.items()
        for symbol in component["symbols"]
    ]
    attestor_path = "services/human/crates/layerx-human-kms/src/attestor.rs"
    attestor_patterns = [
        ("envelope-signing", symbol, symbol_pattern(symbol))
        for symbol in components["envelope-signing"]["symbols"]
    ]
    supported_call = (
        "let mut future = signer::sign_disclosed_call("
        "&transaction_signer, call, disclosure, from);"
    )
    check_equal(
        match_line(attestor_path, 1, supported_call, attestor_patterns, allowances),
        [],
        "attestor disclosed transaction signing is allowed",
    )
    check_equal(
        match_line(
            attestor_path,
            1,
            "let mut future = signer::sign_disclosed(&transaction_signer, call, disclosure, from);",
            attestor_patterns,
            allowances,
        ),
        [(attestor_path, 1, "envelope-signing", "sign_disclosed")],
        "attestor activity envelope signing is detected",
    )
    check_equal(
        match_line("live.rs", 1, supported_call, attestor_patterns, allowances),
        [("live.rs", 1, "envelope-signing", "sign_disclosed")],
        "disclosed signing allowance is path scoped",
    )
    for package, address in (
        ("layerxverify", "LayerXVerifyAddress"),
        ("layerxcustody", "LayerXCustodyAddress"),
        ("layerxanchor", "LayerXAnchorAddress"),
    ):
        registration = (
            f"ecommon.HexToAddress({package}.{address}): "
            f"{package}.GetVersioned(latestUpgrade, keepers),"
        )
        check_equal(
            match_line(
                "precompiles/setup.go", 81, registration, all_patterns, allowances
            ),
            [],
            "kept registration",
        )
    verifier_import = (
        "custodytypes "
        '"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"'
    )
    check_equal(
        match_line(
            "custodyproof/verify.go", 260, verifier_import, all_patterns, allowances
        ),
        [],
        "kept commit and header verifier",
    )
    check_equal(
        match_line("live.go", 260, verifier_import, all_patterns, allowances) != [],
        True,
        "kept verifier allowance is path scoped",
    )
    for path in (
        "modules/layerxcustody/keeper/keeper.go",
        "precompiles/layerxcustody/abi.json",
        "api/layerxcustody/types.pb.go",
        "contracts/src/precompiles/ILayerXCustody.sol",
    ):
        check_equal(
            match_line(
                path, 1, "layerxcustody requestWithdrawal", all_patterns, allowances
            ),
            [],
            "own disabled custody tree " + path,
        )
        check_equal(
            match_line(path + ".outside", 1, "layerxverify", all_patterns, allowances),
            [(path + ".outside", 1, "verify", "layerxverify")],
            "own tree cannot allow another removal " + path,
        )
    check_equal(
        match_line(
            "modules/layerxcustody-next/keeper.go",
            1,
            "layerxcustody",
            all_patterns,
            allowances,
        ),
        [("modules/layerxcustody-next/keeper.go", 1, "custody-deposit", "layerxcustody")],
        "own tree allowance has a directory boundary",
    )
    for entry in allowances:
        for text in entry.get("lines", []):
            check_equal(
                match_line(entry["path"], 81, text, all_patterns, allowances),
                [],
                "kept line " + entry["path"],
            )
    for code in sorted(KEPT_CODES):
        code_patterns = [("verify", code, symbol_pattern(code)), patterns[0]]
        check_equal(
            match_line(
                "include/layerx/lxp_result.h", 42, code, code_patterns, allowances
            ),
            [],
            "kept disabled code",
        )
        check_equal(
            match_line("live.c", 3, code + " layerxverify", code_patterns, allowances),
            [("live.c", 3, "verify", "layerxverify")],
            "disabled code cannot hide callers",
        )
    check_equal(
        match_line(
            "precompiles/setup.go", 81, "layerxverify.execute();", patterns, allowances
        ),
        [("precompiles/setup.go", 81, "verify", "layerxverify")],
        "kept line number is not a blanket exclusion",
    )
    with tempfile.TemporaryDirectory(prefix="retired-callers-filter-") as directory:
        root = Path(directory)
        git(root, ["init", "-q"])
        retired = components["paxeer-verifier"]["deleted_trees"][0]
        retired_file = (
            retired if PurePosixPath(retired).suffix else retired + "/src/lib.rs"
        )
        write_tracked(
            root,
            {
                retired_file: "layerxverify\n",
                "live_test.rs": "layerxverify\n",
                **documentation_files(catalogue, "layerxverify\n"),
                CATALOGUE_PATH: json.dumps(catalogue) + "\n",
                "live.rs": "use layerxverify;\n",
            },
        )
        check_equal(
            scan(root, catalogue, ["verify"], []),
            [("live.rs", 1, "verify", "layerxverify")],
            "single grep filters deleted trees tests documentation and catalogue",
        )
    with tempfile.TemporaryDirectory(prefix="retired-callers-") as directory:
        root = Path(directory)
        git(root, ["init", "-q"])
        files = {
            "services/platform/hosted/indexer/src/abi.rs": 'retained\n("layerxverify", 0x1012),\n',
            "live.c": "lxp_guarantor_init();\n",
            "examples/vectors.rs": "layerxverify\n",
            "pkg/keeper_test.go": "layerxverify\n",
            "pkg/tests/live.rs": "layerxverify\n",
            "odd:name\nfile.rs": "layerxverify\n",
            "clean.rs": "mylayerxverify layerxverifyExtra layerxverifying\n",
            "embedded.rs": "fn retained_layerxverifys_case() {}\n",
            **documentation_files(catalogue, "retained\n"),
        }
        for entry in allowances:
            if "lines" in entry:
                files[entry["path"]] = "\n".join(entry["lines"]) + "\n"
        retired = components["paxeer-verifier"]["deleted_trees"][0]
        retired_file = (
            retired if PurePosixPath(retired).suffix else retired + "/src/lib.rs"
        )
        files[retired_file] = "layerxverify\n"
        write_tracked(root, files)
        (root / "untracked.rs").write_text("layerxverify\n")
        expected = [
            ("embedded.rs", 1, "verify", "layerxverify"),
            ("examples/vectors.rs", 1, "verify", "layerxverify"),
            ("odd:name\nfile.rs", 1, "verify", "layerxverify"),
            ("services/platform/hosted/indexer/src/abi.rs", 2, "verify", "layerxverify"),
        ]
        check_equal(
            scan(root, catalogue, ["verify"], []),
            expected,
            "real git tracked scan and exclusions",
        )
        target = "services/platform/hosted/indexer/src/abi.rs"
        check_equal(
            scan(root, catalogue, ["verify"], [target]),
            [expected[-1]],
            "component and file filters",
        )
        check_equal(
            scan(root, catalogue, ["verify"], ["services/platform/hosted/indexer"]),
            [expected[-1]],
            "directory filter",
        )
        check_equal(
            scan(root, catalogue, ["verify"], [str(root / target)]),
            [expected[-1]],
            "absolute in-repository filter",
        )
        check_equal(
            scan(root, catalogue, ["guarantor-runtime"], [target]),
            [],
            "component filter isolates symbols",
        )
        check_equal(
            scan(root, catalogue, ["verify", "guarantor-runtime"], [target, "live.c"]),
            [("live.c", 1, "guarantor-runtime", "lxp_guarantor"), expected[-1]],
            "repeated component and path filters",
        )
        for arguments in (
            ["--component", "verify", "--path", target],
            ["--component", "verify", "--path", "./" + target],
        ):
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                exit_code = main(arguments, root)
            check_equal(exit_code, 1, "live caller exit code")
            check_equal(
                output.getvalue(),
                target + ":2: verify: layerxverify\n",
                "live caller exact diagnostic",
            )
        (root / target).write_text("retained\n")
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            exit_code = main(["--component", "verify", "--path", target], root)
        check_equal(
            (exit_code, output.getvalue()), (0, ""), "working-tree removal exits zero"
        )
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], [], deleted=True),
            [(retired, 1, "paxeer-verifier", "deleted tree still exists")],
            "deleted gate refuses remaining tree",
        )
        (root / retired_file).unlink()
        if retired_file != retired:
            parent = (root / retired_file).parent
            while parent != root / retired:
                parent.rmdir()
                parent = parent.parent
            parent.rmdir()
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], [], deleted=True),
            [],
            "unstaged tree deletion accepted",
        )
        for arguments in (
            ["--component", "unknown"],
            ["--path", "missing.rs"],
            ["--path", "../live.c"],
            ["--path", str(root.parent / "live.c")],
            ["--path", ":(glob)*"],
            ["--path", "untracked.rs"],
        ):
            output, errors = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                exit_code = main(arguments, root)
            check_equal(exit_code, 2, "invalid filter refused")
            check_equal(output.getvalue(), "", "refusal is not a passing scan")
        (root / retired).symlink_to(root / "absent")
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], [], deleted=True),
            [(retired, 1, "paxeer-verifier", "deleted tree still exists")],
            "dangling deleted-tree symlink refused",
        )
    with tempfile.TemporaryDirectory(prefix="retired-callers-docs-") as directory:
        root = Path(directory)
        git(root, ["init", "-q"])
        title = "Retire the layerx-paxeer-verifier crate\n"
        write_tracked(
            root,
            {
                **documentation_files(catalogue, "retained\n"),
                "spec/layerx-merge/spec.kvx": 'title = "' + title.strip() + '"\n',
                "spec/layerx-merge/docs/retirements.md": "# " + title,
            },
        )
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], []),
            [],
            "reviewed documentation is not a caller",
        )
        write_tracked(
            root,
            {
                "services/agent/Cargo.toml": (
                    "[workspace]\nmembers = [\n"
                    '    "crates/layerx-agentd",\n'
                    '    "crates/layerx-paxeer-verifier",\n]\n'
                ),
                "services/agent/crates/layerx-agentd/src/main.rs": (
                    "fn main() {\n    layerx_paxeer_verifier::verify();\n}\n"
                ),
            },
        )
        callers = [
            ("services/agent/Cargo.toml", 4, "paxeer-verifier", "layerx-paxeer-verifier"),
            (
                "services/agent/crates/layerx-agentd/src/main.rs",
                2,
                "paxeer-verifier",
                "layerx_paxeer_verifier",
            ),
        ]
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], []),
            callers,
            "real callers reported beside documentation",
        )
        write_tracked(
            root,
            {
                "spec/layerx-merge/docs/build/lib.rs": "use layerx_paxeer_verifier;\n",
                "spec/layerx-merge/docs/build/Cargo.toml": (
                    "[dependencies]\nlayerx_paxeer_verifier = "
                    '{ path = "../../../../agent" }\n'
                ),
                "spec/other/notes.kvx": 'title = "' + title.strip() + '"\n',
            },
        )
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], []),
            sorted(
                callers
                + [
                    (
                        "spec/layerx-merge/docs/build/Cargo.toml",
                        2,
                        "paxeer-verifier",
                        "layerx_paxeer_verifier",
                    ),
                    (
                        "spec/layerx-merge/docs/build/lib.rs",
                        1,
                        "paxeer-verifier",
                        "layerx_paxeer_verifier",
                    ),
                    (
                        "spec/other/notes.kvx",
                        1,
                        "paxeer-verifier",
                        "layerx-paxeer-verifier",
                    ),
                ]
            ),
            "only reviewed documentation text is excluded",
        )
        for path, expected_exit, expected_output in (
            ("spec/layerx-merge/spec.kvx", 0, ""),
            (
                "services/agent/Cargo.toml",
                1,
                "services/agent/Cargo.toml:4: paxeer-verifier: layerx-paxeer-verifier\n",
            ),
        ):
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                exit_code = main(
                    ["--component", "paxeer-verifier", "--path", path], root
                )
            check_equal(
                (exit_code, output.getvalue()),
                (expected_exit, expected_output),
                "documentation filter exit " + path,
            )
        missing = {
            **catalogue,
            "documentation_paths": [
                {"path": "spec/absent", "reason": "reviewed retirement notes"}
            ],
        }
        check_equal(
            refusal(scan, root, missing, ["paxeer-verifier"], []),
            "documentation path has no tracked files: spec/absent",
            "untracked documentation path refused",
        )
    with tempfile.TemporaryDirectory(prefix="retired-callers-records-") as directory:
        root = Path(directory)
        git(root, ["init", "-q"])
        write_tracked(
            root,
            {
                **documentation_files(catalogue, "layerxverify\n"),
                "repair_root.kvx": "layerxverify\n",
                "pkg/src/repair_nested.kvx": "layerxverify\n",
                "pkg/src/repair_live.rs": "layerxverify\n",
                "pkg/src/live.kvx": "layerxverify\n",
                "tools/hyperfide/vendor/parser.c": "layerxverify\n",
                "tools/hyperfide-next/parser.c": "layerxverify\n",
                "programs/vendor/library/src/lib.rs": "layerxverify\n",
                "apps/launchpad/web/public/charting_library/bundles/vendor.js": "layerxverify\n",
                "core/deps/geth/signer/fourbyte/4byte.json": "layerxverify\n",
                "services/platform/hosted/authority/src/repair_22_31_207/producer.go": "layerxverify\n",
                "services/platform/ramps/toolkit/src/repair_22_31_211/record.kvx": "layerxverify\n",
                "services/agent/crates/layerx-wire/src/lib.rs": "use layerx_wire::activity;\n",
                "live.rs": "px_getUnifiedHistory px_getNetwork px_listAssets\n",
            },
        )
        expected = [
            ("pkg/src/live.kvx", 1, "verify", "layerxverify"),
            ("pkg/src/repair_live.rs", 1, "verify", "layerxverify"),
            ("tools/hyperfide-next/parser.c", 1, "verify", "layerxverify"),
        ]
        check_equal(
            scan(root, catalogue, ["verify"], []),
            expected,
            "records and vendored data excluded with live neighbors retained",
        )
        check_equal(
            scan(root, catalogue, ["verify"], ["pkg"]),
            expected[:2],
            "record patterns support package-local scans",
        )
        check_equal(
            scan(root, catalogue, ["router-joins"], ["live.rs"]),
            [],
            "retained router methods are callable",
        )
        check_equal(
            scan(
                root, catalogue, ["wire-codec"],
                ["services/agent/crates/layerx-wire"], deleted=True,
            ),
            [(
                "services/agent/crates/layerx-wire/src/lib.rs",
                1, "wire-codec", "layerx_wire::activity",
            )],
            "live wire crate still detects the retired activity codec",
        )
        retired = components["paxeer-verifier"]["deleted_trees"][0]
        write_tracked(root, {retired + "/repair_history.kvx": "layerxverify\n"})
        check_equal(
            scan(root, catalogue, ["paxeer-verifier"], [retired], deleted=True),
            [(retired, 1, "paxeer-verifier", "deleted tree still exists")],
            "record pattern never hides a deleted tree",
        )
        retiring, task = next(iter(components["lni"]["retiring_trees"].items()))
        write_tracked(root, {retiring + "/repair_history.kvx": "layerxverify\n"})
        check_equal(
            scan(root, catalogue, ["lni"], [retiring], deleted=True),
            [(retiring, 1, "lni", "tree retirement pending task " + task)],
            "record pattern never hides a pending retirement",
        )
        missing = {
            **catalogue,
            "documentation_paths": [
                {"pattern": "missing_*.kvx", "reason": "Historical records."}
            ],
        }
        check_equal(
            refusal(scan, root, missing, ["verify"], ["pkg"]),
            "documentation path has no tracked files: missing_*.kvx",
            "untracked documentation pattern refused",
        )
    for pattern, message in (
        ("*", "documentation path covers the checker: *"),
        ("tools/paxeer-x/**", "documentation path covers the checker: tools/paxeer-x/**"),
        (
            "services/agent/crates/layerx-paxeer-verifier/**",
            "documentation path overlaps deleted tree: "
            "services/agent/crates/layerx-paxeer-verifier/**",
        ),
        ("../repair_*.kvx", "path must name a repository-relative file or tree"),
        ("/repair_*.kvx", "path must name a repository-relative file or tree"),
        ("repair.kvx", "invalid documentation pattern: repair.kvx"),
        ("repair_[a].kvx", "invalid documentation pattern: repair_[a].kvx"),
    ):
        check_equal(
            refusal(
                validate_catalogue,
                {
                    **catalogue,
                    "documentation_paths": [
                        {"pattern": pattern, "reason": "Historical records."}
                    ],
                },
            ),
            message,
            "documentation pattern exact refusal " + pattern,
        )
    with tempfile.TemporaryDirectory(prefix="retired-callers-doc-files-") as directory:
        root = Path(directory)
        git(root, ["init", "-q"])
        documents = ["history.txt", "history.rst", "history.json", "HISTORY"]
        document = validate_catalogue(
            {
                **catalogue,
                "documentation_paths": [
                    {"path": path, "reason": "history quotes retired names by design"}
                    for path in [*documents, "notes"]
                ],
            }
        )
        write_tracked(
            root,
            {
                **{path: "layerxverify\n" for path in documents},
                "notes/retired.md": "layerxverify\n",
                "notes/retired.kvx": "layerxverify\n",
                "notes/live.rs": "use layerxverify;\n",
                "history.txt.next": "layerxverify\n",
                "HISTORY_NEXT": "layerxverify\n",
            },
        )
        check_equal(
            scan(root, document, ["verify"], [], deleted=True),
            [
                ("HISTORY_NEXT", 1, "verify", "layerxverify"),
                ("history.txt.next", 1, "verify", "layerxverify"),
                ("notes/live.rs", 1, "verify", "layerxverify"),
            ],
            "listed files allow every suffix but directory code and neighbours remain callers",
        )
        for path in documents:
            check_equal(
                scan(root, document, ["verify"], [path], deleted=True),
                [],
                "explicit documentation file filter " + path,
            )
    for path, reason, message in (
        (
            "services/agent/crates/layerx-paxeer-verifier",
            "reviewed retirement notes",
            "documentation path overlaps deleted tree: "
            "services/agent/crates/layerx-paxeer-verifier",
        ),
        (
            "services/agent/crates/layerx-paxeer-verifier/README.md",
            "reviewed retirement notes",
            "documentation path overlaps deleted tree: "
            "services/agent/crates/layerx-paxeer-verifier/README.md",
        ),
        (
            "services/agent",
            "reviewed retirement notes",
            "documentation path overlaps deleted tree: services/agent",
        ),
        (
            "/spec/layerx-merge/docs",
            "reviewed retirement notes",
            "path must name a repository-relative file or tree",
        ),
        (
            "spec/layerx-merge/../docs",
            "reviewed retirement notes",
            "path must name a repository-relative file or tree",
        ),
        (
            "spec/layerx-merge/./docs",
            "reviewed retirement notes",
            "noncanonical documentation path: spec/layerx-merge/./docs",
        ),
        (
            "spec/layerx-merge/docs",
            " ",
            "documentation reason is missing: spec/layerx-merge/docs",
        ),
        (
            "tools/paxeer-x",
            "reviewed retirement notes",
            "documentation path covers the checker: tools/paxeer-x",
        ),
        (
            CHECKER_PATH,
            "reviewed retirement notes",
            "documentation path covers the checker: " + CHECKER_PATH,
        ),
    ):
        document = {
            **catalogue,
            "documentation_paths": [{"path": path, "reason": reason}],
        }
        check_equal(
            refusal(validate_catalogue, document),
            message,
            "documentation path exact refusal " + path,
        )
    check_equal(
        refusal(
            validate_catalogue,
            {
                **catalogue,
                "documentation_paths": [
                    {"path": "spec/layerx-merge/docs", "reason": "reviewed notes"},
                    {"path": "spec/layerx-merge/docs", "reason": "reviewed notes"},
                ],
            },
        ),
        "duplicate documentation path: spec/layerx-merge/docs",
        "duplicate documentation path refused",
    )
    check_equal(
        refusal(
            validate_catalogue,
            {
                **catalogue,
                "allowed_references": [
                    {"path": "live.rs", "lines": ["retained"], "citation": " "}
                ],
            },
        ),
        "allowed reference citation is invalid: live.rs",
        "blank allowance citation refused",
    )
    for document, message in (
        ({}, "components are missing"),
        ({"components": {}}, "components are missing"),
        ({"components": {"verify": {"symbols": []}}}, "incomplete component: verify"),
    ):
        try:
            validate_catalogue(document)
        except Invalid as error:
            check_equal(str(error), message, "malformed catalogue exact refusal")
        else:
            raise Invalid("self-test failed: malformed catalogue accepted")
    try:
        json.loads('{"components":{},"components":{}}', object_pairs_hook=unique_object)
    except Invalid as error:
        check_equal(
            str(error),
            "duplicate catalogue key: components",
            "duplicate catalogue key exact refusal",
        )
    else:
        raise Invalid("self-test failed: duplicate catalogue key accepted")


def main(argv=None, root=ROOT):
    parser = argparse.ArgumentParser(
        description="Check tracked files for retired callers."
    )
    parser.add_argument("--component", action="append", default=[])
    parser.add_argument("--path", action="append", default=[])
    parser.add_argument(
        "--deleted",
        action="store_true",
        help="also require the selected deleted trees to be absent",
    )
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args(argv)
    if args.self_test and (args.component or args.path or args.deleted):
        parser.error("--self-test cannot be combined with scan filters")
    try:
        catalogue = load_catalogue(CATALOGUE)
        if args.self_test:
            self_test(catalogue)
            print(f"PAXEER_X_GATE tests={COUNT} skipped=0")
            return 0
        findings = scan(root, catalogue, args.component, args.path, args.deleted)
        for path, number, component, symbol in findings:
            display_path = path.replace("\n", "\\n").replace("\r", "\\r")
            print(f"{display_path}:{number}: {component}: {symbol}")
        return 1 if findings else 0
    except (Invalid, OSError, ValueError, TypeError, KeyError) as error:
        message = str(error) if isinstance(error, Invalid) else type(error).__name__
        print("retired-callers: refusal: " + message, file=sys.stderr)
        if args.self_test:
            print(f"PAXEER_X_GATE tests={COUNT} skipped=0")
        return 2


if __name__ == "__main__":
    sys.exit(main())
