from __future__ import annotations

import argparse
import re
import subprocess
from collections import Counter
from os import PathLike
from pathlib import Path

if __package__:
    from .kvx import KvxError, KvxValue, _value, _without_comment
else:
    from kvx import KvxError, KvxValue, _value, _without_comment


ROOT = Path(__file__).resolve().parents[3]
RESULTS = ("passed", "failed", "not_run", "no_tests")
ReportValue = KvxValue | float


def _load_report(path: Path) -> dict[str, dict[str, ReportValue]]:
    """Read sweep KVX, including decimal measurements in report metadata."""
    sections: dict[str, dict[str, ReportValue]] = {}
    current = None
    with path.open(encoding="utf-8") as source:
        for number, raw in enumerate(source, start=1):
            location = f"{path}:{number}"
            if "\t" in raw:
                raise KvxError(f"{location}: tab is not allowed")
            line = _without_comment(raw)
            if not line:
                continue
            header = re.fullmatch(r"\[([A-Za-z0-9_.-]+)\]", line)
            if header:
                name = header.group(1)
                if name in sections:
                    raise KvxError(f"{location}: duplicate section [{name}]")
                current = sections[name] = {}
                continue
            if current is None:
                raise KvxError(f"{location}: expected a section header")
            key, separator, text = line.partition("=")
            key = key.strip()
            if not separator or not re.fullmatch(r"[A-Za-z0-9_.-]+", key):
                raise KvxError(f"{location}: expected key = value")
            if key in current:
                raise KvxError(f"{location}: duplicate key {key}")
            text = text.strip()
            current[key] = (
                float(text)
                if re.fullmatch(r"[+-]?[0-9]+\.[0-9]+", text)
                else _value(text, location)
            )
    return sections


def _text(
    section: dict[str, ReportValue], field: str, location: str, *, required: bool = True
) -> str:
    value = section.get(field, "")
    if not isinstance(value, str) or (required and not value.strip()):
        raise ValueError(f"{location}: {field} must be a non-empty string")
    return value


def _quoted(value: str) -> str:
    return '"' + value.replace('"', r'\"') + '"'


def sweeps_summary(spec_dir: str | PathLike[str]) -> Path:
    directory = Path(spec_dir) / "programme" / "sweeps"
    destination = directory / "summary.kvx"
    totals: Counter[str] = Counter()
    outcomes: dict[tuple[str, str], tuple[int, dict[str, str]]] = {}
    names: set[str] = set()
    revisions: set[str] = set()

    for path in sorted(directory.glob("*.kvx")):
        if path == destination:
            continue
        sections = _load_report(path)
        sweeps = {
            key.removeprefix("sweep."): section
            for key, section in sections.items()
            if key.startswith("sweep.")
        }
        targets: dict[str, list[tuple[str, dict[str, ReportValue]]]] = {
            name: [] for name in sweeps
        }
        for key, section in sections.items():
            if not key.startswith("target."):
                continue
            name, separator, index = key.removeprefix("target.").rpartition(".")
            if not separator or not index.isdecimal() or name not in sweeps:
                raise ValueError(f"{path}: [{key}] target has no matching sweep")
            targets[name].append((key, section))

        for name, sweep in sweeps.items():
            location = f"{path}: [sweep.{name}]"
            if name in names:
                raise ValueError(f"{location}: sweep is duplicated")
            names.add(name)
            family, separator, suffix = name.rpartition("-")
            if separator and suffix.isdecimal():
                pass_number = int(suffix)
            else:
                family, pass_number = name, 1
            _text(sweep, "command", location)
            revision = _text(sweep, "revision", location)
            if not re.fullmatch(r"[0-9a-f]{40}", revision):
                raise ValueError(f"{location}: revision must be a 40-hex commit")
            if revision not in revisions:
                ancestry = subprocess.run(
                    ["git", "merge-base", "--is-ancestor", revision, "HEAD"],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    check=False,
                )
                if ancestry.returncode != 0:
                    raise ValueError(
                        f"{location}: revision {revision} is not an ancestor of HEAD "
                        f"(git exit {ancestry.returncode})"
                    )
                revisions.add(revision)

            counts: Counter[str] = Counter()
            target_names: set[str] = set()
            for key, section in targets[name]:
                target_location = f"{path}: [{key}]"
                target = _text(section, "target", target_location)
                if target in target_names:
                    raise ValueError(f"{target_location}: target {target!r} is duplicated")
                target_names.add(target)
                result = _text(section, "result", target_location)
                if result not in RESULTS:
                    raise ValueError(f"{target_location}: result {result!r} is invalid")
                counts[result] += 1
                outcome = {"sweep": name, "target": target, "result": result}
                if result in ("failed", "not_run"):
                    outcome["observed"] = _text(
                        section,
                        "observed",
                        target_location,
                        required=result == "failed",
                    )
                    outcome["assumption"] = _text(
                        section,
                        "assumption",
                        target_location,
                        required=result == "failed",
                    )
                identity = (family, target)
                previous = outcomes.get(identity)
                if previous is None or pass_number > previous[0]:
                    outcomes[identity] = (pass_number, outcome)

            expected = {"targets_total": len(targets[name])}
            expected.update(
                {f"targets_{result}": counts[result] for result in RESULTS[:3]}
            )
            if counts["no_tests"] or "targets_no_tests" in sweep:
                expected["targets_no_tests"] = counts["no_tests"]
            for field, count in expected.items():
                actual = sweep.get(field)
                if type(actual) is not int or actual != count:
                    raise ValueError(
                        f"{location}: {field} is {actual!r}, expected {count} from target blocks"
                    )

    if not names:
        raise ValueError(f"{directory}: reports contains no [sweep.<name>] blocks")

    failures: list[dict[str, str]] = []
    for _, outcome in outcomes.values():
        totals["targets_total"] += 1
        totals[f"targets_{outcome['result']}"] += 1
        if outcome["result"] in ("failed", "not_run"):
            failures.append(outcome)

    lines = ["[summary]", f"reports = {len(names)}"]
    for field in ("targets_total", *(f"targets_{result}" for result in RESULTS)):
        lines.append(f"{field} = {totals[field]}")
    for index, failure in enumerate(
        sorted(failures, key=lambda row: (row["sweep"], row["target"])), start=1
    ):
        lines.extend(("", f"[failing.{index}]"))
        lines.extend(f"{field} = {_quoted(value)}" for field, value in failure.items())
    destination.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return destination


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spec_dir", type=Path)
    args = parser.parse_args()
    try:
        sweeps_summary(args.spec_dir)
    except (OSError, ValueError) as error:
        parser.exit(1, f"{error}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
