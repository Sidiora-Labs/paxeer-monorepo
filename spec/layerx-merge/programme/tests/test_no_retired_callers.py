import json
import re
import shlex
import subprocess
import time
from collections import Counter, defaultdict
from pathlib import Path
from tempfile import mkdtemp

import pytest

from programme import evidence, retired

ROOT = Path(__file__).resolve().parents[4]


def test_scan_reports_no_caller(monkeypatch: pytest.MonkeyPatch) -> None:
    command = ["python3", "tools/paxeer-x/tests/retired_callers_test.py", "--deleted"]
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    log_path = Path(mkdtemp(prefix="layerx-merge-retired-callers-")) / "scan.log"
    monkeypatch.chdir(ROOT)
    started = time.monotonic()
    exit_code = evidence.run(command, log_path)
    elapsed = time.monotonic() - started
    record = evidence.evidence_line(revision, shlex.join(command), exit_code, log_path)
    print(record, flush=True)
    output = log_path.read_text(encoding="utf-8")
    findings = []
    domains = defaultdict(list)
    counts = Counter()
    for line in output.splitlines():
        match = re.fullmatch(r"(.+):([1-9][0-9]*): ([a-z][a-z0-9-]*): (.+)", line)
        assert match is not None, f"{record}\nUnexpected scanner output: {output}"
        path, number = match[1], match[2]
        domain = retired.owners([f"{path}:{number}"])[path]
        findings.append(f"domain {domain}: {line}")
        domains[domain].append((path, match[4]))
        counts[path] += 1
    qualification = []
    for domain, callers in sorted(domains.items()):
        domain_counts = Counter(path for path, _ in callers)
        largest = min(domain_counts, key=lambda path: (-domain_counts[path], path))
        symbols = Counter(symbol for path, symbol in callers if path == largest)
        symbol = min(symbols, key=lambda symbol: (-symbols[symbol], symbol))
        live = (
            Path(largest).suffix in {".c", ".h", ".go", ".rs", ".py", ".ts", ".tsx"}
            and not largest.endswith((".pb.go", ".pb.gw.go"))
            and "/generated/" not in largest
        )
        row = {
            "task": "17.2.8",
            "file": largest,
            "symbol": symbol,
            "observed": (
                f"{record}; domain {domain} has {len(callers)} hits in "
                f"{len(domain_counts)} files, largest {largest} "
                f"({domain_counts[largest]} hits), scan wall time {elapsed:.3f}s."
            ),
            "assumption": (
                f"Domain {domain} must retire every reported caller "
                "before the zero-caller gate can pass."
            ),
            "severity": "blocker" if live else "suspect",
        }
        qualification.append(f"[finding.17.2.8.domain-{domain}]")
        qualification.extend(f"{key} = {json.dumps(value)}" for key, value in row.items())
        qualification.append("")
    findings_path = log_path.with_name("qualification.kvx")
    findings_path.write_text("\n".join(qualification), encoding="utf-8")
    summary_path = log_path.with_name("summary.json")
    summary_path.write_text(
        json.dumps(
            {
                "revision": revision,
                "command": shlex.join(command),
                "exit": exit_code,
                "log": str(log_path),
                "elapsed_seconds": elapsed,
                "excluded_tree": "core/deps",
                "hits": sum(counts.values()),
                "domains": {
                    domain: len(callers) for domain, callers in sorted(domains.items())
                },
                "files": dict(sorted(counts.items())),
            },
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )
    diagnostic = "\n".join(
        [
            record,
            f"scan wall time {elapsed:.3f}s; findings={findings_path}; summary={summary_path}",
            *findings,
        ]
    )
    assert exit_code == 0, diagnostic
    assert output == "", diagnostic
