#!/usr/bin/env python3
"""Validate stale dispositions and, optionally, coverage of an open-selector report."""

from __future__ import annotations

import re
import sys
from collections import defaultdict
from pathlib import Path

from kvx import load


def parse(path):
    document = load(path)
    entries = []
    for section, values in document.items():
        if section == "meta":
            continue
        match = re.fullmatch(r"stale\.([1-9][0-9]*)", section)
        if match is None:
            raise ValueError(f"unexpected disposition section: {section}")
        entries.append(dict(values, n=int(match[1])))
    if [entry["n"] for entry in entries] != list(range(1, len(entries) + 1)):
        raise ValueError("stale entries must be numbered consecutively from 1")
    if len(entries) < 136:
        raise ValueError(
            f"expected at least the original 136 entries, found {len(entries)}"
        )
    if document.get("meta", {}).get("count") != len(entries):
        raise ValueError("meta.count must equal the number of stale entries")
    return entries


def selector_for(entry):
    return entry["file"] + ("::" + entry["test"] if entry["test"] else "")


def validate_entries(entries, root):
    errors = []
    seen = set()
    for entry in entries:
        label = f"stale.{entry['n']}"
        for key in ("acceptance", "file", "test", "disposition"):
            if not isinstance(entry.get(key), str):
                errors.append(f"{label} must carry a string {key}")
        if not all(
            isinstance(entry.get(key), str) for key in ("acceptance", "file", "test")
        ):
            continue
        if not re.fullmatch(r"acceptance\.[0-9]+(?:\.[0-9]+)+", entry["acceptance"]):
            errors.append(f"{label} has an invalid acceptance id")
        if not entry["file"].strip():
            errors.append(f"{label} has an empty selector path")
        identity = (entry["acceptance"], selector_for(entry))
        if identity in seen:
            errors.append(f"{label} duplicates {identity[0]} {identity[1]}")
        seen.add(identity)
        disposition = entry.get("disposition", "")
        if disposition == "replacement":
            replacement = entry.get("replacement", "")
            if not isinstance(replacement, str):
                errors.append(f"{label} replacement must be path::TestName")
                continue
            file, separator, name = replacement.partition("::")
            if not separator or not file.strip() or not name.strip():
                errors.append(f"{label} replacement must be path::TestName")
                continue
            relative = Path(file)
            if relative.is_absolute() or ".." in relative.parts:
                errors.append(f"{label} replacement must stay inside the repository")
                continue
            full = root / relative
            if not full.is_file():
                errors.append(f"{label} replacement file missing: {file}")
                continue
            if name not in full.read_text(encoding="utf-8", errors="replace"):
                errors.append(f"{label} test {name} not found in {file}")
        elif disposition == "retired":
            reason = entry.get("reason", "")
            if not isinstance(reason, str) or len(reason.split()) < 20:
                errors.append(f"{label} retired without a reason of twenty words")
        else:
            errors.append(f"{label} still {disposition or 'undecided'}")
    return errors


def read_record(path):
    stale, uncovered = [], []
    section = None
    sections = set()
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line in ("[stale]", "[uncovered]"):
            if line in sections:
                raise ValueError(f"{path}:{number}: duplicate {line} section")
            sections.add(line)
            section = line
        elif section == "[stale]":
            match = re.fullmatch(r"\[(acceptance\.[0-9.]+)\] (.+); owners: .+", line)
            if match is None:
                raise ValueError(f"{path}:{number}: malformed stale selector")
            stale.append((match[1], match[2]))
        elif section == "[uncovered]" and not line.startswith("["):
            uncovered.append(line)
        else:
            raise ValueError(f"{path}:{number}: unexpected record line")
    if not stale or not uncovered:
        raise ValueError(
            "the record must contain nonempty stale and uncovered sections"
        )
    if len(set(stale)) != len(stale) or len(set(uncovered)) != len(uncovered):
        raise ValueError("the record contains duplicate selectors within a section")
    return stale, uncovered


def canonical_selector(selector):
    # The report omits these spec planning notes; a nonliteral match still needs
    # a disposition. Normalization resolves its acceptance, never its coverage.
    return re.sub(r" \((?:planned(?:,[^()]*)?|existing)\)$", "", selector)


def selector_path(selector):
    return re.sub(r"\s+\([^()]*\)$", "", selector.partition("::")[0])


def required_selectors(record_path, spec, root):
    stale, uncovered = read_record(record_path)
    acceptances = {
        name: values for name, values in spec.items() if name.startswith("acceptance.")
    }
    selectors = defaultdict(list)
    owners = defaultdict(list)
    for name, acceptance in acceptances.items():
        test = acceptance.get("test", "")
        if isinstance(test, str) and test:
            selectors[canonical_selector(test)].append((name, acceptance))
    for name, task in spec.items():
        if name.startswith("task."):
            for requirement in task.get("reqs", []):
                owners[requirement].append(task)

    required = set(stale)
    covered = []
    for acceptance, selector in stale:
        if acceptance not in acceptances:
            raise ValueError(f"record names unknown {acceptance}")
        if canonical_selector(acceptances[acceptance].get("test", "")) != selector:
            raise ValueError(
                f"record selector does not belong to {acceptance}: {selector}"
            )
    for selector in uncovered:
        matches = selectors.get(canonical_selector(selector), [])
        if not matches:
            raise ValueError(
                f"uncovered selector has no acceptance in the spec: {selector}"
            )
        command = re.match(r"^(?:make|cargo|grep|go)(?:\s|$)", selector) is not None
        missing = not (root / selector_path(selector)).exists()
        nonliteral = not any(
            acceptance.get("test") == selector for _, acceptance in matches
        )
        needs_entry = False
        for name, acceptance in matches:
            held = owners.get(acceptance.get("req"), [])
            if command or missing or nonliteral or not held:
                required.add((name, selector))
                needs_entry = True
            elif any(
                task.get("status") not in ("done", "superseded") for task in held
            ):
                raise ValueError(
                    f"ownership does not establish completed coverage: {name} {selector}"
                )
        if not needs_entry:
            covered.append(selector)
    return required, covered


def main():
    if len(sys.argv) > 3:
        print("usage: parity_stale_check.py [map.kvx [open-selectors.txt]]")
        return 1
    path = Path(sys.argv[1] if len(sys.argv) > 1 else __file__).resolve()
    if len(sys.argv) == 1:
        path = path.with_name("parity-stale.kvx")
    try:
        root = Path.cwd()
        entries = parse(path)
        errors = validate_entries(entries, root)
        if len(sys.argv) == 3:
            record = Path(sys.argv[2])
            spec = load(path.parent.parent / "spec.kvx")
            required, covered = required_selectors(record, spec, root)
            present = {
                (entry["acceptance"], selector_for(entry))
                for entry in entries
                if all(
                    isinstance(entry.get(key), str)
                    for key in ("acceptance", "file", "test")
                )
            }
            for acceptance, selector in sorted(required - present):
                errors.append(f"missing disposition: {acceptance} {selector}")
            print(
                f"required selectors {len(required)} covered by ownership {len(covered)}"
            )
        for error in errors:
            print(error)
        print(f"entries {len(entries)} errors {len(errors)}")
        return 1 if errors else 0
    except (OSError, ValueError) as error:
        print(error)
        return 1


if __name__ == "__main__":
    sys.exit(main())
