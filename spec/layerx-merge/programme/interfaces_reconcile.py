from __future__ import annotations

import argparse
import bisect
import re
import subprocess
from collections import defaultdict
from os import PathLike
from pathlib import Path

if __package__:
    from .kvx import KvxValue, load
else:
    from kvx import KvxValue, load


CLASSES = (
    "served_ok",
    "planned_pending",
    "stale_planned",
    "missing",
    "line_drift",
    "retired_ok",
    "retire_pending",
    "kept",
    "change_present",
    "change_absent",
)
_IDENTIFIER = r"[A-Za-z_$][A-Za-z0-9_$]*"
_TOKENS = re.compile(r"[\w$]+")
_QUOTED = re.compile(r'"([^"\n]+)"|\x27([^\x27\n]+)\x27|`([^`\n]+)`')
_DECLARATIONS = tuple(
    re.compile(pattern)
    for pattern in (
        rf"\bfunc\s+(?:\([^)]*\)\s*)?({_IDENTIFIER})\s*(?:\[|\()",
        rf"\b(?:fn|def|function)\s*\*?\s+({_IDENTIFIER})",
        rf"#\s*define\s+({_IDENTIFIER})",
        rf"\b(?:class|struct|enum|trait|type|interface|event|error|union)\s+({_IDENTIFIER})",
        rf"\b(?:const|constant|let|var|static|mod)\s+(?:mut\s+)?({_IDENTIFIER})",
        rf"\b(?:message|service)\s+(?:{_IDENTIFIER}\.)*({_IDENTIFIER})",
        rf"\brpc\s+({_IDENTIFIER})",
        rf"\benum\s*\{{\s*({_IDENTIFIER})",
        rf"\bexport\s+(?:default\s+|\{{\s*)({_IDENTIFIER})",
    )
)
_LITERAL_KINDS = {
    "cli_command",
    "config_key",
    "env_var",
    "file",
    "file_format",
    "http_route",
    "rest_route",
    "route",
    "ws_route",
    "metric",
}


def _symbol(kind: str, signature: str) -> tuple[str, bool]:
    if kind == "c_symbol" and not re.search(r"\b(?:fn|func|def|function)\b", signature):
        declarator = signature.split("{", 1)[0]
        for pattern in (
            rf"#\s*define\s+({_IDENTIFIER})",
            rf"\(\s*\*\s*({_IDENTIFIER})\s*\)\s*\(",
            rf"\b({_IDENTIFIER})\s*\(",
        ):
            match = re.search(pattern, declarator)
            if match and match[1] not in ("X", "_Atomic", "sizeof", "__attribute__"):
                return match[1], False
    declared_format = kind == "file_format" and signature.startswith(("pub ", "export "))
    if kind not in _LITERAL_KINDS or declared_format:
        for pattern in _DECLARATIONS:
            match = pattern.search(signature)
            if match:
                return match[1], False
        match = re.match(rf"X\(\s*({_IDENTIFIER})\s*,", signature)
        if match:
            return match[1], False
        match = re.search(rf"\b({_IDENTIFIER})\s*\(", signature)
        if match:
            return match[1], False
    quoted = _QUOTED.search(signature)
    if quoted:
        return next(value for value in quoted.groups() if value is not None), True
    route = re.match(
        r"(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|ANY)\s+(\S+)", signature
    )
    if route:
        return route[1], True
    target = (
        re.match(r"([^\s:]+)\s*:", signature) if kind == "makefile_target" else None
    )
    if target:
        return target[1], True
    assignment = re.match(rf"({_IDENTIFIER})\s*(?::=|=|:)", signature)
    if assignment:
        return assignment[1], False
    if re.fullmatch(_IDENTIFIER, signature):
        return signature, False
    return signature, True


def _quote(value: str) -> str:
    return '"' + value.replace('"', r"\"") + '"'


