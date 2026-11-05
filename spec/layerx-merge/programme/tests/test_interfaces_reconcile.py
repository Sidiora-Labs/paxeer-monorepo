import collections
import subprocess
import sys
from pathlib import Path

from programme.interfaces_reconcile import CLASSES, interfaces_reconcile
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]


def test_interfaces_reconcile_classifies_each_case(tmp_path: Path) -> None:
    tree = tmp_path / "tree"
    tree.mkdir()
    subprocess.run(
        ["git", "init", "--quiet", str(tree)],
        check=True,
        capture_output=True,
        text=True,
    )
    spec = tree / "spec"
    spec.mkdir()
    source = tree / "shared.txt"
    source.write_text(
        "func (s *Server) Serve() {}\n"
        "type Record struct {}\n"
        "pub fn execute() {}\n"
        "pub struct State {}\n"
        "pub enum Mode {}\n"
        "pub trait Transport {}\n"
        "pub const LIMIT: usize = 4;\n"
        "export function Widget() {}\n"
        "export class Client {}\n"
        "def reconcile(): pass\n"
        "class Report: pass\n"
        "int c_serve(void);\n"
        "const char *c_name(void);\n"
        "struct Result *c_result(void);\n"
        "#define C_LIMIT 4\n"
        "function transfer(address to) external;\n"
        "event Transfer(address to);\n"
        'router.get("/live", handler);\n'
        'const char *key = "feature.enabled";\n'
        'const char *file = "snapshot.bin";\n'
        "func ServeSuffix() {}\n" + "\n" * 80 + "Serve();\n",
        encoding="utf-8",
    )
    cases = [
        ("go_exported", "func (s *Server) Serve()", "served", 1, "served_ok", "Serve"),
        ("go_exported", "type Record struct {}", "existing", 0, "served_ok", "Record"),
        ("rust_pub_item", "pub fn execute()", "stable", 0, "served_ok", "execute"),
        ("rust_pub_item", "pub struct State", "stable", 0, "served_ok", "State"),
        ("rust_pub_item", "pub enum Mode", "stable", 0, "served_ok", "Mode"),
        ("rust_pub_item", "pub trait Transport", "stable", 0, "served_ok", "Transport"),
        (
            "rust_pub_item",
            "pub const LIMIT: usize = 4",
            "stable",
            0,
            "served_ok",
            "LIMIT",
        ),
        ("ts_export", "export function Widget()", "stable", 0, "served_ok", "Widget"),
        ("ts_export", "export class Client", "stable", 0, "served_ok", "Client"),
        ("python", "def reconcile()", "stable", 0, "served_ok", "reconcile"),
        ("python", "class Report", "stable", 0, "served_ok", "Report"),
        ("c_symbol", "int c_serve(void)", "stable", 0, "served_ok", "c_serve"),
        ("c_symbol", "const char *c_name(void)", "stable", 0, "served_ok", "c_name"),
        (
            "c_symbol",
            "struct Result *c_result(void)",
            "stable",
            0,
            "served_ok",
            "c_result",
        ),
        ("c_symbol", "#define C_LIMIT 4", "stable", 0, "served_ok", "C_LIMIT"),
        (
            "sol_function",
            "function transfer(address to)",
            "stable",
            0,
            "served_ok",
            "transfer",
        ),
        (
            "sol_event",
            "event Transfer(address to)",
            "stable",
            0,
            "served_ok",
            "Transfer",
        ),
        ("http_route", 'GET "/live"', "served", 0, "served_ok", "/live"),
        (
            "config_key",
            'setting "feature.enabled"',
            "served",
            0,
            "served_ok",
            "feature.enabled",
        ),
        (
            "file_format",
            'file "snapshot.bin"',
            "served",
            0,
            "served_ok",
            "snapshot.bin",
        ),
        ("http_route", "GET /live", "served", 0, "served_ok", "/live"),
        ("go_exported", "func Absent()", "existing", 0, "missing", "Absent"),
        ("go_exported", "func Serv()", "retire", 0, "retired_ok", "Serv"),
        ("go_exported", "func Absent()", "new", 0, "planned_pending", "Absent"),
        ("go_exported", "func Absent()", "planned", 0, "planned_pending", "Absent"),
        ("go_exported", "func Serve()", "new", 500, "stale_planned", "Serve"),
        ("go_exported", "func Serve()", "planned", 500, "stale_planned", "Serve"),
        ("go_exported", "func Serve()", "stable", 42, "line_drift", "Serve"),
        ("go_exported", "func Serve()", "keep", 41, "served_ok", "Serve"),
        ("go_exported", "func Serve()", "stable", 99, "served_ok", "Serve"),
        ("go_exported", "func Gone()", "served", 1, "missing", "Gone"),
        ("go_exported", "func Gone()", "new", 1, "planned_pending", "Gone"),
        ("go_exported", "func Gone()", "planned", 1, "planned_pending", "Gone"),
    ]
    cases = [(*case, None) for case in cases] + [
        ("go_exported", "func Serve()", "retire", 500, "retire_pending", "Serve", None),
        (
            "go_exported",
            "func Serve()",
            "retire",
            500,
            "kept",
            "Serve",
            "override.2478",
        ),
        (
            "go_exported",
            "func Absent()",
            "retire",
            0,
            "kept",
            "Absent",
            "override.2478",
        ),
        ("go_exported", "func Serve()", "change", 500, "change_present", "Serve", None),
        ("go_exported", "func Absent()", "change", 0, "change_absent", "Absent", None),
        ("go_exported", "func Gone()", "retire", 1, "retired_ok", "Gone", None),
        ("go_exported", "func Gone()", "change", 1, "change_absent", "Gone", None),
    ]
    registry = ['[meta]\nname = "test registry"\n']
    expected = {}
    expected_domains = {}
    for number, case in enumerate(cases, 1):
        kind, signature, status, line, classification, symbol, overridden_by = case
        path = "absent.go" if symbol == "Gone" else "shared.txt"
        domain = str(number % 2 + 1)
        name = f"interface.{domain}.case-{number}"
        escaped = signature.replace('"', r"\"")
        registry.append(
            f'[{name}]\nkind = "{kind}"\npath = "{path}"\n'
            f'line = {line}\nsignature = "{escaped}"\ncallers = []\n'
            f'status = "{status}"\ncontract = "test"\nnotes = "case # {number}"\n'
        )
        if overridden_by is not None:
            registry[-1] += f'overridden_by = "{overridden_by}"\n'
        domain_counts = expected_domains.setdefault(
            f"domain.{domain}", {"rows": 0, **dict.fromkeys(CLASSES, 0)}
        )
        domain_counts["rows"] += 1
        domain_counts[classification] += 1
        if classification != "served_ok":
            expected[f"row.{number}"] = {
                "interface": name,
                "path": path,
                "status": status,
                "class": classification,
                "symbol": symbol,
            }
    (spec / "interfaces.kvx").write_text("\n".join(registry), encoding="utf-8")
    subprocess.run(
        ["git", "-C", str(tree), "add", "shared.txt", "spec/interfaces.kvx"],
        check=True,
        capture_output=True,
        text=True,
    )
    commit = [
        "git",
        "-C",
        str(tree),
        "-c",
        "user.name=Interface test",
        "-c",
        "user.email=interfaces@example.invalid",
        "-c",
        "commit.gpgsign=false",
        "commit",
        "--quiet",
        "-m",
    ]
    subprocess.run(
        commit + ["Add registry and source"],
        check=True,
        capture_output=True,
        text=True,
    )
    reads = []
    observing = True

    def record_open(event: str, args: tuple) -> None:
        if observing and event == "open" and args[0] == str(source):
            reads.append(args[0])

    sys.addaudithook(record_open)
    try:
        report = interfaces_reconcile(spec, tree)
    finally:
        observing = False
    assert reads == [str(source)]
    counts = collections.Counter(case[4] for case in cases)
    assert set(counts) == set(CLASSES)
    header = report["interfaces"]
    assert header["rows"] == len(cases)
    assert {name: header[name] for name in CLASSES} == dict(counts)
    revision = subprocess.run(
        ["git", "-C", str(tree), "rev-parse", "HEAD"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    assert header["revision"] == revision
    assert {
        name: row for name, row in report.items() if name.startswith("row.")
    } == expected
    assert {
        name: row for name, row in report.items() if name.startswith("domain.")
    } == expected_domains
    output = spec / "programme/sweeps/interfaces.kvx"
    assert load(output) == report
    first = output.read_bytes()
    assert interfaces_reconcile(spec, tree) == report
    assert output.read_bytes() == first
    subprocess.run(
        ["git", "-C", str(tree), "add", "spec/programme/sweeps/interfaces.kvx"],
        check=True,
        capture_output=True,
        text=True,
    )
    subprocess.run(
        commit + ["Publish report"], check=True, capture_output=True, text=True
    )
    published_revision = subprocess.run(
        ["git", "-C", str(tree), "rev-parse", "HEAD"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    assert published_revision != revision
    assert interfaces_reconcile(spec, tree) == report
    assert output.read_bytes() == first
    source.write_text(
        source.read_text(encoding="utf-8") + "func Absent() {}\n", encoding="utf-8"
    )
    subprocess.run(
        ["git", "-C", str(tree), "add", "shared.txt"],
        check=True,
        capture_output=True,
        text=True,
    )
    subprocess.run(
        commit + ["Implement absent symbol"],
        check=True,
        capture_output=True,
        text=True,
    )
    source_revision = subprocess.run(
        ["git", "-C", str(tree), "rev-parse", "HEAD"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    updated = interfaces_reconcile(spec, tree)
    assert updated["interfaces"]["revision"] == source_revision
    assert source_revision != published_revision
    assert updated["interfaces"]["missing"] == header["missing"] - 1
    assert updated["interfaces"]["planned_pending"] == header["planned_pending"] - 2
    assert updated["interfaces"]["stale_planned"] == header["stale_planned"] + 2
    assert updated["interfaces"]["kept"] == header["kept"]
    assert updated["interfaces"]["change_absent"] == header["change_absent"] - 1
    assert updated["interfaces"]["change_present"] == header["change_present"] + 1
    assert "row.22" not in updated
    assert updated["row.24"]["class"] == "stale_planned"
    assert updated["row.25"]["class"] == "stale_planned"


def test_interfaces_reconcile_reads_the_registry() -> None:
    spec = ROOT / "spec/layerx-merge"
    registry = load(spec / "interfaces.kvx")
    interfaces = {
        name: row for name, row in registry.items() if name.startswith("interface.")
    }
    report = interfaces_reconcile(spec, ROOT)
    assert load(spec / "programme/sweeps/interfaces.kvx") == report
    header = report["interfaces"]
    assert header["rows"] == len(interfaces) > 0
    assert sum(header[name] for name in CLASSES) == header["rows"]
    findings = [row for name, row in report.items() if name.startswith("row.")]
    assert len(findings) == header["rows"] - header["served_ok"]
    assert len({row["interface"] for row in findings}) == len(findings)
    counts = collections.Counter(row["class"] for row in findings)
    for name in CLASSES[1:]:
        assert counts[name] == header[name]
    for row in findings:
        original = interfaces[row["interface"]]
        assert row["path"] == original["path"]
        assert row["status"] == original["status"]
        assert row["symbol"]
    domains = {
        name: row for name, row in report.items() if name.startswith("domain.")
    }
    expected_rows = collections.Counter(name.split(".", 2)[1] for name in interfaces)
    assert set(domains) == {f"domain.{domain}" for domain in expected_rows}
    for domain, count in expected_rows.items():
        domain_counts = domains[f"domain.{domain}"]
        assert domain_counts["rows"] == count
        assert sum(domain_counts[name] for name in CLASSES) == count
        classified = collections.Counter(
            row["class"]
            for row in findings
            if row["interface"].split(".", 2)[1] == domain
        )
        assert domain_counts["served_ok"] == count - sum(classified.values())
        for name in CLASSES[1:]:
            assert domain_counts[name] == classified[name]
    for name in ("rows", *CLASSES):
        assert sum(row[name] for row in domains.values()) == header[name]
    revision = subprocess.run(
        [
            "git",
            "-C",
            str(ROOT),
            "log",
            "-1",
            "--format=%H",
            "HEAD",
            "--",
            "spec/layerx-merge/interfaces.kvx",
            *(
                f":(literal){path}"
                for path in sorted({str(row["path"]) for row in interfaces.values()})
            ),
        ],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    assert header["revision"] == revision
    subprocess.run(
        ["git", "-C", str(ROOT), "merge-base", "--is-ancestor", revision, "HEAD"],
        check=True,
        capture_output=True,
        text=True,
    )
