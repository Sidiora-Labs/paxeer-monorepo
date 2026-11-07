"""Check one interface reconciliation fragment against its slice, the registry and the tree.

usage: python3 spec/layerx-merge/programme/reconcile_check.py <slice> [<reconcile_dir>]

The slice file <reconcile_dir>/slices/<slice>.txt lists registry ids, one per line. The fragment
<reconcile_dir>/<slice>.kvx carries a [reconcile.<slice>] header (revision, rows) and one
[row.<slice>.k] block per id, in slice order, with the keys interface, verdict, path, line,
symbol and evidence; a retired row adds remaining_refs and, above zero, remaining (path:line
references; a reference inside the registered path marks a test-code line the absence check skips). Exit 0 when
every check passes, 1 with one line per failure otherwise. Run from the repository root.
"""
from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from kvx import KvxError, load  # noqa: E402

VERDICTS = ("served", "served_elsewhere", "retired", "pending", "kept")
PRESENT = ("served", "served_elsewhere", "pending", "kept")
KEYS = {"interface", "verdict", "path", "line", "symbol", "evidence"}
ROOT = Path.cwd()


def _present(text: str, symbol: str) -> bool:
    if re.fullmatch(r"[A-Za-z_$][A-Za-z0-9_$]*", symbol):
        return re.search(r"(?<![A-Za-z0-9_$])" + re.escape(symbol) + r"(?![A-Za-z0-9_$])", text) is not None
    return symbol in text


def _revision_known(revision: str) -> bool:
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        return False
    done = subprocess.run(["git", "cat-file", "-e", revision + "^{commit}"], cwd=ROOT, capture_output=True)
    return done.returncode == 0


def check(slice_name: str, reconcile_dir: Path) -> list[str]:
    failures: list[str] = []
    slice_path = reconcile_dir / "slices" / f"{slice_name}.txt"
    fragment_path = reconcile_dir / f"{slice_name}.kvx"
    if not slice_path.is_file():
        return [f"slice file {slice_path} is missing"]
    ids = [line.strip() for line in slice_path.read_text(encoding="utf-8").splitlines() if line.strip()]
    if not fragment_path.is_file():
        return [f"fragment {fragment_path} is missing"]
    try:
        fragment = load(fragment_path)
        registry = load(ROOT / "spec/layerx-merge/interfaces.kvx")
    except KvxError as error:
        return [str(error)]
    header = fragment.get(f"reconcile.{slice_name}")
    if header is None:
        failures.append(f"[reconcile.{slice_name}] header is missing")
    else:
        revision = str(header.get("revision", ""))
        if not _revision_known(revision):
            failures.append(f"header revision {revision!r} is not a 40-hex commit of this repository")
        if header.get("rows") != len(ids):
            failures.append(f"header rows = {header.get('rows')!r}, the slice lists {len(ids)}")
    rows = {name: body for name, body in fragment.items() if name.startswith(f"row.{slice_name}.")}
    expected_names = [f"row.{slice_name}.{k}" for k in range(1, len(ids) + 1)]
    for name in rows:
        if name not in expected_names:
            failures.append(f"[{name}] is not one of the {len(ids)} expected row blocks")
    seen: set[str] = set()
    for k, (name, interface) in enumerate(zip(expected_names, ids), 1):
        body = rows.get(name)
        if body is None:
            failures.append(f"[{name}] for {interface} is missing")
            continue
        missing = KEYS - set(body)
        if missing:
            failures.append(f"[{name}] lacks {sorted(missing)}")
            continue
        if body["interface"] != interface:
            failures.append(f"[{name}] names {body['interface']!r}, slice position {k} is {interface}")
            continue
        seen.add(interface)
        registered = registry.get(interface)
        if registered is None:
            failures.append(f"[{name}] {interface} is not in the registry")
            continue
        verdict = str(body["verdict"])
        if verdict not in VERDICTS:
            failures.append(f"[{name}] verdict {verdict!r} is not one of {VERDICTS}")
            continue
        path, line, symbol, evidence = str(body["path"]), body["line"], str(body["symbol"]), str(body["evidence"])
        if not isinstance(line, int) or line < 1:
            failures.append(f"[{name}] line must be a positive integer")
            continue
        if not symbol.strip():
            failures.append(f"[{name}] symbol is empty")
            continue
        if len(evidence.strip()) < 20:
            failures.append(f"[{name}] evidence is shorter than twenty characters")
        target = ROOT / path
        if verdict in PRESENT:
            if not target.is_file():
                failures.append(f"[{name}] {verdict}: {path} does not exist")
                continue
            lines = target.read_text(encoding="utf-8", errors="replace").splitlines()
            window = "\n".join(lines[max(0, line - 4) : line + 3])
            if not _present(window, symbol):
                failures.append(f"[{name}] {verdict}: {symbol!r} is not within three lines of {path}:{line}")
            if verdict == "served" and path != str(registered["path"]):
                failures.append(f"[{name}] served must keep the registered path {registered['path']}")
            if verdict == "served_elsewhere" and path == str(registered["path"]):
                failures.append(f"[{name}] served_elsewhere must name a path other than {registered['path']}")
            if verdict == "kept" and "overridden_by" not in registered:
                failures.append(f"[{name}] kept needs an overridden_by decision on the registry row")
            if verdict == "kept" and "decision" not in evidence.lower():
                failures.append(f"[{name}] kept evidence must name the decision")
            if registered["status"] == "change" and verdict == "served" and len(evidence.strip()) < 40:
                failures.append(f"[{name}] a served change row needs evidence quoting the declaring line")
        else:
            registered_file = ROOT / str(registered["path"])
            listed = {
                int(ref.rsplit(":", 1)[1])
                for ref in (body.get("remaining") or [])
                if isinstance(ref, str) and ref.rsplit(":", 1)[0] == str(registered["path"]) and ref.rsplit(":", 1)[1].isdigit()
            }
            if registered_file.is_file():
                for number, text in enumerate(registered_file.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                    if number not in listed and _present(text, symbol):
                        failures.append(f"[{name}] retired: {symbol!r} is still present in {registered['path']}:{number}")
                        break
            remaining_refs = body.get("remaining_refs")
            if not isinstance(remaining_refs, int) or remaining_refs < 0:
                failures.append(f"[{name}] retired needs remaining_refs as a non-negative integer")
            elif remaining_refs > 0:
                remaining = body.get("remaining")
                if not isinstance(remaining, list) or len(remaining) != remaining_refs:
                    failures.append(f"[{name}] remaining must list exactly {remaining_refs} path:line references")
                else:
                    for ref in remaining:
                        ref_path = ref.rsplit(":", 1)[0]
                        if not (ROOT / ref_path).is_file():
                            failures.append(f"[{name}] remaining reference {ref} names a missing file")
            elif "remaining" in body and body["remaining"]:
                failures.append(f"[{name}] remaining must be empty when remaining_refs is zero")
    return failures


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print(__doc__)
        return 2
    reconcile_dir = Path(argv[2]) if len(argv) > 2 else ROOT / "spec/layerx-merge/programme/reconcile"
    failures = check(argv[1], reconcile_dir)
    for failure in failures:
        print(failure)
    print(f"reconcile_check {argv[1]}: {'ok' if not failures else str(len(failures)) + ' failure(s)'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