def interfaces_reconcile(
    spec_dir: str | PathLike[str], repo_root: str | PathLike[str], *, write: bool = True
) -> dict[str, dict[str, KvxValue]]:
    spec_dir, repo_root = Path(spec_dir).resolve(), Path(repo_root).resolve()
    registry = load(spec_dir / "interfaces.kvx")
    grouped: dict[Path, list[tuple[str, dict[str, KvxValue]]]] = defaultdict(list)
    interfaces = [
        (name, row) for name, row in registry.items() if name.startswith("interface.")
    ]
    for name, row in interfaces:
        grouped[(repo_root / str(row["path"])).resolve()].append((name, row))

    inputs = [spec_dir / "interfaces.kvx", *grouped]
    revision = subprocess.run(
        [
            "git",
            "-C",
            str(repo_root),
            "log",
            "-1",
            "--format=%H",
            "HEAD",
            "--",
            *(f":(literal){path.relative_to(repo_root).as_posix()}" for path in inputs),
        ],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    if not revision:
        raise ValueError("the registry and referenced files have no revision at HEAD")
    counts = dict.fromkeys(CLASSES, 0)
    domains: dict[str, dict[str, int]] = {}
    findings: dict[str, dict[str, KvxValue]] = {}
    for path, rows in grouped.items():
        exists = path.is_file()
        content = path.read_text(encoding="utf-8", errors="replace") if exists else ""
        newlines = [match.start() for match in re.finditer("\n", content)]
        symbols = [_symbol(str(row["kind"]), str(row["signature"])) for _, row in rows]
        identifiers: dict[str, list[int]] = {
            symbol: [] for symbol, literal in symbols if not literal
        }
        if identifiers:
            for match in _TOKENS.finditer(content):
                symbol = match.group()
                if symbol in identifiers:
                    identifiers[symbol].append(
                        bisect.bisect_left(newlines, match.start()) + 1
                    )
        occurrences = {(symbol, False): lines for symbol, lines in identifiers.items()}
        for (name, row), (symbol, literal) in zip(rows, symbols):
            key = symbol, literal
            if key not in occurrences:
                occurrences[key] = []
                if symbol and exists:
                    start = content.find(symbol)
                    while start != -1:
                        occurrences[key].append(bisect.bisect_left(newlines, start) + 1)
                        start = content.find(symbol, start + 1)
            locations = occurrences[key]
            status = row["status"]
            planned = status in ("new", "planned")
            if "overridden_by" in row:
                classification = "kept"
            elif status == "retire":
                classification = "retire_pending" if locations else "retired_ok"
            elif status == "change":
                classification = "change_present" if locations else "change_absent"
            elif not locations:
                classification = "planned_pending" if planned else "missing"
            elif planned:
                classification = "stale_planned"
            elif row["line"] and not any(
                abs(number - int(row["line"])) <= 40 for number in locations
            ):
                classification = "line_drift"
            else:
                classification = "served_ok"
            counts[classification] += 1
            domain = name.split(".", 2)[1]
            domain_counts = domains.setdefault(
                domain, {"rows": 0, **dict.fromkeys(CLASSES, 0)}
            )
            domain_counts["rows"] += 1
            domain_counts[classification] += 1
            if classification != "served_ok":
                findings[name] = {
                    "interface": name,
                    "path": row["path"],
                    "status": row["status"],
                    "class": classification,
                    "symbol": symbol,
                }

    report: dict[str, dict[str, KvxValue]] = {
        "interfaces": {"revision": revision, "rows": len(interfaces), **counts}
    }
    for domain in sorted(domains):
        report[f"domain.{domain}"] = domains[domain]
    for number, (name, _) in enumerate(interfaces, 1):
        if name in findings:
            report[f"row.{number}"] = findings[name]
    output = []
    for section, fields in report.items():
        output.append(f"[{section}]")
        for key, value in fields.items():
            rendered = _quote(value) if isinstance(value, str) else str(value)
            output.append(f"{key} = {rendered}")
        output.append("")
    if write:
        destination = spec_dir / "programme/sweeps/interfaces.kvx"
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text("\n".join(output), encoding="utf-8")
    return report


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spec_dir", type=Path)
    parser.add_argument("repo_root", type=Path)
    parser.add_argument("--no-write", action="store_true")
    args = parser.parse_args()
    report = interfaces_reconcile(args.spec_dir, args.repo_root, write=not args.no_write)
    print(report["interfaces"])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
