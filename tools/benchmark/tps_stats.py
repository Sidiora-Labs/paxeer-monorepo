#!/usr/bin/env python3

import argparse
import math
import re
import statistics
from pathlib import Path
from typing import TypedDict


class Summary(TypedDict):
    count: int
    median: float
    average: float
    min: float
    max: float


ANSI_SGR = re.compile(r"\x1b\[[0-9;]*m")
TPS_READING = re.compile(r".*tps=([0-9.]*)")


def summarise(path: str | Path) -> Summary | None:
    readings = []
    with Path(path).open(encoding="utf-8") as log:
        for line in log:
            match = TPS_READING.match(ANSI_SGR.sub("", line))
            if match:
                readings.append(float(match.group(1) or "0"))

    # The shell pipeline sorts before applying awk's NR > 3 filter.
    retained = [value for value in sorted(readings)[3:] if value > 1000]
    if not retained:
        return None
    return {
        "count": len(retained),
        "median": statistics.median(retained),
        "average": statistics.mean(retained),
        "min": retained[0],
        "max": retained[-1],
    }


def print_summary(path: str | Path, summary: Summary | None) -> None:
    if summary is None:
        print(f"{path}: NO DATA")
        return
    print(
        f"{path}: Readings: {summary['count']} | Median: {summary['median']:.0f}"
        f" | Avg: {summary['average']:.0f} | Min: {summary['min']:.0f}"
        f" | Max: {summary['max']:.0f}"
    )


def compare(
    baseline: str | Path, candidate: str | Path, max_regression_pct: float
) -> int:
    baseline_summary = summarise(baseline)
    candidate_summary = summarise(candidate)
    print_summary(baseline, baseline_summary)
    print_summary(candidate, candidate_summary)
    if baseline_summary is None or candidate_summary is None:
        return 2

    baseline_median = baseline_summary["median"]
    candidate_median = candidate_summary["median"]
    regression_pct = 100 * (baseline_median - candidate_median) / baseline_median
    print(f"Median regression: {regression_pct:.1f} percent")
    return int(regression_pct > max_regression_pct)


def regression_limit(value: str) -> float:
    limit = float(value)
    if not math.isfinite(limit) or limit < 0:
        raise argparse.ArgumentTypeError("must be a finite, non-negative percentage")
    return limit


def main() -> int:
    parser = argparse.ArgumentParser(description="Summarise and compare benchmark TPS logs")
    commands = parser.add_subparsers(dest="command", required=True)
    summary_parser = commands.add_parser("summarise")
    summary_parser.add_argument("path", type=Path)
    compare_parser = commands.add_parser("compare")
    compare_parser.add_argument("--baseline", type=Path, required=True)
    compare_parser.add_argument("--candidate", type=Path, required=True)
    compare_parser.add_argument("--max-regression-pct", type=regression_limit, default=5.0)
    args = parser.parse_args()
    try:
        if args.command == "compare":
            return compare(args.baseline, args.candidate, args.max_regression_pct)
        summary = summarise(args.path)
        print_summary(args.path, summary)
        return 2 if summary is None else 0
    except (OSError, ValueError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    raise SystemExit(main())
