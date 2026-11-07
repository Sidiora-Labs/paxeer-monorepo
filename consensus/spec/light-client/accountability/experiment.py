"""Validate the full accountability benchmark and the harness's result CSV."""

import argparse
import csv
from pathlib import Path


FIELDS = ("no", "filename", "tool", "timeout", "init", "inv", "next", "args")
MODELS = ("MC_n4_f1.tla", "MC_n4_f2.tla", "MC_n5_f1.tla", "MC_n5_f2.tla")


def validate_benchmarks(path):
    with Path(path).open(encoding="utf-8", newline="") as stream:
        reader = csv.DictReader(stream)
        if reader.fieldnames != list(FIELDS):
            raise ValueError("Unexpected benchmark columns")
        rows = list(reader)
    expected = []
    for phase in range(3):
        for index, model in enumerate(MODELS):
            expected.append(
                dict(
                    zip(
                        FIELDS,
                        (
                            str(len(expected) + 1),
                            model,
                            "apalache",
                            "10h" if phase == 0 else "20h",
                            "Init" if phase == 1 else "TypedInv",
                            ("Agreement" if index % 2 == 0 else "Accountability")
                            if phase == 2
                            else "TypedInv",
                            "",
                            "--length=1 --cinit=ConstInit"
                            if phase == 0
                            else "--length=0 --cinit=ConstInit",
                        ),
                    )
                )
            )
    if rows != expected:
        raise ValueError("The complete twelve-case benchmark must be preserved")
    return rows


def validate_results(benchmark_path, results_path):
    benchmarks = validate_benchmarks(benchmark_path)
    with Path(results_path).open(encoding="utf-8", newline="") as stream:
        reader = csv.DictReader(stream, quotechar="'")
        required = {"01:no", "02:tool", "03:status"}
        if not required.issubset(reader.fieldnames or []):
            raise ValueError("Missing result identity, tool or status columns")
        results = list(reader)
    expected = {row["no"] for row in benchmarks}
    actual = [row["01:no"] for row in results]
    if len(actual) != len(expected) or set(actual) != expected:
        raise ValueError("Results must cover every benchmark exactly once")
    if any(
        row["02:tool"] != "apalache" or row["03:status"] != "NoError"
        for row in results
    ):
        raise ValueError("Every Apalache experiment must report NoError")
    return results


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    benchmarks = commands.add_parser("validate-benchmarks")
    benchmarks.add_argument("benchmark", type=Path)
    results = commands.add_parser("validate-results")
    results.add_argument("benchmark", type=Path)
    results.add_argument("results", type=Path)
    args = parser.parse_args(argv)
    if args.command == "validate-benchmarks":
        validate_benchmarks(args.benchmark)
    else:
        validate_results(args.benchmark, args.results)


if __name__ == "__main__":
    main()
